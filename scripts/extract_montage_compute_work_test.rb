require 'minitest/autorun'
require 'stringio'
require_relative 'extract_montage_compute_work'

class MontageComputeWorkTest < Minitest::Test
  def test_header_dimensions_define_work_not_compressed_size_or_old_time
    cards = ['SIMPLE  = T','BITPIX  = 16','NAXIS   = 2','NAXIS1  = 2531','NAXIS2  = 2536','END'].map { |c| c.ljust(80) }.join
    feature = MontageComputeWork.input_feature(MontageComputeWork.fits_header(StringIO.new(cards)))
    assert_in_delta 6.418616, feature.fetch('computeWorkUnits'), 1e-12
    assert_equal 'input-megapixels', feature.fetch('unit')
    assert_equal 16, feature.fetch('bitpix')
    refute feature.key?('runtimeSeconds')
  end

  def test_bad_headers_fail_closed
    assert_raises(RuntimeError) { MontageComputeWork.fits_header(StringIO.new('short')) }
    assert_raises(RuntimeError) { MontageComputeWork.input_feature({'NAXIS'=>3}) }
    assert_raises(RuntimeError) { MontageComputeWork.input_feature({'NAXIS'=>2,'NAXIS1'=>0,'NAXIS2'=>1}) }
  end
end
