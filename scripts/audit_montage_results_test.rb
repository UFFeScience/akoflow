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
end
