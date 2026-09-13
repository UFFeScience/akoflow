# Montage 58: GCP `e2-medium` projection, local completion

This bundle derives its 58 real Montage commands from `source-workflow.yaml`.
The original image `ovvesley/akoflow-wf-montage:050d` contains the twelve input
FITS files, headers, lookup tables, and Montage binaries. The twelve `mProject`
activities run on GCP `e2-medium`; the remaining 46 run on the AkôFlow server's
local execution resource. `workflow.json` declares 138 named file handoffs,
including the projected FITS files sent from GCP to local activities.

`reference-runtimes.csv` is a historical profile measured on `c3d-standard-16`.
Its times are **not** measurements on `e2-medium` or the local resource. The
imported plan's predicted time and cost are illustrative metadata, not a
calibrated forecast or an observed result.

## Requirements

- A GCP connection with the `goal-gcp-e2-medium` capacity target and a working
  SSH/Docker cloud runtime. Adapt IDs in the bundle when using another instance.
- The local AkôFlow daemon needs Docker CLI and access to the Docker daemon.
  Its local adapter starts host processes; it does not directly run OCI images.
  The bundle's local commands use `docker create/cp/start` to run the *same*
  Montage image without assuming the server container path is a host bind path.
- The daemon needs enough disk for intermediate FITS files and the image. The
  GCP project must allow an `e2-medium` VM and eventual destruction.
- `curl`, `jq`, and a daemon API token to submit from a shell.

## Generate and inspect

```sh
ruby examples/real/montage-cloud-local/generate.rb
jq '[.spec.activities[] | .runtime] | group_by(.) | map({runtime:.[0], count:length})' \
  examples/real/montage-cloud-local/workflow.json
```

Import `scope.json`, `network.json`, `workflow.json`, then `plan.json` using
their corresponding AkôFlow API endpoints. Use the request JSON content type
for all four. The plan is *manual*: it does not invoke PRISM or HEFT.

```sh
export AKOFLOW_API_URL=http://127.0.0.1:8080/akoflow-api
read -rsp 'AkôFlow API token: ' AKOFLOW_API_TOKEN; printf '\n'; export AKOFLOW_API_TOKEN
base=examples/real/montage-cloud-local
for pair in 'execution-scopes scope.json' 'network-topologies network.json' \
            'workflow-definitions workflow.json' 'schedule-plans/import plan.json'; do
  set -- $pair
  curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/json' --data-binary "@$base/$2" \
    "$AKOFLOW_API_URL/$1/"
done
sh "$base/submit-run.sh" montage-58-cloud-local-run-v1
```

Before submitting, verify the imported plan has exactly twelve assignments to
`goal-gcp-e2-medium` and 46 to `local-environment-entrypoint`. Run only one
copy at a time. A successful run ends with `mosaic-color.png`; inspect the
run's activity records, transferred bytes and file checksums. After the run,
verify the provisioned GCP VM has status `destroyed` in AkôFlow and in GCP.

The example has not yet produced a completed observed run in the repository.
Do not treat the reference runtimes or manual plan prediction as a result.
