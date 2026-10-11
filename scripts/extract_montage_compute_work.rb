#!/usr/bin/env ruby
require 'json'
require 'open3'
require 'zlib'
require 'stringio'
require 'digest'

# Read immutable input headers, not execution durations. SIF extraction is
# architecture independent and never starts a workload or modifies the image.
module MontageComputeWork
  module_function

  def fits_header(reader)
    values = {}
    16_384.times do
      card = reader.read(80)
      raise 'Truncated FITS header' unless card && card.bytesize == 80
      key = card[0,8].strip
      return values if key == 'END'
      if %w[BITPIX NAXIS NAXIS1 NAXIS2].include?(key)
        raise 'Invalid FITS numeric card' unless card[8,2] == '= '
        values[key] = Integer(card[10..].split('/').first.strip)
      end
    end
    raise 'FITS header exceeds bounded input limit'
  end

  def input_feature(header)
    raise 'Expected two-dimensional FITS input' unless header.fetch('NAXIS') == 2
    x, y = header.values_at('NAXIS1','NAXIS2')
    raise 'Invalid image dimensions' unless x && y && x > 0 && y > 0
    {'unit'=>'input-megapixels','computeWorkUnits'=>x*y/1_000_000.0,
      'naxis1'=>x,'naxis2'=>y,'bitpix'=>header.fetch('BITPIX')}
  end
end

if $PROGRAM_NAME == __FILE__
  abort 'Usage: extract_montage_compute_work.rb CONTAINER SIF_PATH PROJECTION_JSON OUTPUT_JSON' unless ARGV.length == 4
  container, image, projection_path, output = ARGV
  listing, status = Open3.capture2('docker','exec',container,'apptainer','sif','list',image)
  raise 'SIF inventory failed' unless status.success?
  row = listing.lines.find { |line| line.include?('FS (Squashfs/') }
  raise 'Missing Squashfs partition' unless row
  offset = Integer(row.split('|')[3].strip.split('-').first)
  digest, status = Open3.capture2('docker','exec',container,'sha256sum',image)
  raise 'Image digest failed' unless status.success?
  digest = digest.split.first
  raise 'Invalid source image identity' unless digest.match?(/\A[0-9a-f]{64}\z/)
  projection = JSON.parse(File.binread(projection_path))
  packaging = projection.fetch('activityModels').values.first.fetch('inputs')
  inputs = packaging.select { |f| f.fetch('path').match?(/\Aposs2ukstu_(blue|red|ir)_\d{3}_\d{3}\.fits\z/) }
  raise 'Expected twelve immutable input images' unless inputs.length == 12
  features = inputs.sort_by { |f| f.fetch('path') }.map do |file|
    header = nil
    Open3.popen3('docker','exec',container,'unsquashfs','-o',offset.to_s,'-cat',image,'akoflow-wfa-shared/'+file.fetch('path')) do |stdin, stdout, stderr, wait|
      stdin.close
      # Drain the whole compressed stream, then read only the scientific header.
      compressed = stdout.read
      raise 'Input content identity differs from audited dataset' unless 'sha256:'+Digest::SHA256.hexdigest(compressed) == file.fetch('contentId')
      diagnostics = stderr.read
      raise "Scientific input extraction failed: #{diagnostics}" unless wait.value.success?
      header = Zlib::GzipReader.wrap(StringIO.new(compressed)) { |reader| MontageComputeWork.fits_header(reader) }
    end
    {'path'=>file.fetch('path'),'contentId'=>file.fetch('contentId'),
      'compressedBytes'=>file.fetch('sizeBytes'),'band'=>file.fetch('path').split('_')[1]}.merge(MontageComputeWork.input_feature(header))
  end
  File.write(output, JSON.pretty_generate({'schemaVersion'=>1,'sourceImageSHA256'=>digest,
    'basis'=>'immutable-input-FITS-header','inputs'=>features,
    'limitations'=>['Pixel count is input work, not measured CPU service demand; band and hardware may need separate calibrated coefficients.']})+"\n")
  puts "Extracted #{features.length} independent input-work records. No workload executed."
end
