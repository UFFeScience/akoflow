require 'minitest/autorun'
require_relative 'build_montage_work_features'

class MontageWorkFeaturesTest < Minitest::Test
  def test_input_work_ignores_old_duration_and_keeps_band
    activity = {'id'=>'workflow-mprojectid0000001','name'=>'mprojectid0000001','command'=>{'arguments'=>['-c','mProject -X input.fits output.fits region.hdr']},'simulation'=>{'durationSeconds'=>999}}
    request = {'workflow'=>{'activities'=>[activity],'dependencies'=>[]}}
    inputs = {'sourceImageSHA256'=>'source','inputs'=>[{'path'=>'input.fits','band'=>'red','unit'=>'input-megapixels','computeWorkUnits'=>6.0}]}
    feature = MontageWorkFeatures.build(request,inputs).values.first
    assert_equal 'mproject_red',feature.fetch('activityFamily')
    assert_equal 6.0,feature.fetch('computeWorkUnits')
    activity['simulation']['durationSeconds'] = 1
    assert_equal feature, MontageWorkFeatures.build(request,inputs).values.first
    activity['command']['arguments'][-1] = 'mProject missing.fits output.fits region.hdr'
    assert_raises(RuntimeError) { MontageWorkFeatures.build(request,inputs) }
  end
end
