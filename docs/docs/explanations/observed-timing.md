---

title: Interpreting observed timing and cost
sidebar_label: Observed timing and cost
description: The difference between flows, queue and stage totals, makespan, and observed execution cost.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

Execution evidence contains both a wall-clock result and accumulated activity
measurements. They answer different questions. A large accumulated transfer or
queue total does not by itself mean that the workflow took that many seconds on
the clock, because activities and transfers can overlap.

Use this page when reading a run detail, a planning-versus-execution comparison,
or an experiment chart. It explains the current persisted metrics; it does not
replace [network modeling](./network-modeling) or the [execution state reference](../reference/planning-and-execution-states).

## A flow is a scheduled movement of data

A **network flow** exists when a control dependency also has a data dependency,
the producer and consumer are assigned to different resources, and the selected
route requires movement. It has a producer, consumer, source resource, target
resource, logical byte volume, and a route. During a completed execution,
AkôFlow persists a `DataTransfer` observation with start/finish time, duration,
cost, source/target, strategy, route, logical bytes, and network bytes when the
runtime reports them.

The link bandwidth is in bits per second while dependency size is in bytes. For
a single 10 GiB flow over a 10 Gbit/s link, the raw payload time is roughly
eight seconds before latency. A multi-hop route adds latency for its hops and is
limited by its effective available bandwidth.

## Contention means simultaneous users of a bottleneck

Two flows contend when they overlap and use a shared bottleneck. In the current
PRISM complete-state evaluator, that can be a shared route hop, a shared source
resource, or a shared target resource. It divides modeled bandwidth among the
active users. The model is event-based: a flow begins after route latency, then
its remaining bytes progress at the current shared rate until another task or
flow event changes the set of active users.

This is not a claim that every real runtime reports network contention as a
separate observed number. It is a planning-model effect used by PRISM. Inspect
the actual transfer records to determine whether a completed run moved the
expected bytes and how long that movement lasted.

## Four activity-stage timings

Each task attempt has timing fields that describe stages around execution.

| Metric | Current meaning | When it is populated |
|---|---|---|
| Runtime | `finishedAt − executionStartedAt`; `executionStartedAt` is container start when reported, otherwise handle start | A completed runtime handle |
| Queue | `handle.startedAt − submittedAt` | Only when the adapter supplies `submittedAt` metadata |
| Overhead | `containerStartedAt − handle.startedAt` | Only when the adapter supplies container-start metadata |
| Transfer | Sum of transfer-run durations prepared for that task | When its preparation gate reports transfers |

`InterferenceSeconds` is also a persisted task and run-breakdown field. The
current supervisor does not derive it from a runtime handle, so zero means "no
observed value was recorded" rather than proof that CPU interference was absent.
That is different from PRISM's predicted CPU-interference slowdown.

## Accumulated stage time is not makespan

For a completed workflow run, the control plane calculates observed makespan as:

```text
last completed task finish − first completed task start
```

The run feed separately sums per-task runtime, queue, interference, and
overhead, and separately sums observed transfer durations and transferred bytes.
Those sums are **accumulated stage time**. Parallel work is counted once for each
activity that experienced it.

<img src={useBaseUrl('/img/architecture/accumulated-time.svg')} alt="Two ten-second activities run concurrently. Makespan is ten seconds, while accumulated compute time is twenty seconds." />

The same applies to concurrent transfers and queue waits. Use makespan to answer
"how long did the workflow take?" Use accumulated values to answer "where did
the activity work and waiting occur across the whole run?"

## Cost is an observed accounting model, not a bill

For completed execution traces, task cost is task runtime multiplied by the
assigned resource's `pricePerSecond`. The trace also includes observed transfer
cost. For an allocated cloud instance, the control plane adds the idle portion
of the resource active window plus persistent-disk price when the resource
metadata has `diskPricePerGiBMonth`.

This is an internal cost model. A provider invoice can differ because it may use
different billing periods, minimum charges, taxes, discounts, network rules, or
unmodeled services. Compare a plan's predicted cost with the run's observed
modelled cost only when they use the same resource price and scope.

## Reading a plan-versus-observed gap

1. Confirm the plan and run use the intended workflow version, execution scope,
   and mode.
2. Compare assigned and allocated resources; a different allocation changes the
   interpretation of the prediction.
3. Check individual task queue, overhead, runtime, and transfer fields before
   using an aggregate chart.
4. Inspect transfer source, target, bytes, strategy, and route. A shared copy or
   existing verified data can explain a lower-than-predicted network cost.
5. Treat a zero optional stage field as unavailable unless the selected runtime
   is known to report that field.

## Related material

- [PRISM and HEFT: search, objectives, and prediction](./prism-and-heft)
- [Plan-versus-observed evidence and provenance](./evidence-and-provenance)
- [30 GB network fan-out Showcase](../showcase/network-fanout)
