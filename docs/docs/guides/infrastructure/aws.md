---
title: Configure AWS
description: Configure AWS credentials and S3 data transfer without overstating v1.0 compute support.
---

# Configure AWS

AkôFlow v1.0 accepts AWS credentials and can move artifacts through Amazon S3 or an S3-compatible service. It does **not** yet discover EC2 machine types or provision EC2 workers. Creating an AWS credential therefore enables storage operations; it does not create schedulable cloud capacity.

## Configure an S3 credential

Create a dedicated IAM principal or short-lived credential for the bucket used by AkôFlow. Grant access only to the required bucket and prefix. Typical operations require listing the bucket prefix and reading, writing, and deleting objects that AkôFlow owns.

In Desktop, open **Settings → Credentials**, create an AWS cloud credential, and provide the access-key material expected by your deployment. Do not paste credentials into workflow, environment, or plan YAML. For temporary credentials, include the session token and replace the record before it expires.

## Register S3 storage

Create storage under **Infrastructure → Storage** and select the S3 adapter. Configure:

- the bucket and optional AkôFlow prefix;
- the AWS region;
- the stored credential reference;
- `s3.amazonaws.com` for AWS, or the explicit endpoint for an S3-compatible service;
- TLS and path-style addressing according to the selected service.

Test the storage connection before using it in an environment. A successful credential save only verifies the document shape; a storage test verifies endpoint reachability and authorization.

## Common failures

| Symptom | What to inspect |
| --- | --- |
| `AccessDenied` | IAM action, bucket policy, KMS permission, and prefix restriction |
| `SignatureDoesNotMatch` | Region, endpoint, system clock, and access/secret pair |
| Redirect to another region | Bucket region differs from the configured region |
| TLS or hostname failure | Custom endpoint and certificate chain |
| Upload succeeds but execution cannot read | Runtime binding uses a different storage or credential |

## Compute capacity

Do not create a nominal AWS execution environment expecting EC2 capacity to appear. Until the EC2 provider is implemented, connect existing AWS-hosted machines through the same SSH/direct-runtime path used for remote workers, or use Kubernetes when those machines belong to a cluster. The scheduler only sees capacity after a real resource and runtime binding are registered.

See [HPC and SLURM clusters](./hpc-slurm) for the SSH connection pattern and [Storage](./storage) for artifact placement. The cloud support matrix is maintained in [Cloud capacity](./cloud-capacity).
