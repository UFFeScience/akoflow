---
title: Artifacts, storage, and builds
description: Browse data, register executable artifacts, and follow materialization and build runs in Desktop or through the API.
---

# Artifacts, storage, and builds

AkôFlow separates **scientific data** from **executable artifacts**. Files produced by a workflow can be promoted to the scientific record. Executable artifacts are immutable, versioned definitions whose bytes may have verified locations or be materialized on a target resource.

The Desktop is the easiest way to perform these operations. Every view described below uses the same HTTP API, so the API examples are suitable for scripts and integrations.

## Browse storage

In Desktop, open **Infrastructure**, select an environment, then open **Storage**. Choose a storage card and one of its declared roots. Entries are loaded lazily; opening this view does not scan an entire filesystem.

The actions offered for an entry depend on the storage capabilities reported by discovery. The current interface can inspect an entry, download a file, archive a directory for download, calculate a checksum, copy to another storage, promote a file, delete an entry, and start or inspect an index run. A storage may be read-only or visible only from a login node.

List the storage resources for an environment and browse a directory:

```bash
curl -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/akoflow-api/environments/$ENVIRONMENT_ID/storages/"

curl -G -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  --data-urlencode "path=/shared/project" \
  --data-urlencode "limit=100" \
  "$AKOFLOW_URL/akoflow-api/storages/$STORAGE_ID/entries/"
```

Use the returned `nextCursor` as `cursor` to continue when the response is paginated. Paths are interpreted within a root allowed by the storage adapter; do not assume host filesystem semantics.

Calculate a digest or queue a copy:

```bash
curl -X POST -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"path":"/shared/project/result.csv"}' \
  "$AKOFLOW_URL/akoflow-api/storages/$STORAGE_ID/checksum/"

curl -X POST -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"path":"/shared/project/result.csv","destinationStorageId":"storage-archive"}' \
  "$AKOFLOW_URL/akoflow-api/storages/$STORAGE_ID/copies/"
```

Copy and archive operations return `202 Accepted`. Download creation returns a run that can be polled at `/storage-downloads/{downloadId}/`; fetch completed content from `/storage-downloads/{downloadId}/content/`.

## Promote existing files

Use the entry menu in **Storage** to promote an existing file. **Promote data** associates it with workflow, run, and activity context. **Promote artifact** registers executable content in the artifact catalog.

The minimal API calls are:

```bash
curl -X POST -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "path":"/shared/project/result.csv",
    "workflowVersionId":"workflow-version-1",
    "runId":"run-1",
    "activityId":"analyse"
  }' \
  "$AKOFLOW_URL/akoflow-api/storages/$STORAGE_ID/promote-data/"

curl -X POST -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "path":"/shared/bin/model.sif",
    "name":"model",
    "version":"1.0.0",
    "scope":"project",
    "scopeId":"project-1"
  }' \
  "$AKOFLOW_URL/akoflow-api/storages/$STORAGE_ID/promote-artifact/"
```

If `id` is omitted, the server generates one. Supply meaningful provenance identifiers when promoting scientific data; an anonymous promotion is valid at the transport layer but loses useful context.

## Build an executable from a Docker image

Open **Artifacts** and choose **Build artifact**. Enter an artifact ID, semantic version, registry image reference, and architecture. Desktop registers an immutable catalog version, creates a Docker-image-to-SIF build specification, and immediately starts its build run.

The equivalent two-call API flow is:

```bash
REGISTERED=$(curl -sS -X POST \
  -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "artifactId":"busybox",
    "version":"1.36",
    "image":"docker.io/library/busybox:1.36",
    "architecture":"amd64"
  }' \
  "$AKOFLOW_URL/akoflow-api/artifacts/docker/")

# Read .build.id from REGISTERED, then start it:
curl -X POST -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/akoflow-api/artifact-builds/$BUILD_ID/runs/"
```

The Docker registry pull and SIF conversion run in the build service, not in the browser. Poll `/build-runs/{runId}/`. When complete, `/build-runs/{runId}/output/` streams the SIF file.

For custom recipes, first upload a build context as multipart form data:

```bash
curl -X POST -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  -F "context=@context.tar.gz" \
  "$AKOFLOW_URL/akoflow-api/build-contexts/"
```

Then create an immutable build specification at `/artifact-builds/`. It requires `id`, `artifactVersionId`, `contextDigest`, `recipeDigest`, and `cacheKey`; target and recipe fields describe the desired output. A repeated cache key returns the existing build rather than creating a duplicate. The JSON form of `/build-contexts/` only records metadata for bytes already present in the artifact store and requires `digest`, `storageUri`, and a positive `sizeBytes`.

## Locations and materializations

Use **Artifacts** to see executable versions, **Artifact locations** to see verified byte locations, and **Materializations** to see preparation on target resources.

A materialization identifies a variant and digest, target resource and destination path, plus its lifecycle status: `planned`, `reconciling`, `transferring`, `verifying`, `committed`, or `failed`. A materialization is considered committed only when `verifiedDigest` equals the requested `digest`.

```bash
curl -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  "$AKOFLOW_URL/akoflow-api/artifact-locations/"

curl -G -H "Authorization: Bearer $AKOFLOW_TOKEN" \
  --data-urlencode "runId=$RUN_ID" \
  "$AKOFLOW_URL/akoflow-api/artifact-materializations/"
```

Execution detail also shows prepared artifacts and transfer activity. Use it to relate catalog identity to the bytes actually made available for an activity.

:::warning Credentials and paths
Do not put registry credentials, SSH secrets, or cloud secrets in artifact payloads. Use configured credential references. Browser-local file paths are not server build contexts; upload the bytes or register artifact-store metadata.
:::
