---
title: Configure cloud capacity
---

Use this guide after [connecting Google Cloud](../../tutorials/connect-cloud).
Choose a machine from its catalog, save a capacity target for planning, and
provision an instance when a run needs it. Google Cloud is the current compute
provider; check [cloud provider support](./cloud-support) for AWS and object
storage limits. For optional Ansible setup, create a
[machine configuration](./machine-configurations) before saving the target.

For the API commands on this page, complete [API connection setup](../../tutorials/api-access) and register `research-gcp` through the [Google Cloud connection tutorial](../../tutorials/connect-cloud) first. Run the commands in the same Bash session.

This procedure has not yet passed a live provision-and-destroy cycle in a
disposable project. [Configure Google Cloud](./gcp) covers account access and
the checks to perform before a real worker run.

## Synchronize the provider catalog

### Using AkôFlow Desktop

Open a cloud environment and select **Cloud capacity**. If no cached catalog exists, refresh it. Search and filter machine families and images, then choose a compatible disk. Displayed estimates combine catalog compute pricing and configured disk size; they are estimates rather than provider invoices.

### Using the API

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -X POST "$AKOFLOW_API_URL/environments/research-gcp/cloud-catalog/refresh/"

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environments/research-gcp/cloud-catalog/"
```

The GET endpoint returns `404` until a catalog has been synchronized. Provider credentials must already be stored and referenced by the cloud environment connection.

## Create a capacity target

### Using AkôFlow Desktop

1. Choose a catalog machine, image, disk, and disk size.
2. Select the region, provisioning mode, maximum instance count, and lifecycle policy.
3. Optionally attach an additional machine-configuration version.
4. Save the target. It becomes a capacity option available to planning; saving it does not create a VM.

If you need a machine configuration, [create its version](./machine-configurations) before saving the target and attach that version's actual ID. Provisioning needs the referenced version.

For a required zone, set `fixedZone` through the target API below.

### Using the API

Keep the `AKOFLOW_GCP_PROJECT` value from the validated [connection tutorial](../../tutorials/connect-cloud). Read a compatible Ubuntu image's `providerImageId` from the synchronized catalog. Confirm that `e2-standard-4` is available in the selected region, or replace the machine type and its CPU/memory values with a catalog match. Enter the approved daemon or bastion CIDR before the request is sent.

```bash
set -o pipefail
: "${AKOFLOW_GCP_PROJECT:?Complete the GCP connection tutorial first}"
read -r -p 'Ubuntu providerImageId from the catalog: ' AKOFLOW_GCP_IMAGE_ID || exit 1
read -r -p 'Approved SSH source CIDR: ' AKOFLOW_SSH_CIDR || exit 1
[ -n "$AKOFLOW_GCP_IMAGE_ID" ] && [ -n "$AKOFLOW_SSH_CIDR" ] || exit 1

jq -n --arg project "$AKOFLOW_GCP_PROJECT" \
  --arg image "$AKOFLOW_GCP_IMAGE_ID" --arg cidr "$AKOFLOW_SSH_CIDR" \
  --arg configVersion "${AKOFLOW_MACHINE_CONFIGURATION_VERSION_ID:-}" '{
    name:"E2 standard worker",
    provider:"gcp",
    providerMachineType:"e2-standard-4",
    region:"us-central1",
    imageReference:$image,
    architecture:"amd64",
    vcpu:4,
    memoryMiB:16384,
    provisioningMode:"standard",
    maximumInstances:2,
    lifecyclePolicy:"destroy-after-run",
    configuration:{
      projectId:$project,
      diskType:"pd-balanced",
      diskSizeGiB:30,
      network:"default",
      sshSourceRanges:[$cidr]
    }
  } + (if $configVersion == "" then {} else {
    machineConfigurations:[{
      configurationVersionId:$configVersion,
      executionOrder:1,
      required:true,
      enabled:true
    }]
  } end)' | curl --fail-with-body \
    -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/json' --data-binary @- \
    "$AKOFLOW_API_URL/environments/research-gcp/cloud-capacity-targets/" \
    -o cloud-target.json || exit 1

AKOFLOW_CAPACITY_TARGET_ID=$(jq -er '.id' cloud-target.json) || exit 1
```

The project ID is required by the current Terraform target; it is not copied from the environment connection. The built-in worker configuration requires `amd64`, even when Google Cloud labels a machine `X86_64`. If the CIDR is omitted, the Terraform target defaults SSH ingress to `0.0.0.0/0`.

Set `fixedZone` in the target if the worker must use a particular zone. Without it, the current Terraform module chooses the first active zone returned for the region. The saved `zonePolicy` field does not currently affect that choice.

The server supplies the target ID and environment ID when omitted, enables the target, and creates a schedulable capacity record; no VM is created yet. The command saves the returned ID for provisioning. The machine type must also come from the synchronized catalog.

## Provision and follow an instance

### Using AkôFlow Desktop

Open a cloud resource or the environment **Provisioning** tab and start provisioning from a capacity target. The operation view separates Terraform provisioning from machine configuration and shows events and logs as they become available. Provisioning is asynchronous.

### Using the API

```bash
set -o pipefail
jq -n --arg id "$AKOFLOW_CAPACITY_TARGET_ID" '{capacityTargetId:$id}' | \
  curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
    -H 'Content-Type: application/json' --data-binary @- \
    "$AKOFLOW_API_URL/environments/research-gcp/cloud-provisioning/" \
    -o cloud-operation.json || exit 1

AKOFLOW_CLOUD_OPERATION_ID=$(jq -er '.id' cloud-operation.json) || exit 1

# Inspect the operation
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$AKOFLOW_API_URL/cloud-operations/"
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$AKOFLOW_API_URL/cloud-operations/$AKOFLOW_CLOUD_OPERATION_ID/"
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" "$AKOFLOW_API_URL/cloud-operations/$AKOFLOW_CLOUD_OPERATION_ID/events/"
```

The provisioning request queues an operation. Read its status, failure reason,
and events until it completes or fails. A `202 Accepted` response does not
confirm that the target belongs to this environment, the credential works, or
the VM is ready; those checks run later. Lifecycle endpoints also exist for
configure, validate, start, stop, and destroy. Before destructive actions,
inspect the instance and active operation state in Desktop or through the API.
