#!/bin/sh
set -eu
mode=${1:?simulation or real}
case "$mode" in simulation|real) ;; *) exit 2 ;; esac
: "${AKOFLOW_API_TOKEN:?Load token without printing it}"
: "${RUN_ID:?Use a new holdout run identity}"
base=${MONTAGE_EXPERIMENT_DIR:-storage/montage-measured-20261011}
compute_model=${MONTAGE_COMPUTE_MODEL:-experiments/montage-micro-hpc-e2/compute-model-v2.json}
api=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
get() { curl -fsS -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$api/$1/"; }
get execution-runs > "$base/runs-before-$RUN_ID.json"
test "$(jq '[.[]|select(.status=="running" or .status=="created")]|length' "$base/runs-before-$RUN_ID.json")" = 0
test "$(jq --arg id "$RUN_ID" '[.[]|select(.id==$id)]|length' "$base/runs-before-$RUN_ID.json")" = 0
if [ "$mode" = real ]; then
  test "$(get cloud-operations | jq '[.[]|select(.status=="running" or .status=="queued")]|length')" = 0
  test "$(get environments/gcp-environment/cloud-instances | jq '[.[]|select(.status!="destroyed")]|length')" = 0
fi
get environments/gcp-environment > "$base/gcp-for-$RUN_ID.json"
get environments/plafrim-execution > "$base/hpc-for-$RUN_ID.json"
jq -n --arg mode "$mode" --arg id "$RUN_ID" \
  --slurpfile gcp "$base/gcp-for-$RUN_ID.json" --slurpfile hpc "$base/hpc-for-$RUN_ID.json" \
  --slurpfile plan "$base/selected-plan.json" --slurpfile workflow "$base/workflow-created.json" \
  --slurpfile scope "$base/scope.json" --slurpfile network "$base/network.json" \
  --slurpfile relay "$base/relay-resource.json" \
  --slurpfile compute "$compute_model" \
  --slurpfile baseline storage/montage-fog-20261006/real-request.json '
  ($plan[0].assignments|map(.resourceId)|unique) as $ids |
  ($baseline[0].runtimes[]|select(.id=="montage-simgrid")) as $simulation |
  {run:{id:$id,mode:$mode,seed:1,schedulePlanId:$plan[0].id},
   workflow:$workflow[0].version,plan:$plan[0],executionScope:$scope[0],networkTopology:$network[0],
   resources:(([$gcp[0].resources[],$hpc[0].resources[]]|map(select(.id as $r|$ids|index($r))))+[$relay[0]]),
   activityProfiles:($compute[0].activityProfiles|map(select(.resourceId as $r|$ids|index($r)))),
   runtimes:([$gcp[0].runtimes[],$hpc[0].runtimes[],$simulation]|unique_by(.id)),
   runtimeBindings:(if $mode=="simulation" then [$ids[]|{resourceId:.,runtimeId:"montage-simgrid",enabled:true}]
     else [$gcp[0].runtimeBindings[]?,$gcp[0].resourceRuntimeBindings[]?,$hpc[0].runtimeBindings[]?,$hpc[0].resourceRuntimeBindings[]?]|map(select(.resourceId as $r|$ids|index($r)))|unique_by(.resourceId+":"+.runtimeId) end)}
' > "$base/$RUN_ID-request.json"
curl --fail-with-body -sS -H "Authorization: Bearer $AKOFLOW_API_TOKEN" -H 'Content-Type: application/json' \
  --data-binary "@$base/$RUN_ID-request.json" "$api/execution-runs/" > "$base/$RUN_ID-submitted.json"
jq '{id,status,error}' "$base/$RUN_ID-submitted.json"
