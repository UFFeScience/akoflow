#!/bin/sh
set -eu

base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?AKOFLOW_API_TOKEN is required}
artifact_id=$(jq -er '.artifactId' "$base/artifact.json")
image=$(jq -er '.image' "$base/artifact.json")

get() {
  curl --fail-with-body -sS -H "Authorization: Bearer $api_token" "$api_base/$1/"
}

build=$(get "artifacts/$artifact_id/builds" | jq -c --arg image "$image" '[.[] | select(.recipePath == $image)] | first // empty')
if [ -z "$build" ]; then
  registered=$(curl --fail-with-body -sS -H "Authorization: Bearer $api_token" \
    -H 'Content-Type: application/json' --data-binary "@$base/artifact.json" \
    "$api_base/artifacts/docker/")
  build=$(printf '%s' "$registered" | jq -c '.build')
fi
build_id=$(printf '%s' "$build" | jq -er '.id')
run=$(get "artifact-builds/$build_id/runs" | jq -c '[.[] | select(.status == "completed" or .status == "queued" or .status == "running")] | first // empty')
if [ -z "$run" ]; then
  run=$(curl --fail-with-body -sS -X POST -H "Authorization: Bearer $api_token" \
    "$api_base/artifact-builds/$build_id/runs/")
fi
printf '%s\n' "$run" | jq '{id,artifactBuildId,status,error}'
