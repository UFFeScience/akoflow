#!/bin/sh
set -eu

api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?AKOFLOW_API_TOKEN is required}
target_url="$api_base/environments/goal-gcp/cloud-capacity-targets/"
targets=$(curl --fail-with-body -sS -H "Authorization: Bearer $api_token" "$target_url")
template=$(printf '%s' "$targets" | jq -c '.[] | select(.id == "goal-gcp-e2-medium")')
test -n "$template" || { echo 'Missing goal-gcp-e2-medium template target' >&2; exit 1; }

for slot in 1 2 3 4; do
  target_id="montage-gcp-e2-medium-$slot"
  if printf '%s' "$targets" | jq -e --arg id "$target_id" 'any(.[]; .id == $id)' >/dev/null; then
    printf '%s already exists\n' "$target_id"
    continue
  fi
  payload=$(printf '%s' "$template" | jq -c --arg id "$target_id" --arg name "Montage GCP e2-medium $slot" \
    'del(.createdAt) | .id=$id | .name=$name | .maximumInstances=1 | .lifecyclePolicy="destroy-after-run"')
  printf '%s' "$payload" | curl --fail-with-body -sS -H "Authorization: Bearer $api_token" \
    -H 'Content-Type: application/json' --data-binary @- "$target_url" >/dev/null
  printf 'Created %s\n' "$target_id"
done
