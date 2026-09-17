# Montage 58: four GCP `e2-medium` workers, local completion

This bundle derives its 58 real Montage commands from `source-workflow.yaml`.
The original image `ovvesley/akoflow-wf-montage:050d` contains the twelve input
FITS files, headers, lookup tables, and Montage binaries. The twelve `mProject`
activities are distributed three per GCP `e2-medium` worker; the remaining 46 run on the AkôFlow server's
local execution resource. `workflow.json` declares 138 named file handoffs,
including the projected FITS files sent from GCP to local activities.
`workflow.json` pins that image by its registry digest. `artifact.json` registers
the same OCI image in the AkôFlow catalog and requests an `amd64` SIF build.
The Docker image is already published; no Docker rebuild or new registry tag is
needed to preserve these commands. The SIF is an additional representation for
environments that use Apptainer. Docker workers use the pinned OCI image.

`reference-runtimes.csv` is a historical profile measured on `c3d-standard-16`.
Its times are **not** measurements on `e2-medium` or the local resource. The
imported plan's predicted time and cost are illustrative metadata, not a
calibrated forecast or an observed result.

## Requirements

- A GCP connection with the `goal-gcp-e2-medium` capacity target as a template,
  quota for four `e2-medium` instances, and a working SSH/Docker cloud runtime.
  `prepare-capacity.sh` clones four distinct one-instance targets with
  `destroy-after-run` lifecycle; it does not provision a VM.
- The local AkôFlow daemon needs Docker CLI and access to the Docker daemon.
  The local runtime mounts the shared workspace volume into the activity
  container. Each activity stores only its Montage command; the runtime seeds
  image inputs and the engine stages dependency outputs in that workspace.
- The daemon needs enough disk for intermediate FITS files and the image. The
  GCP project must allow four `e2-medium` VMs and their eventual destruction.
- `curl`, `jq`, and a daemon API token to submit from a shell.

## Generate and inspect

```sh
ruby examples/real/montage-cloud-local/generate.rb
jq -r '.spec.image' examples/real/montage-cloud-local/workflow.json
jq '[.spec.activities[] | .runtime] | group_by(.) | map({runtime:.[0], count:length})' \
  examples/real/montage-cloud-local/workflow.json
```

Import `scope.json`, `network.json`, `workflow.json`, then `plan.json` using
their corresponding AkôFlow API endpoints. Use the request JSON content type
for all four. The plan is *manual*: it does not invoke PRISM or HEFT.
Register the image first and inspect the returned build run. Its SIF conversion
is asynchronous and is independent from the four-VM Docker run.

```sh
export AKOFLOW_API_URL=http://127.0.0.1:8080/akoflow-api
read -rsp 'AkôFlow API token: ' AKOFLOW_API_TOKEN; printf '\n'; export AKOFLOW_API_TOKEN
base=examples/real/montage-cloud-local
sh "$base/register-artifact.sh"
sh "$base/prepare-capacity.sh"
for pair in 'execution-scopes scope.json' 'network-topologies network.json' \
            'workflow-definitions workflow.json' 'schedule-plans/import plan.json'; do
  set -- $pair
  curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/json' --data-binary "@$base/$2" \
    "$AKOFLOW_API_URL/$1/"
done
sh "$base/submit-run.sh" montage-58-four-vm-local-050d-run-v1
```

Before submitting, verify the imported plan has three `mProject` assignments
to each of the four `montage-gcp-e2-medium-*` targets and 46 assignments to
`local-environment-entrypoint`. The four targets represent four separate VMs;
the local workspace volume is not shared between them. AkôFlow transfers
declared outputs across resources. Run only one
copy at a time. A successful run ends with `mosaic-color.png`; inspect the
run's activity records, transferred bytes and file checksums. After the run,
verify all four provisioned GCP VMs have status `destroyed` in AkôFlow and in GCP.

The example has not yet produced a completed observed run in the repository.
Do not treat the reference runtimes or manual plan prediction as a result.
