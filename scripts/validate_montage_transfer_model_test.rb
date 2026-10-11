require 'minitest/autorun'
require_relative 'validate_montage_transfer_model'

class MontageTransferValidationTest < Minitest::Test
  def audit
    {'run_id'=>'observed', 'activities'=>[{'activity_id'=>'a','resource_id'=>'vm'},{'activity_id'=>'b','resource_id'=>'vm'}],
     'routes'=>[{'producer_activity_id'=>'a','consumer_activity_id'=>'b','strategy'=>'direct-runtime',
       'network_bytes'=>0,'phases'=>[{'name'=>'shared-conflict-check-copy-and-verify','durationSeconds'=>1000}]}],
     'real_api_reported'=>{'makespanSeconds'=>100}}
  end

  def request
    {'run'=>{'mode'=>'simulation'},'plan'=>{'assignments'=>[{'activityId'=>'a','resourceId'=>'vm'},{'activityId'=>'b','resourceId'=>'vm'}]},
     'networkTopology'=>{'metadata'=>{}}}
  end

  def test_fixed_placement_preserved_and_real_rejected
    original = request
    prepared = MontageTransferValidation.fixed_request(original,{'source'=>'old'},audit,'sim-only')
    assert_equal original.fetch('plan'),prepared.fetch('plan')
    refute original.fetch('run').key?('id')
    original['run']['mode'] = 'real'
    assert_raises(RuntimeError) { MontageTransferValidation.fixed_request(original,{},audit,'bad') }
    original = request; original['plan']['assignments'][0]['resourceId'] = 'other'
    assert_raises(RuntimeError) { MontageTransferValidation.fixed_request(original,{},audit,'bad') }
  end

  def test_local_route_and_merge_not_duplicated_or_summed_as_makespan
    result = MontageTransferValidation.compare(audit,{'transfers'=>[],'executed'=>{'makespanSeconds'=>90}},'calibration-fit-not-independent-validation',{'source'=>'observed'})
    assert_equal 0,result.fetch('routeMismatchCount')
    assert_equal 0,result.fetch('dependencies')[0].fetch('observedDependencyPhaseSeconds')
    assert_equal(-10,result.fetch('makespanErrorPercent'))
    assert result.fetch('withinInitialMakespanTolerance')
    refute result.fetch('accepted') # tolerance alone never grants acceptance
    assert_raises(RuntimeError) { MontageTransferValidation.compare(audit,{},'unlabeled',{}) }
  end

  def test_real_network_route_is_not_silently_normalized_to_local
    data = audit; data['routes'][0]['network_bytes'] = 10
    result = MontageTransferValidation.compare(data,{'transfers'=>[],'executed'=>{'makespanSeconds'=>90}},'original-frozen-forecast',{'source'=>'old'})
    assert_equal 1,result.fetch('routeMismatchCount')
    assert_equal(-100,result.fetch('networkBytesErrorPercent'))
  end

  def test_source_shared_capacity_excludes_queue_and_is_not_nic_bandwidth
    flows = 3.times.map do |i|
      {'status'=>'completed','route'=>{'strategy'=>'gateway','sourceResourceId'=>'hpc','targetResourceId'=>'montage-gcp-fog-e2-micro-02',
        'archiveBytes'=>100,'sourcePackExecutionSeconds'=>10,'sourcePackSshChannelWaitSeconds'=>999,
        'phases'=>[{'name'=>'pack-and-receive','finishedAt'=>20.0}]}}
    end
    capacity = MontageTransferValidation.shared_source_capacity({'artifactTransferRuns'=>flows},'hpc','e2-micro')
    assert_equal 3,capacity.fetch('concurrentCommandsMaximum')
    assert_equal 10,capacity.fetch('busySeconds')
    assert_equal 30,capacity.fetch('durationWeightedMedianBytesPerSecond')
    assert_includes capacity.fetch('interpretation'),'NOT NIC bandwidth'
    request = {'networkTopology'=>{'metadata'=>{'workspaceMaterializationModel'=>{'relayResourceId'=>'relay'}},
      'links'=>[{'id'=>'physical','sourceResourceId'=>'relay','targetResourceId'=>'hpc','bidirectional'=>true,'bandwidthBitsPerSecond'=>8000}]}}
    modified = MontageTransferValidation.apply_shared_source_capacity(request,'hpc',capacity)
    assert request['networkTopology']['links'][0]['bidirectional']
    links = modified['networkTopology']['links']
    refute links[0]['bidirectional']
    assert_equal 8000,links[0]['bandwidthBitsPerSecond'] # physical reverse unchanged
    assert_equal 240,links[1]['bandwidthBitsPerSecond']
    assert_equal 'hpc',links[1]['sourceResourceId']
    assert_raises(RuntimeError) { MontageTransferValidation.shared_source_capacity({'artifactTransferRuns'=>[]},'hpc','e2-micro') }
  end
end
