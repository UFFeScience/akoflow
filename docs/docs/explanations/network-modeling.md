---

id: network-modeling
title: Network modeling and data movement
sidebar_label: Network modeling
description: How data dependencies, topology links, routes, and observed transfers relate to a workflow plan.
---

import useBaseUrl from '@docusaurus/useBaseUrl';

Network modeling lets planning distinguish a local dependency from data that must cross a resource boundary. It starts with bytes declared by the workflow, but its result depends on the selected resource assignments and the directed topology included in the execution scope.

This is an explanation of the model. Use [SimGrid modeling](../guides/infrastructure/simgrid) to configure bandwidth and latency, or the [topology reference](../reference/execution-scopes-and-topologies) for the exact document fields.

## From dependency to possible flow

<img src={useBaseUrl('/img/architecture/network-flow-model.svg')} alt="A control dependency orders producer and consumer activities, a data dependency declares logical bytes, and a directed topology route is considered when their selected resources differ." />

The control dependency makes the consumer wait for the producer. The matching data dependency gives the planner a logical byte volume. If a selected plan puts both activities on the same resource, no network transfer time is added for that edge. If they are on different resources, the topology is consulted for a route.

The portable workflow importer deliberately requires the matching control edge for data bytes to participate in scheduling. A data declaration without that ordering relationship remains data metadata, not an implicit workflow edge.

## What a topology models

A `NetworkLink` is directed. It identifies source and target resources and can carry bandwidth in **bits per second**, latency in seconds, byte price, whether the reverse direction is available, a sharing group, and a transfer concurrency limit. A bidirectional link makes the same link available in reverse; it does not create a second independently configured route.

PRISM precomputes routes from the frozen topology and includes communication in its candidate evaluation. Its shared-network evaluator can account for known overlapping flows on a route. HEFT's baseline scheduling path uses the direct matching link lookup. Neither behavior alone guarantees that one algorithm will produce the better observed run.

## Planned route versus executed transfer

The plan predicts transfer time for each assignment. At execution, preparation selects a concrete strategy such as a verified existing copy, shared storage, destination pull, source push, gateway, runtime-local, or direct-runtime transfer. The resulting transfer observation can record source, target, logical and network bytes, timing, cost, strategy, and route.

That distinction matters: the model describes a possible network penalty for a placement; the observation says how bytes were actually made available. Shared storage or a verified existing copy may satisfy a dependency without a new network transfer.

## Units and a small example

For a 10 GiB data dependency over a 10 Gbit/s link, the raw serialization time is roughly eight seconds before latency and sharing. `10 GiB` is a byte volume; `10 Gbit/s` is a bit rate, so the rate is divided by eight before comparing it with bytes. The observed elapsed time can be higher because of setup, route selection, sharing, or provider behavior.

Inspect a completed run's transfer records alongside the assignment and plan prediction. The [30 GB network fan-out Showcase](../showcase/network-fanout) provides a checked-in topology and workflow where these effects are intentional.

## Related material

- [Workflow simulation semantics](../internal/workflow-spec)
- [Execution scopes and topologies reference](../reference/execution-scopes-and-topologies)
- [Plan-versus-observed evidence](./evidence-and-provenance)
