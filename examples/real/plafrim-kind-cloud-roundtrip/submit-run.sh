#!/bin/sh
set -eu

run_id=${1:?usage: submit-run.sh RUN_ID}
api_base=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
api_token=${AKOFLOW_API_TOKEN:?API token required}

get() {
  curl --fail-with-body -sS -H "Authorization: Bearer $api_token" "$api_base/$1/"
}

plan=$(get schedule-plans/plafrim-kind-cloud-roundtrip-plan-v2)
definition=$(get workflow-definitions/plafrim-kind-cloud-roundtrip-v2)
scope=$(get execution-scopes/plafrim-kind-cloud-roundtrip-scope-v1)
network=$(get network-topologies/plafrim-kind-cloud-roundtrip-network-v1)
plafrim=$(get environments/hpc-environment)
kind=$(get environments/kind-akoflow)
cloud=$(get environments/goal-gcp-worker)
plafrim_resource=$(get resources/hpc-environment-ssh-connection-partition-testing)
plafrim_entrypoint=$(get resources/hpc-environment-entrypoint)
kind_resource=$(get resources/kind-worker)
cloud_resource=$(get resources/goal-gcp-worker-e2-small)

jq -nc --arg runId "$run_id" \
  --argjson plan "$plan" --argjson definition "$definition" \
  --argjson scope "$scope" --argjson network "$network" \
  --argjson plafrim "$plafrim" --argjson kind "$kind" --argjson cloud "$cloud" \
  --argjson plafrimResource "$plafrim_resource" \
  --argjson plafrimEntrypoint "$plafrim_entrypoint" \
  --argjson kindResource "$kind_resource" --argjson cloudResource "$cloud_resource" \
  '{run:{id:$runId,schedulePlanId:$plan.id,mode:"real",status:"created"},
    plan:$plan,workflow:$definition.version,executionScope:$scope,networkTopology:$network,
    resources:[$plafrimResource,$plafrimEntrypoint,$kindResource,$cloudResource],
    runtimes:($plafrim.runtimes+$kind.runtimes+$cloud.runtimes),
    runtimeBindings:($plafrim.resourceRuntimeBindings+$kind.resourceRuntimeBindings+$cloud.resourceRuntimeBindings)}' |
  curl --fail-with-body -sS -H "Authorization: Bearer $api_token" \
    -H 'Content-Type: application/json' --data-binary @- "$api_base/execution-runs/"
