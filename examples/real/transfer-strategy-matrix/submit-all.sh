#!/bin/sh
set -eu

base=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
run_suffix=${1:-$(date -u +%Y%m%dT%H%M%SZ)}

while IFS="$(printf '\t')" read -r case_id _; do
  "$base/submit-run.sh" "$case_id" "transfer-matrix-$case_id-$run_suffix"
done < "$base/cases.tsv"

"$base/submit-run.sh" roundtrip "transfer-matrix-roundtrip-$run_suffix"

echo 'Submitted all seven runs. AkôFlow environment leases control their execution order.'
