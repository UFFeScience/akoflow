#!/usr/bin/env ruby
require 'json'
require 'digest'
require_relative 'build_montage_execution_calibration'
require_relative 'build_montage_workspace_projection'
require_relative 'freeze_montage_materialization'

# Private API input is read only for exact final manifests. Exports never contain
# connection endpoints, credentials, Terraform state or raw command logs.
module MontageTransferValidation
  module_function

  def measured_volumes(audit, raw, request, digest)
    MontageExecutionCalibration.build(audit, digest)
    raise 'Completed reference identity mismatch' unless raw.fetch('run').values_at('id', 'status') == [audit.fetch('run_id'), 'completed']
    snapshots = raw.fetch('handles').to_h do |handle|
      raise 'Unsuccessful reference handle' unless handle.fetch('status') == 'completed' && handle.fetch('exitCode') == 0
      [handle.fetch('activityId'), MontageWorkspaceProjection.files(handle.fetch('artifacts').fetch('finalSnapshot'))]
    end
    ids = audit.fetch('activities').map { |a| a.fetch('activity_id') }.sort
    raise 'Reference snapshots do not match activities' unless snapshots.keys.sort == ids
    parents = request.fetch('workflow').fetch('dependencies').group_by { |e| e.fetch('activityId') }
    inputs = ids.to_h do |id|
      sources = parents.fetch(id, []).map { |e| snapshots.fetch(e.fetch('dependsOnActivityId')) }
      union = {}
      sources.each do |source|
        source.each do |path, file|
          raise 'Conflicting reference workspace contents' if union.key?(path) && union[path] != file
          union[path] = file
        end
      end
      [id, {'inputTotalBytes'=>sources.sum { |s| s.values.sum { |f| f.fetch('sizeBytes') } },
            'inputUniqueBytes'=>union.values.sum { |f| f.fetch('sizeBytes') }}]
    end
    {'baselineRunId'=>audit.fetch('run_id'), 'auditedEvidenceSHA256'=>digest,
     'activityReferenceDurations'=>request.fetch('workflow').fetch('activities').to_h { |a| [a.fetch('id'),a.fetch('simulation').fetch('durationSeconds')] },
     'observedProducerWorkspaceBytes'=>snapshots.transform_values { |s| s.values.sum { |f| f.fetch('sizeBytes') } },
     'observedConsumerInputVolumes'=>inputs}
  end

  def fixed_request(original, model, observed, id)
    request = Marshal.load(Marshal.dump(original))
    raise 'Simulation-only request required' unless request.fetch('run').fetch('mode') == 'simulation'
    assigned = request.fetch('plan').fetch('assignments').to_h { |a| [a.fetch('activityId'), a.fetch('resourceId')] }
    actual = observed.fetch('activities').to_h { |a| [a.fetch('activity_id'), a.fetch('resource_id')] }
    raise 'Fixed observed placement differs from plan' unless assigned == actual
    request['run']['id'] = id
    request['networkTopology']['metadata']['workspaceMaterializationModel'] = model
    request
  end

  # Auxiliary completed transfer commands from an interrupted diagnostic run
  # can identify source service. They are not a completed-run benchmark or a
  # holdout. Never use their total run time or incomplete command durations.
  def supplement_source_service(calibration, diagnostic, digest)
    manifests = diagnostic.fetch('handles').select { |h| h['status']=='completed' && h['exitCode']==0 }.to_h do |h|
      [h.fetch('activityId'),MontageWorkspaceProjection.files(h.fetch('artifacts').fetch('finalSnapshot'))]
    end
    snapshots = manifests.transform_values { |m| m.values.sum { |f| f.fetch('sizeBytes') } }
    samples = diagnostic.fetch('artifactTransferRuns').select { |t| t['status']=='completed' }.map do |t|
      r = t.fetch('route',{})
      next unless r['strategy']=='gateway' && r['sourcePackExecutionSeconds'] && r['sourcePackPreflightServiceSeconds'] && r['archiveBytes'] && snapshots[r['producerActivityId']]
      next unless MontageExecutionCalibration.resource_class(r.fetch('sourceResourceId'))=='hpc' && MontageExecutionCalibration.resource_class(r.fetch('targetResourceId'))=='e2-micro'
      {'seconds'=>r.fetch('sourcePackExecutionSeconds'),'archiveBytes'=>r.fetch('archiveBytes'),
       'workspaceBytes'=>snapshots.fetch(r.fetch('producerActivityId')),
       'wait'=>r.fetch('sourcePackSshChannelWaitSeconds',0),'preflight'=>r.fetch('sourcePackPreflightServiceSeconds')}
    end.compact
    raise 'No successful separately measured source service' if samples.empty?
    updated = Marshal.load(Marshal.dump(calibration))
    phase = updated.fetch('materializationPhaseObservations').find { |p| p.values_at('strategy','sourceClass','targetClass','phase') == ['gateway','hpc','e2-micro','pack-and-receive'] }
    raise 'Completed primary reference lacks this route class' unless phase
    phase['seconds'] = MontageExecutionCalibration.summary(samples.map { |s| s.fetch('seconds') })
    phase['secondsBases'] = ['source-command-execution-excludes-channel-queue']
    phase['pooledArchiveServiceRate'] = MontageExecutionCalibration.pooled_service_rate(samples,'archiveBytes')
    phase['pooledWorkspaceServiceRate'] = MontageExecutionCalibration.pooled_service_rate(samples,'workspaceBytes')
    phase['sourceChannelWaitSeconds'] = MontageExecutionCalibration.summary(samples.map { |s| s.fetch('wait') })
    phase['sourcePreflightServiceSeconds'] = MontageExecutionCalibration.summary(samples.map { |s| s.fetch('preflight') })
    updated['auxiliarySourceServiceEvidence'] = {'runId'=>diagnostic.fetch('run').fetch('id'),'sha256'=>digest,
      'completedTransferSamples'=>samples.length,'role'=>'completed commands from interrupted diagnostic; not independent validation',
      'queueNotAddedToService'=>true}
    # HPC merge commands contend on the same source session. Refit their pure
    # check/copy/verify service and residual from complete consumers only, so
    # their measured SSH queue is not added again by the semaphore.
    completed = diagnostic.fetch('artifactTransferRuns').select { |t| t['status']=='completed' && t.fetch('route',{})['producerActivityId'] }.map { |t| t.fetch('route') }
    merges = completed.group_by { |r| r.fetch('consumerActivityId') }.map do |consumer, routes|
      next unless manifests[consumer]
      r = routes.first
      next unless MontageExecutionCalibration.resource_class(r.fetch('targetResourceId'))=='hpc' && r['mergeCommandOverheadSeconds'] && r['mergeSshChannelWaitSeconds']
      parents = routes.map { |p| p.fetch('producerActivityId') }.uniq
      next unless parents.all? { |p| manifests[p] }
      union = {}
      parents.each { |p| manifests.fetch(p).each { |path,f| raise 'Auxiliary merge content conflict' if union[path] && union[path]!=f; union[path]=f } }
      total = parents.sum { |p| snapshots.fetch(p) }
      unique = union.values.sum { |f| f.fetch('sizeBytes') }
      next unless total > 0 && unique > 0
      {'checkSecondsPerInputByte'=>r.fetch('mergeCheckSeconds')/total,
       'copySecondsPerUniqueByte'=>r.fetch('mergeCopySeconds')/unique,
       'verifySecondsPerUniqueByte'=>r.fetch('mergeVerifySeconds')/unique,
       'overhead'=>r.fetch('mergeCommandOverheadSeconds'),'wait'=>r.fetch('mergeSshChannelWaitSeconds')}
    end.compact
    raise 'Queue-free auxiliary HPC merge service missing' if merges.empty?
    merge = updated.fetch('mergeRateObservations').find { |m| m.fetch('targetClass')=='hpc' }
    raise 'Primary HPC merge reference missing' unless merge
    merge['observations'] = %w[checkSecondsPerInputByte copySecondsPerUniqueByte verifySecondsPerUniqueByte].to_h { |key| [key,MontageExecutionCalibration.summary(merges.map { |m| m.fetch(key) })] }
    merge['commandOverheadSeconds'] = MontageExecutionCalibration.summary(merges.map { |m| m.fetch('overhead') })
    merge['channelWaitSeconds'] = MontageExecutionCalibration.summary(merges.map { |m| m.fetch('wait') })
    updated['auxiliarySourceServiceEvidence']['completedHPCMergeSamples']=merges.length
    updated
  end

  def error_percent(predicted, observed)
    observed > 0 ? 100.0 * (predicted - observed) / observed : nil
  end

  def shared_source_capacity(diagnostic, source_id, target_class)
    intervals = diagnostic.fetch('artifactTransferRuns').select { |t| t['status']=='completed' }.map do |t|
      r = t.fetch('route',{})
      next unless r['strategy']=='gateway' && r['sourceResourceId']==source_id && r['sourcePackExecutionSeconds'].to_f > 0 && r['archiveBytes'].to_i > 0
      next unless MontageExecutionCalibration.resource_class(r.fetch('targetResourceId'))==target_class
      phase = r.fetch('phases').find { |p| p['name']=='pack-and-receive' }
      next unless phase && phase['finishedAt']
      finish = Float(phase.fetch('finishedAt'))
      seconds = Float(r.fetch('sourcePackExecutionSeconds'))
      {'start'=>finish-seconds,'finish'=>finish,'rate'=>r.fetch('archiveBytes').to_f/seconds}
    end.compact
    raise 'Insufficient completed source-capacity observations' unless intervals.length >= 3
    boundaries = intervals.flat_map { |i| i.values_at('start','finish') }.uniq.sort
    windows = boundaries.each_cons(2).map do |a,b|
      active = intervals.select { |i| i.fetch('start') <= a && i.fetch('finish') >= b }
      next if active.empty?
      {'seconds'=>b-a,'bytesPerSecond'=>active.sum { |i| i.fetch('rate') },'concurrentCommands'=>active.length}
    end.compact
    total = windows.sum { |w| w.fetch('seconds') }
    cumulative = 0.0
    median = windows.sort_by { |w| w.fetch('bytesPerSecond') }.find { |w| cumulative += w.fetch('seconds'); cumulative >= total/2 }.fetch('bytesPerSecond')
    {'samples'=>intervals.length,'busySeconds'=>total,'concurrentCommandsMaximum'=>windows.map { |w| w.fetch('concurrentCommands') }.max,
     'durationWeightedMedianBytesPerSecond'=>median,'minimumBytesPerSecond'=>windows.map { |w| w.fetch('bytesPerSecond') }.min,
     'maximumBytesPerSecond'=>windows.map { |w| w.fetch('bytesPerSecond') }.max,
     'interpretation'=>'Aggregate source pack/ingress service, NOT NIC bandwidth; assumes uniform byte progress within each queue-free command.',
     'limitations'=>['Small auxiliary diagnostic sample; capacity is conditioned on workload and concurrent source work.','Admission queue excluded from command intervals.']}
  end

  def apply_shared_source_capacity(request, source, capacity)
    result = Marshal.load(Marshal.dump(request))
    model = result.fetch('networkTopology').fetch('metadata').fetch('workspaceMaterializationModel')
    relay = model.fetch('relayResourceId')
    links = result.fetch('networkTopology').fetch('links')
    physical = links.find { |l| l['sourceResourceId']==relay && l['targetResourceId']==source && l['bidirectional'] }
    raise 'Expected explicit bidirectional relay endpoint link' unless physical
    # Retain the reverse physical link, and model the forward combined service
    # as one shared logical bottleneck. No extra packing CPU is added to it.
    physical['bidirectional'] = false
    links << physical.merge('id'=>physical.fetch('id')+'-measured-source-service',
      'sourceResourceId'=>source,'targetResourceId'=>relay,'bandwidthBitsPerSecond'=>8*capacity.fetch('durationWeightedMedianBytesPerSecond'),
      'sharingPolicy'=>'SHARED','metadata'=>{'measurementBasis'=>'combined-source-pack-and-ingress-service-not-nic', 'evidence'=>capacity})
    result
  end

  def compare(observed, simulated, role, model)
    raise 'Unknown comparison role' unless %w[original-frozen-forecast historical-calibration-completed-run-holdout calibration-fit-not-independent-validation].include?(role)
    actual = observed.fetch('routes').select { |r| r['producer_activity_id'] }.to_h { |r| [[r.fetch('producer_activity_id'),r.fetch('consumer_activity_id')], r] }
    predicted = simulated.fetch('transfers').to_h { |r| [[r.fetch('producerActivityId'),r.fetch('consumerActivityId')],r] }
    resources = observed.fetch('activities').to_h { |a| [a.fetch('activity_id'),a.fetch('resource_id')] }
    rows = actual.map do |key, route|
      estimate = predicted[key]
      # Real route duration repeats the shared barrier/merge. Compare only the
      # dependency-specific phases; compare the shared preparation separately.
      service = route.fetch('phases').reject { |p| p.fetch('name').start_with?('shared-') }.sum { |p| p.fetch('durationSeconds') }
      service = 0.0 if route.fetch('strategy') == 'runtime-local'
      # The executor labels same-VM workspaces direct-runtime, but no peer
      # phases or network traffic occur. This is semantically runtime-local.
      local = resources.fetch(key[0]) == resources.fetch(key[1]) && route['network_bytes'].to_i == 0 && route.fetch('phases').all? { |p| p.fetch('name').start_with?('shared-') }
      canonical = local ? 'runtime-local' : route.fetch('strategy')
      {'producer'=>key[0], 'consumer'=>key[1], 'observedStrategy'=>canonical, 'executorStrategy'=>route.fetch('strategy'),
       'predictedStrategy'=>estimate ? estimate.fetch('strategy') : 'runtime-local',
       'observedNetworkBytes'=>route['network_bytes'].to_i, 'predictedNetworkBytes'=>estimate ? estimate.fetch('networkBytes',0) : 0,
       'observedDependencyPhaseSeconds'=>service, 'predictedDependencySeconds'=>estimate ? estimate.fetch('durationSeconds') : 0,
       'durationErrorPercent'=>estimate ? error_percent(estimate.fetch('durationSeconds'),service) : nil,
       'predictedServiceWaitSeconds'=>estimate ? estimate.fetch('route').fetch('phases').sum { |p| p.fetch('serviceWaitSeconds',0) } : 0}
    end
    makespan = simulated.fetch('executed').fetch('makespanSeconds')
    real = observed.fetch('real_api_reported').fetch('makespanSeconds')
    {'schemaVersion'=>1, 'comparisonRole'=>role, 'observedRunId'=>observed.fetch('run_id'),
     'modelSource'=>model.fetch('source'), 'simulatedMakespanSeconds'=>makespan, 'observedMakespanSeconds'=>real,
     'makespanErrorPercent'=>error_percent(makespan,real), 'withinInitialMakespanTolerance'=>error_percent(makespan,real).abs <= 20,
     'predictedNetworkBytes'=>rows.sum { |r| r.fetch('predictedNetworkBytes') },
     'observedNetworkBytes'=>rows.sum { |r| r.fetch('observedNetworkBytes') },
     'networkBytesErrorPercent'=>error_percent(rows.sum { |r| r.fetch('predictedNetworkBytes') },rows.sum { |r| r.fetch('observedNetworkBytes') }),
     'routeMismatchCount'=>rows.count { |r| r.fetch('observedStrategy') != r.fetch('predictedStrategy') },
     'dominantTransfers'=>rows.sort_by { |r| -r.fetch('observedDependencyPhaseSeconds') }.first(15), 'dependencies'=>rows,
     'accepted'=>false, 'limitations'=>[
       'Makespan tolerance alone is insufficient; dominant transfer residuals must also be reviewed.',
       'Historical receiver phases may include unseparated queues; destination bulk pools must remain disabled until service is separately identified.',
       'No summed parallel phase durations are treated as makespan.',
       'Compute, ordering and provisioning assumptions retained from the frozen original request.',
       role == 'calibration-fit-not-independent-validation' ? 'This comparison reuses calibration data and is not independent validation.' : 'Observed comparison run was not used to fit these transfer coefficients; this remains a retrospective test, not a new prospective real validation.']}
  end
