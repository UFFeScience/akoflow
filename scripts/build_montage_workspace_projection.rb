#!/usr/bin/env ruby
require 'json'
require 'digest'
require_relative 'build_montage_execution_calibration'

module MontageWorkspaceProjection
  module_function

  def files(snapshot)
    (snapshot || []).each_with_object({}) do |file, result|
      name = file.fetch('path')
      raise 'Unsafe scientific manifest path' if name.start_with?('/') || name.split('/').include?('..') || name.include?("\0") || name.include?('\\')
      raise 'Missing content identity' unless file.fetch('checksum').match?(/\Asha256:[0-9a-f]{64}\z/)
      raise 'Negative manifest size' if file.fetch('sizeBytes') < 0
      raise 'Duplicate manifest path' if result.key?(name)
      result[name] = {'path'=>name,'sizeBytes'=>file.fetch('sizeBytes'),'contentId'=>file.fetch('checksum')}
    end
  end

  def activity_model(inherited, final, packaging, source)
    {'contract'=>'whole-workspace-v1','source'=>source,
      'inputs'=>packaging.values,
      'inputsByResourceType'=>{'cloud_vm'=>packaging.values,'hpc_machine'=>[]},
      'outputs'=>final.values.select { |file| inherited[file.fetch('path')] != file && packaging[file.fetch('path')] != file },
      'removedPaths'=>inherited.keys.reject { |path| final.key?(path) }}
  end

  def build(audited, raw, request, digest)
    MontageExecutionCalibration.build(audited, digest) # Require the full audit.
    raise 'Baseline identity mismatch' unless raw.fetch('run').fetch('id') == audited.fetch('run_id')
    raise 'Baseline is not completed' unless raw.fetch('run').fetch('status') == 'completed'
    workflow = request.fetch('workflow')
    ids = workflow.fetch('activities').map { |activity| activity.fetch('id') }
    handles = raw.fetch('handles')
    raise 'Baseline handle mismatch' unless handles.length == ids.length && handles.map { |h| h.fetch('activityId') }.sort == ids.sort
    manifests = {}
    handles.each do |handle|
      raise 'Baseline handle unsuccessful' unless handle.fetch('status') == 'completed' && handle.fetch('exitCode') == 0
      manifests[handle.fetch('activityId')] = files(handle.fetch('artifacts').fetch('finalSnapshot'))
    end
    preds = Hash.new { |hash, key| hash[key] = [] }
    workflow.fetch('dependencies').each { |edge| preds[edge.fetch('activityId')] << edge.fetch('dependsOnActivityId') }
    resources = request.fetch('resources').each_with_object({}) { |resource, map| map[resource.fetch('id')] = resource.fetch('type') }
    assignments = request.fetch('plan').fetch('assignments').each_with_object({}) { |assignment,map| map[assignment.fetch('activityId')] = resources.fetch(assignment.fetch('resourceId')) }
    # Root activities expose the packaging difference between OCI cloud and
    # HPC runtime. Common files observed in every OCI root are runtime assets;
    # per-activity scientific outputs are the remaining files. This derivation
    # is tied to this immutable image/input dataset, not a generic heuristic.
    roots = ids.select { |id| preds[id].empty? }
    cloud_roots = roots.select { |id| assignments.fetch(id) == 'cloud_vm' }
    raise 'Need multiple OCI root observations' unless cloud_roots.length >= 2
    common = manifests.fetch(cloud_roots.first).dup
    cloud_roots.drop(1).each { |id| common.delete_if { |path,file| manifests.fetch(id)[path] != file } }
    raise 'Packaging evidence missing' if common.empty?
    hpc_roots = roots.select { |id| assignments.fetch(id) == 'hpc_machine' }
    raise 'HPC packaging evidence missing' if hpc_roots.empty?
    hpc_roots.each { |id| raise 'OCI packaging unexpectedly present on HPC root' if common.keys.any? { |path| manifests.fetch(id).key?(path) } }
    input_volumes = {}
    models = ids.each_with_object({}) do |id, result|
      inherited = {}
      preds[id].each do |parent|
        manifests.fetch(parent).each do |path,file|
          raise 'Baseline parent content conflict' if inherited.key?(path) && inherited[path] != file
          inherited[path] = file
        end
      end
      final = manifests.fetch(id)
      input_volumes[id] = {'inputTotalBytes'=>preds[id].sum { |parent| manifests.fetch(parent).values.sum { |file| file.fetch('sizeBytes') } },
        'inputUniqueBytes'=>inherited.values.sum { |file| file.fetch('sizeBytes') },'inputFiles'=>inherited.length}
      model = activity_model(inherited, final, common, "baseline:#{audited.fetch('run_id')}")
      # The remote Docker adapter seeds every activity, not just roots. Exact
      # immutable packaging identities are inputs; changed contents at the same
      # paths remain scientific outputs. Inherited files remain in the workspace
      # even when the consumer is moved to HPC.
      result[id] = model
    end
    {'schemaVersion'=>1,'baselineRunId'=>audited.fetch('run_id'),'auditedEvidenceSHA256'=>digest,
      'contract'=>'whole-workspace-v1','runtimePackagingFiles'=>common.length,
      'activityModels'=>models,
      'dependencies'=>workflow.fetch('dependencies').map { |edge| {'activityId'=>edge.fetch('activityId'),'dependsOnActivityId'=>edge.fetch('dependsOnActivityId')} },
      'observedProducerWorkspaceBytes'=>manifests.transform_values { |manifest| manifest.values.sum { |file| file.fetch('sizeBytes') } },
      'observedConsumerInputVolumes'=>input_volumes,
      'baselineAssignedResourceTypes'=>assignments,
      'activityReferenceDurations'=>workflow.fetch('activities').each_with_object({}) { |activity, map| map[activity.fetch('id')] = activity.fetch('simulation').fetch('durationSeconds') },
      'limitations'=>['File sizes and identities are previous-run observations for the same immutable image and dataset, not future observations.',
        'Runtime packaging variant is supported only for evidenced cloud_vm and hpc_machine types.',
        'No transfer deduplication, snapshot reuse or shared storage is inferred by this manifest model.']}
  end
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: build_montage_workspace_projection.rb AUDITED_JSON PRIVATE_API_JSON PRIVATE_REQUEST_JSON OUTPUT_JSON' unless ARGV.length == 4
  audited = File.binread(ARGV[0])
  result = MontageWorkspaceProjection.build(JSON.parse(audited), JSON.parse(File.binread(ARGV[1])), JSON.parse(File.binread(ARGV[2])), Digest::SHA256.hexdigest(audited))
  File.write(ARGV[3], JSON.pretty_generate(result) + "\n")
  puts "Workspace projection: #{result['activityModels'].length} activities, #{result['runtimePackagingFiles']} runtime packaging files."
end
