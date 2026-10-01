---
id: validation
title: Validating examples against the engine
sidebar_label: Validating examples
slug: /guides/validation
description: How the in-tree validation harness exercises every shipped example through the real AkôFlow API and Desktop UI.
---

# Validating examples against the engine

AkôFlow ships a deterministic, hermetic validation harness that exercises every
example end-to-end through the real API and the Desktop UI. The harness lives
under `internal/validation` and `scripts/validation` in the main repository.
Running it requires only Go; the Playwright smoke test additionally requires
Playwright and a reachable Desktop binary.

## What the harness covers

The harness runs four layers of checks for every example:

1. **Schema decode** — every YAML is fed through the same
   `YAML→JSON→strict-unmarshal` pipeline that the server uses. An undocumented
   field or a renamed DTO fails immediately, before the server boots.
2. **End-to-end HTTP** — for the canonical examples
   (`local/direct-hello`, `simulation`, `slurm/local-fixture`) the harness boots
   a real `akoflow-server` on a loopback port with isolated database,
   instance archive and artifact store roots, then POSTs every artefact in the
   order the example's `run.sh` does and asserts the documented status codes.
3. **Reference drift** — each README must mention `workflow.yaml` (or
   `run.sh`); each `image:` reference must resolve to a `docker/Dockerfile`
   unless the image is published to `ghcr.io/uffescience/` or is a stock
   public image.
4. **External drift detection** — when `AKOFLOW_EXAMPLES_ROOT` points at the
   standalone `akoflow-examples` repository, the same four checks run
   against every fixture there. Any drift fails the build.

## Running the suite

From the repository root:

```sh
# full in-tree validation suite
go test ./internal/validation/...

# inspect which fixtures the harness sees without running the tests
go run ./scripts/validation

# same inspection, but inside the docs workspace (used by docs CI)
npm --prefix docs run test:examples:list

# end-to-end smoke test against the Desktop UI
npm --prefix docs run test:examples:playwright

# external akoflow-examples drift detection
AKOFLOW_EXAMPLES_ROOT=/path/to/akoflow-examples \
  go test ./internal/validation/... -run External
```

The Playwright smoke test captures screenshots into
`docs/.generated/desktop-journey/` for each validated step. It is skipped when
`AKOFLOW_DESKTOP_URL` (default `http://127.0.0.1:5173`) is not reachable, so
CI that does not start the Desktop remains green.

## Local, simulation, and external-runtime examples

AkôFlow examples fall into three categories that the docs surface explicitly:

- **Local** — examples that run on the AkôFlow daemon host with no external
  service. `examples/local/direct-hello` is the canonical first-run fixture.
- **Simulation** — examples that use the deterministic SimGrid runner.
  `examples/simulation`, `examples/simulation/30gb-fanout`, and
  `examples/simulation/50core-fanout` are validated in CI; the heavyweight
  `30gb` and `50core` fixtures take longer to plan but run locally.
- **External runtime** — examples that require an actual cluster or cloud
  account. `examples/slurm/local-fixture` ships disposable `sbatch` and
  `singularity` stubs so it remains hermetic; the
  `akoflow-examples/real/workspace-host-convergence` and
  `akoflow-examples/real/plafrim-kind-cloud-roundtrip` examples require
  real Kubernetes, HPC and SSH proxies and are documented as runnable only
  against a verified environment. They are inspected by the validation
  harness but their `runtimeId` and credential fields are not exercised
  through the API.

## Known drift surfaced by the suite

The validation harness currently reports one real drift:

- `akoflow-examples/real/workspace-host-convergence/plan.yaml` declares
  `runtimeId` at the top level of each assignment. The production
  `PlanAssignment` DTO nests `runtimeId` under `metadata`. The fixture
  must be updated to:

  ```yaml
  assignments:
    - activityId: workspace-host-convergence-k1
      resourceId: kubernetes-environment-entrypoint
      orderOnResource: 0
      metadata:
        runtimeId: kubernetes-environment-kubernetes
  ```

  This is tracked separately and does not block the in-tree validation
  suite.
