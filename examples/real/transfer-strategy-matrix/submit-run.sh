#!/bin/sh
set -eu

case_id=${1:?usage: submit-run.sh CASE_ID RUN_ID}
run_id=${2:?usage: submit-run.sh CASE_ID RUN_ID}
base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?API token required}

if [ "$case_id" = roundtrip ]; then
  workflow_id=transfer-matrix-roundtrip
else
  if ! awk -F '\t' -v wanted="$case_id" '$1 == wanted {found=1} END {exit !found}' "$base/cases.tsv"; then
    echo "unknown case: $case_id" >&2
    exit 2
  fi
  workflow_id="transfer-matrix-$case_id"
fi
plan_id="$workflow_id-plan"

get() {
  curl --fail-with-body -sS \
    -H "Authorization: Bearer $api_token" \
    "$api_base/$1/"
}

plan_json=$(get "schedule-plans/$plan_id")
definition_json=$(get "workflow-definitions/$workflow_id")
scope_json=$(get execution-scopes/transfer-matrix-scope-v1)
network_json=$(get network-topologies/transfer-matrix-network-v1)
local_environment_json=$(get environments/local-environment)
hpc_environment_json=$(get environments/hpc-environment)
cloud_environment_json=$(get environments/gcp-environment)
local_resource_json=$(get resources/local-environment-entrypoint)
hpc_entrypoint_json=$(get resources/hpc-environment-entrypoint)
hpc_partition_json=$(get resources/hpc-environment-ssh-connection-partition-testing)
cloud_resource_json=$(get resources/montage-gcp-e2-medium-1)

jq -nc \
  --arg runId "$run_id" \
  --argjson plan "$plan_json" \
  --argjson definition "$definition_json" \
  --argjson scope "$scope_json" \
  --argjson network "$network_json" \
  --argjson localEnvironment "$local_environment_json" \
  --argjson hpcEnvironment "$hpc_environment_json" \
  --argjson cloudEnvironment "$cloud_environment_json" \
  --argjson localResource "$local_resource_json" \
  --argjson hpcEntrypoint "$hpc_entrypoint_json" \
  --argjson hpcPartition "$hpc_partition_json" \
  --argjson cloudResource "$cloud_resource_json" \
  '{run:{id:$runId,schedulePlanId:$plan.id,mode:"real",status:"created"},
    plan:$plan,workflow:$definition.version,executionScope:$scope,networkTopology:$network,
    resources:[$localResource,$hpcEntrypoint,$hpcPartition,$cloudResource],
    runtimes:($localEnvironment.runtimes+$hpcEnvironment.runtimes+$cloudEnvironment.runtimes),
    runtimeBindings:($localEnvironment.resourceRuntimeBindings+$hpcEnvironment.resourceRuntimeBindings+$cloudEnvironment.resourceRuntimeBindings)}' |
  curl --fail-with-body -sS \
    -H "Authorization: Bearer $api_token" \
    -H 'Content-Type: application/json' \
    --data-binary @- \
    "$api_base/execution-runs/"
printf '\n'
