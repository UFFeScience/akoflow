require 'minitest/autorun'
require_relative 'supplement_montage_root_compute'

class MontageRootComputeTest < Minitest::Test
  def fixture
    names = 12.times.map { |i| "mprojectid#{i.to_s.rjust(7,'0')}" }
    projection = {'activityComputeWork'=>names.to_h { |name| [name,{'activityName'=>name,'activityFamily'=>'mproject_red','computeWorkUnits'=>6.0}] }}
    request = {'workflow'=>{'activities'=>names.map { |n| {'id'=>n,'name'=>n} }},'plan'=>{'assignments'=>names.map { |n| {'activityId'=>n,'resourceId'=>'plafrim-bora'} }}}
    response = {'run'=>{'id'=>'training','status'=>'failed'},'activityAttempts'=>names.map { |n| {'activityId'=>n,'attempt'=>1,'status'=>'completed','runtimeSeconds'=>300.0} },
      'handles'=>names.map { |n| {'id'=>'training:'+n,'activityId'=>n,'status'=>'completed','exitCode'=>0,'artifacts'=>{'finalSnapshot'=>[{},{}]}} }}
    calibration = {'computeWorkBasis'=>'immutable-dataset-independent-work-v1','computeObservations'=>[]}
    [calibration,projection,request,response]
  end

  def test_only_completed_roots_are_added_and_resource_coverage_is_restricted
    args = fixture
    result = MontageRootCompute.build(*args,'digest')
    row = result.fetch('computeObservations').first
    assert_equal ['plafrim-bora'],row.fetch('allowedResourceIds')
    assert_equal 12,row.fetch('runtimeSeconds').fetch('samples')
    assert_equal 50.0,row.fetch('wallSecondsPerReferenceSecond').fetch('median')
    assert_empty args.first.fetch('computeObservations')
    assert_match(/not end-to-end validation/,row.fetch('interpretation'))
  end

  def test_failed_or_retried_root_cannot_be_training_data
    args = fixture
    args.last.fetch('activityAttempts').first['status'] = 'failed'
    assert_raises(RuntimeError) { MontageRootCompute.build(*args,'digest') }
    args = fixture
    args.last.fetch('handles').first['exitCode'] = 70
    assert_raises(RuntimeError) { MontageRootCompute.build(*args,'digest') }
  end
end
