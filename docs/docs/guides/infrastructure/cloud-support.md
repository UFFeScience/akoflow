---
title: Cloud provider support
description: Current compute and object-storage support for Google Cloud and AWS.
---

Use this page to check what AkôFlow can do with each provider before connecting
an account. Google Cloud has a compute path; AWS currently has a separate,
partially supported S3 transfer path.

| Capability | Google Cloud | AWS |
| --- | --- | --- |
| Save a provider credential | Yes | The record can be saved, but S3 transfers do not use it |
| Discover compute machines, images, and disks | Yes | No |
| Choose a zone and estimate prices | A fixed zone can be set; otherwise provisioning uses the first active zone returned for the region. Estimates depend on Cloud Billing access | No |
| Provision a compute worker | Terraform path exists | No EC2 provisioning |
| Transfer artifacts through object storage | Direct `gs://` transfer is unavailable in the current server; signed HTTPS may be used when appropriate | S3-compatible transfer uses server environment credentials; live AWS validation is pending |

The Google Cloud compute path is implemented, but a complete live
provision-and-destroy cycle in a disposable project has not been verified.
Local tests cover the S3 transfer connector; live bucket access has not been
verified. A saved AWS credential does not enable S3 transfers.

For compute, start with [Connect Google Cloud](../../tutorials/connect-cloud),
then [configure cloud capacity](./cloud-capacity). For object transfers, read
[AWS and S3 support](./aws) before planning data movement.
