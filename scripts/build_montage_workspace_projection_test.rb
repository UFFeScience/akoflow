require 'minitest/autorun'
require_relative 'build_montage_workspace_projection'

class MontageWorkspaceProjectionTest < Minitest::Test
  def file(path, size, identity)
    {'path'=>path, 'sizeBytes'=>size, 'contentId'=>identity}
  end

  def test_nonroot_runtime_files_follow_resource_type_not_historical_outputs
    seed = file('runtime.hdr', 10, 'immutable-image-header')
    input = file('parent.fits', 100, 'parent')
    output = file('difference.fits', 50, 'difference')
    model = MontageWorkspaceProjection.activity_model({'parent.fits'=>input},
      {'parent.fits'=>input,'runtime.hdr'=>seed,'difference.fits'=>output}, {'runtime.hdr'=>seed}, 'audited')
    assert_equal [seed], model.fetch('inputsByResourceType').fetch('cloud_vm')
    assert_empty model.fetch('inputsByResourceType').fetch('hpc_machine')
    assert_equal [output], model.fetch('outputs')
    # Reassigning this non-root to HPC must not manufacture cloud packaging.
    hpc = {'parent.fits'=>input}
    model.fetch('outputs').each { |entry| hpc[entry.fetch('path')] = entry }
    refute hpc.key?('runtime.hdr')
  end

  def test_modified_packaging_path_is_not_discarded_as_runtime_input
    seed = file('runtime.hdr', 10, 'original')
    changed = file('runtime.hdr', 20, 'consumer-modified')
    model = MontageWorkspaceProjection.activity_model({}, {'runtime.hdr'=>changed}, {'runtime.hdr'=>seed}, 'audited')
    assert_equal [changed], model.fetch('outputs')
  end

  def test_inherited_packaging_is_preserved_when_consumer_moves_to_hpc
    seed = file('runtime.hdr', 10, 'immutable')
    model = MontageWorkspaceProjection.activity_model({'runtime.hdr'=>seed}, {'runtime.hdr'=>seed}, {'runtime.hdr'=>seed}, 'audited')
    assert_empty model.fetch('removedPaths')
    assert_empty model.fetch('outputs')
  end
end
