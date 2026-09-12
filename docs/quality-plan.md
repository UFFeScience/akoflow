# AkôFlow documentation quality plan

This file is the editorial backlog for preparing the AkôFlow documentation for external open-source users. Update it after each documentation unit. A checked item must point to evidence in the repository or to a recorded verification command; absence of a known defect is not sufficient evidence.

Last audited: 2026-09-12. The current editorial pass found an unresolved Desktop first-workflow gap and generated-reference defects. Earlier checks below describe their historical scope, not a clean completion gate.

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
- [x] Provider limitations are stated explicitly. Evidence: `guides/infrastructure/cloud-capacity.md`, `gcp.md`, and `aws.md` distinguish GCP compute provisioning from partial object-storage support. The current GCS connector rejects direct `gs://` transfer, and saved AWS credentials are not wired to the S3 transfer connector.
- [x] GCP catalog and provisioning access are documented as source-audited behavior rather than an unverified IAM recipe. Evidence: `guides/infrastructure/gcp.md`, `internal/provider/cloud/gcp/catalog.go`, and `internal/provider/cloud/terraform/runner.go`; a disposable-project validation remains required.
- [x] HPC concepts and the proxy-aware connection path are documented. Evidence: `guides/infrastructure/hpc-slurm.md` and `guides/operations/interactive-console.md`.
- [x] Existing Showcase download URLs use `raw.githubusercontent.com` and the 50-core bundle was checked against repository files on 2026-09-11.
- [x] No screenshot markers remain. `rg '<!--\\s*screenshot:' docs/docs` returned no matches on 2026-09-11; relevant guides now use checked-in captures or executable verification steps.
- [x] All internal links and every downloadable asset pass an automated link check. Evidence: `docs/scripts/check-links.mjs` and `.github/workflows/docs-checks.yaml`; the latest check covered 368 local route/asset links and 53 Showcase downloads on 2026-09-12.
- [x] Navigation separates Tutorials, How-to guides, Explanations, Reference, and Developing AkôFlow. Evidence: `docs/sidebars.ts`; at 390 × 844 the built site's menu opened, showed the new first-workflow link, and had no horizontal overflow or page error on 2026-09-12.
- [x] Desktop is the only end-user installation and download path; self-managed server deployment is a separate operator guide. Evidence: `installation.md`, `downloads.md`, `guides/operations/server-instance.md`, and `static/examples/server-instance/compose.yaml`; Compose interpolation, documentation typecheck, production build, and link check passed on 2026-09-12.
- [ ] Every supported runtime has an end-to-end, independently verified showcase.

## Runtime and provider coverage

| Runtime/provider | Tutorial or showcase | How-to | Reference | Explanation | Status |
| --- | --- | --- | --- | --- | --- |
| SimGrid | Edge–cloud, 30 GB fan-out, 50-core fan-out | Environment/scope/planning pages | API + workflow specification | Runtime and scheduler pages | Partial: verify each bundle end to end |
| Kubernetes | Kind real execution | Environment and execution pages | API + example YAML | Runtime page | Partial: capture current UI and verify outputs |
| SLURM/HPC | Local SLURM batch fixture | `hpc-slurm.md` | Fixture YAML + API | Runtime page | Complete for the local adapter boundary; real cluster configuration still requires site validation |
| Local/direct | Local direct execution | Environment and console pages | API | Runtime page | Complete: isolated daemon run produced an observed filesystem artifact on 2026-09-12 |
| GCP | None complete | `gcp.md` and cloud capacity | Cloud API reference | Provider support notes | Missing safe, reproducible tutorial and current captures |
| AWS/S3 | None complete | `aws.md` and storage | Storage/cloud credential API | Support matrix | Partial: transfer connector uses server environment credentials; saved AWS credential is not wired; no verified AWS procedure |

## Prioritized editorial units

Work on the first unchecked unit only. Do not combine units unless the changes are inseparable.

### P0 — first successful run

- [x] Rewrite `guides/workflows/first-run.md` as one complete SimGrid tutorial. Evidence: the corrected six-file bundle produced run `simulation-example-run-v1` with 3/3 completed activities, two transfers, 120,000,000 transferred bytes, and 21.593 s observed makespan on 2026-09-11; all seven screenshot markers were removed because commands and invariant output provide the clearer verification path.
- [x] Review `getting-started.md` as a short orientation page. It leads to the verified local Desktop run and identifies the SimGrid tutorial as a separate API path.
- [x] Create and verify a Desktop-only first-workflow path to visible results. The extracted v1.0.8 Linux Desktop package, launched with Docker access, created a local environment, workflow, scope, manual plan, and real run `run-1789254029488`; activity exit code was 0 and its generated-file evidence recorded `result.txt` (20 B, SHA-256 `8dcc517ee6ace065324746726b7f6b5cd67b8c22956ec75bd8b713a0aeaca070`). `guides/workflows/first-local-run.md` now gives the UI steps. Clean package-manager and other-platform installation remain separate validation work.
- [ ] Verify installation on a clean supported host. The pending first-start screenshot was removed because text-based preflight checks are more useful and safe. The next tagged release must attach version-matched Desktop installers plus SHA-256-checked runtime archives for each supported architecture; verify that Desktop loads those release assets locally without a container-registry pull.

Installation progress on 2026-09-12: the v1.0.8 Linux DEB was downloaded in full, its asset SHA-256 and package metadata checked, and its extracted application opened with a fresh profile. Desktop loaded the version-matched runtime archives and reached successful daemon/Docker/BuildKit preflight. Real screenshots and Desktop/API onboarding tutorials now live in `installation.md` and `tutorials/`. This does not close the clean-host, package-manager or macOS/Windows validation requirement. See `onboarding-verification-2026-09-12.md` for evidence and scope.

