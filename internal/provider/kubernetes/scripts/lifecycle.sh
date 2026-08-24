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

"$@"
activity_exit_code=$?

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
