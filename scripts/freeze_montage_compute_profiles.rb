#!/usr/bin/env ruby
require 'json'
require 'digest'

module MontageComputeFreeze
  module_function

  def build(calibration, digest, resources, namespace)
    independent = calibration['computeWorkBasis'] == 'immutable-dataset-independent-work-v1'
    source = "#{calibration.fetch('baselineRunId')}:sha256:#{digest}"
    type_ids = calibration.fetch('computeObservations').map { |row| row.fetch('activityFamily') }.uniq.sort.to_h { |family| [family, "#{namespace}-#{family}"] }
    profiles = calibration.fetch('computeObservations').flat_map do |row|
      stats = row.fetch('wallSecondsPerReferenceSecond')
      raise 'Reference-normalized compute observations required' unless stats && stats.fetch('samples') > 0
      coefficient = Float(stats.fetch('median'))
      raise 'Invalid compute coefficient' unless coefficient.finite? && coefficient > 0
      resources.map do |id, resource|
        next if resource.fetch('class') != row.fetch('resourceClass')
        next if row['allowedResourceIds'] && !row.fetch('allowedResourceIds').include?(id)
        { 'id'=>"#{namespace}-#{row.fetch('activityFamily')}-#{id}",
          'activityTypeId'=>type_ids.fetch(row.fetch('activityFamily')), 'resourceId'=>id,
          'runtimeSeconds'=>coefficient, 'modelVersion'=>independent ? 'compute-work-v1' : 'reference-work-v1',
          'source'=>row.fetch('source',source), 'sampleSize'=>stats.fetch('samples'),
          'metadata'=>{'units'=>independent ? 'wall-seconds-per-independent-work-unit' : 'wall-seconds-per-reference-second',
            'observedRange'=>stats.values_at('minimum','maximum'),
            'includesPhysicalSharing'=>true, 'taskRuntimeFallbackSamples'=>row.fetch('taskRuntimeFallbackSamples')} }
      end.compact
    end
    { 'schemaVersion'=>1, 'source'=>source, 'activityFamilyTypeIds'=>type_ids,
      'activityProfiles'=>profiles, 'limitations'=>[
        'Class-level historical wall rates; no per-activity replay.',
        'Unobserved family/class pairs have no measured profile.',
        'Includes physical sharing: additional interference factors require separate evidence.'] }
  end
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: freeze_montage_compute_profiles.rb CALIBRATION_JSON RESOURCE_MODEL_JSON TYPE_NAMESPACE OUTPUT_JSON' unless ARGV.length == 4
  raw = File.binread(ARGV[0])
  result = MontageComputeFreeze.build(JSON.parse(raw), Digest::SHA256.hexdigest(raw), JSON.parse(File.binread(ARGV[1])), ARGV[2])
  File.write(ARGV[3], JSON.pretty_generate(result) + "\n")
  puts "Frozen #{result.fetch('activityProfiles').length} resource profiles in #{result.fetch('activityProfiles').map { |p| p.fetch('modelVersion') }.uniq.join(', ')} units."
end
