---
title: Configure Google Cloud
description: Connect a GCP project and review the current worker-provisioning requirements.
---

# Configure Google Cloud

For a guided first registration with interface screenshots and complete API steps,
start with [the connection tutorial](../../tutorials/connect-cloud). This page provides
the detailed operational requirements.

AkôFlow uses a service account to read the Compute catalog and, when requested, run Terraform to create a worker. The credential is stored locally by the daemon; the documentation examples never embed the private key in an environment YAML file.

The provider calls and Terraform resources below were checked against the source. A full provision-and-destroy cycle has not yet been verified in a disposable GCP project; confirm permissions, network policy, and cleanup before relying on this procedure.

## Before you begin

You need a GCP project with billing enabled and a service account JSON key. Enable the Compute Engine API. Enable the Cloud Billing API if you want catalog price estimates; without it, machine discovery can still succeed but pricing may be incomplete.

Grant only the permissions needed by your lifecycle policy. The read-only catalog calls Compute Engine to list aggregated machine types and disk types in the selected project, and ready images in that project plus the Ubuntu, Debian, and Rocky public image projects. It also requests public Compute Engine SKUs from the Cloud Billing Catalog API; a pricing failure becomes a catalog warning and does not prevent machine discovery.

Provisioning uses the Terraform target shipped with the daemon. It lists available zones in the chosen region, creates and manages one Compute Engine instance and its boot disk, and creates a tagged ingress firewall rule for SSH. It references the VPC or subnetwork selected in the capacity target; it does not create a network, attach a service account to the instance, or manage IAM bindings. Confirm the exact least-privilege role set in a disposable project before adopting it as an institutional policy.

## Permissions to review

The following is an inventory of the current daemon behavior, not a claim that one predefined Google role is least privilege. It gives the cloud administrator a concrete review surface before they create a service-account policy. The service-account token requests the broad OAuth scope `cloud-platform`; IAM still controls the operations that token can perform.

| AkôFlow operation | Current provider call or managed resource | Review before enabling |
| --- | --- | --- |
| Refresh catalog | Compute Engine aggregated machine types and disk types in the selected project | Read access to machine and disk-type metadata in the selected project and region |
| Refresh catalog | Ready images in the project plus `ubuntu-os-cloud`, `debian-cloud`, and `rocky-linux-cloud` | Read access to project-owned images; public-image visibility for the three named publisher projects |
| Refresh catalog | Cloud Billing Catalog SKUs for Compute Engine | Billing Catalog read access if estimates are required; the catalog remains usable with a pricing warning when this call fails |
| Choose a zone | Available Compute Engine zones in the selected region | Zone metadata read access |
| Provision or update | One `google_compute_instance` and its initialized boot disk | Instance and boot-disk lifecycle permissions in the selected project and zone |
| Reach the worker | One tagged `google_compute_firewall` ingress rule for TCP/22 | Firewall lifecycle permissions on the VPC named by the capacity target; restrict the source range before approval |
| Stop, start, or destroy | The Terraform-managed instance, disk, and firewall rule | Lifecycle and deletion rights only for resources managed by the target; review cleanup ownership before using automatic destroy |

The implementation does **not** create a VPC, subnet, Cloud NAT, service-account attachment, or project IAM binding. It does create one SSH firewall rule. If `sshSourceRanges` is omitted, the current Terraform target falls back to `0.0.0.0/0`; always set a daemon or bastion CIDR explicitly before provisioning. A target using a custom VPC or subnetwork must name resources that already exist and are authorized for the service account.

For an institutional least-privilege policy, first run catalog refresh in a disposable project with audit logging enabled, then provision and destroy one short-lived worker. Export the provider audit entries and derive the policy from the observed permission checks. This is safer than copying a broad owner/editor role from an example, and it is the validation still required before this guide can claim a tested minimal role set.

## 1. Connect the project

In Desktop, open **Infrastructure → Environments → Connect environment**, select **Cloud on demand**, and keep **Google Cloud** as the provider. Enter the project ID, region, and approved service-account JSON. Choose **Test connection** before saving. The [connection tutorial](../../tutorials/connect-cloud) shows the current form and its result checks.

The server stores the credential separately and saves its reference with the environment. Keep the original key outside the repository, restrict its filesystem permissions, and rotate it according to your institution's policy. A failed validation can indicate a malformed key, a disabled API, or missing access; read the returned message before changing settings.

## 2. Inspect the cloud environment

After saving, open the environment's **Cloud capacity** tab. Check that catalog synchronization returned machines, compatible images, and disks. Refresh the catalog if it is absent or stale. AkôFlow reads machine types from your project and public images from the project itself plus `ubuntu-os-cloud`, `debian-cloud`, and `rocky-linux-cloud`.

If refresh fails, check the daemon log before changing the credential. A `403` normally identifies a disabled API or missing IAM permission; an empty price field with otherwise valid machines normally points to the Cloud Billing API.

## 3. Define a capacity target

A capacity target is the template offered to the scheduler. Choose the machine type, image, disk, region, optional fixed zone, maximum instances, provisioning mode, and lifecycle policy. For the first worker, use a standard instance and a common Debian or Ubuntu image. The current Terraform module uses `fixedZone` when set; otherwise it takes the first active zone returned for the region. A saved `zonePolicy` does not change that selection.

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

For automation, follow [API connection setup](../../tutorials/api-access) and the [cloud connection tutorial](../../tutorials/connect-cloud) to validate and store the credential and register the environment. Then refresh and inspect the catalog:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -X POST "$AKOFLOW_API_URL/environments/research-gcp/cloud-catalog/refresh/"

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environments/research-gcp/cloud-catalog/"
```

Continue with [Configure cloud capacity](./cloud-capacity) for the target and provisioning payloads. Use [Machine configurations](./machine-configurations) if the worker needs an Ansible playbook, and [Interactive console](../operations/interactive-console) to open a shell after validation.
