---
title: Build an executable from a Docker image
description: Register a Docker image and build a versioned SIF executable for later runs.
---

# Build an executable from a Docker image

Use this guide when a workflow needs a versioned executable built from a Docker image. AkôFlow registers the image as an artifact, then starts a build that produces a SIF file. To register a SIF file already on storage, use [Browse and manage storage](../infrastructure/storage#register-an-existing-file).

For the API commands below, complete [API connection setup](../../tutorials/api-access) first.

## Register and build

Open **Artifacts** and choose **Build artifact**. Enter an artifact ID, semantic version, registry image reference, and architecture. Desktop registers an immutable catalog version, creates a Docker-image-to-SIF build specification, and immediately starts its build run.

To do the same through the API, run these two calls in order:

```bash
curl --fail-with-body -X POST \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "artifactId":"busybox",
    "version":"1.36",
    "image":"docker.io/library/busybox:1.36",
    "architecture":"amd64"
  }' \
  "$AKOFLOW_API_URL/artifacts/docker/"

# Copy build.id from the response before starting the build.
read -r -p 'Build ID from the response: ' BUILD_ID
curl --fail-with-body -X POST -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/artifact-builds/$BUILD_ID/runs/"
```

The Docker registry pull and SIF conversion run in the build service, not in the browser. Poll `/build-runs/{runId}/`. When complete, `/build-runs/{runId}/output/` streams the SIF file.

## Use a custom build recipe

First upload a build context as multipart form data:

```bash
curl -X POST -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -F "context=@context.tar.gz" \
  "$AKOFLOW_API_URL/build-contexts/"
```

Then create an immutable build specification at `/artifact-builds/`. It requires `id`, `artifactVersionId`, `contextDigest`, `recipeDigest`, and `cacheKey`; target and recipe fields describe the desired output. A repeated cache key returns the existing build rather than creating a duplicate. The JSON form of `/build-contexts/` only records metadata for bytes already present in the artifact store and requires `digest`, `storageUri`, and a positive `sizeBytes`.

:::warning Credentials and paths
Do not put registry credentials, SSH secrets, or cloud secrets in artifact payloads. Use configured credential references. Browser-local file paths are not server build contexts; upload the bytes or register artifact-store metadata.
:::
