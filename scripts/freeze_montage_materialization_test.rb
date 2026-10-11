require 'minitest/autorun'
require_relative 'freeze_montage_materialization'

class MontageMaterializationFreezeTest < Minitest::Test
  def observations
    JSON.parse(File.read(File.expand_path('../experiments/montage-micro-hpc-e2/execution-calibration-v2.json', __dir__)))
  end

  def resources
    { 'relay'=>{'class'=>'relay', 'domain'=>'relay'}, 'micro'=>{'class'=>'e2-micro','domain'=>'cloud'}, 'hpc'=>{'class'=>'hpc','domain'=>'hpc'} }
  end

  def test_only_measured_combinations_and_normalized_phases
    frozen = MontageMaterializationFreeze.build(observations, 'baseline-digest', resources, 'relay')
    assert_equal 7, frozen.fetch('profiles').length
    refute frozen.fetch('profiles').any? { |p| p.values_at('sourceClass','targetClass') == %w[hpc e2-medium] }
    gateway = frozen.fetch('profiles').find { |p| p.values_at('sourceClass','targetClass') == %w[hpc e2-micro] }
    assert gateway.fetch('ingressIncludesSourcePreparation')
    assert_operator gateway.fetch('ingressArchiveBytesPerSecond'), :>, 0
    assert_equal 3, frozen.fetch('mergeProfiles').length
    assert_includes frozen.fetch('source'), 'baseline-digest'
  end

  def test_rejects_missing_measured_rate_and_wrong_volume_basis
    calibration = observations
    calibration.fetch('materializationPhaseObservations').find { |r| r.fetch('phase') == 'archive-upload' }['effectiveArchiveBytesPerSecond'] = nil
    assert_raises(RuntimeError) { MontageMaterializationFreeze.build(calibration, 'digest', resources, 'relay') }
    calibration['workspaceVolumeBasis'] = 'changed-output-only'
    assert_raises(RuntimeError) { MontageMaterializationFreeze.build(calibration, 'digest', resources, 'relay') }
  end

  def test_source_preflight_service_is_preserved_without_queue_double_count
    calibration = observations
    pack = calibration.fetch('materializationPhaseObservations').find { |r| r.values_at('sourceClass', 'targetClass', 'phase') == ['hpc', 'e2-micro', 'pack-and-receive'] }
    pack['sourcePreflightServiceSeconds'] = {'samples'=>2, 'median'=>0.4}
    pack['sourcePreflightChannelWaitSeconds'] = {'samples'=>2, 'median'=>9}
    frozen = MontageMaterializationFreeze.build(calibration, 'digest', resources, 'relay')
    profile = frozen.fetch('profiles').find { |r| r.values_at('sourceClass', 'targetClass') == ['hpc', 'e2-micro'] }
    assert_equal 0.4, profile.fetch('probeSeconds')
  end

  def test_merge_command_overhead_preserves_service_not_channel_queue
    calibration = observations
    row = calibration.fetch('mergeRateObservations').find { |r| r.fetch('targetClass') == 'hpc' }
    row['commandOverheadSeconds'] = {'samples'=>3, 'median'=>2.5}
    row['channelWaitSeconds'] = {'samples'=>3, 'median'=>30}
    frozen = MontageMaterializationFreeze.build(calibration, 'digest', resources, 'relay')
    profile = frozen.fetch('mergeProfiles').find { |r| r.fetch('targetClass') == 'hpc' }
    assert_equal 2.5, profile.fetch('commandOverheadSeconds')
    refute profile.key?('channelWaitSeconds')
  end

  def test_bulk_pool_rejects_unseparated_or_mixed_historical_pack_queue
    calibration = observations
    pooled = resources
    pooled['hpc'].merge!('bulkServiceGroupId'=>'hpc-transport', 'bulkServiceSlots'=>3, 'bulkServiceSource'=>'fixture')
    assert_raises(RuntimeError) { MontageMaterializationFreeze.build(calibration, 'digest', pooled, 'relay') }
    packs = calibration.fetch('materializationPhaseObservations').select { |r| r.fetch('sourceClass') == 'hpc' && r.fetch('phase') == 'pack-and-receive' }
    packs.each do |pack|
      pack['secondsBases'] = ['source-command-execution-excludes-channel-queue']
      pack['sourcePreflightServiceSeconds'] = {'samples'=>2, 'median'=>0.4}
    end
    calibration.fetch('materializationPhaseObservations').select { |r| r.fetch('targetClass') == 'hpc' && r.fetch('phase').end_with?('receiver-verify-and-extract') }.each do |receiver|
      receiver['secondsBases'] = ['phase-command-execution-excludes-channel-queue']
    end
    frozen = MontageMaterializationFreeze.build(calibration, 'digest', pooled, 'relay')
    assert_equal 3, frozen.fetch('resources').fetch('hpc').fetch('bulkServiceSlots')
    packs.first['secondsBases'] << 'combined-phase-wall-includes-unseparated-service-queue'
    assert_raises(RuntimeError) { MontageMaterializationFreeze.build(calibration, 'digest', pooled, 'relay') }
  end

  def test_receiver_pool_requires_queue_free_receiver_command_measurements
    calibration = observations
    pooled = resources
    pooled['micro'].merge!('bulkServiceGroupId'=>'cloud-transport', 'bulkServiceSlots'=>3, 'bulkServiceSource'=>'fixture')
    assert_raises(RuntimeError) { MontageMaterializationFreeze.build(calibration, 'digest', pooled, 'relay') }
    receivers = calibration.fetch('materializationPhaseObservations').select { |r| r.fetch('targetClass') == 'e2-micro' && r.fetch('phase').end_with?('receiver-verify-and-extract') }
    receivers.each { |receiver| receiver['secondsBases'] = ['phase-command-execution-excludes-channel-queue'] }
    calibration.fetch('materializationPhaseObservations').select { |r| r.fetch('sourceClass') == 'e2-micro' && r.fetch('phase') == 'pack-and-receive' }.each do |pack|
      pack['secondsBases'] = ['source-command-execution-excludes-channel-queue']
      pack['sourcePreflightServiceSeconds'] = {'samples'=>2, 'median'=>0.4}
    end
    frozen = MontageMaterializationFreeze.build(calibration, 'digest', pooled, 'relay')
    assert_equal 3, frozen.fetch('resources').fetch('micro').fetch('bulkServiceSlots')
  end

  def test_pooled_service_rate_is_used_without_rewriting_legacy_observations
    row = {'pooledArchiveServiceRate'=>{'samples'=>2, 'bytesPerSecond'=>36.5},
           'effectiveArchiveBytesPerSecond'=>{'samples'=>2, 'median'=>999}}
    assert_equal 36.5, MontageMaterializationFreeze.service_rate(row, 'pooledArchiveServiceRate', 'effectiveArchiveBytesPerSecond')
    row.delete('pooledArchiveServiceRate')
    assert_equal 999, MontageMaterializationFreeze.service_rate(row, 'pooledArchiveServiceRate', 'effectiveArchiveBytesPerSecond')
    row['pooledArchiveServiceRate'] = {'samples'=>2, 'bytesPerSecond'=>0}
    assert_raises(RuntimeError) { MontageMaterializationFreeze.service_rate(row, 'pooledArchiveServiceRate', 'effectiveArchiveBytesPerSecond') }
  end
end
