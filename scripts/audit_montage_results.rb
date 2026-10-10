#!/usr/bin/env ruby
require 'json'
require 'time'
require 'digest'

# Explicit allowlist: never copy runtime metadata, addresses, credentials or logs.
def montage_audit(request, response, simulation, require_fresh: false)
  expected = request.fetch('workflow').fetch('activities').map { |a| a.fetch('id') }
  raise 'Run is not completed' unless response.fetch('run').fetch('status') == 'completed'
  raise 'Duplicate expected activity IDs' unless expected.uniq.length == expected.length
  assignments = request.fetch('plan').fetch('assignments').to_h { |a| [a.fetch('activityId'), a] }
  latest = response.fetch('activityAttempts').group_by { |a| a.fetch('activityId') }.transform_values do |attempts|
    attempts.max_by { |a| [a.fetch('attempt'), a['startedAt'].to_f] }
  end
  if require_fresh
    attempts = response.fetch('activityAttempts')
    raise 'Fresh experiment contains retries or failed attempts' unless attempts.length == 58 && attempts.all? { |task| task['attempt'] == 1 && task['status'] == 'completed' }
    raise 'Unexpected runtime handles in fresh experiment' unless response.fetch('handles').length == 58
    expected.each do |id|
      # Domain HandleID binds the first attempt as run:activity, with an
      # :attempt-N suffix for later attempts. Some adapters omit metadata.
      handles = response.fetch('handles').select do |handle|
        attempt = handle.fetch('metadata', {})['executionAttempt']
        handle['id'] == "#{response.fetch('run').fetch('id')}:#{id}" && handle['runId'] == response.fetch('run').fetch('id') && handle['activityId'] == id && (attempt.nil? || attempt == 1)
      end
      raise "Missing successful exact-attempt handle: #{id}" unless handles.length == 1 && handles.first['status'] == 'completed' && handles.first['exitCode'] == 0
    end
    billing = response.fetch('cloudBilling')
    raise 'Cloud lifecycle prices are incomplete' unless billing['status'] == 'estimated' && billing['currency'] == 'USD' && billing['estimatedCost'].is_a?(Numeric) && billing['estimatedCost'] > 0
  end
  transfers = response.fetch('artifactTransferRuns')
  dependency_routes = transfers.select { |t| t.fetch('route', {})['producerActivityId'] }
  dependencies = request.fetch('workflow').fetch('dependencies', [])
  missing_routes = dependencies.reject do |edge|
    dependency_routes.any? do |t|
      route = t.fetch('route')
      route['producerActivityId'] == edge['dependsOnActivityId'] && route['consumerActivityId'] == edge['activityId']
    end
  end
  resource_types = request.fetch('resources', []).to_h { |r| [r.fetch('id'), r.fetch('type', '')] }
  cross_routes = dependency_routes.select do |t|
    route = t.fetch('route')
    source = assignments.fetch(route.fetch('producerActivityId')).fetch('resourceId')
    target = assignments.fetch(route.fetch('consumerActivityId')).fetch('resourceId')
    types = [resource_types[source], resource_types[target]]
    types.any? { |type| type == 'cloud_vm' } && types.any? { |type| %w[hpc_machine hpc_partition].include?(type) }
  end
  single_lane_micros = request.fetch('resources', []).select do |r|
    r.fetch('id').include?('micro') && r['cpuCapacity'].to_f == 1
  end.map { |r| r.fetch('id') }
  micro_overlap_count = 0
  single_lane_micros.each do |resource|
    intervals = latest.values.select { |task| assignments[task['activityId']]&.fetch('resourceId') == resource }
    intervals.each do |task|
      raise 'Missing or invalid micro execution timestamps' unless task['startedAt'] && task['finishedAt'] && task['finishedAt'].to_f >= task['startedAt'].to_f
    end
    intervals.sort_by { |task| task['startedAt'].to_f }.each_cons(2) do |left, right|
      micro_overlap_count += 1 if left['finishedAt'].to_f > right['startedAt'].to_f
    end
  end
  barriers = transfers.select do |t|
    task = latest[t['activityId']]
    task && t['finishedAt'].to_f > task['startedAt'].to_f
  end
  audit = {
    expected_count: expected.length,
    missing: expected - latest.keys,
    unexpected: latest.keys - expected,
    not_completed: expected.reject { |id| latest[id] && latest[id]['status'] == 'completed' },
    incomplete_transfer_count: transfers.count { |t| t['status'] != 'completed' },
    barrier_violation_count: barriers.length,
    unreleased_lease_count: response.fetch('workspaceLeases').count { |l| !l['released'] },
    single_lane_micro_overlap_count: micro_overlap_count
  }
  audit[:missing_dependency_route_count] = missing_routes.length
  audit[:cross_cloud_hpc_non_relay_count] = cross_routes.count { |t| t['strategy'] != 'gateway' }
  raise 'Activity/transfer audit failed' unless audit[:expected_count] == 58 && audit.values.drop(1).all? { |v| v == 0 || v == [] }
  metric_keys = %w[makespanSeconds cost transferredBytes breakdown]
  metrics = ->(run) { run.select { |k, _| metric_keys.include?(k) } }
  run = response.fetch('run')
  multiple_attempts = response.fetch('activityAttempts').group_by { |a| a.fetch('activityId') }.any? do |_, attempts|
    attempts.length > 1 || attempts.any? { |a| a.fetch('attempt') > 1 }
  end
  wall_seconds = if run['startedAt'] && run['finishedAt']
    Time.parse(run['finishedAt']) - Time.parse(run['startedAt'])
  end
  {
    schema_version: 1,
    run_id: response.fetch('run').fetch('id'),
    attempt_history: multiple_attempts ? 'multiple_attempts' : 'single_observed_attempt_per_activity',
    audit: audit,
    plan: metrics.call(request.fetch('plan').fetch('predicted')),
    plan_assignment_compute_work_seconds: assignments.values.sum { |a| a['predictedRuntimeSeconds'].to_f },
    plan_assignment_transfer_work_seconds: assignments.values.sum { |a| a['predictedTransferSeconds'].to_f },
    simulation: metrics.call(simulation.fetch('run')),
    real_api_reported: metrics.call(response.fetch('run')),
    real_cloud_lifecycle_estimate: response.fetch('cloudBilling', {}).select { |key, _| %w[status currency estimatedCost knownCost costBasis excludedCosts asOf].include?(key) },
    real_cloud_generations: response.fetch('cloudBilling', {}).fetch('instances', []).map do |instance|
      {instance_id: instance['instanceId'], status: instance['status'], billing: instance.fetch('billing', {}).select { |key, _| %w[pricingStatus priceCurrency priceSource priceObservedAt priceRegion priceMachineType startedAt finishedAt computeStartedAt accumulatedComputeSeconds computePricePerSecond diskPricePerSecond asOf currentCost missingPrices].include?(key) }}
    end,
    real_api_run_timestamp_wall_seconds: wall_seconds,
    real_network_bytes_from_routes: transfers.sum { |t| t['networkBytes'].to_i },
    transfer_route_count: transfers.length,
    dependency_route_count: dependency_routes.length,
    cross_cloud_hpc_relay_count: cross_routes.length,
    relay_server_network_bytes: transfers.sum { |t| t.fetch('route', {})['serverNetworkBytes'].to_i },
    routes: transfers.map do |t|
      route = t.fetch('route', {})
      {
        consumer_activity_id: t['activityId'], producer_activity_id: route['producerActivityId'],
        strategy: t['strategy'], duration_seconds: t['durationSeconds'], logical_bytes: t['logicalBytes'],
        network_bytes: t['networkBytes'], relay_server_network_bytes: route['serverNetworkBytes'],
        archive_bytes: route['archiveBytes'], files: route['filesTransferred'], snapshot_reused: route['snapshotReused'],
        ssh_channel_wait_seconds: route['sshChannelWaitSeconds'], rsync_execution_seconds: route['rsyncExecutionSeconds'],
        phases: route.fetch('phases', []).map { |p| p.select { |k, _| %w[name durationSeconds].include?(k) } }
      }
    end,
    # Joint phases are repeated on each predecessor route. Group by exact consumer
    # and preparation interval, not by route, to avoid multiplying merge work.
    shared_preparations: dependency_routes.group_by do |t|
      r = t.fetch('route')
      phase = r.fetch('phases', []).find { |p| p['name'] == 'shared-preparation-barrier' } || {}
      [r['consumerActivityId'], phase['startedAt'], phase['finishedAt']]
    end.map do |key, group|
      next unless key[1] && key[2]
      r = group.first.fetch('route')
      phase = r.fetch('phases').find { |p| p['name'] == 'shared-preparation-barrier' }
      merge = r.fetch('phases').find { |p| p['name'] == 'shared-conflict-check-copy-and-verify' }
      {consumer_activity_id: key[0], barrier_seconds: phase['durationSeconds'],
       merge_seconds: merge && merge['durationSeconds'],
       merge_check_seconds: r['mergeCheckSeconds'], merge_copy_seconds: r['mergeCopySeconds'],
       merge_verify_seconds: r['mergeVerifySeconds']}
    end.compact,
    activities: expected.sort.map do |id|
      task = latest.fetch(id)
      plan = assignments.fetch(id)
      metadata = task.fetch('metadata', {})
      handle = response.fetch('handles', []).select do |h|
        h['activityId'] == id && h.fetch('metadata', {}).to_h['executionAttempt'] == task.fetch('attempt')
      end.max_by { |h| h['startedAt'].to_f }
      observed = handle && handle.fetch('metadata', {}).to_h['terminalObservedAt']
      native_seconds = if observed && handle['finishedAt'] && handle['startedAt'] && observed.to_f >= handle['finishedAt'].to_f && handle['finishedAt'].to_f >= handle['startedAt'].to_f
        handle['finishedAt'].to_f - handle['startedAt'].to_f
      end
      {
        activity_id: id, resource_id: plan.fetch('resourceId'), attempt: task.fetch('attempt'),
        planned_compute_seconds: plan['predictedRuntimeSeconds'],
        planned_transfer_seconds: plan['predictedTransferSeconds'],
        observed_runtime_seconds: task['runtimeSeconds'],
        validated_handle_runtime_seconds: native_seconds,
        terminal_observation_delay_seconds: native_seconds && observed.to_f - handle['finishedAt'].to_f,
        preparation_elapsed_seconds: metadata['transferElapsedSeconds'],
        overlapping_transfer_work_seconds: metadata['transferWorkSeconds'],
        launch_wait_seconds: metadata['launchWaitSeconds'],
        queue_seconds: task['queueSeconds'], network_bytes: task['transferBytes']
      }
    end,
    limitations: [
      multiple_attempts ? 'Multiple attempts: performance can be contaminated by recovery.' : 'No multiple attempts observed; this alone does not prove an uninterrupted benchmark.',
      'Runtime finish semantics depend on the driver; observation delay is exported only for exact-attempt handles with validated terminal timestamps.',
      'API aggregate counters can contain older attempts; audit uses latest distinct IDs.',
      'Transfer work is summed overlapping route time, not wall-clock preparation.',
      'Plan assignment work sums are not makespan; API timestamp wall and recovered makespan can differ.',
      response.fetch('cloudBilling', {})['status'] == 'estimated' ? 'Cloud lifecycle cost is a list-rate estimate with explicit exclusions, not the provider invoice. The task/window API cost and plan retention-window cost have different bases.' : 'Cloud prices are incomplete; cost zero does not mean free cloud execution.',
      'e2-medium interference extrapolated to HPC/micro; micro speedup 0.25 is approximate.',
      'Whole-workspace real traffic differs from estimated plan edge volumes.',
      'Zero reported interference does not prove absence of physical contention.'
    ]
  }
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: audit_montage_results.rb request.json real.json sim.json product.png' unless ARGV.length == 4
  request, real, sim = ARGV.take(3).map { |path| JSON.parse(File.read(path)) }
  report = montage_audit(request, real, sim, require_fresh: ENV['MONTAGE_REQUIRE_FRESH'] == '1')
  report[:product] = {filename: File.basename(ARGV[3]), bytes: File.size(ARGV[3]), sha256: Digest::SHA256.file(ARGV[3]).hexdigest}
  puts JSON.pretty_generate(report)
end
