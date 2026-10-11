#!/usr/bin/env ruby
require 'json'
require 'digest'
require_relative 'build_montage_execution_calibration'

# Explicitly scoped training observations from completed roots of a failed run.
# They do not qualify that run as a successful end-to-end validation.
module MontageRootCompute
  module_function
  def build(calibration, projection, request, response, digest)
    raise 'Independent compute units required' unless calibration.fetch('computeWorkBasis') == 'immutable-dataset-independent-work-v1'
    raise 'Expected failed training experiment' unless response.fetch('run').fetch('status') == 'failed'
    work = projection.fetch('activityComputeWork').values.to_h { |f| [f.fetch('activityName'), f] }
    roots = request.fetch('workflow').fetch('activities').select { |a| a.fetch('name').start_with?('mprojectid') }
    raise 'Need twelve Montage roots' unless roots.length == 12
    assignments = request.fetch('plan').fetch('assignments').to_h { |a| [a.fetch('activityId'),a.fetch('resourceId')] }
    samples = roots.map do |activity|
      id = activity.fetch('id')
      attempts = response.fetch('activityAttempts').select { |t| t.fetch('activityId') == id }
      raise 'Root is not a successful single attempt' unless attempts.length == 1 && attempts.first.fetch('attempt') == 1 && attempts.first.fetch('status') == 'completed'
      handles = response.fetch('handles').select { |h| h.fetch('activityId') == id }
      raise 'Missing exact successful root handle' unless handles.length == 1 && handles.first.fetch('status') == 'completed' && handles.first.fetch('exitCode') == 0 && handles.first.fetch('id') == response.fetch('run').fetch('id')+':'+id
      raise 'Missing physical root output snapshot' unless handles.first.fetch('artifacts').fetch('finalSnapshot').length >= 2
      resource = assignments.fetch(id)
      raise 'Training roots must preserve Bora resource family' unless resource.include?('bora')
      seconds = Float(attempts.first.fetch('runtimeSeconds'))
      raise 'Invalid observed root runtime' unless seconds.finite? && seconds > 0
      feature = work.fetch(activity.fetch('name'))
      {'activityFamily'=>feature.fetch('activityFamily'),'resourceId'=>resource,
        'runtimeSeconds'=>seconds,'secondsPerWorkUnit'=>seconds/feature.fetch('computeWorkUnits')}
    end
    result = Marshal.load(Marshal.dump(calibration))
    source = response.fetch('run').fetch('id')+':sha256:'+digest
    result['computeSupplementSource'] = source
    # Keep the original pooled HPC observation for Diablo only. New root
    # observations are restricted to the actually exercised Bora resource.
    result.fetch('computeObservations').each do |row|
      if row.fetch('activityFamily').start_with?('mproject_') && row.fetch('resourceClass') == 'hpc'
        row['allowedResourceIds'] = ['plafrim-execution-ssh-node-diablo03']
      end
    end
    samples.group_by { |s| [s.fetch('activityFamily'),s.fetch('resourceId')] }.each do |(family, resource), group|
      result.fetch('computeObservations') << {'activityFamily'=>family,'resourceClass'=>'hpc',
        'allowedResourceIds'=>[resource],'source'=>source,'taskRuntimeFallbackSamples'=>group.length,
        'runtimeSeconds'=>MontageExecutionCalibration.summary(group.map { |s| s.fetch('runtimeSeconds') }),
        'wallSecondsPerReferenceSecond'=>MontageExecutionCalibration.summary(group.map { |s| s.fetch('secondsPerWorkUnit') }),
        'interpretation'=>'Completed single-attempt root observations only; source run failed later and is not end-to-end validation.'}
    end
    result
  end
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: supplement_montage_root_compute.rb CALIBRATION PROJECTION PRIVATE_REQUEST PRIVATE_RESPONSE OUTPUT' unless ARGV.length == 5
  calibration, projection, request = ARGV.first(3).map { |p| JSON.parse(File.binread(p)) }
  raw = File.binread(ARGV[3])
  result = MontageRootCompute.build(calibration,projection,request,JSON.parse(raw),Digest::SHA256.hexdigest(raw))
  File.write(ARGV[4],JSON.pretty_generate(result)+"\n")
  puts 'Added twelve verified root training observations; no end-to-end success claimed.'
end
