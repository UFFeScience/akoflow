#!/bin/sh
# Submit a fresh run from live catalog, selected priced plan and immutable image targets.
set -eu
mode=${1:?simulation or real}
case "$mode" in simulation|real) ;; *) exit 2 ;; esac
: "${AKOFLOW_API_TOKEN:?Load the API token without printing it}"
: "${RUN_ID:?Use a new explicit experiment identity}"
api=${AKOFLOW_API_URL:-http://127.0.0.1:8080/akoflow-api}
base=storage/montage-image-20261010
get() { curl -fsS -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$api/$1/"; }
test "$(get execution-runs | jq '[.[]|select(.status=="running" or .status=="created")]|length')" = 0
if [ "$mode" = real ]; then
  test "$(get cloud-operations | jq '[.[]|select(.status=="running" or .status=="queued")]|length')" = 0
  test "$(get environments/gcp-environment/cloud-instances | jq '[.[]|select(.status!="destroyed")]|length')" = 0
fi
gcp=$(get environments/gcp-environment)
hpc=$(get environments/plafrim-execution)
jq --arg mode "$mode" --arg id "$RUN_ID" --argjson gcp "$gcp" --argjson hpc "$hpc" --slurpfile baseline storage/montage-fog-20261006/real-request.json --slurpfile plan "$base/selected-priced-v2.json" --slurpfile scope "$base/scope.json" --slurpfile network "$base/network.json" '
  .run={id:$id,mode:$mode,seed:1,schedulePlanId:$plan[0].id} |
  .plan=$plan[0] | .executionScope=$scope[0] | .networkTopology=$network[0] |
  (.plan.assignments|map(.resourceId)|unique) as $ids |
  .resources=([$gcp.resources[],$hpc.resources[]]|map(select(.id as $resource|$ids|index($resource)))) |
  ($baseline[0].runtimes[]|select(.id=="montage-simgrid")) as $simulation |
  .runtimes=([$gcp.runtimes[],$hpc.runtimes[],$simulation]|unique_by(.id)) |
  .runtimeBindings=(if $mode=="simulation" then [$ids[]|{resourceId:.,runtimeId:"montage-simgrid",enabled:true}]
    else [$gcp.runtimeBindings[]?,$gcp.resourceRuntimeBindings[]?,$hpc.runtimeBindings[]?,$hpc.resourceRuntimeBindings[]?]|map(select(.resourceId as $resource|$ids|index($resource)))|unique_by(.resourceId+":"+.runtimeId) end)
' storage/montage-fog-20261006/single-lane-real-request.json |
  tee "$base/$RUN_ID-request.json" |
  curl --fail-with-body -sS -H "Authorization: Bearer $AKOFLOW_API_TOKEN" -H 'Content-Type: application/json' --data-binary @- "$api/execution-runs/" > "$base/$RUN_ID-submitted.json"
jq '{id,status,error}' "$base/$RUN_ID-submitted.json"
