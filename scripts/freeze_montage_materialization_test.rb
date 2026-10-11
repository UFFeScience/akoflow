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
end