### P0 — runnable showcases

- [x] Execute and validate the edge–cloud SimGrid showcase through the documented API sequence. Evidence: on 2026-09-11, the six-file bundle completed as `simulation-example-run-v1` with 3/3 activities, two transfers, 120,000,000 bytes, and a 21.593 s observed makespan. Desktop showed the imported three-node DAG and individual SimGrid activity capability, but Desktop-only submission of the full bundle remains unverified.
- [x] Complete the 30 GB fan-out bundle with an execution request or explicitly reclassify it as an explanation. Evidence: `examples/simulation/30gb-fanout/execution-request.yaml` and `run.sh` now provide the full portable flow; on 2026-09-11 it completed as `simgrid-30gb-fanout-run-v1` with 5/5 activities, 6 transfers, 60,000,000,000 bytes, and a 36.495 s observed makespan. The page records the observed sharing trace and includes a current planning-screen capture.
- [x] Execute and validate the 50-core showcase; confirm 100 settled activities, 50-core scheduling, and evidence rendering. Evidence: the versioned bundle completed as `simgrid-50core-workers-run-v2` on 2026-09-11 with 100/100 activities, 50 distinct M2 core IDs in the saved plan, 20 s planned and observed makespan, and zero transfers/bytes. The Showcase records the reproducible API verification and explains its accumulated execution and queue values.
- [x] Execute and validate the Kind showcase from a clean cluster, including output-file capture. Evidence: on 2026-09-11 `kind-dag-run-v8` completed on an isolated three-node Kind cluster; `prepare` wrote `result.txt`, `process` consumed it and wrote `consumed.txt`, both were 9 bytes with the same SHA-256 checksum, and the recorded workspace transfer moved 9 bytes. The versioned README and Showcase include setup, API/UI paths, verification, PVC/node-affinity recovery, and cleanup.
- [x] Execute and validate a local/direct showcase, including observed output-file capture. Evidence: `examples/local/direct-hello` completed through an isolated daemon on 2026-09-12 as `local-direct-hello-run-v1`; its one activity created `result.txt` (24 bytes, SHA-256 `602b3cbda35539a2cd4c0504fe198cb423493832d93cf78147a470b2d82329db`) and the run persisted a filesystem-diff artifact manifest. The Showcase documents the daemon-host execution boundary and equivalent Desktop/API paths.
- [x] Add one complete SLURM showcase through a reproducible local fixture. Evidence: on 2026-09-12, `examples/slurm/local-fixture` completed `slurm-fixture-run-v1` through the real SLURM adapter path: generated `sbatch` script, fixture submission, completion sentinel, and persisted artifact manifest. The one activity completed with `result.txt` (22 bytes, SHA-256 `c8920140392a98da804ba516844aa2f241ab8332487e5173c4fd57d9e3e73292`). The fixture is explicitly limited to the local adapter boundary; it does not claim real scheduler, SSH, allocation, or site-policy verification. `npm run typecheck`, `npm run build`, and `npm run check:links` passed after the page was added.

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
- [x] Reorganize `sidebars.ts` by Diátaxis purpose while retaining product-area landing pages and existing URLs. Tutorials, How-to guides, Explanations, and Reference are now the primary sidebar sections; workflow and infrastructure groupings remain nested under the appropriate purpose. Verified in the running documentation site on 2026-09-11.
- [x] Remove or redirect legacy overlapping pages (`examples.md`, `user-guide.md`, `internal/api.md`, and legacy CLI material) after checking inbound links. The source pages and their sidebar/footer entries were removed; `@docusaurus/plugin-client-redirects` preserves their public URLs by directing readers to the Showcase, Tutorials, or current API Reference. Repository links were audited on 2026-09-11 before removal.
- [ ] Perform a final plain-language pass across every authored page after P0/P1 corrections. The 2026-09-11 pass predates the current onboarding and claim findings and cannot serve as final evidence.
- [x] Replace active structural ASCII diagrams with accessible versioned SVGs. Evidence: `static/img/architecture/` now contains the lifecycle, control-plane, planning, network, scope, evidence, timing, interface-hierarchy, and troubleshooting diagrams; each has a `<title>` and `<desc>`, its page supplies alternative text, and the production documentation build passed on 2026-09-12.

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

1. The Linux v1.0.8 Desktop package completed the local first-workflow path through a generated file; clean-host package-manager and other-platform first runs still need verification. The SimGrid tutorial remains a separate API path.
2. GCP and AWS/S3 procedures still require disposable provider accounts to verify minimum permissions, cleanup, and current interface behavior.
3. Focused Desktop captures now cover planning, environment catalog, execution scope, workflow history, SimGrid run detail, provenance exploration, lineage, read-only SQL, and audit events. These captures were reviewed in light and dark themes; the automated link checker is in CI. Further captures should be added only where they clarify a verified procedure.
4. The v1.0.8 Linux package and first launch were exercised, but clean-host package-manager installation and macOS/Windows installation remain unverified. See `onboarding-verification-2026-09-12.md`.
5. Generated endpoint pages identify inferred JSON shapes, handle ZIP instance import, link six runnable SimGrid POST payloads, and add handler-checked notes to ten more routes. Remaining priority request contracts still need required fields, valid examples, and handler checks. Build and link validation do not prove that a copied request works.
