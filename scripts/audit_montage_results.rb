#!/usr/bin/env ruby
require 'json'
require 'time'
require 'digest'

# Explicit allowlist: never copy runtime metadata, addresses, credentials or logs.
def montage_audit(request, response, simulation)
  expected = request.fetch('workflow').fetch('activities').map { |a| a.fetch('id') }
  raise 'Run is not completed' unless response.fetch('run').fetch('status') == 'completed'
  raise 'Duplicate expected activity IDs' unless expected.uniq.length == expected.length
  assignments = request.fetch('plan').fetch('assignments').to_h { |a| [a.fetch('activityId'), a] }
  latest = response.fetch('activityAttempts').group_by { |a| a.fetch('activityId') }.transform_values do |attempts|
    attempts.max_by { |a| [a.fetch('attempt'), a['startedAt'].to_f] }
  end
  transfers = response.fetch('artifactTransferRuns')
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
    unreleased_lease_count: response.fetch('workspaceLeases').count { |l| !l['released'] }
  }
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
    real_api_run_timestamp_wall_seconds: wall_seconds,
    real_network_bytes_from_routes: transfers.sum { |t| t['networkBytes'].to_i },
    transfer_route_count: transfers.length,
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
      'Cost zero is uncalibrated, not free cloud execution.',
      'e2-medium interference extrapolated to HPC/micro; micro speedup 0.25 is approximate.',
      'Whole-workspace real traffic differs from estimated plan edge volumes.',
      'Zero reported interference does not prove absence of physical contention.'
    ]
  }
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: audit_montage_results.rb request.json real.json sim.json product.png' unless ARGV.length == 4
  request, real, sim = ARGV.take(3).map { |path| JSON.parse(File.read(path)) }
  report = montage_audit(request, real, sim)
  report[:product] = {filename: File.basename(ARGV[3]), bytes: File.size(ARGV[3]), sha256: Digest::SHA256.file(ARGV[3]).hexdigest}
  puts JSON.pretty_generate(report)
end
