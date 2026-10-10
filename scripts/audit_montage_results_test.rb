require 'minitest/autorun'
require_relative 'audit_montage_results'

class MontageAuditTest < Minitest::Test
  def test_strict_fresh_audit_requires_exact_attempt_success_and_prices
    args = fixture
    args[1]['activityAttempts'].each { |task| task['attempt'] = 1 }
    args[1]['handles'] = args[1]['activityAttempts'].map { |task| {'id'=>"run:#{task['activityId']}", 'runId'=>'run', 'activityId'=>task['activityId'], 'status'=>'completed', 'exitCode'=>0, 'metadata'=>{'executionAttempt'=>1}} }
    args[1]['cloudBilling'] = {'status'=>'estimated', 'currency'=>'USD', 'estimatedCost'=>0.2, 'instances'=>[{'instanceId'=>'vm-1', 'billing'=>{'priceCurrency'=>'USD', 'currentCost'=>0.2, 'privateKey'=>'never-export'}}]}
    report = montage_audit(*args, require_fresh: true)
    assert_equal 0.2, report[:real_cloud_lifecycle_estimate]['estimatedCost']
    refute_match(/privateKey|never-export/, JSON.generate(report))
    args[1]['handles'][0]['exitCode'] = nil
    assert_raises(RuntimeError) { montage_audit(*args, require_fresh: true) }
    args[1]['handles'][0]['exitCode'] = 0
    args[1]['handles'][0]['metadata'].delete('executionAttempt')
    montage_audit(*args, require_fresh: true)
    args[1]['handles'][0]['id'] += ':attempt-2'
    assert_raises(RuntimeError) { montage_audit(*args, require_fresh: true) }
    args[1]['handles'][0]['id'] = 'run:activity-1'
    args[1]['activityAttempts'][0]['attempt'] = 2
    assert_raises(RuntimeError) { montage_audit(*args, require_fresh: true) }
    args[1]['activityAttempts'][0]['attempt'] = 1
    args[1]['cloudBilling']['status'] = 'incomplete'
    assert_raises(RuntimeError) { montage_audit(*args, require_fresh: true) }
  end

  def test_dependency_coverage_relay_policy_and_joint_stage_deduplication
    args = fixture
    cloud = 'montage-gcp-e2-medium-1'
    args[0]['resources'] = [{'id'=>cloud, 'type'=>'cloud_vm'}, {'id'=>'micro', 'type'=>'hpc_machine'}]
    args[0]['plan']['assignments'][0]['resourceId'] = cloud
    args[0]['workflow']['dependencies'] = [{'activityId'=>'activity-2', 'dependsOnActivityId'=>'activity-1'}]
    route = {'producerActivityId'=>'activity-1', 'consumerActivityId'=>'activity-2',
      'serverNetworkBytes'=>100, 'address'=>'never-export', 'mergeCopySeconds'=>2,
      'phases'=>[{'name'=>'shared-preparation-barrier', 'startedAt'=>1, 'finishedAt'=>4, 'durationSeconds'=>3},
        {'name'=>'shared-conflict-check-copy-and-verify', 'startedAt'=>4, 'finishedAt'=>6, 'durationSeconds'=>2}]}
    transfer = {'activityId'=>'activity-2', 'status'=>'completed', 'strategy'=>'gateway', 'finishedAt'=>19, 'route'=>route}
    args[1]['artifactTransferRuns'] = [transfer, Marshal.load(Marshal.dump(transfer))]
    report = montage_audit(*args)
    assert_equal 1, report[:shared_preparations].length
    assert_equal 3, report[:shared_preparations].first[:barrier_seconds]
    assert_equal 2, report[:shared_preparations].first[:merge_seconds]
    assert_equal 200, report[:relay_server_network_bytes]
    refute_match(/never-export/, JSON.generate(report))
    args[1]['artifactTransferRuns'][0]['strategy'] = 'direct-runtime'
    assert_raises(RuntimeError) { montage_audit(*args) }
    args[1]['artifactTransferRuns'] = []
    assert_raises(RuntimeError) { montage_audit(*args) }
  end

  def fixture
    ids = (1..58).map { |i| "activity-#{i}" }
    request = {'workflow'=>{'activities'=>ids.map { |id| {'id'=>id} }}, 'plan'=>{'predicted'=>{'makespanSeconds'=>10, 'secret'=>'never-export'}, 'assignments'=>ids.map { |id| {'activityId'=>id, 'resourceId'=>'micro'} }}}
    real = {'run'=>{'id'=>'run', 'status'=>'completed'}, 'activityAttempts'=>ids.map { |id| {'activityId'=>id, 'attempt'=>2, 'status'=>'completed', 'startedAt'=>20, 'metadata'=>{'address'=>'secret-address'}} }, 'artifactTransferRuns'=>[{'activityId'=>ids.first, 'status'=>'completed', 'finishedAt'=>19}], 'workspaceLeases'=>[{'released'=>true}]}
    [request, real, {'run'=>{}}]
  end

  def test_latest_attempt_and_safe_allowlist
    request, real, sim = fixture
    real['activityAttempts'] << {'activityId'=>'activity-1', 'attempt'=>1, 'status'=>'failed'}
    report = montage_audit(request, real, sim)
    assert_equal 58, report[:activities].length
    refute_match(/secret|address/, JSON.generate(report))
  end

  def test_barrier_and_missing_activity_block_export
    args = fixture
    args[1]['artifactTransferRuns'][0]['finishedAt'] = 21
    assert_raises(RuntimeError) { montage_audit(*args) }
    args = fixture
    args[1]['activityAttempts'].pop
    assert_raises(RuntimeError) { montage_audit(*args) }
  end

  def test_terminal_run_required
    args = fixture
    args[1]['run']['status'] = 'running'
    assert_raises(RuntimeError) { montage_audit(*args) }
  end

  def test_canonical_hpc_handle_preserves_validated_runtime_without_attempt_metadata
    args = fixture
    args[1]['activityAttempts'].each { |task| task['attempt'] = 1 }
    args[1]['handles'] = [{'id'=>'run:activity-1', 'runId'=>'run', 'activityId'=>'activity-1', 'startedAt'=>20, 'finishedAt'=>25, 'metadata'=>{'terminalObservedAt'=>28}}]
    report = montage_audit(*args)
    assert_equal 5, report[:activities].first[:validated_handle_runtime_seconds]
    assert_equal 3, report[:activities].first[:terminal_observation_delay_seconds]
    args[1]['handles'].first['id'] = 'run:activity-1:attempt-2'
    assert_nil montage_audit(*args)[:activities].first[:validated_handle_runtime_seconds]
  end

  def test_timestamp_wall_is_separate_from_reported_makespan
    args = fixture
    args[1]['run'].merge!('startedAt'=>'2026-10-08T00:00:00Z', 'finishedAt'=>'2026-10-08T00:01:00Z', 'makespanSeconds'=>100)
    args[0]['plan']['assignments'][0]['predictedRuntimeSeconds'] = 7
    args[0]['plan']['assignments'][0]['predictedTransferSeconds'] = 3
    report = montage_audit(*args)
    assert_equal 60, report[:real_api_run_timestamp_wall_seconds]
    assert_equal 100, report[:real_api_reported]['makespanSeconds']
    assert_equal 7, report[:plan_assignment_compute_work_seconds]
    assert_equal 3, report[:plan_assignment_transfer_work_seconds]
  end

  def test_attempt_history_is_evidence_based
    args = fixture
    assert_equal 'multiple_attempts', montage_audit(*args)[:attempt_history]
    args[1]['activityAttempts'].each { |task| task['attempt'] = 1 }
    report = montage_audit(*args)
    assert_equal 'single_observed_attempt_per_activity', report[:attempt_history]
    refute report[:limitations].any? { |text| text.start_with?('Multiple attempts:') }
  end

  def test_single_lane_micro_requires_non_overlapping_execution_intervals
    args = fixture
    args[0]['resources'] = [{'id'=>'micro', 'cpuCapacity'=>1}]
    args[1]['activityAttempts'].each_with_index do |task, index|
      task['startedAt'] = 20 + index * 10
      task['finishedAt'] = 25 + index * 10
    end
    assert_equal 0, montage_audit(*args)[:audit][:single_lane_micro_overlap_count]
    args[1]['activityAttempts'][1]['startedAt'] = 24
    assert_raises(RuntimeError) { montage_audit(*args) }
    args[1]['activityAttempts'][1]['startedAt'] = 30
    args[1]['activityAttempts'][1].delete('finishedAt')
    assert_raises(RuntimeError) { montage_audit(*args) }
  end

  def test_native_timing_requires_exact_attempt_and_valid_timestamps
    args = fixture
    handle = {'activityId'=>'activity-1', 'startedAt'=>20, 'finishedAt'=>25, 'metadata'=>{'executionAttempt'=>2, 'terminalObservedAt'=>28, 'address'=>'never-export'}}
    args[1]['handles'] = [handle]
    task = montage_audit(*args)[:activities].find { |a| a[:activity_id] == 'activity-1' }
    assert_equal 5, task[:validated_handle_runtime_seconds]
    assert_equal 3, task[:terminal_observation_delay_seconds]
    handle['metadata']['executionAttempt'] = 1
    assert_nil montage_audit(*args)[:activities].first[:validated_handle_runtime_seconds]
    handle['metadata']['executionAttempt'] = 2
    handle['metadata']['terminalObservedAt'] = 24
    assert_nil montage_audit(*args)[:activities].first[:validated_handle_runtime_seconds]
    refute_match(/never-export/, JSON.generate(montage_audit(*args)))
  end
end
