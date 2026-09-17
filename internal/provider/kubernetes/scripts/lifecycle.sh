root=${AKOFLOW_OBSERVATION_ROOT:-/tmp/akoflow/workspace}
state_dir=/tmp/akoflow-lifecycle-$$
before=$state_dir/before.tsv
after=$state_dir/after.tsv
changes=$state_dir/changes.tsv
max_files=${AKOFLOW_MAX_ARTIFACTS:-10000}

cleanup() { rm -rf "$state_dir"; }
trap cleanup EXIT HUP INT TERM

if ! mkdir -p "$root" "$state_dir" || ! cd "$root"; then
  printf 'AKOFLOW_LIFECYCLE_ERROR=cannot prepare activity workspace: %s\n' "$root" >&2
  exit 125
fi

snapshot() {
  destination=$1
  printf '#\t0\t-\n' > "$destination"
  find "$root" -type f -exec sh -c '
    root=$1
    destination=$2
    shift 2
    for file do
      relative=${file#"$root"/}
      size=$(wc -c < "$file" 2>/dev/null) || continue
      checksum=$(sha256sum "$file" 2>/dev/null | awk "{print \$1}") || continue
      printf "%s\t%s\t%s\n" "$relative" "$size" "$checksum" >> "$destination"
    done
  ' snapshot "$root" "$destination" {} + 2>/dev/null
  sort -o "$destination" "$destination"
}

started_at=$(date +%s 2>/dev/null || printf '0')
snapshot "$before" || printf 'AKOFLOW_OBSERVATION_WARNING=initial snapshot failed\n' >&2

# A private cgroup gives activity-level counters without a monitoring API.
# The file stays outside the observed workspace and is published only after
# the command exits, so a short-lived pod cannot lose its final measurements.
metrics=$state_dir/metrics.tsv
metric_cgroup=$(awk -F: '$1=="0" {print $3; exit}' /proc/self/cgroup 2>/dev/null || true)
metric_root="/sys/fs/cgroup${metric_cgroup}"
metric_pid=''
metric_epoch_anchor=$(date +%s)
metric_uptime_anchor=$(awk '{print $1}' /proc/uptime)
metric_interval=${AKOFLOW_METRIC_INTERVAL_SECONDS:-10}
case "$metric_interval" in ''|*[!0-9]*) metric_interval=10 ;; esac
if [ "$metric_interval" -lt 5 ] || [ "$metric_interval" -gt 60 ]; then metric_interval=10; fi
sample_metrics() {
  metric_uptime_now=$(awk '{print $1}' /proc/uptime)
  observed_at=$(awk -v epoch="$metric_epoch_anchor" -v anchor="$metric_uptime_anchor" -v current="$metric_uptime_now" 'BEGIN {printf "%.3f", epoch+current-anchor}')
  cpu_seconds=$(awk '$1=="usage_usec" {printf "%.6f", $2/1000000}' "$metric_root/cpu.stat" 2>/dev/null)
  memory_bytes=$(cat "$metric_root/memory.current" 2>/dev/null)
  disk_bytes=$(awk '{for (i=2;i<=NF;i++) {if ($i ~ /^rbytes=/) {split($i,a,"="); r+=a[2]} if ($i ~ /^wbytes=/) {split($i,a,"="); w+=a[2]}}} END {printf "%.0f %.0f",r,w}' "$metric_root/io.stat" 2>/dev/null)
  set -- $disk_bytes
  if [ -n "$cpu_seconds" ] && [ -n "$memory_bytes" ]; then
    metric=$(printf '%s\t%s\t%s\t%s\t%s' "$observed_at" "$cpu_seconds" "$memory_bytes" "${1:-0}" "${2:-0}")
    printf '%s\n' "$metric" >> "$metrics"
    printf 'AKOFLOW_ACTIVITY_METRIC=%s\n' "$metric"
  fi
}
if [ -f "$metric_root/cpu.stat" ] && [ -f "$metric_root/memory.current" ]; then
  sample_metrics
  (
    while :; do
      sleep "$metric_interval"
      sample_metrics
    done
  ) 2>/dev/null &
  metric_pid=$!
