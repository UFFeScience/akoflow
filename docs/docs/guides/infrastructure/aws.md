---
title: AWS and S3 support
description: What AkôFlow currently implements for S3 transfers and where AWS setup remains incomplete.
---

# AWS and S3 support

AkôFlow has an S3-compatible transfer connector. It can read and write objects at an `s3://bucket/prefix` endpoint when the server process has `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`; temporary credentials also need `AWS_SESSION_TOKEN`. The connector accepts an optional endpoint and region for S3-compatible services. Its behavior is covered by local connector tests, but the documentation has not yet verified a run against an AWS account or test bucket.

This is **partial AWS support**. AkôFlow does not discover or provision EC2 workers. A stored AWS cloud credential is not currently wired into the S3 transfer connector. Its default credential resolver reads the server's environment variables; a different nonempty credential reference fails unless the deployment supplies its own resolver.

## Before using S3 transfers

1. Ask the operator to provide the approved bucket, prefix, region, and server-side credential setup. Limit permissions to the required objects.
2. Confirm that the server process receives the credentials. Avoid placing access keys in workflow or environment files.
3. Use an `s3://bucket/prefix` transfer endpoint and check the resulting transfer and object evidence after a small run. A successful credential save or environment registration alone does not prove that data movement works.

The **Infrastructure → Storage** screen browses storage already registered with an environment; it does not create an S3 storage connection. The current S3 browsing driver is constructed without a credential resolver, so its saved `credentialReference` is not applied to signed AWS requests. Do not rely on that screen to validate private-bucket access.

For S3-compatible services, the transfer connector can use its `endpoint`, `region`, and `secure` settings. Its default endpoint is `s3.amazonaws.com`; `secure` defaults to TLS. The [environment YAML reference](../../reference/environment-yaml#connections-and-transfer-connectors) describes connector bindings, while [Storage](./storage) explains browsing registered storage.

To run compute on existing AWS-hosted machines, connect them through a supported SSH or Kubernetes runtime. [Cloud capacity](./cloud-capacity) lists the provider limits. An end-to-end S3 tutorial remains pending validation with a disposable bucket and cleanup procedure.
