---
title: Browse and manage storage
description: Browse approved storage paths and perform supported file operations.
---

Use **Storage** to browse approved roots and act on files. Available actions depend on the driver and storage settings. Try the intended path before relying on a catalog status.

The current S3 browser sends unsigned requests, so a private bucket may fail to open even when an S3 transfer works with server credentials. See [AWS and S3 support](./aws) for that distinction and the current cloud limits.

For the API commands on this page, complete [API connection setup](../../tutorials/api-access) first.
The IDs `hpc`, `hpc-scratch`, and `archive-store` and the `/scratch/project-a` paths below are examples. Replace them with an environment, storage IDs, and approved paths returned by your own server before running a command.
For a self-managed daemon browsing local files, first configure the root as shown in the [environment YAML reference](../../reference/environment-yaml#storage).

## Browse files

### Using AkôFlow Desktop

1. Open an environment and select **Storage**.
2. Choose a discovered storage in the left column.
3. Select an approved root and navigate folders with the breadcrumb.
4. Use **Refresh** to reload the current listing. Use **Index** only when indexing is enabled for that storage.

Entries load for the selected path; opening Storage does not scan the entire filesystem. Read/write badges reflect the available driver and storage settings. Compute-node visibility reflects the `shared` setting or a runtime binding, not a fresh access test on a compute node.

### Using the API

```bash
# Discover storage IDs for an environment
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environments/hpc/storages/"

# Inspect approved roots
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/storages/hpc-scratch/roots/"

# Browse one path; preserve nextCursor when the response is paginated
curl --fail-with-body -G -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  --data-urlencode 'path=/scratch/project-a' --data-urlencode 'limit=100' \
  "$AKOFLOW_API_URL/storages/hpc-scratch/entries/"
```

Do not construct paths outside the returned roots. The server validates the requested path against storage policy.

## Download, archive, copy, and verify

### Using AkôFlow Desktop

The actions column can download a file, archive and download a directory, copy an entry to another storage, calculate a checksum, or delete an entry. Buttons are disabled when the selected storage is unhealthy or lacks the required capability. Deletion asks for confirmation and cannot be undone by AkôFlow.

### Using the API

```bash
# Prepare a file download (POST /archives/ for a directory)
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST \
  "$AKOFLOW_API_URL/storages/hpc-scratch/downloads/" \
  -d '{"path":"/scratch/project-a/result.csv","id":"download-result-1"}'

# Copy to another registered storage
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST \
  "$AKOFLOW_API_URL/storages/hpc-scratch/copies/" \
  -d '{"path":"/scratch/project-a/result.csv","destinationStorageId":"archive-store","id":"copy-result-1"}'

# Calculate a checksum
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST \
  "$AKOFLOW_API_URL/storages/hpc-scratch/checksum/" \
  -d '{"path":"/scratch/project-a/result.csv"}'

# Archive a directory on the same storage
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST \
  "$AKOFLOW_API_URL/storages/hpc-scratch/archives/" \
  -d '{"path":"/scratch/project-a","id":"archive-project-a-1"}'
```

The file-download request returns a `ready` record for the path; it does not freeze the file's bytes. The content endpoint opens that path when you fetch it. If the file can change, compare the downloaded file with a fresh checksum from the storage request above.

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/storage-downloads/download-result-1/content/" -o result.csv

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/storage-downloads/copy-result-1/"
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/storage-downloads/archive-project-a-1/"
```

Compare the downloaded file's SHA-256 with the `checksum` returned by the source request: use `sha256sum result.csv` on Linux, `shasum -a 256 result.csv` on macOS, or `Get-FileHash result.csv -Algorithm SHA256` in Windows PowerShell. The API checksum includes a `sha256:` prefix.

The copy runs in the background at the same path in the destination storage; wait for `completed` before using it. The archive writes a `.tar.gz` beside the directory. When its record reports `ready`, fetch `/storage-downloads/archive-project-a-1/content/` to save the archive.

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/storage-downloads/archive-project-a-1/content/" \
  -o project-a.tar.gz
```

## Register an existing file

### Using AkôFlow Desktop

Use **Register as DataObject** for a file that should enter the workflow data model. For a `.sif` file, **Register executable artifact** is also available. The current quick actions register the selected path with generated identifiers; use the API when you need explicit lineage or artifact metadata.

### Using the API

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST \
  "$AKOFLOW_API_URL/storages/hpc-scratch/promote-data/" \
  -d '{"path":"/scratch/project-a/result.csv","id":"data-result-1"}'

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' -X POST \
  "$AKOFLOW_API_URL/storages/hpc-scratch/promote-artifact/" \
  -d '{"path":"/scratch/images/solver.sif","id":"solver-sif-1","name":"Solver","version":"1.2.0","scope":"environment","scopeId":"hpc"}'
```

Promotion registers the existing path; it does not upload or move the file. Add `workflowVersionId`, `runId`, and `activityId` to the data request only when you have matching existing records and want to associate the file with them.
