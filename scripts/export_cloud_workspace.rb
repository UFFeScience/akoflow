#!/usr/bin/env ruby
# AKO-121: export scientific workspace artifacts through the daemon API only.
require 'net/http'
require 'json'
require 'base64'
require 'digest'
require 'shellwords'
$stdout.sync = true

target, run, output = ARGV
abort 'usage: export_cloud_workspace.rb TARGET RUN OUTPUT.tar.gz' unless output && target.match?(/\Amontage-gcp-[a-z0-9-]+\z/) && run.match?(/\Amontage-[a-z0-9-]+\z/)
token = ENV.fetch('AKOFLOW_API_TOKEN')
endpoint = URI('http://127.0.0.1:8080/akoflow-api/console-commands/')
console = lambda do |command|
  request = Net::HTTP::Post.new(endpoint)
  request['Authorization'] = "Bearer #{token}"
  request['Content-Type'] = 'application/json'
  request.body = JSON.generate(resourceId: target, command: command, timeoutSeconds: 600)
  response = Net::HTTP.start(endpoint.hostname, endpoint.port, read_timeout: 660) { |http| http.request(request) }
  abort "console HTTP #{response.code}" unless response.is_a?(Net::HTTPSuccess)
  result = JSON.parse(response.body)
  abort 'remote archive command failed' unless result['status'] == 'completed' && result['exitCode'] == 0
  result.fetch('stdout')
end
source = "/akoflow/workspace/runs/#{run}"
archive = "/tmp/akoflow-preserve-#{run}.tar.gz"
metadata = console.call("set -eu; test -d #{source.shellescape}; tar -czf #{archive.shellescape} -C #{source.shellescape} .; stat -c %s #{archive.shellescape}; sha256sum #{archive.shellescape}")
lines = metadata.lines.map(&:strip)
size = Integer(lines.fetch(0))
hash = lines.fetch(1).split.first
abort 'invalid archive metadata' unless size.positive? && size <= 512 * 1024 * 1024 && hash.match?(/\A[0-9a-f]{64}\z/)
abort 'output already exists' if File.exist?(output) || File.exist?(output + '.partial')
chunk_size = 4 * 1024 * 1024
File.open(output + '.partial', 'wb', 0600) do |file|
  (0...size).step(chunk_size) do |offset|
    count = [chunk_size, size - offset].min
    encoded = console.call("dd if=#{archive.shellescape} bs=1M skip=#{offset / (1024 * 1024)} count=4 status=none | base64 -w0")
    decoded = Base64.strict_decode64(encoded.strip)
    abort 'archive chunk length mismatch' unless decoded.bytesize == count
    file.write(decoded)
    puts "#{target}: #{offset + count}/#{size} bytes preserved"
  end
end
abort 'archive checksum mismatch; partial retained' unless Digest::SHA256.file(output + '.partial').hexdigest == hash
File.rename(output + '.partial', output)
puts "VERIFIED #{target}: #{size} bytes SHA256 #{hash}"
