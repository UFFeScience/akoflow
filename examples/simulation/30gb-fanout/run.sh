#!/usr/bin/env bash
set -euo pipefail

: "${AKOFLOW_API_URL:=http://127.0.0.1:8080/akoflow-api}"
: "${AKOFLOW_API_TOKEN:?Set AKOFLOW_API_TOKEN before running this example.}"

root=$(cd "$(dirname "$0")" && pwd)

post_yaml() {
  local endpoint=$1 file=$2
  curl --fail-with-body \
    -H "Authorization: Bearer ${AKOFLOW_API_TOKEN}" \
    -H 'Content-Type: application/yaml' \
    --data-binary "@${file}" "${AKOFLOW_API_URL}/${endpoint}/"
}

post_yaml environments "${root}/environment.yaml"
post_yaml execution-scopes "${root}/scope.yaml"
post_yaml network-topologies "${root}/topology.yaml"
post_yaml workflow-definitions "${root}/workflow.yaml"
post_yaml schedule-plans "${root}/plan-request.yaml"
post_yaml execution-runs "${root}/execution-request.yaml"

echo
echo 'Submitted simgrid-30gb-fanout-run-v1. Poll it with:'
printf 'curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "%s/execution-runs/simgrid-30gb-fanout-run-v1/"\n' "${AKOFLOW_API_URL}"
