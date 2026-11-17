---
title: Validate execution prerequisites
---

Select **Start execution** to create a run from the selected plan. The daemon checks runtime prerequisites for the queued run before launching its activities. Missing images are pulled or built during activity preparation. If a blocking check fails, the run remains visible with the failure reason. Warnings and unverified observations do not require acknowledgment.

The daemon checks configuration, local allocation, workspace storage budgets, artifact compatibility, connection access and declared existing inputs. Simulation checks configuration without probing real infrastructure. The optional preflight report can also inspect cached images and registry manifests.

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

The preflight report is diagnostic. Requests to create executions do not require a report ID or warning acknowledgment. Runtime prerequisites are checked from current state after the run is queued.

Operational errors preserve the `error` message and add `details` with a stable code, stage, optional affected entity, retryability, next action and correlation ID. Retryable preparation is distinct from permission to repeat a scientific workload. Runtime observation failures retain live-process ownership; after three consecutive failures, the supervisor attempts cancellation and retains reconciliation when stopping cannot be confirmed.

## Artifact compatibility

- Local Docker can prepare a pinned OCI image or a portable source context.
- Kubernetes needs an OCI image available to its nodes. A portable source context must first be built and published.
- Slurm needs a completed compatible SIF build and a usable transfer route.

A registry manifest visible to the daemon is evidence of registry access from that daemon, not proof that Kubernetes nodes or a remote cluster can pull it.
