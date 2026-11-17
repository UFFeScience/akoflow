---
title: Validate execution prerequisites
---

Desktop validates the selected execution before starting it. Select **Validate execution**, review every check, correct blockers and explicitly acknowledge warnings or requirements that could not be verified. Then select **Start execution**. Changing the run ID or selected plan clears the review.

The daemon checks configuration, local allocation, workspace storage budgets, artifact/runtime compatibility, configured connection access and declared existing input files. It inspects locally available OCI images or reads registry manifests without pulling images. Portable source contexts are checked and BuildKit workers inspected; actual build success remains unverified until preparation. Kubernetes node registry access and resources that do not exist until provisioning remain runtime checks.

Checks have individual five-second deadlines and a thirty-second report budget. A timeout blocks launch. Results distinguish passed, warning, blocked and unverified; unknown capacity is never displayed as sufficient storage. Simulation validates configuration without probing real infrastructure.

Reports are persisted using the existing event store, contain sanitized observations rather than credentials or execution payloads, and expire after two minutes. Old reports beyond the most recent 1,000 are pruned once they are at least one day old. A report is bound to the submitted workflow, plan, scope, resources, runtimes and execution mode. It does not reserve capacity. The daemon checks critical prerequisites again before queueing an execution.

## Declare existing input assertions

External files cannot be inferred reliably from shell commands. Add `requiredInputs` to an activity's existing metadata to assert that a file must already exist on configured storage:

```yaml
metadata:
  requiredInputs:
    - storageId: input-storage
      path: /data/input.csv
```

These are read-only assertions; they do not stage files or change workspace paths. A failed storage lookup blocks launch. Omit secrets from metadata. An explicitly empty list declares that no external input assertion is required.

An input produced by a declared direct control or data predecessor can instead carry `producerActivityId`. Such files are checked by normal output/preparation gates when produced, rather than required before the workflow starts:

```yaml
metadata:
  requiredInputs:
    - producerActivityId: prepare
      path: output.csv
```

Undeclared external input requirements are reported as unverified. The platform cannot prove that arbitrary user commands will succeed.

## API and MCP

`POST /execution-preflights/` accepts the same execution request as `POST /execution-runs/` and returns a completed bounded report. `GET /execution-preflights/:id/` reads it. MCP exposes the corresponding daemon operations.

To launch a reviewed request, include `preflightId` and, when warnings were reviewed, `acknowledgeWarnings: true`. Expired or changed request receipts are rejected. Real API/MCP execution requests without a receipt still undergo fresh checks and must explicitly acknowledge warnings; they cannot bypass blockers. Existing simulation callers retain configuration-only execution behavior.

Operational errors preserve the `error` message and add `details` with a stable code, stage, optional affected entity, retryability, next action and correlation ID. Retryable preparation is distinct from permission to repeat a scientific workload. Runtime observation failures retain live-process ownership; after three consecutive failures, the supervisor attempts cancellation and retains reconciliation when stopping cannot be confirmed.

## Artifact compatibility

- Local Docker can prepare a pinned OCI image or a portable source context.
- Kubernetes needs an OCI image available to its nodes. A portable source context must first be built and published.
- Slurm needs a completed compatible SIF build and a usable transfer route.

A registry manifest visible to the daemon is evidence of registry access from that daemon, not proof that Kubernetes nodes or a remote cluster can pull it.
