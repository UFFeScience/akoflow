---
title: Configure Google Cloud
description: Connect a GCP project, discover capacity, and provision an AkôFlow worker safely.
---

# Configure Google Cloud

AkôFlow uses a service account to read the Compute catalog and, when requested, run Terraform to create a worker. The credential is stored locally by the daemon; the documentation examples never embed the private key in an environment YAML file.

## Before you begin

You need a GCP project with billing enabled and a service account JSON key. Enable the Compute Engine API. Enable the Cloud Billing API if you want catalog price estimates; without it, machine discovery can still succeed but pricing may be incomplete.

Grant only the permissions needed by your lifecycle policy. The read-only catalog calls Compute Engine to list aggregated machine types and disk types in the selected project, and ready images in that project plus the Ubuntu, Debian, and Rocky public image projects. It also requests public Compute Engine SKUs from the Cloud Billing Catalog API; a pricing failure becomes a catalog warning and does not prevent machine discovery.

Provisioning uses the Terraform target shipped with the daemon. It lists available zones in the chosen region, creates and manages one Compute Engine instance and its boot disk, and creates a tagged ingress firewall rule for SSH. It references the VPC or subnetwork selected in the capacity target; it does not create a network, attach a service account to the instance, or manage IAM bindings. Confirm the exact least-privilege role set in a disposable project before adopting it as an institutional policy.

## 1. Store the service-account credential

In Desktop, open **Settings → Credentials**, choose **Cloud credential**, select **GCP**, and paste or import the service-account JSON. AkôFlow validates the presence of `project_id`, `client_email`, `private_key`, and `token_uri`. Give the credential a stable name such as `gcp-research-project`; environments refer to this record, not to the JSON file.

Keep the downloaded key outside the repository, restrict its filesystem permissions, and rotate it according to your institution's policy. A failed validation usually means the JSON is truncated, belongs to a different credential type, or lacks one of the required fields.

## 2. Create the cloud environment

Open **Infrastructure → Environments → New environment** and choose a cloud execution environment. Set:

- **Provider:** `gcp`
- **Project ID:** the service account's GCP project
- **Region:** for example `us-central1`
- **Credential:** the record created above

After saving, open **Cloud capacity** and select **Refresh catalog**. AkôFlow reads machine types from your project and public images from the project itself plus `ubuntu-os-cloud`, `debian-cloud`, and `rocky-linux-cloud`.

If refresh fails, check the daemon log before changing the credential. A `403` normally identifies a disabled API or missing IAM permission; an empty price field with otherwise valid machines normally points to the Cloud Billing API.

## 3. Define a capacity target

A capacity target is the reproducible template offered to the scheduler. Choose the machine type, image, disk, region/zone policy, maximum instances, provisioning mode, and lifecycle policy. For the first worker, use a standard instance and a common Debian or Ubuntu image.

Network settings deserve explicit review:

- Select an existing VPC and subnetwork when the project does not use the default network.
- Restrict `sshSourceRanges` to the daemon or bastion CIDR. Do not leave `0.0.0.0/0` in a production project.
- Supply the SSH user and public key that AkôFlow will use after Terraform completes.
- Use `pd-balanced` unless the chosen machine family and disk type are known to be compatible. AkôFlow falls back from unsupported Hyperdisk combinations for E2 machines, but an explicit compatible choice is easier to audit.

## 4. Provision and verify

Start provisioning from the target. The operation has two distinct stages: Terraform creates the infrastructure, then AkôFlow validates and configures the machine. Wait until the instance and its resource binding are online before including it in an execution scope.

Verify these points in Desktop:

1. The provisioned instance has an IP address and provider instance ID.
2. SSH validation succeeds with the selected credential and user.
3. The resource reports cores, memory, architecture, and runtime capabilities.
4. A terminal can be opened if interactive access is enabled.
5. The execution scope lists the worker as schedulable.

Stopping an instance preserves provider resources and can continue to incur disk charges. **Destroy** removes the Terraform-managed instance; inspect active runs and export required data first.

## API checkpoints

Use the API when automating onboarding. Store the secret through the credentials endpoint used by your deployment, create the environment with `provider: gcp`, then refresh and inspect the catalog:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -X POST "$AKOFLOW_URL/environments/gcp-lab/cloud-catalog/refresh/"

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/environments/gcp-lab/cloud-catalog/"
```

Continue with [Cloud capacity and machine configuration](./cloud-capacity) for the target and provisioning payloads, and [Interactive console and commands](../operations/interactive-console) to open a shell after validation.
