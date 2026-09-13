---
title: Inspect artifact locations
description: Check where executable bytes are stored and whether preparation on a resource finished.
---

# Inspect artifact locations

Use this guide after registering or building an executable artifact. It shows where AkôFlow has verified its bytes and whether the executable was prepared on the resource selected for a run.

For the API commands below, complete [API connection setup](../../tutorials/api-access) first.

## Find verified bytes and preparation status

Use **Artifacts** to see executable versions, **Artifact locations** to see verified byte locations, and **Materializations** to see preparation on target resources.

A materialization identifies a variant and digest, target resource and destination path, plus its lifecycle status: `planned`, `reconciling`, `transferring`, `verifying`, `committed`, or `failed`. A materialization is considered committed only when `verifiedDigest` equals the requested `digest`.

```bash
curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/artifact-locations/"

curl -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/artifact-materializations/"
```

Add `--data-urlencode "runId=<your-run-id>"` and `-G` to the second request when you want one run. Execution detail also shows prepared artifacts and transfer activity. Use it to relate catalog identity to the bytes actually made available for an activity.
