---
title: Inspect artifact locations
description: Inspect recorded executable locations and preparation status on a resource.
---

# Inspect artifact locations

Use this guide after registering or building an executable artifact. It shows the catalog locations recorded for its bytes and whether preparation on a run's target resource finished.

For the API commands below, complete [API connection setup](../../tutorials/api-access) first.

## Find recorded locations and preparation status

Use **Artifacts** to see executable versions, **Artifact locations** to see their recorded URI, digest, and `available` flag, and **Materializations** to see preparation on target resources. The location list reads saved catalog records; `available: true` does not run a fresh storage or network check.

A materialization identifies a variant and digest, target resource and destination path, plus its lifecycle status: `planned`, `reconciling`, `transferring`, `verifying`, `committed`, or `failed`. For a prepared copy, check both `status: "committed"` and that `verifiedDigest` matches `digest`. These are saved observations; listing them does not recheck the destination bytes.

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/artifact-locations/"

curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/artifact-materializations/"
```

Add `--data-urlencode "runId=<your-run-id>"` and `-G` to the second request when you want one run. Execution detail also shows prepared artifacts and transfer activity. Use it to relate catalog identity to the bytes actually made available for an activity.
