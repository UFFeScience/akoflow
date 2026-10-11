#!/usr/bin/env ruby
require 'json'
require 'digest'
require 'fileutils'

abort 'Usage: prepare_montage_measured_requests.rb BASELINE_REQUEST PROJECTION COMPUTE_MODEL MATERIALIZATION_MODEL GCP_CATALOG HPC_CATALOG SCOPE NETWORK OUTPUT_DIR' unless ARGV.length == 9
baseline, projection, compute, materialization, gcp, hpc, scope, network = ARGV[0,8].map { |path| JSON.parse(File.binread(path)) }
out = ARGV[8]
FileUtils.mkdir_p(out)
save = ->(name, value) { File.write(File.join(out, name + '.json'), JSON.pretty_generate(value) + "\n") }
workflow_name = ENV.fetch('MONTAGE_WORKFLOW_NAME', 'montage-reference-work-v1')
revision = ENV.fetch('MONTAGE_MODEL_REVISION', 'v1')
workflow = baseline.fetch('workflow')
raise 'Expected complete Montage58' unless workflow.fetch('activities').length == 58
names = workflow.fetch('activities').to_h { |activity| [activity.fetch('id'), activity.fetch('name')] }
dependencies = workflow.fetch('dependencies').group_by { |dependency| dependency.fetch('activityId') }
activities = workflow.fetch('activities').map do |activity|
  id = activity.fetch('id')
  family = id.match(/-([a-z]+)id\d+$/)&.captures&.first
  work = projection['activityComputeWork'] && projection.fetch('activityComputeWork').fetch(id)
  family = work.fetch('activityFamily') if work
  raise "Unclassified family #{id}" unless family && compute.fetch('activityFamilyTypeIds').fetch(family) == "#{workflow_name}-#{family.tr('_','-')}"
  command = Marshal.load(Marshal.dump(activity.fetch('command')))
  command.delete('resolvedExecutable')
  metadata = activity.fetch('metadata', {}).merge('workspaceProjection'=>projection.fetch('activityModels').fetch(id), 'calibrationSource'=>compute.fetch('source'))
  metadata.delete('workspaceProjectionVolumes')
  metadata = metadata.merge('computeWorkUnits'=>work.fetch('computeWorkUnits'),'computeWorkUnit'=>work.fetch('unit'),'computeWorkSource'=>work.fetch('sourceImageSHA256')) if work
  { 'name'=>activity.fetch('name'), 'activityType'=>family, 'metadata'=>metadata,
    'command'=>command, 'simulation'=>activity.fetch('simulation'),
    'cpuLimit'=>activity.fetch('resources').fetch('cpu').to_s,
    'memoryLimit'=>activity.fetch('resources').fetch('memoryBytes').to_s,
    'dependsOn'=>dependencies.fetch(id, []).map { |dependency| names.fetch(dependency.fetch('dependsOnActivityId')) },
    'workspaceSeedPath'=>metadata.fetch('workspaceSeedPath', ''), 'expectedOutputs'=>metadata['expectedOutputs'],
    'runtime'=>metadata.fetch('runtime', ''), 'resourceSelector'=>metadata.fetch('resourceSelector', '') }
end
save.call('workflow-import', { 'name'=>workflow_name, 'spec'=>{'namespace'=>'montage-measured-20261011','activities'=>activities} })
relay = {'id'=>materialization.fetch('relayResourceId'), 'environmentVersionId'=>gcp.fetch('version').fetch('id'),
  'name'=>'Existing Akoflow relay controller (model-only, not schedulable)', 'type'=>'local_machine',
  'cpuCores'=>8, 'cpuCapacity'=>8, 'memoryBytes'=>8*1024**3, 'computeSpeedup'=>1,
  'schedulable'=>false, 'metadata'=>{'modelOnly'=>true,'role'=>'transfer-relay','provision'=>false}}
save.call('relay-resource', relay)
network['id'] = "montage-reference-network-20261011-#{revision}"
network['name'] = 'Measured workspace materialization with explicit controller relay'
network['metadata'] = (network['metadata'] || {}).merge('workspaceMaterializationModel'=>materialization,
  'calibrationSource'=>materialization.fetch('source'), 'linkBandwidthBasis'=>'Inherited topology assumptions; phase flow caps use measured historical effective rates')
network.fetch('links').each do |link|
  link['metadata'] = (link['metadata'] || {}).merge('sourceTopologyLinkId'=>link.fetch('id'))
  link['id'] = "#{network.fetch('id')}-#{link.fetch('id')}"
  link['topologyId'] = network.fetch('id')
end
materialization.fetch('resources').each do |id, resource|
  next if id == relay.fetch('id')
  # Keep the original physical link assumptions rather than claiming a new
  # NIC benchmark. Phase rates are separately frozen and provenance-tagged.
  bandwidth = resource.fetch('domain') == 'cloud' ? 200_000_000 : 200_000_000
  network.fetch('links') << { 'id'=>"#{network.fetch('id')}-relay-#{id}", 'topologyId'=>network.fetch('id'),
    'sourceResourceId'=>relay.fetch('id'), 'targetResourceId'=>id,
    'bandwidthBitsPerSecond'=>bandwidth, 'latencySeconds'=>0.05, 'pricePerByte'=>0,
    'bidirectional'=>true, 'sharingPolicy'=>'SHARED', 'metadata'=>{'measurementStatus'=>'assumed-inherited-200Mbps','networkCostCalibrated'=>false} }
end
scope['id'] = "montage-reference-scope-20261011-#{revision}"
scope['networkTopologyId'] = network.fetch('id')
network['executionScopeId'] = scope.fetch('id')
save.call('scope', scope)
save.call('network', network)
save.call('planning-session', {'id'=>ENV.fetch('MONTAGE_PLANNING_ID', 'montage-reference-planning-20261011-v2'),
  'workflowVersionId'=>workflow_name+'-v1', 'executionScopeId'=>scope.fetch('id'),
  'networkTopologyId'=>network.fetch('id'), 'algorithms'=>[{'id'=>'prism-time','configuration'=>{'requireMeasuredProfiles'=>true}}],
  'configuration'=>{'calibrationSource'=>compute.fetch('source'),
    'activityProfileSnapshot'=>compute.fetch('activityProfiles'), 'requireMeasuredProfiles'=>true,
    'computeCalibrationUnits'=>projection['activityComputeWork'] ? 'compute-work-v1' : 'reference-work-v1', 'computeCalibrationIncludesPhysicalSharing'=>true,
    'additionalPairwiseSlowdown'=>'not-applied-to-contention-conditioned-wall-profiles'}})
puts 'Prepared 58 activities, class-level compute profiles and explicit non-schedulable relay. No run submitted.'
