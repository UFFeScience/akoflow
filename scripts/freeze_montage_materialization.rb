#!/usr/bin/env ruby
require 'json'
require 'digest'

# Freeze class-level coefficients from audited historical measurements. No
# per-activity observed time or future execution trace is used as a prediction.
module MontageMaterializationFreeze
  module_function

  def median(row, key, positive: false)
    stats = row.fetch(key)
    raise "Missing measured #{key}" unless stats && stats.fetch('samples') > 0
    value = Float(stats.fetch('median'))
    raise "Invalid measured #{key}" unless value.finite? && value >= 0 && (!positive || value > 0)
    value
  end

  def seconds_per_byte(row)
    1.0 / median(row, 'effectiveWorkspaceBytesPerSecond', positive: true)
  end

  def build(calibration, calibration_digest, resources, relay_id)
    raise 'Manifest-based workspace volumes required' unless calibration.fetch('workspaceVolumeBasis') == 'producer-final-snapshot-manifest'
    raise 'Explicit relay resource required' unless resources.fetch(relay_id).fetch('domain') == 'relay'
    source = "#{calibration.fetch('baselineRunId')}:sha256:#{calibration_digest}"
    profiles = calibration.fetch('materializationPhaseObservations').group_by { |row| row.values_at('strategy', 'sourceClass', 'targetClass') }.map do |(strategy, from, to), rows|
      phases = rows.to_h { |row| [row.fetch('phase'), row] }
      raise 'Unsupported measured strategy' unless %w[gateway direct-runtime].include?(strategy)
      upload = phases.fetch(strategy == 'gateway' ? 'archive-upload' : 'peer-private-upload')
      target_pool = resources.values.any? { |r| r.fetch('class') == to && !r.fetch('bulkServiceGroupId', '').empty? }
      if target_pool
        receiver = phases.fetch(strategy == 'gateway' ? 'receiver-verify-and-extract' : 'peer-receiver-verify-and-extract')
        unless receiver.fetch('secondsBases', []) == ['phase-command-execution-excludes-channel-queue']
          raise "Receiver #{from}->#{to} requires queue-free command service before adding a bulk pool"
        end
      end
      profile = { 'strategy'=>strategy, 'sourceClass'=>from, 'targetClass'=>to, 'source'=>source,
        'samples'=>upload.fetch('seconds').fetch('samples'),
        'archiveToWorkspaceRatio'=>median(upload, 'archiveToWorkspaceRatio', positive: true),
        'uploadArchiveBytesPerSecond'=>median(upload, 'effectiveArchiveBytesPerSecond', positive: true) }
      if strategy == 'gateway'
        pack = phases.fetch('pack-and-receive')
        source_pool = resources.values.any? { |r| r.fetch('class') == from && !r.fetch('bulkServiceGroupId', '').empty? }
        if source_pool && pack.fetch('secondsBases', []) != ['source-command-execution-excludes-channel-queue']
          raise "Gateway #{from}->#{to} requires queue-free source service measurements before adding a bulk pool"
        end
        if source_pool && !pack['sourcePreflightServiceSeconds']
          raise "Gateway #{from}->#{to} requires separately measured preflight service"
        end
        profile['ingressArchiveBytesPerSecond'] = median(pack, 'effectiveArchiveBytesPerSecond', positive: true)
        if pack['sourcePreflightServiceSeconds']
          profile['probeSeconds'] = median(pack, 'sourcePreflightServiceSeconds')
        end
        profile['ingressIncludesSourcePreparation'] = true
        profile['validationSecondsPerByte'] = seconds_per_byte(phases.fetch('archive-validation'))
        profile['receiverSecondsPerByte'] = seconds_per_byte(phases.fetch('receiver-verify-and-extract'))
      else
        profile['probeSeconds'] = median(phases.fetch('peer-authenticated-connectivity-probe'), 'seconds')
        profile['snapshotSecondsPerByte'] = seconds_per_byte(phases.fetch('peer-snapshot-acquire'))
        profile['receiverSecondsPerByte'] = seconds_per_byte(phases.fetch('peer-receiver-verify-and-extract'))
      end
      profile
    end
    merges = calibration.fetch('mergeRateObservations').map do |row|
      rates = row.fetch('observations')
      profile = { 'targetClass'=>row.fetch('targetClass'), 'source'=>source,
        'samples'=>rates.fetch('checkSecondsPerInputByte').fetch('samples') }.merge(
          rates.to_h { |key, _| [key, median(rates, key)] })
      # Execution residual excludes CHECK/COPY/VERIFY and SSH admission queue.
      # Charge it once per consumer, not once per predecessor or per byte.
      profile['commandOverheadSeconds'] = median(row, 'commandOverheadSeconds') if row['commandOverheadSeconds']
      profile
    end
    { 'schemaVersion'=>1, 'source'=>source, 'relayResourceId'=>relay_id,
      'resources'=>resources, 'profiles'=>profiles, 'mergeProfiles'=>merges }
  end
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: freeze_montage_materialization.rb CALIBRATION_JSON RESOURCE_MODEL_JSON RELAY_ID OUTPUT_JSON' unless ARGV.length == 4
  raw = File.binread(ARGV[0])
  result = MontageMaterializationFreeze.build(JSON.parse(raw), Digest::SHA256.hexdigest(raw), JSON.parse(File.binread(ARGV[1])), ARGV[2])
  File.write(ARGV[3], JSON.pretty_generate(result) + "\n")
  puts "Frozen #{result.fetch('profiles').length} measured route profiles; unobserved combinations remain unavailable."
end
