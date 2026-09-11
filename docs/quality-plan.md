# AkôFlow documentation quality plan

This file is the editorial backlog for preparing the AkôFlow documentation for external open-source users. Update it after each documentation unit. A checked item must point to evidence in the repository or to a recorded verification command; absence of a known defect is not sufficient evidence.

Last audited: 2026-09-11 after the verified SimGrid first-run exercise.

## Editorial contract

Every user-facing page has one primary Diátaxis purpose:

- **Tutorial:** a learning path that starts from stated prerequisites and ends in a result the reader can verify.
- **How-to:** a focused procedure for a reader who already understands the relevant concepts.
- **Reference:** precise facts about interfaces, schemas, fields, defaults, states, limits, and compatibility.
- **Explanation:** the reasoning, architecture, models, and trade-offs behind the system.

Showcases are extended tutorials. They may link to how-to and reference pages, but must not duplicate those pages. Generated endpoint pages are reference material. Provider setup pages are how-to guides. Architecture and scheduling-model pages are explanations.

## Current evidence

- [x] The documentation builds from a clean generated state. Evidence: `cd docs && npm run clear && npm run typecheck && npm run build`, passed on 2026-09-11.
- [x] API endpoint reference is generated from `internal/api/httpserver/httpserver.go`. Evidence: `docs/scripts/generate-api-reference.mjs`; 125 generated endpoint pages in the current tree.
- [x] Provider limitations are stated explicitly. Evidence: `guides/infrastructure/cloud-capacity.md`, `gcp.md`, and `aws.md` distinguish GCP compute provisioning from AWS S3 support.
- [x] HPC concepts and the proxy-aware connection path are documented. Evidence: `guides/infrastructure/hpc-slurm.md` and `guides/operations/interactive-console.md`.
- [x] Existing Showcase download URLs use `raw.githubusercontent.com` and the 50-core bundle was checked against repository files on 2026-09-11.
- [ ] No screenshot markers remain. Current audit: 13 pages still contain `<!-- screenshot: ... -->` markers.
- [ ] All internal links and every downloadable asset pass an automated link check. No link-check script exists yet.
- [ ] Navigation is organized visibly by Tutorial, How-to, Reference, and Explanation. The current sidebar is organized mostly by product domain.
- [ ] Every supported runtime has an end-to-end, independently verified showcase.

## Runtime and provider coverage

| Runtime/provider | Tutorial or showcase | How-to | Reference | Explanation | Status |
| --- | --- | --- | --- | --- | --- |
| SimGrid | Edge–cloud, 30 GB fan-out, 50-core fan-out | Environment/scope/planning pages | API + workflow specification | Runtime and scheduler pages | Partial: verify each bundle end to end |
| Kubernetes | Kind real execution | Environment and execution pages | API + example YAML | Runtime page | Partial: capture current UI and verify outputs |
| SLURM/HPC | None complete | `hpc-slurm.md` | Example environment/scope + API | Runtime page | Missing complete workflow/run showcase |
| Local/direct | First-run material is simulation-focused | Environment and console pages | API | Runtime page | Missing dedicated verified tutorial |
| GCP | None complete | `gcp.md` and cloud capacity | Cloud API reference | Provider support notes | Missing safe, reproducible tutorial and current captures |
| AWS/S3 | None complete | `aws.md` and storage | Storage/cloud credential API | Support matrix | Correctly limited; missing verified S3 procedure |

## Prioritized editorial units

Work on the first unchecked unit only. Do not combine units unless the changes are inseparable.

### P0 — first successful run

- [x] Rewrite `guides/workflows/first-run.md` as one complete SimGrid tutorial. Evidence: the corrected six-file bundle produced run `simulation-example-run-v1` with 3/3 completed activities, two transfers, 120,000,000 transferred bytes, and 21.593 s observed makespan on 2026-09-11; all seven screenshot markers were removed because commands and invariant output provide the clearer verification path.
- [x] Review `getting-started.md` as a short orientation page. Evidence: the page now routes by user goal to installation, the verified first run, infrastructure-specific guides, concepts, troubleshooting, and API/reference material without repeating tutorial commands.
- [ ] Verify installation on a clean supported host. The pending first-start screenshot was removed because text-based preflight checks are more useful and safe. Re-verified on 2026-09-11: the public `v1.0.3` macOS/arm64 server artifact downloads but reports `Binary was compiled with 'CGO_ENABLED=0', go-sqlite3 requires cgo to work`; anonymous inspection of `ghcr.io/uffescience/akoflow-daemon:v1.0.3` still returns `unauthorized`. Re-test after a distributable SQLite-enabled server artifact and public package visibility are fixed.

