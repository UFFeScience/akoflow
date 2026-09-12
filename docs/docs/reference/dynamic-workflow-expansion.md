---
title: Dynamic workflow expansion
---

# Dynamic workflow expansion

Dynamic activities emit an expansion request; they do not edit a workflow definition. AkôFlow stores three distinct records:

1. the authored, immutable `WorkflowVersion`;
2. append-only `WorkflowExpansion` decisions associated with a source event and optionally an execution run;
3. an `ExpandedWorkflow` projection with its own monotonically increasing `revision`.

The UI must render these server records and must not derive, merge, or edit the expanded graph locally.

## Output contract

Submit `POST /akoflow-api/workflow-versions/{versionId}/expansions/` with:

```json
{
  "executionRunId": "run-42",
  "sourceActivityId": "discover",
  "sourceEventId": "provider-event-107",
  "sequence": 1,
  "activities": [
    {
      "key": "sample-001",
      "activity": {
        "name": "Process sample 001",
        "kind": "task",
        "capabilities": ["real"],
        "command": {"entrypoint": "process-sample"},
        "resources": {},
        "policy": {}
      }
    }
  ],
  "dependencies": [
    {"activityKey": "sample-001", "dependsOnActivityId": "discover", "type": "control"}
  ]
}
```

`sourceEventId` is the replay key. Activity IDs are SHA-256-derived from the workflow version, source event, and activity `key`; clients must not provide or predict them. Repeating the same event returns the existing expansion, and the persistent queue uses the same idempotency boundary.

Use `GET /akoflow-api/workflow-versions/{versionId}/expanded/?executionRunId={runId}` to retrieve the original definition, ordered expansion records, and server-materialized resulting graph together.

## Validation and failure policy

- Every activity key is unique inside an event. All dependencies must resolve to the base graph, a prior accepted expansion, or the current event.
- Self-dependencies and cycles across the complete resulting graph are rejected.
- The default projection limits are 1,000 activities and 4,000 dependencies. Limits are enforced against the complete base-plus-expansion graph.
- Per-run `sequence` is contiguous across applied expansions. Rejected decisions remain in the audit history but do not consume or reserve a sequence, so an out-of-order producer can recover by submitting the expected sequence with a new source event. This serializes concurrent discoveries and makes the projection revision reproducible.
- Invalid output is persisted as a `rejected` expansion with its reason. It does not partially add activities or dependencies. Other already-applied expansions remain valid.
- A rejected source event is immutable; a producer corrects it with a new source event and the sequence currently expected from the applied projection.

## Planning and execution

Schedule plans remain frozen against their `WorkflowVersion`. An accepted expansion creates a new expanded projection revision; it never mutates the version or an existing plan. Activities absent from the frozen plan are not dispatched accidentally. A coordinator must create/select a replacement plan for the new projection before those activities execute. Completed activity observations remain attached to their original run and can be reused only when the replanning policy explicitly selects them.

Static workflows require no expansion records. Their expanded projection has revision `0` and is byte-for-byte equivalent in activities and dependencies to the immutable workflow version.

## Audit and provenance

The provenance catalog exposes `workflow_expansions`, linked to workflow versions, runs, and source activities. Read-only provenance SQL also exposes `workflow_expansions`, `workflow_expansion_activities`, and `workflow_expansion_dependencies`. Metadata and serialized activity definitions remain blocked from generic SQL output; the versioned expanded-workflow API is the public contract for those fields.
