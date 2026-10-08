require 'minitest/autorun'
require_relative 'audit_montage_results'

class MontageAuditTest < Minitest::Test
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
