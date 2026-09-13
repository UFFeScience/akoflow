#!/usr/bin/env ruby
# Build a real-execution bundle from the original 58-activity Montage DAG.
require 'csv'
require 'json'
require 'shellwords'
require 'yaml'

ROOT = __dir__
IMAGE = 'ovvesley/akoflow-wf-montage:050d'
NAME = 'montage-58-cloud-local'
source = YAML.load_file(File.join(ROOT, 'source-workflow.yaml'))
original = source.fetch('spec').fetch('activities')
runtimes = CSV.read(File.join(ROOT, 'reference-runtimes.csv'), headers: true)
seconds = runtimes.to_h { |row| [row.fetch('activity_id'), row.fetch('duration_seconds').to_f] }
abort 'expected 58 distinct activities with reference runtimes' unless original.length == 58 && original.map { |a| a.fetch('name') }.uniq.length == 58 && (original.map { |a| a['name'] } - seconds.keys).empty?

def output_files(command)
  tokens = Shellwords.split(command)
  case tokens.first
  when 'mProject'
    projected = tokens.fetch(3)
    [projected, projected.sub(/\.fits\z/, '_area.fits')]
  when 'mDiffFit' then [tokens.fetch(3)]
  when 'mConcatFit' then [tokens.fetch(2)]
  when 'mBgModel' then [tokens.last]
  when 'mBackground'
    corrected = tokens.fetch(3)
    [corrected, corrected.sub(/\.fits\z/, '_area.fits')]
  when 'mImgtbl', 'mAdd', 'mViewer' then [tokens.last]
  else abort "unknown Montage command: #{command}"
  end
end

by_name = original.to_h { |a| [a.fetch('name'), a] }
outputs = by_name.transform_values { |a| output_files(a.fetch('run')) }
dependencies = []
original.each do |activity|
  activity.fetch('dependsOn', []).each do |parent|
    abort "unknown predecessor #{parent}" unless outputs.key?(parent)
    produced = outputs.fetch(parent)
    # mConcatFit and mImgtbl discover their inputs through table/directory
    # arguments, so their consumed filenames are not explicit in the command.
    used = produced.select { |file| activity.fetch('run').include?(file) }
    used = produced.select { |file| file.end_with?('.txt') } if activity.fetch('run').start_with?('mConcatFit')
    used = produced.select { |file| file.end_with?('.fits') } if activity.fetch('run').start_with?('mImgtbl', 'mAdd') && used.empty?
    abort "unmapped data edge #{parent} -> #{activity['name']}" if used.empty?
    used.each do |file|
      dependencies << { 'producerActivity' => parent, 'consumerActivity' => activity['name'],
                        'logicalName' => file, 'sizeBytes' => file.end_with?('.fits') ? 10_000_000 : 4096 }
    end
  end
end

activities = original.map do |activity|
  command = activity.fetch('run')
  produced = outputs.fetch(activity.fetch('name'))
  if command.start_with?('mProject')
    # SSH cloud adapter mounts an empty per-activity directory. Input FITS and
    # headers live in the image; copy only produced files to the mounted workdir.
    script = "set -eu; work=$PWD; cd /akoflow-wfa-shared; #{command}; cp #{produced.map { |f| Shellwords.escape(f) }.join(' ')} \"$work/\""
    runtime = 'cloud'
  else
    # The local adapter launches host commands (not image commands). Docker's
    # daemon may run outside the AkôFlow container, so use docker cp, not -v.
    inputs = dependencies.select { |d| d['consumerActivity'] == activity['name'] }.map { |d| d['logicalName'] }.uniq
    script = <<~SH
      set -eu
      cid=$(docker create --workdir /akoflow-wfa-shared #{IMAGE} sh -c #{Shellwords.escape("set -eu; #{command}")})
      trap 'docker rm -f "$cid" >/dev/null 2>&1 || true' EXIT
      #{inputs.map { |f| "docker cp #{Shellwords.escape(f)} \"$cid:/akoflow-wfa-shared/#{f}\"" }.join("\n")}
      docker start -a "$cid"
      #{produced.map { |f| "docker cp \"$cid:/akoflow-wfa-shared/#{f}\" #{Shellwords.escape(f)}" }.join("\n")}
    SH
    runtime = 'local'
  end
  { 'name' => activity['name'], 'runtime' => runtime, 'run' => script,
    'cpuLimit' => '1', 'memoryLimit' => '256Mi', 'dependsOn' => activity.fetch('dependsOn', []) }
end

workflow = { 'name' => NAME, 'spec' => { 'namespace' => 'showcase', 'image' => IMAGE,
  'activities' => activities, 'dataDependencies' => dependencies } }
File.write(File.join(ROOT, 'workflow.json'), JSON.pretty_generate(workflow) + "\n")

cloud_order = 0
local_order = 0
assignments = original.map do |a|
  cloud = a.fetch('run').start_with?('mProject')
  order = cloud ? cloud_order : local_order
  cloud ? cloud_order += 1 : local_order += 1
  { 'id' => "#{NAME}-assignment-#{a['name']}", 'activityId' => "#{NAME}-#{a['name']}",
    'resourceId' => cloud ? 'goal-gcp-e2-medium' : 'local-environment-entrypoint',
    'orderOnResource' => order,
    'metadata' => { 'runtimeId' => cloud ? 'goal-gcp-cloud' : 'local-environment-local' } }
end
reference_cloud_cost = original.select { |a| a.fetch('run').start_with?('mProject') }.sum { |a| seconds.fetch(a.fetch('name')) } * 0.000015365916666666668
plan = { 'plan' => { 'id' => "#{NAME}-manual-v1", 'workflowVersionId' => "#{NAME}-v1",
  'executionScopeId' => "#{NAME}-scope-v1", 'networkTopologyId' => "#{NAME}-network-v1",
  'source' => 'imported', 'algorithm' => 'manual', 'algorithmVersion' => '1',
  'objective' => 'custom',
  # Upper-bound-like sum of reference activity runtimes, not a calibrated
  # makespan. Cloud compute price excludes startup, disk and egress charges.
  'predicted' => { 'makespanSeconds' => seconds.values.sum, 'cost' => reference_cloud_cost.round(6), 'feasible' => true },
  'assignments' => assignments } }
File.write(File.join(ROOT, 'plan.json'), JSON.pretty_generate(plan) + "\n")
puts "#{activities.length} real activities (#{cloud_order} GCP, #{local_order} local), #{dependencies.length} named data dependencies"
