#!/bin/sh
set -eu

run_id=${1:?usage: submit-run.sh RUN_ID}
api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?AKOFLOW_API_TOKEN is required}

get() {
  curl --fail-with-body -sS -H "Authorization: Bearer $api_token" "$api_base/$1/"
}

plan_json=$(get schedule-plans/montage-58-cloud-local-manual-v1)
definition_json=$(get workflow-definitions/montage-58-cloud-local)
scope_json=$(get execution-scopes/montage-58-cloud-local-scope-v1)
network_json=$(get network-topologies/montage-58-cloud-local-network-v1)
local_environment_json=$(get environments/local-environment)
cloud_environment_json=$(get environments/goal-gcp)
local_resource_json=$(get resources/local-environment-entrypoint)
cloud_resource_json=$(get resources/goal-gcp-e2-medium)

jq -nc \
  --arg runId "$run_id" \
  --argjson plan "$plan_json" \
  --argjson definition "$definition_json" \
  --argjson scope "$scope_json" \
  --argjson network "$network_json" \
  --argjson localEnvironment "$local_environment_json" \
  --argjson cloudEnvironment "$cloud_environment_json" \
  --argjson localResource "$local_resource_json" \
  --argjson cloudResource "$cloud_resource_json" \
  '{run:{id:$runId,schedulePlanId:$plan.id,mode:"real",status:"created"},
    plan:$plan,workflow:$definition.version,executionScope:$scope,networkTopology:$network,
    resources:[$localResource,$cloudResource],
    runtimes:($localEnvironment.runtimes+$cloudEnvironment.runtimes),
    runtimeBindings:($localEnvironment.resourceRuntimeBindings+$cloudEnvironment.resourceRuntimeBindings)}' |
  curl --fail-with-body -sS -H "Authorization: Bearer $api_token" \
    -H 'Content-Type: application/json' --data-binary @- "$api_base/execution-runs/"
