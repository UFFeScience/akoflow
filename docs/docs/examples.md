---
id: examples
title: Example catalog
sidebar_label: Examples
description: Runnable AkôFlow scenarios maintained with the current API contracts.
---

Use the examples checked into the repository instead of copying isolated payloads from older documentation. Each scenario keeps its workflow, infrastructure, topology, planning, and execution inputs together.

## Simulation

`examples/simulation/` contains the smallest complete modeled execution:

| File | Purpose |
|---|---|
| `environment.yaml` | Simulation environment, runtime, and resource inventory |
| `scope.yaml` | Environment versions available to planning |
| `topology.yaml` | Network links and transfer characteristics |
| `workflow.yaml` | Versioned activity DAG |
| `plan-request.yaml` | Imported/manual schedule plan envelope |
| `execution-request.yaml` | Simulation execution envelope |

Additional scenarios include:

- `30gb-fanout`: data-intensive fan-out;
- `50core-fanout`: compute-intensive fan-out with a runnable script;
- `stress`: generated workflows for planner and execution stress tests.

Follow [Your first workflow run](guides/workflows/first-run) for Desktop and API instructions.

## Kind and Kubernetes

`examples/kind/` creates a local Kubernetes test environment. It includes the Kind cluster definition, AkôFlow access resources, environment, storage, scope, topology, workflow, plan, and execution request.

Read `examples/kind/README.md` before applying the manifests. Kubernetes access and tokens are environment-specific; never copy credentials into documentation or commit them with an example.

## Real and HPC execution

- `examples/real/workspace-host-convergence/` demonstrates workspace and executable convergence for a real run.
- `examples/slurm/` provides the environment and scope structure for a SLURM-backed environment.
- `examples/connections/` contains connection-oriented environment definitions for Kind, PlaFRIM, and Santos Dumont.

Real infrastructure examples contain identifiers and placeholders that must be adapted to the target installation. Validate connections in Desktop or with the connection-test API before creating the environment.

## Submit an example through the API

Set a base URL that includes the API prefix:

```bash
export AKOFLOW_API_URL="http://127.0.0.1:<port>/akoflow-api"
export AKOFLOW_API_TOKEN="<token>"
```

Then submit the resource to its matching endpoint. For example:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer ${AKOFLOW_API_TOKEN}" \
  -H "Content-Type: application/yaml" \
  --data-binary @examples/simulation/workflow.yaml \
  "${AKOFLOW_API_URL}/workflow-definitions/"
```

Planning and execution request envelopes are deliberately separate from workflow creation. See [Plan a workflow](guides/workflows/planning) and [Execute and monitor a workflow](guides/workflows/executions) before submitting those files.
