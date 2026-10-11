require 'minitest/autorun'
require_relative 'build_montage_execution_calibration'

class MontageExecutionCalibrationTest < Minitest::Test
  def baseline
    JSON.parse(File.read(File.expand_path('../experiments/montage-micro-hpc-e2/docker-image-real-v2-results.json', __dir__)))
  end

  def test_baseline_and_shared_merge_deduplication
    data = baseline
    result = MontageExecutionCalibration.build(data, 'baseline-digest')
    assert_equal 'whole-workspace-v1', result['synchronizationContract']
    assert_equal 58, result['computeObservations'].sum { |g| g['runtimeSeconds']['samples'] }
    assert_equal data['shared_preparations'].length, result['consumerMergeObservations'].sum { |g| g['mergeSeconds']['samples'] }
    refute result['materializationPhaseObservations'].any? { |p| p['phase'].start_with?('shared-') }
    assert result['materializationPhaseObservations'].any? { |p| p['strategy'] == 'gateway' }
    refute_match(/credentials|privateKey|sshSourceRanges|terraform/i, JSON.generate(result))
  end

  def test_reject_unaudited_baseline
    data = baseline
    data['audit']['barrier_violation_count'] = 1
    assert_raises(RuntimeError) { MontageExecutionCalibration.build(data, 'digest') }
    data = baseline
    data['activities'][1] = data['activities'][0]
    assert_raises(RuntimeError) { MontageExecutionCalibration.build(data, 'digest') }
  end

  def test_no_false_precision_or_nonfinite_samples
    assert_equal({'samples'=>4,'median'=>2.5,'minimum'=>1.0,'maximum'=>4.0}, MontageExecutionCalibration.summary([4,2,1,3]))
    assert_nil MontageExecutionCalibration.summary([nil])
    assert_raises(RuntimeError) { MontageExecutionCalibration.summary([Float::INFINITY]) }
  end

  def test_manifest_volume_is_distinct_from_output_logical_bytes
    data = baseline
    projection = JSON.parse(File.read(File.expand_path('../experiments/montage-micro-hpc-e2/workspace-projection-v2.json', __dir__)))
    result = MontageExecutionCalibration.build(data, projection.fetch('auditedEvidenceSHA256'), projection)
    assert_equal 'producer-final-snapshot-manifest', result['workspaceVolumeBasis']
    assert result['materializationPhaseObservations'].all? { |phase| phase['workspaceBytes'] }
    assert result['materializationPhaseObservations'].any? { |phase| phase['workspaceBytes']['median'] > phase['logicalBytes']['median'] }
    projection['baselineRunId'] = 'another-run'
    assert_raises(RuntimeError) { MontageExecutionCalibration.build(data, projection.fetch('auditedEvidenceSHA256'), projection) }
  end
end
