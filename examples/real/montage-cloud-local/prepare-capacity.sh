#!/bin/sh
set -eu

api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?AKOFLOW_API_TOKEN is required}
ssh_cidr=${AKOFLOW_SSH_CIDR:?Set AKOFLOW_SSH_CIDR to the approved public IPv4 CIDR}
environment_id=${AKOFLOW_GCP_ENVIRONMENT_ID:-gcp-environment}
target_url="$api_base/environments/$environment_id/cloud-capacity-targets/"

get() {
  curl --fail-with-body -sS -H "Authorization: Bearer $api_token" "$api_base/$1/"
}

environment=$(get "environments/$environment_id")
catalog=$(get "environments/$environment_id/cloud-catalog")
targets=$(get "environments/$environment_id/cloud-capacity-targets")
project=$(printf '%s' "$environment" | jq -er '.connections[0].configuration.projectId')
offering=$(printf '%s' "$catalog" | jq -ec '[.machines[] | select(.providerTypeId == "e2-medium" and .architecture == "amd64" and .available)] | first')
image=$(printf '%s' "$catalog" | jq -er '[.images[] | select(.architecture == "amd64" and (.name | test("^ubuntu-2404-noble-amd64-")))] | max_by(.name) | .providerImageId')
region=$(printf '%s' "$offering" | jq -er '.region')

for slot in 1 2 3 4; do
  target_id="montage-gcp-e2-medium-$slot"
  existing=$(printf '%s' "$targets" | jq -c --arg id "$target_id" '[.[]? | select(.id == $id)] | first // empty')
  if [ -n "$existing" ]; then
    printf '%s' "$existing" | jq -e --arg project "$project" --arg cidr "$ssh_cidr" \
      '.providerMachineType == "e2-medium" and .maximumInstances == 1 and
       .lifecyclePolicy == "destroy-after-run" and .configuration.projectId == $project and
       any(.configuration.sshSourceRanges[]?; . == $cidr)' >/dev/null || {
      printf 'Existing target %s has incompatible settings\n' "$target_id" >&2
      exit 1
    }
    printf '%s already exists\n' "$target_id"
    continue
  fi
  payload=$(printf '%s' "$offering" | jq -c \
    --arg id "$target_id" --arg name "Montage GCP e2-medium $slot" \
    --arg project "$project" --arg image "$image" --arg cidr "$ssh_cidr" \
    '{id:$id,name:$name,provider:.provider,providerMachineType:.providerTypeId,
      region:.region,imageReference:$image,architecture:.architecture,
      vcpu:.vcpu,memoryMiB:.memoryMiB,provisioningMode:"standard",
      maximumInstances:1,lifecyclePolicy:"destroy-after-run",
      configuration:{projectId:$project,diskType:"pd-balanced",diskSizeGiB:30,
        network:"default",sshSourceRanges:[$cidr]}}')
  printf '%s' "$payload" | curl --fail-with-body -sS \
    -H "Authorization: Bearer $api_token" -H 'Content-Type: application/json' \
    --data-binary @- "$target_url" >/dev/null
  printf 'Created %s (%s, %s)\n' "$target_id" "$(printf '%s' "$offering" | jq -r '.providerTypeId')" "$region"
done
