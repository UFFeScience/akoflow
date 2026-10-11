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

  def test_pooled_service_coefficient_weights_byte_work_not_flow_speeds
    rate = MontageExecutionCalibration.pooled_service_rate([
      {'archiveBytes'=>100, 'seconds'=>10}, {'archiveBytes'=>1000, 'seconds'=>20},
      {'archiveBytes'=>nil, 'seconds'=>999}], 'archiveBytes')
    assert_equal 2, rate.fetch('samples')
    assert_equal 1100, rate.fetch('bytes')
    assert_equal 30, rate.fetch('serviceSeconds')
    assert_in_delta 1100.0 / 30, rate.fetch('bytesPerSecond'), 1e-9
    assert_nil MontageExecutionCalibration.pooled_service_rate([], 'archiveBytes')
  end

  def test_source_command_service_excludes_measured_channel_queue
    data = baseline
    gateway = data.fetch('routes').select { |r| r['strategy'] == 'gateway' && r['producer_activity_id'] }
    gateway.each do |r|
      r['source_pack_execution_seconds'] = 10
      r['source_pack_ssh_channel_wait_seconds'] = 20
      r['source_pack_preflight_seconds'] = 2
    end
    result = MontageExecutionCalibration.build(data, 'digest')
    packs = result.fetch('materializationPhaseObservations').select { |p| p['phase'] == 'pack-and-receive' }
    refute_empty packs
    packs.each do |p|
      assert_equal 10, p.fetch('seconds').fetch('median')
      assert_equal 20, p.fetch('sourceChannelWaitSeconds').fetch('median')
      assert_equal ['source-command-execution-excludes-channel-queue'], p.fetch('secondsBases')
    end
    legacy = MontageExecutionCalibration.build(baseline, 'digest')
    assert legacy.fetch('materializationPhaseObservations').select { |p| p['phase'] == 'pack-and-receive' }.all? { |p| p['sourceChannelWaitSeconds'].nil? }
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

  def test_receiver_command_service_not_phase_wall_or_queue
    data = baseline
    data.fetch('routes').each do |route|
      route.fetch('phases').select { |phase| phase.fetch('name').end_with?('receiver-verify-and-extract') }.each do |phase|
        phase['executionSeconds'] = 3
        phase['sshChannelWaitSeconds'] = 12
        phase['durationSeconds'] = 15
      end
    end
    result = MontageExecutionCalibration.build(data, 'digest')
    receivers = result.fetch('materializationPhaseObservations').select { |phase| phase.fetch('phase').end_with?('receiver-verify-and-extract') }
    refute_empty receivers
    receivers.each do |phase|
      assert_equal 3, phase.fetch('seconds').fetch('median')
      assert_equal 12, phase.fetch('phaseChannelWaitSeconds').fetch('median')
      assert_equal ['phase-command-execution-excludes-channel-queue'], phase.fetch('secondsBases')
    end
  end
end