end

if $PROGRAM_NAME == __FILE__
  mode = ARGV.shift
  case mode
  when 'calibrate'
    abort 'calibrate AUDIT RAW REQUEST RESOURCES OUTPUT_PREFIX' unless ARGV.length == 5
    audit_text = File.binread(ARGV[0]); audit = JSON.parse(audit_text)
    volumes = MontageTransferValidation.measured_volumes(audit, JSON.parse(File.binread(ARGV[1])), JSON.parse(File.binread(ARGV[2])), Digest::SHA256.hexdigest(audit_text))
    calibration = MontageExecutionCalibration.build(audit,Digest::SHA256.hexdigest(audit_text),volumes)
    model = MontageMaterializationFreeze.build(calibration,Digest::SHA256.hexdigest(JSON.pretty_generate(calibration)+"\n"),JSON.parse(File.binread(ARGV[3])),'montage-relay-controller')
    {'volumes'=>volumes,'calibration'=>calibration,'model'=>model}.each { |name,value| File.write("#{ARGV[4]}-#{name}.json",JSON.pretty_generate(value)+"\n") }
  when 'request'
    abort 'request ORIGINAL MODEL AUDIT RUN_ID OUTPUT' unless ARGV.length == 5
    result = MontageTransferValidation.fixed_request(*ARGV[0,3].map { |p| JSON.parse(File.binread(p)) },ARGV[3])
    File.write(ARGV[4],JSON.pretty_generate(result)+"\n")
  when 'supplement'
    abort 'supplement CALIBRATION DIAGNOSTIC_API RESOURCES OUTPUT_PREFIX' unless ARGV.length == 4
    text = File.binread(ARGV[1])
    calibration = MontageTransferValidation.supplement_source_service(JSON.parse(File.binread(ARGV[0])),JSON.parse(text),Digest::SHA256.hexdigest(text))
    resources = JSON.parse(File.binread(ARGV[2]))
    resources.each do |_, resource|
      next unless resource.fetch('domain')=='hpc'
      # All HPC targets in this reference share one measured SSH control
      # connection. Do not transfer this capacity claim to unrelated endpoints.
      resource['bulkServiceGroupId']='plafrim-reference-ssh-bulk'
      resource['bulkServiceSlots']=3
      resource['bulkServiceSource']='AKO-148 SSHChannelLimit4 reserve1; diagnostic source waits prove two waves of three'
    end
    model = MontageMaterializationFreeze.build(calibration,Digest::SHA256.hexdigest(JSON.pretty_generate(calibration)+"\n"),resources,'montage-relay-controller')
    {'calibration'=>calibration,'model'=>model}.each { |name,value| File.write("#{ARGV[3]}-#{name}.json",JSON.pretty_generate(value)+"\n") }
  when 'compare'
    abort 'compare AUDIT SIM_RESULT MODEL ROLE OUTPUT' unless ARGV.length == 5
    audit, sim, model = ARGV[0,3].map { |p| JSON.parse(File.binread(p)) }
    result = MontageTransferValidation.compare(audit,sim,ARGV[3],model)
    File.write(ARGV[4],JSON.pretty_generate(result)+"\n")
    puts JSON.generate(result.reject { |key,_| %w[dependencies dominantTransfers].include?(key) })
  when 'shared-capacity'
    abort 'shared-capacity REQUEST DIAGNOSTIC_API HPC_RESOURCE OUTPUT_PREFIX' unless ARGV.length == 4
    request = JSON.parse(File.binread(ARGV[0])); diagnostic = JSON.parse(File.binread(ARGV[1]))
    capacity = MontageTransferValidation.shared_source_capacity(diagnostic,ARGV[2],'e2-micro')
    capacity['runId']=diagnostic.fetch('run').fetch('id')
    capacity['sha256']=Digest::SHA256.file(ARGV[1]).hexdigest
    result = MontageTransferValidation.apply_shared_source_capacity(request,ARGV[2],capacity)
    File.write(ARGV[3]+'-request.json',JSON.pretty_generate(result)+"\n")
    File.write(ARGV[3]+'-capacity.json',JSON.pretty_generate(capacity)+"\n")
  when 'report'
    abort 'report PRIVATE_EXPERIMENT_DIR OUTPUT_JSON' unless ARGV.length == 2
    base = ARGV[0]
    cases = %w[ako151-original-comparison ako151-historical-comparison ako151-clean-v2-comparison ako151-shared-source-v2-comparison].map do |name|
      result = JSON.parse(File.binread(File.join(base,name+'.json')))
      result.reject { |key,_| key=='dependencies' }
    end
    provenance = %w[ako151-clean-v2-calibration ako151-source-pool-v3-calibration ako151-source-pool-v3-model ako151-shared-source-v2-capacity].to_h do |name|
      text = File.binread(File.join(base,name+'.json'))
      [name,{'sha256'=>Digest::SHA256.hexdigest(text),'evidence'=>JSON.parse(text)}]
    end
    previous = %w[montage-clean-simulation-20261011-v1-request.json selected-plan.json simulation-results.json].to_h { |name| [name,Digest::SHA256.file(File.join(base,name)).hexdigest] }
    report = {'schemaVersion'=>1,'issue'=>'AKO-151','realExecutionStarted'=>false,'daemonRestarted'=>false,
      'validationMode'=>'native SimGrid offline; exact production export; fixed frozen observed placement and compute',
      'previousForecastFilesSHA256'=>previous,'cases'=>cases,'provenance'=>provenance,
      'gate'=>{'accepted'=>false,'newPlanningReleased'=>false,'independentRealPrepared'=>false,
        'reason'=>'Shared source-capacity fit meets makespan tolerance but dominant dependency residuals still exceed 100%; do not accept compensating errors.'},
      'remainingLimitations'=>[
        'Receiver command service is not identifiable separately from queue in completed references. No destination pool is added to those mixed samples.',
        'Nine completed source commands and measured HPC merges from an interrupted diagnostic are auxiliary calibration evidence, not a completed-run benchmark.',
        'Aggregate source capacity varies with workload/concurrency; constant-rate fit cannot explain every dominant dependency.',
        'New planning must retain only evidenced route classes. Unobserved placement/storage identity must not be silently inferred.',
        'Provisioning and orchestration timing retained from the frozen original forecast; not yet independently validated.',
        'Compute and lifecycle cost bases unchanged; e2-medium interference remains extrapolated and network/public-IP billing remains incomplete.']}
    File.write(ARGV[1],JSON.pretty_generate(report)+"\n")
  else
    abort 'Use calibrate, supplement, request or compare; this tool never submits real runs.'
  end
end
