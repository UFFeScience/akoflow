#!/bin/sh
set -eu

base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

resource() {
  case "$1" in
    local) printf '%s' local-environment-entrypoint ;;
    hpc) printf '%s' hpc-environment-ssh-connection-partition-testing ;;
    cloud) printf '%s' montage-gcp-e2-medium-1 ;;
  esac
}

runtime() {
  case "$1" in
    local) printf '%s' local-environment-local ;;
    hpc) printf '%s' hpc-environment-slurm ;;
    cloud) printf '%s' gcp-environment-cloud ;;
  esac
}

authored_runtime() {
  case "$1" in
    local) printf '%s' local ;;
    hpc) printf '%s' slurm ;;
    cloud) printf '%s' cloud ;;
  esac
}

while IFS="$(printf '\t')" read -r case_id source target; do
  workflow_id="transfer-matrix-$case_id"
  source_resource=$(resource "$source")
  target_resource=$(resource "$target")
  source_runtime=$(runtime "$source")
  target_runtime=$(runtime "$target")
	  source_authored_runtime=$(authored_runtime "$source")
	  target_authored_runtime=$(authored_runtime "$target")
	  payload_size=$(printf 'route=%s-%s\nsequence=1\n' "$source" "$target" | wc -c | tr -d ' ')

  jq -n \
    --arg name "$workflow_id" \
    --arg source "$source" \
    --arg target "$target" \
	    --arg sourceRuntime "$source_authored_runtime" \
	    --arg targetRuntime "$target_authored_runtime" \
	    --argjson payloadSize "$payload_size" \
    '{name:$name,spec:{namespace:"transfer-matrix",image:"docker.io/library/busybox:1.36",activities:[
	      {name:"produce",runtime:$sourceRuntime,cpuLimit:"0.1",memoryLimit:"16Mi",run:("set -eu\nprintf 'route="+$source+"-"+$target+"\\nsequence=1\\n' > payload.txt\nsha256sum payload.txt > payload.sha256\ntest -s payload.txt")},
	      {name:"consume",runtime:$targetRuntime,cpuLimit:"0.1",memoryLimit:"16Mi",dependsOn:["produce"],run:("set -eu\ntest -s payload.txt\nsha256sum -c payload.sha256\ngrep -qx 'route="+$source+"-"+$target+"' payload.txt\nprintf 'consumed="+$target+"\\n' > receipt.txt")}
    ],dataDependencies:[
	      {producerActivity:"produce",consumerActivity:"consume",logicalName:"payload.txt",sizeBytes:$payloadSize},
      {producerActivity:"produce",consumerActivity:"consume",logicalName:"payload.sha256",sizeBytes:78}
    ]}}' > "$base/workflow-$case_id.json"

  jq -n \
    --arg id "$workflow_id-plan" \
    --arg workflowVersionId "$workflow_id-v1" \
    --arg producerId "$workflow_id-produce" \
    --arg consumerId "$workflow_id-consume" \
    --arg sourceResource "$source_resource" \
    --arg targetResource "$target_resource" \
    --arg sourceRuntime "$source_runtime" \
    --arg targetRuntime "$target_runtime" \
    '{plan:{id:$id,workflowVersionId:$workflowVersionId,executionScopeId:"transfer-matrix-scope-v1",networkTopologyId:"transfer-matrix-network-v1",source:"imported",algorithm:"manual",algorithmVersion:"1",objective:"transfer-validation",predicted:{makespanSeconds:300,cost:0.01,feasible:true},assignments:[
      {id:($id+"-produce"),activityId:$producerId,resourceId:$sourceResource,orderOnResource:0,metadata:{runtimeId:$sourceRuntime}},
      {id:($id+"-consume"),activityId:$consumerId,resourceId:$targetResource,orderOnResource:0,metadata:{runtimeId:$targetRuntime}}
    ]}}' > "$base/plan-$case_id.json"
done < "$base/cases.tsv"
