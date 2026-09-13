---

id: evidence-and-provenance
title: Compare a plan with a completed run
sidebar_label: Evidence and provenance
description: How AkôFlow preserves predictions, runtime observations, artifacts, lineage, and audit history.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

AkôFlow keeps a plan's predictions alongside what happened during the run. Compare them to see whether activities took longer than expected, used different resources, or moved data differently.

Use [Trace a result with provenance](../guides/data/provenance) or [Inspect audit events](../guides/data/audit-events) to query the records. This explanation describes why they remain separate.

## Two timelines for one selected plan

<img src={useBaseUrl('/img/architecture/evidence-provenance-timeline.svg')} alt="A selected plan holds predictions; the execution run produces runtime observations; those observations form the execution trace and provenance records." />

The plan retains predicted duration and cost. A task attempt can record its planned and actual resource, runtime, queue time, transfers, and startup time, depending on what the runtime reports. The execution trace combines these observations into run metrics. A completed run does not mean the prediction was accurate.

## What the run records

AkôFlow keeps an identifier for each started activity so it can check its status again after a restart. The identifier depends on the runtime: it may point to a process, container, Kubernetes Job, Slurm job, or simulation event.

Transfer records can show how data reached a task and how many bytes moved. Where the runtime supports it, artifact records show files created or changed by the task. If required output observation fails, a task's zero exit code alone does not establish a valid result.

## Lineage and audit answer different questions

**Provenance** connects scientific entities, produced data, their locations, and the workflow activity that created them. It answers questions such as "which run produced this file?" or "which input lineage fed this result?"

**Audit** records operational and security-relevant actions, such as a user or service changing a record or requesting an operation. It answers "who changed this state and when?" It is not a replacement for data lineage.

## Compare before drawing conclusions

Compare a selected plan with a completed run at the same scope and execution mode. Check assignments, transfer records, activity attempts, and provider conditions before attributing a makespan gap to the scheduling algorithm. The execution trace reports both wall-clock makespan and accumulated activity-stage totals; [interpreting observed timing](./observed-timing) defines the distinction.

## Related material

- [Execution control plane](../engine)
- [Planning and plans](./planning)
- [Network modeling](./network-modeling)
- [Planning and execution state reference](../reference/planning-and-execution-states)
