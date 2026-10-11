require 'minitest/autorun'
require_relative 'freeze_montage_compute_profiles'

class MontageComputeFreezeTest < Minitest::Test
  def test_normalizes_by_work_and_never_fills_unmeasured_classes
    data = {'baselineRunId'=>'baseline', 'computeObservations'=>[
      {'activityFamily'=>'mproject', 'resourceClass'=>'e2-micro', 'taskRuntimeFallbackSamples'=>0,
        'wallSecondsPerReferenceSecond'=>{'samples'=>5,'median'=>2.5,'minimum'=>2,'maximum'=>3}}]}
    resources = {'micro'=>{'class'=>'e2-micro'}, 'medium'=>{'class'=>'e2-medium'}}
    result = MontageComputeFreeze.build(data, 'digest', resources, 'test')
    assert_equal 1, result.fetch('activityProfiles').length
    profile = result.fetch('activityProfiles').first
    assert_equal 2.5, profile.fetch('runtimeSeconds')
    assert_equal 'reference-work-v1', profile.fetch('modelVersion')
    assert_equal 'test-mproject', profile.fetch('activityTypeId')
    assert_equal 'micro', profile.fetch('resourceId')
    refute JSON.generate(result).include?('producer_activity_id')
    data.fetch('computeObservations').first['wallSecondsPerReferenceSecond'] = nil
    assert_raises(RuntimeError) { MontageComputeFreeze.build(data, 'digest', resources, 'test') }
  end
end
