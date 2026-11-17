---
title: Build and inspect OCI executable images
description: Save portable Dockerfile recipes, prepare missing OCI images locally, and inspect image layers.
---

# Build and inspect OCI executable images

Activities can use a published OCI image or a portable Dockerfile recipe. Saving a recipe does not require an existing image or start a build. Before local execution, AkôFlow verifies and reuses the local image or builds it with BuildKit, loads it into Docker, and verifies its platform and digest. Activity execution waits for preparation to finish.

## Save a Dockerfile before the first build

In an activity's executable picker, choose **Build from Dockerfile**. Enter the Dockerfile or upload a sanitized `.tar.gz` context containing it. Set the relative Dockerfile path, platform (`linux/amd64` or `linux/arm64`), and non-secret build arguments. Base images must use immutable `reference@sha256` digests; `FROM scratch` is also supported.

The context archive is limited to 4 MiB compressed, 16 MiB expanded, and 2048 entries. AkôFlow saves context and recipe digests, registers an OCI build specification, and makes the source selectable in the catalog before a build exists. Workflow publications carry the source bytes, so another installation can build the image without the original local catalog. Published workflow YAML is limited to 8 MiB and experiment packages to 24 MiB.

Preflight reads availability without pulling or building. A cached verified image passes even when BuildKit is unavailable. A missing image reports **Build required** after checking the context and builder; the actual build happens during execution preparation. Build failures keep the activity from launching.

## Inspect an image

Open an artifact in **Artifacts**, or choose **View image details, Dockerfile and layers** from an activity. Desktop shows platform, local size, image and registry digests, tags, creation time, Dockerfile, context digests, image configuration, filesystem layer digests, and command history. Select a history row to inspect its complete command and size. Unbuilt recipes show their source; layer history is available once the image exists locally. Inspection never starts a build or pull.

The Hub shows images and build recipes for published versions, including versions without a recorded execution. It displays the Dockerfile and source digests without executing uploaded content.

## Docker and Apptainer

Dockerfile recipes describe OCI containers. Local execution prepares OCI images in Docker. The existing SIF build flow derives an Apptainer artifact for HPC environments. Portable source builds currently execute through the local runtime; Kubernetes needs a registry image, and Slurm needs a completed SIF build. Local preparation does not push images to a registry.

For the API commands below, complete [API connection setup](/docs/tutorials/api-access) first.

## Register and build

Open **Artifacts** and choose **Build artifact**. Enter an artifact ID, semantic version, registry image reference, and architecture. Desktop registers an immutable catalog version, creates a Docker-image-to-SIF build specification, and immediately starts its build run.

To do the same through the API, the server needs an artifact store and an Apptainer builder that can reach the image registry. Register the image, start the build, and keep the returned IDs:

```bash
set -o pipefail
curl --fail-with-body -X POST \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "artifactId":"busybox",
    "version":"1.36",
    "image":"docker.io/library/busybox:1.36",
    "architecture":"amd64"
  }' \
  "$AKOFLOW_API_URL/artifacts/docker/" -o registered-artifact.json || exit 1

BUILD_ID=$(jq -er '.build.id' registered-artifact.json) || exit 1
curl --fail-with-body -X POST -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/artifact-builds/$BUILD_ID/runs/" -o started-build.json || exit 1
RUN_ID=$(jq -er '.id' started-build.json) || exit 1

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/build-runs/$RUN_ID/" | jq '{id,status,error,logs}'
```

The build starts asynchronously. Repeat the last GET until `status` is `completed` or `failed`; if it fails, read `error` and `logs` before retrying. Once completed, download the SIF:

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/build-runs/$RUN_ID/output/" -o "$RUN_ID.sif"
```

## Use a custom build recipe

First upload a build context as multipart form data:

```bash
curl --fail-with-body -X POST -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -F "context=@context.tar.gz" \
  "$AKOFLOW_API_URL/build-contexts/"
```

Then create an immutable build specification at `/artifact-builds/`. It requires `id`, `artifactVersionId`, `contextDigest`, `recipeDigest`, and `cacheKey`; target and recipe fields describe the desired output. A repeated cache key returns the existing build rather than creating a duplicate. The JSON form of `/build-contexts/` only records metadata for bytes already present in the artifact store and requires `digest`, `storageUri`, and a positive `sizeBytes`.

:::warning Credentials and paths
Do not put registry credentials, SSH secrets, or cloud secrets in artifact payloads. Use configured credential references. Browser-local file paths are not server build contexts; upload the bytes or register artifact-store metadata.
:::
