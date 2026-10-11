#!/usr/bin/env ruby
require 'json'
require 'digest'

# Consume only the audited allowlisted scientific export, never raw API logs.
# Estimates retain their sample count and range; one run is not a confidence
# interval and concurrent route durations are not aggregate wall-clock time.
module MontageExecutionCalibration
  module_function

  def resource_class(id)
    return 'e2-micro' if id.include?('fog-e2-micro')
    return 'e2-medium' if id.include?('e2-medium')
    return 'hpc' if id.include?('bora') || id.include?('diablo')
    raise "Unclassified resource #{id.inspect}"
  end

  def family(id)
    match = id.match(/-([a-z]+)id\d+$/)
    raise "Unclassified activity #{id.inspect}" unless match
    match[1]
  end

  def summary(values)
    values = values.compact.map { |value| Float(value) }.sort
    raise 'Invalid non-finite or negative measurement' if values.any? { |v| !v.finite? || v < 0 }
    return nil if values.empty?
    middle = values.length / 2
    median = values.length.odd? ? values[middle] : (values[middle - 1] + values[middle]) / 2
    { 'samples' => values.length, 'median' => median, 'minimum' => values.first, 'maximum' => values.last }
  end

  # Fit a class-level service coefficient using byte-weighted work, not the
  # arithmetic median of speeds. Summed service seconds are deliberately not
  # wall time or aggregate link bandwidth: queues remain modeled separately.
  def pooled_service_rate(samples, byte_key)
    usable = samples.select { |p| p[byte_key] && p[byte_key] > 0 && p['seconds'] > 0 }
    return nil if usable.empty?
    bytes = usable.sum { |p| p.fetch(byte_key) }
    seconds = usable.sum { |p| p.fetch('seconds') }
    {'samples'=>usable.length, 'bytes'=>bytes, 'serviceSeconds'=>seconds,
     'bytesPerSecond'=>bytes.to_f / seconds,
     'interpretation'=>'byte-weighted per-command service coefficient; summed service is not concurrent wall time'}
  end

  def build(data, digest, projection = nil)
    if projection && (projection.fetch('baselineRunId') != data.fetch('run_id') || projection.fetch('auditedEvidenceSHA256') != digest)
      raise 'Workspace projection baseline provenance mismatch'
    end
    audit = data.fetch('audit')
    raise 'Baseline must contain 58 single-attempt completed activities' unless audit.fetch('expected_count') == 58 && data.fetch('attempt_history') == 'single_observed_attempt_per_activity'
    %w[missing unexpected not_completed].each { |key| raise "Unsafe baseline #{key}" unless audit.fetch(key).empty? }
    %w[incomplete_transfer_count barrier_violation_count unreleased_lease_count single_lane_micro_overlap_count missing_dependency_route_count cross_cloud_hpc_non_relay_count].each { |key| raise "Unsafe baseline #{key}" unless audit.fetch(key).zero? }
    activities = data.fetch('activities')
    raise 'Duplicate or missing baseline activity' unless activities.length == 58 && activities.map { |a| a.fetch('activity_id') }.uniq.length == 58
    indexed = activities.each_with_object({}) { |activity, map| map[activity.fetch('activity_id')] = activity }
    independent = projection && projection['activityComputeWork']
    compute = activities.group_by { |a| [independent ? independent.fetch(a.fetch('activity_id')).fetch('activityFamily') : family(a.fetch('activity_id')), resource_class(a.fetch('resource_id'))] }.map do |(program, machine), samples|
      { 'activityFamily' => program, 'resourceClass' => machine,
        'runtimeSeconds' => summary(samples.map { |a| a.fetch('validated_handle_runtime_seconds') || a.fetch('observed_runtime_seconds') }),
        'taskRuntimeFallbackSamples' => samples.count { |a| a['validated_handle_runtime_seconds'].nil? },
        'wallSecondsPerReferenceSecond' => projection && summary(samples.map do |a|
          reference = Float(independent ? independent.fetch(a.fetch('activity_id')).fetch('computeWorkUnits') : projection.fetch('activityReferenceDurations').fetch(a.fetch('activity_id')))
          raise 'Invalid reference work' unless reference.finite? && reference > 0
          (a.fetch('validated_handle_runtime_seconds') || a.fetch('observed_runtime_seconds')) / reference
        end),
        'interpretation' => 'observed wall runtime; includes physical sharing, not clean CPU service demand' }
    end
    routes = data.fetch('routes').select { |route| route['producer_activity_id'] }
    phases = []
    routes.each do |route|
      source = indexed.fetch(route.fetch('producer_activity_id'))
      target = indexed.fetch(route.fetch('consumer_activity_id'))
      route.fetch('phases').each do |phase|
        # Consumer barrier and final merge appear in multiple dependency route
        # records. Use shared_preparations once instead of fitting them here.
        next if phase.fetch('name').start_with?('shared-')
        measured_service = phase.fetch('name') == 'pack-and-receive' && !route['source_pack_execution_seconds'].nil?
        phase_service = !phase['executionSeconds'].nil?
        seconds = phase_service ? phase.fetch('executionSeconds') : measured_service ? route.fetch('source_pack_execution_seconds') : phase.fetch('durationSeconds')
        phases << { 'strategy' => route.fetch('strategy'), 'sourceClass' => resource_class(source.fetch('resource_id')),
          'targetClass' => resource_class(target.fetch('resource_id')), 'phase' => phase.fetch('name'),
          'seconds' => seconds, 'secondsBasis' => phase_service ? 'phase-command-execution-excludes-channel-queue' : measured_service ? 'source-command-execution-excludes-channel-queue' : 'combined-phase-wall-includes-unseparated-service-queue',
          'phaseChannelWaitSeconds' => phase_service ? phase['sshChannelWaitSeconds'].to_f : nil,
          'sourceChannelWaitSeconds' => measured_service ? route['source_pack_ssh_channel_wait_seconds'].to_f : nil,
          'sourcePreflightServiceSeconds' => route['source_pack_preflight_service_seconds'],
          'sourcePreflightChannelWaitSeconds' => route['source_pack_preflight_service_seconds'].nil? ? nil : route['source_pack_preflight_ssh_channel_wait_seconds'].to_f,
          'sourcePreflightSeconds' => route['source_pack_preflight_seconds'], 'logicalBytes' => route['logical_bytes'],
          'archiveBytes' => route['archive_bytes'], 'files' => route['files'],
          'workspaceBytes' => projection && projection.fetch('observedProducerWorkspaceBytes').fetch(route.fetch('producer_activity_id')) }
      end
    end
    grouped = phases.group_by { |p| p.values_at('strategy','sourceClass','targetClass','phase') }.map do |(strategy, source, target, phase), samples|
      { 'strategy' => strategy, 'sourceClass' => source, 'targetClass' => target, 'phase' => phase,
        'seconds' => summary(samples.map { |p| p['seconds'] }),
        'secondsBases' => samples.map { |p| p['secondsBasis'] }.uniq.sort,
        'pooledArchiveServiceRate' => pooled_service_rate(samples, 'archiveBytes'),
        'pooledWorkspaceServiceRate' => pooled_service_rate(samples, 'workspaceBytes'),
        'phaseChannelWaitSeconds' => summary(samples.map { |p| p['phaseChannelWaitSeconds'] }),
        'sourceChannelWaitSeconds' => phase == 'pack-and-receive' ? summary(samples.map { |p| p['sourceChannelWaitSeconds'] }) : nil,
        'sourcePreflightSeconds' => phase == 'pack-and-receive' ? summary(samples.map { |p| p['sourcePreflightSeconds'] }) : nil,
        'sourcePreflightServiceSeconds' => phase == 'pack-and-receive' ? summary(samples.map { |p| p['sourcePreflightServiceSeconds'] }) : nil,
        'sourcePreflightChannelWaitSeconds' => phase == 'pack-and-receive' ? summary(samples.map { |p| p['sourcePreflightChannelWaitSeconds'] }) : nil,
        'logicalBytes' => summary(samples.map { |p| p['logicalBytes'] }),
        'archiveBytes' => summary(samples.map { |p| p['archiveBytes'] }),
        'workspaceBytes' => summary(samples.map { |p| p['workspaceBytes'] }),
        'archiveToWorkspaceRatio' => summary(samples.map { |p| p['archiveBytes'].to_f / p['workspaceBytes'] if p['archiveBytes'] && p['workspaceBytes'] && p['workspaceBytes'] > 0 }.compact),
        'effectiveWorkspaceBytesPerSecond' => summary(samples.map { |p| p['workspaceBytes'].to_f / p['seconds'] if p['workspaceBytes'] && p['seconds'] > 0 }.compact),
        'files' => summary(samples.map { |p| p['files'] }),
        'effectiveArchiveBytesPerSecond' => summary(samples.map { |p| p['archiveBytes'].to_f / p['seconds'] if p['archiveBytes'] && p['seconds'] > 0 }.compact),
        'interpretation' => 'concurrency-conditioned effective phase rates; not isolated link bandwidth' }
    end
    merges = data.fetch('shared_preparations').group_by { |p| [family(p.fetch('consumer_activity_id')),resource_class(indexed.fetch(p.fetch('consumer_activity_id')).fetch('resource_id'))] }.map do |(program, machine), samples|
      { 'activityFamily' => program, 'resourceClass' => machine,
        'mergeSeconds' => summary(samples.map { |p| p.fetch('merge_seconds') }),
        'checkSeconds' => summary(samples.map { |p| p.fetch('merge_check_seconds') }),
        'copySeconds' => summary(samples.map { |p| p.fetch('merge_copy_seconds') }),
        'verifySeconds' => summary(samples.map { |p| p.fetch('merge_verify_seconds') }) }
    end
    merge_rates = if projection
      data.fetch('shared_preparations').group_by { |p| resource_class(indexed.fetch(p.fetch('consumer_activity_id')).fetch('resource_id')) }.map do |machine, samples|
        rates = {}
        {'checkSecondsPerInputByte'=>['merge_check_seconds','inputTotalBytes'],
          'copySecondsPerUniqueByte'=>['merge_copy_seconds','inputUniqueBytes'],
          'verifySecondsPerUniqueByte'=>['merge_verify_seconds','inputUniqueBytes']}.each do |key,(seconds_key,bytes_key)|
          rates[key] = summary(samples.map do |sample|
            bytes = projection.fetch('observedConsumerInputVolumes').fetch(sample.fetch('consumer_activity_id')).fetch(bytes_key)
            sample.fetch(seconds_key) / bytes if bytes > 0
          end.compact)
        end
        {'targetClass'=>machine,'source'=>"baseline:#{data.fetch('run_id')}",'observations'=>rates,
         'commandOverheadSeconds'=>summary(samples.map { |p| p['merge_command_overhead_seconds'] }),
         'channelWaitSeconds'=>summary(samples.map { |p| p['merge_ssh_channel_wait_seconds'] })}
      end
    end
    { 'schemaVersion' => 1, 'baselineRunId' => data.fetch('run_id'), 'baselineSHA256' => digest,
      'computeWorkBasis'=>projection && projection['computeWorkBasis'],
      'synchronizationContract' => 'whole-workspace-v1', 'workspaceVolumeBasis' => projection ? 'producer-final-snapshot-manifest' : 'unavailable-do-not-use-logical-output-bytes-as-workspace-volume', 'computeObservations' => compute,
      'materializationPhaseObservations' => grouped, 'consumerMergeObservations' => merges,
      'mergeRateObservations'=>merge_rates,
      'limitations' => ['Single baseline; ranges are observations, not confidence intervals.',
        'Do not replay per-activity durations or total makespan as a holdout prediction.',
        'No destination cache hit or shared-storage route may be assumed without evidence.',
        'Workspace manifests and topology-specific contention are required before applying these observations.',
        'Compute samples are contention-conditioned; do not apply interference slowdown a second time.'] }
  end
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: build_montage_execution_calibration.rb AUDITED_RESULTS_JSON OUTPUT_JSON [WORKSPACE_PROJECTION_JSON]' unless [2,3].include?(ARGV.length)
  payload = File.binread(ARGV[0])
  projection = JSON.parse(File.binread(ARGV[2])) if ARGV[2]
  result = MontageExecutionCalibration.build(JSON.parse(payload), Digest::SHA256.hexdigest(payload), projection)
  File.write(ARGV[1], JSON.pretty_generate(result) + "\n")
  puts "Calibration observations: #{result['computeObservations'].length} compute groups, #{result['materializationPhaseObservations'].length} route-phase groups."
end