fi

"$@"
activity_exit_code=$?

if [ -n "$metric_pid" ]; then
  kill "$metric_pid" 2>/dev/null || true
  wait "$metric_pid" 2>/dev/null || true
  sample_metrics
fi

finished_at=$(date +%s 2>/dev/null || printf '0')
snapshot "$after" || printf 'AKOFLOW_OBSERVATION_WARNING=final snapshot failed\n' >&2

awk -F '\t' '
  NR == FNR { if ($1 != "#") { beforeChecksum[$1]=$3; beforeSize[$1]=$2 }; next }
  {
    if ($1 == "#") next
    seen[$1]=1
    if (!($1 in beforeChecksum)) print "created\t" $2 "\tsha256:" $3 "\t" $1
    else if (beforeChecksum[$1] != $3 || beforeSize[$1] != $2)
      print "modified\t" $2 "\tsha256:" $3 "\t" $1
  }
  END {
    for (file in beforeChecksum)
      if (!(file in seen)) print "deleted\t0\tsha256:" beforeChecksum[file] "\t" file
  }
' "$before" "$after" | sort | head -n "$max_files" > "$changes"

initial_files=$(awk -F '\t' '$1!="#" {count++} END {print count+0}' "$before")
final_files=$(awk -F '\t' '$1!="#" {count++} END {print count+0}' "$after")
created_files=$(awk -F '\t' '$1=="created" {count++} END {print count+0}' "$changes")
modified_files=$(awk -F '\t' '$1=="modified" {count++} END {print count+0}' "$changes")
deleted_files=$(awk -F '\t' '$1=="deleted" {count++} END {print count+0}' "$changes")
output_bytes=$(awk -F '\t' '$1!="deleted" {bytes+=$2} END {print bytes+0}' "$changes")
duration=$((finished_at >= started_at ? finished_at - started_at : 0))

files_json='['
separator=''
tab=$(printf '\t')
while IFS="$tab" read -r change size checksum relative; do
  [ -n "$relative" ] || continue
  escaped_path=$(printf '%s' "$relative" | sed 's/\\/\\\\/g; s/"/\\"/g')
  files_json=$files_json$separator'{"path":"'$escaped_path'","change":"'$change'","sizeBytes":'$size',"checksum":"'$checksum'"}'
  separator=','
done < "$changes"
files_json=$files_json']'

manifest_format='{"schemaVersion":1,"runId":"%s","activityId":"%s","attempt":1,'
manifest_format=$manifest_format'"runtime":"kubernetes","root":"%s","startedAt":%s,'
manifest_format=$manifest_format'"finishedAt":%s,"exitCode":%s,"files":%s,"phases":['
manifest_format=$manifest_format'{"phase":"execution","status":"%s","startedAt":%s,'
manifest_format=$manifest_format'"finishedAt":%s,"durationSeconds":%s}],"summary":{'
manifest_format=$manifest_format'"initialFiles":%s,"finalFiles":%s,"createdFiles":%s,'
manifest_format=$manifest_format'"modifiedFiles":%s,"deletedFiles":%s,"outputBytes":%s}}'
manifest=$(printf "$manifest_format" \
  "$AKOFLOW_RUN_ID" "$AKOFLOW_ACTIVITY_ID" "$root" "$started_at" "$finished_at" \
  "$activity_exit_code" "$files_json" \
  "$(if [ "$activity_exit_code" -eq 0 ]; then printf completed; else printf failed; fi)" \
  "$started_at" "$finished_at" "$duration" "$initial_files" "$final_files" \
  "$created_files" "$modified_files" "$deleted_files" "$output_bytes")

if command -v base64 >/dev/null 2>&1; then
  encoded_manifest=$(printf '%s' "$manifest" | base64 | tr -d '\n')
  printf '\n__AKOFLOW_MANIFEST_PREFIX__%s\n' "$encoded_manifest"
else
  printf '\nAKOFLOW_OBSERVATION_ERROR=base64 utility is unavailable; activity result was preserved\n' >&2
fi

exit "$activity_exit_code"
