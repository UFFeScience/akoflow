#!/bin/sh
set -eu

api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?API token required}
image=docker.io/library/busybox:1.36

request() {
  method=$1
  path=$2
  shift 2
  curl --fail-with-body -sS -X "$method" \
    -H "Authorization: Bearer $api_token" \
    "$@" "$api_base/$path"
}

artifact_ready() {
  request GET 'artifacts/?selectable=true' |
    jq -e '(.items? // .)[]? | select(.id == "busybox" and .version == "1.36" and .available == true)' \
      >/dev/null
}

if artifact_ready; then
  echo 'BusyBox 1.36 SIF is already available.'
  exit 0
fi

builds=$(request GET 'artifacts/busybox/builds/')
build_id=$(printf '%s' "$builds" |
  jq -r --arg image "$image" \
    '(.items? // .)[]? | select(.sourceType == "docker-image" and .recipePath == $image and .targetArchitecture == "amd64") | .id' |
  tail -n 1)

if [ -z "$build_id" ]; then
  registration=$(request POST 'artifacts/docker/' \
    -H 'Content-Type: application/json' \
    --data-binary '{"artifactId":"busybox","version":"1.36","image":"docker.io/library/busybox:1.36","architecture":"amd64"}')
  build_id=$(printf '%s' "$registration" | jq -er '.build.id')
fi

runs=$(request GET "artifact-builds/$build_id/runs/")
run_id=$(printf '%s' "$runs" |
  jq -r '(.items? // .)[]? | select(.status == "queued" or .status == "preparing" or .status == "building" or .status == "verifying" or .status == "publishing") | .id' |
  tail -n 1)

if [ -z "$run_id" ]; then
  started=$(request POST "artifact-builds/$build_id/runs/")
  run_id=$(printf '%s' "$started" | jq -er '.id')
fi

attempt=0
while [ "$attempt" -lt 120 ]; do
  run=$(request GET "build-runs/$run_id/")
  status=$(printf '%s' "$run" | jq -r '.status')
  case "$status" in
    completed)
      artifact_ready
      echo "BusyBox 1.36 SIF is available from build run $run_id."
      exit 0
      ;;
    failed | cancelled)
      printf '%s\n' "$run" | jq '{id,status,error,logs}' >&2
      exit 1
      ;;
  esac
  attempt=$((attempt + 1))
  sleep 5
done

echo "timed out waiting for BusyBox build run $run_id" >&2
exit 1
