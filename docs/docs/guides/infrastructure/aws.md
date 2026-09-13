---
title: AWS and S3 support
description: What AkôFlow currently implements for S3 transfers and where AWS setup remains incomplete.
---

# AWS and S3 support

AkôFlow can transfer objects to and from an `s3://bucket/prefix` endpoint when its server has `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. Temporary credentials also require `AWS_SESSION_TOKEN`. You can set an endpoint and region for an S3-compatible service. Local tests cover the transfer code, but a run against an AWS account or test bucket has not yet been verified.

This is **partial AWS support**. AkôFlow does not discover or provision EC2 workers. Saving an AWS cloud credential in AkôFlow does not make it available to S3 transfers; the server needs the environment variables above. For a transfer endpoint, omit `configuration.credentialRef` or set it to `env`. Other values fail with the default server credential resolver.

## Before using S3 transfers

1. Ask the operator to provide the approved bucket, prefix, region, and server-side credential setup. Limit permissions to the required objects.
2. Confirm that the server process receives the credentials. Avoid placing access keys in workflow or environment files.
3. Use an `s3://bucket/prefix` transfer endpoint and check the resulting transfer and object evidence after a small run. A successful credential save or environment registration alone does not prove that data movement works.

The **Infrastructure → Storage** screen browses storage already registered with an environment; it does not create an S3 storage connection. Its current S3 browser sends unsigned requests, even when the storage record has `credentialReference` or the server has AWS environment variables. Use a small transfer to check the separately configured transfer connector; it does not verify Desktop browsing.

For S3-compatible services, set `endpoint`, `region`, and `secure` in the transfer configuration if needed. The default endpoint is `s3.amazonaws.com`, and `secure` defaults to TLS. See the [environment YAML reference](/docs/reference/environment-yaml#connections-and-transfer-connectors) for the fields and [Storage](/docs/guides/infrastructure/storage) for browsing registered storage.

To run compute on existing AWS-hosted machines, connect them through a supported SSH or Kubernetes runtime. [Cloud capacity](/docs/guides/infrastructure/cloud-capacity) lists the provider limits. An end-to-end S3 tutorial remains pending validation with a disposable bucket and cleanup procedure.