### P0 — runnable showcases

- [x] Execute and validate the edge–cloud SimGrid showcase through the documented API sequence and Desktop path. Evidence: on 2026-09-11, the versioned six-file bundle completed as `simulation-example-run-v1` with 3/3 activities, two transfers, 120,000,000 bytes, and a 21.593 s observed makespan; the current Desktop Definition view showed the imported three-node DAG and individual SimGrid activity capability. The page now records the IDs, verification values, recovery checks, and equivalent UI/API paths.
- [x] Complete the 30 GB fan-out bundle with an execution request or explicitly reclassify it as an explanation. Evidence: `examples/simulation/30gb-fanout/execution-request.yaml` and `run.sh` now provide the full portable flow; on 2026-09-11 it completed as `simgrid-30gb-fanout-run-v1` with 5/5 activities, 6 transfers, 60,000,000,000 bytes, and a 36.495 s observed makespan. The page records the observed sharing trace and includes a current planning-screen capture.
- [x] Execute and validate the 50-core showcase; confirm 100 settled activities, 50-core scheduling, and evidence rendering. Evidence: the versioned bundle completed as `simgrid-50core-workers-run-v2` on 2026-09-11 with 100/100 activities, 50 distinct M2 core IDs in the saved plan, 20 s planned and observed makespan, and zero transfers/bytes. The Showcase records the reproducible API verification and explains its accumulated execution and queue values.
- [x] Execute and validate the Kind showcase from a clean cluster, including output-file capture. Evidence: on 2026-09-11 `kind-dag-run-v8` completed on an isolated three-node Kind cluster; `prepare` wrote `result.txt`, `process` consumed it and wrote `consumed.txt`, both were 9 bytes with the same SHA-256 checksum, and the recorded workspace transfer moved 9 bytes. The versioned README and Showcase include setup, API/UI paths, verification, PVC/node-affinity recovery, and cleanup.
- [ ] Add one complete SLURM showcase only when a safe test cluster or reproducible local SLURM fixture is available.

### P1 — infrastructure how-to guides

- [x] Create a dedicated SimGrid setup how-to covering resource speed, per-core capacity, topology links, latency, bandwidth, activity duration, and verification. Evidence: `guides/infrastructure/simgrid.md` is grounded in the checked-in six-file edge-to-cloud bundle and the current SimGrid platform/runner implementation; it records field semantics, bit/s versus bytes, path and sharing behavior, API/Desktop procedures, completion invariants, and recovery checks.
- [x] Create a dedicated Kubernetes setup how-to covering kubeconfig/token, namespace/RBAC, runtime binding, storage, image access, validation, troubleshooting, and cleanup. Evidence: `guides/infrastructure/kubernetes.md` is grounded in the verified Kind bundle and current Kubernetes client, adapter, transfer, and terminal implementations; it distinguishes the local broad-permission fixture from a namespace-scoped role and records the additional node-discovery permission, connection precedence, storage/node-affinity constraints, and cleanup checks.
- [x] Verify `hpc-slurm.md` against the current schemas and runtime behavior. Evidence: `guides/infrastructure/hpc-slurm.md` now distinguishes the static catalog example from a real SSH connection, documents the current SLURM connection factory, SSH proxy/host-key configuration, discovery, storage visibility, `sbatch`/`sacct`/fallback behavior, cancellation, and `srun` interactive sessions. Current interface capture remains deferred because the available Desktop surface is locked; the page uses executable configuration and explicit verification steps rather than an unverified visual.
- [ ] Verify `gcp.md` with a disposable GCP project; document exact minimum IAM roles/permissions from observed API calls, provisioning cleanup, and current interface captures.
- [ ] Verify `aws.md` with an S3 test bucket and document the exact storage payload and cleanup without implying EC2 support.

### P1 — reference completeness

