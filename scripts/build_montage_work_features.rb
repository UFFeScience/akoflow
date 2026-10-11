#!/usr/bin/env ruby
require 'json'
require 'shellwords'

module MontageWorkFeatures
  module_function
  def build(request, inputs)
    by_path = inputs.fetch('inputs').to_h { |f| [f.fetch('path'),f] }
    dependencies = request.fetch('workflow').fetch('dependencies').group_by { |d| d.fetch('activityId') }
    request.fetch('workflow').fetch('activities').to_h do |activity|
      family = activity.fetch('id').match(/-([a-z]+)id\d+$/)&.captures&.first
      raise 'Unknown Montage family' unless %w[mproject mdifffit mconcatfit mbgmodel mbackground mimgtbl madd mviewer].include?(family)
      words = Shellwords.split(activity.fetch('command').fetch('arguments').last)
      unit, work = 'fixed-dataset-operation', 1.0
      if family == 'mproject'
        source = words.select { |word| by_path.key?(word) }
        raise 'Ambiguous mProject input work' unless source.length == 1
        feature = by_path.fetch(source.first)
        family += '_'+feature.fetch('band')
        unit, work = feature.values_at('unit','computeWorkUnits')
      elsif family == 'mconcatfit'
        unit, work = 'predecessor-fit-record', dependencies.fetch(activity.fetch('id')).length.to_f
      elsif family == 'mviewer'
        channels = words.count { |word| %w[-red -green -blue -gray].include?(word) }
        raise 'Missing viewer channel work' unless [1,3].include?(channels)
        unit, work = 'image-channel', channels.to_f
      end
      raise 'Invalid independent compute work' unless work && work.finite? && work > 0
      [activity.fetch('id'), {'activityName'=>activity.fetch('name'),'activityFamily'=>family,
        'computeWorkUnits'=>work,'unit'=>unit,'sourceImageSHA256'=>inputs.fetch('sourceImageSHA256')}]
    end
  end
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: build_montage_work_features.rb REQUEST_JSON INPUT_FEATURES_JSON PROJECTION_JSON OUTPUT_JSON' unless ARGV.length == 4
  request, inputs, projection = ARGV.first(3).map { |p| JSON.parse(File.binread(p)) }
  projection['activityComputeWork'] = MontageWorkFeatures.build(request, inputs)
  projection['computeWorkBasis'] = 'immutable-dataset-independent-work-v1'
  projection.fetch('limitations') << 'Non-project operation units apply only to this fixed dataset; arbitrary dataset scaling is not calibrated.'
  File.write(ARGV[3], JSON.pretty_generate(projection)+"\n")
  puts "Prepared #{projection.fetch('activityComputeWork').length} independent work features."
end
