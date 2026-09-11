---
id: evidence-and-provenance
title: Plan-versus-observed evidence and provenance
sidebar_label: Evidence and provenance
description: How AkôFlow preserves predictions, runtime observations, artifacts, lineage, and audit history.
---

AkôFlow does not overwrite a plan with a completed run. It preserves the prediction used to choose a placement and records the execution evidence beside it. This makes disagreement inspectable: it can indicate an inaccurate model, an unexpected runtime condition, or a different data-preparation path.

Use [Provenance and audit](../guides/data/provenance-and-audit) to query the records. This page explains why the records are separate.

## Two timelines for one selected plan

```text
selected plan
  assignments + predicted metrics + predicted timing
                         |
                         v
                   execution run
                         |
 task attempts + handles + transfers + logs + artifact manifests
                         |
                execution trace and provenance records
```

The plan retains predicted makespan and cost. Each task attempt records its planned and allocated resource, runtime, queue, transfer, interference, and overhead timing where available. The execution trace combines task and transfer observations into observed metrics. A completed trace marks the observed result feasible; it does not certify that the prediction was accurate.

## Evidence follows the runtime boundary

The supervisor persists a runtime handle after starting an activity. The handle can identify a process, Kubernetes Job, Docker container, Slurm job, or simulation event without exposing provider-specific formats to the rest of the control plane. Reinspection uses that saved handle during recovery.

Data preparation and output observation are evidence too. Transfer records can capture the route and actual bytes moved. Artifact manifests capture created or changed workspace files where the adapter supports observation. An observation failure can make a zero-exit task untrustworthy when outputs are required.

## Lineage and audit answer different questions

**Provenance** connects scientific entities, produced data, their locations, and the workflow activity that created them. It answers questions such as "which run produced this file?" or "which input lineage fed this result?"

**Audit** records operational and security-relevant actions, such as a user or service changing a record or requesting an operation. It answers "who changed this state and when?" It is not a replacement for data lineage.

## Compare before drawing conclusions

Compare a selected plan with a completed run at the same scope and execution mode. Check assignments, transfer records, activity attempts, and provider conditions before attributing a makespan gap to the scheduling algorithm. The execution trace reports both wall-clock makespan and accumulated activity-stage totals; interpret those as different measurements.

## Related material

- [Execution control plane](../engine)
- [Planning and plans](./planning)
- [Network modeling](./network-modeling)
- [Planning and execution state reference](../reference/planning-and-execution-states)