- [x] Generate or maintain a field-level environment YAML reference from the authoritative domain types, including required fields, defaults, enum values, and compatibility. Verified against `internal/domain/environment`, `internal/domain/resource`, and the SQLite persistence schema on 2026-09-11.
- [x] Do the same for execution scopes and network topologies. Verified against `internal/domain/environment`, `internal/domain/resource`, the scope/topology repositories, HTTP routes, and SQLite constraints on 2026-09-11.
- [x] Audit the workflow specification against current parsers, especially per-activity simulation duration and data dependencies. `internal/workflow-spec.md` now records importer defaults, exclusive real/simulation capability selection, HEFT/PRISM duration precedence, SimGrid FLOPs conversion, and the fact that data bytes attach only to existing control dependencies; verified against parser, planner, runner, repository constraints, and tests on 2026-09-11.
- [x] Add planning-session, candidate, schedule-plan, and execution-run state diagrams or tables from the authoritative state machines. `reference/planning-and-execution-states.md` records the queue/session/algorithm/candidate/plan/run distinction, transitions, terminal states, API behavior, and task/handle evidence; verified against domain state types, coordinator, queue handlers, supervisor, repositories, and HTTP routes on 2026-09-11.
- [x] Add a documented link checker that validates internal routes, static assets, and Showcase downloads in CI.

### P1 — explanations

- [x] Separate `concepts.md`, `engine.md`, and `runtimes.md` into focused explanations for architecture, planning, execution, network modeling, provenance, and plan-versus-observed evidence. The preserved entry points now have narrow responsibilities and link to dedicated planning, network-modeling, and evidence/provenance explanations; verified against planning, execution, transfer, and runtime contracts on 2026-09-11.
- [x] Explain PRISM versus HEFT without promising that one algorithm always wins; distinguish candidate generation, common evaluation, objective choice, and prediction fidelity. `explanations/prism-and-heft.md` documents the current separate HEFT and PRISM evaluation paths, bounded PRISM beam search, objective ordering, network/interference model, and the absence of a cross-algorithm shared evaluator; verified against the scheduler implementations and tests on 2026-09-11.
- [x] Explain network flow, contention, accumulated stage time, queue time, makespan, cost, and why accumulated activity values are not wall-clock totals. `explanations/observed-timing.md` distinguishes current PRISM modeled contention from persisted run observations and grounds every stage and aggregate in the execution supervisor and run-feed queries, verified on 2026-09-11.

### P2 — visual and editorial cleanup

- [x] Replace or remove screenshot markers in the 15 affected pages. All legacy screenshot markers were removed on 2026-09-11. Checked-in Desktop captures are embedded in the environment, workflow-definition, and planning guides; the planning capture was produced from the current Desktop on 2026-09-11 and reviewed in light and dark themes.
- [ ] Reorganize `sidebars.ts` by Diátaxis purpose while retaining product-area landing pages and redirects for existing URLs.
- [ ] Remove or redirect legacy overlapping pages (`examples.md`, `user-guide.md`, `internal/api.md`, and legacy CLI material) after checking inbound links.
- [ ] Perform a final plain-language edit for repeated introductions, unsupported claims, inconsistent terminology, and generated-sounding filler.

## Completion gate

The documentation is ready for external users only when all of the following are evidenced:

1. A new user can install AkôFlow and complete the first verified execution without undocumented knowledge.
2. The sidebar clearly separates tutorials, how-to guides, reference, and explanations.
3. SimGrid, Kubernetes, SLURM/HPC, GCP, and AWS are documented independently and match implemented support.
4. Every runtime claimed as supported has at least one end-to-end verified showcase; partial examples are not labeled as showcases.
5. API endpoints and YAML formats have verified reference coverage.
6. No screenshot marker, broken internal link, or broken download remains.
7. Typecheck, clean production build, automated link validation, and visual checks of changed pages pass.
8. A final coverage report separates product limitations from remaining documentation gaps.

## Short remaining-gap list

1. The Kind and SLURM showcases remain unverified end to end; the SimGrid edge, 30 GB fan-out, and 50-core bundles are complete.
2. SLURM has no complete showcase, and SimGrid/Kubernetes lack dedicated infrastructure how-to pages.
3. Additional focused Desktop captures would improve the remaining infrastructure and run-inspection guides. The current planning capture is reviewed in light and dark themes; the automated link checker is in CI.
4. Release installation is not externally verifiable until the macOS server artifact and anonymous GHCR image access are fixed.
5. Navigation is not yet organized by the four Diátaxis purposes.
