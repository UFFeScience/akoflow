# Editorial audit ledger

Date: 2026-09-12. Scope: authored documentation pages in the current branch; generated API endpoint pages are reviewed through their generator and component.

This is an iteration ledger, not a completion certificate. The current pass inspected the title, opening, heading structure, and claim language of every authored page. It edited the entry narrative, user-facing terminology, API convention, and versioned example links where findings were confirmed. A full line-by-line read and external-provider validation remain open.

## Open P0/P1 findings

1. **P0 — Desktop first workflow:** installation reaches a connected local environment, but no verified Desktop-only procedure takes a new user through workflow creation, planning, execution, and result inspection. The SimGrid API tutorial has separate prerequisites.
2. **P0 — Generated request contracts:** ZIP instance import is corrected and inferred JSON is labeled, but priority endpoints still need handler-checked required fields and runnable payloads.
3. **P1 — Provider evidence:** GCP and S3 procedures still need disposable-account validation. The SLURM fixture is local adapter evidence, not an institutional batch run.
4. **P1 — Full plain-language review:** after those corrections, re-read every authored page and the generated template as a new user, then repeat the audit until no new P0/P1 issue appears.
5. **P0 resolved in this pass — AWS/S3 narrative:** the former guide described a Desktop storage-creation flow and implied saved AWS credentials enabled S3 operations. Current code exposes a browsing screen without creation and wires the transfer connector to server environment credentials. The guide and support matrix now state that limit; a live S3 procedure still needs validation.
6. **P0 resolved in this pass — GCS claim:** the former support matrix and runtime explanations listed GCS as implemented transfer. The current `gs://` connector returns an unavailable error. The matrix, runtime pages, and schema reference now distinguish accepted `gcs` values from a working transfer path.
7. **P1 resolved in this pass — scope/topology UI:** the SimGrid guide implied Desktop could edit network links, while the execution-scope guide states that Desktop only creates an empty topology. The SimGrid procedure now sends link edits through the API.
8. **P1 resolved in this pass — cloud target language:** saving a capacity target was described as making a provisioned resource. The guide now states that no VM is created at that step.
9. **P1 resolved in this pass — execution state language:** the workflow guide said normal runs move through `created`, though the supervisor creates them as `running` after queue acceptance. The guide now matches the state reference.
10. **P1 resolved in this pass — core concepts:** the concept page conflated executable artifacts with observed scientific files. It now distinguishes executable artifacts from scientific data, matching the data guide.

## Page inventory

| Page | Current pass | Next review |
| --- | --- | --- |
| `concepts.md` | Full read; corrected executable/data distinction | Recheck basic narrative after first-run validation |
| `contributing/documentation-plan.md` | Full read; replaced historical production waves with recurring review contract | Recheck contract at final audit |
| `downloads.md` | Full read; platform limits and verification are explicit | Recheck release links when version changes |
| `engine.md` | Full read; implementation details are in the developer section | Recheck claims against event-loop code |
| `explanations/evidence-and-provenance.md` | Full read and simplified runtime language | Recheck against execution evidence after P0 fixes |
| `explanations/network-modeling.md` | Full read; detailed model belongs in this explanation | Recheck transfer claims after P0 fixes |
| `explanations/observed-timing.md` | Full read; optional observations are qualified | Recheck metrics after P0 fixes |
| `explanations/planning.md` | Full read and simplified plan terminology | Recheck planning claims after P0 fixes |
| `explanations/prism-and-heft.md` | Full read; algorithm detail serves comparison readers | Recheck algorithm claims after P0 fixes |
| `getting-started.md` | Full read; API-only example is clearly qualified | Verify complete first-user path |
| `guides/data/artifacts.md` | Full read; simplified the task-first opening | Recheck API payloads after P0 contracts fix |
| `guides/data/provenance-and-audit.md` | Full read; detailed screen tables serve investigation tasks | Recheck API payloads after P0 contracts fix |
| `guides/infrastructure/aws.md` | Rewritten after code audit | Check provider claims against real-environment evidence |
| `guides/infrastructure/cloud-capacity.md` | Full read; distinguished target from VM and simplified opening | Validate with disposable GCP account |
| `guides/infrastructure/environments.md` | Full read; aligned Desktop actions and shared API setup | Recheck API payloads after P0 contracts fix |
| `guides/infrastructure/execution-scopes.md` | Full read; moved reciprocal IDs to API procedure | Recheck API payloads after P0 contracts fix |
| `guides/infrastructure/gcp.md` | Edited in this pass | Check provider claims against real-environment evidence |
| `guides/infrastructure/hpc-slurm.md` | Edited in this pass | Check provider claims against real-environment evidence |
| `guides/infrastructure/kubernetes.md` | Full read; aligned API setup and task-first opening | Recheck procedure against Kind bundle |
| `guides/infrastructure/simgrid.md` | Full read; corrected unsupported Desktop link editing | Recheck procedure against pinned bundle |
| `guides/infrastructure/storage.md` | Full read; action limits are qualified by capabilities | Recheck API payloads after P0 contracts fix |
| `guides/interface-tour.mdx` | Full read; removed API routes and simplified navigation/terminal language | Recheck rendered page at mobile width |
| `guides/operations/credentials-and-ssh.md` | Full read; aligned shared API setup and server terminology | Recheck secret-handling claims against handlers |
| `guides/operations/instance-management.md` | Full read; aligned shared API setup and removed internal snapshot detail | Recheck import/export contracts against handlers |
| `guides/operations/interactive-console.md` | Full read; removed polling implementation detail and aligned API setup | Recheck session behavior against Desktop |
| `guides/operations/search-and-notifications.md` | Full read; removed debounce/polling details and aligned API setup | Recheck UI behavior against Desktop |
| `guides/operations/server-instance.md` | Full read; prerequisites, verification, and removal are explicit | Recheck release assets when version changes |
| `guides/operations/troubleshooting.md` | Full read; starts with user-visible failure and uses server terminology | Recheck diagnostic claims after P0 fixes |
| `guides/workflows/definitions.md` | Full read; simplified opening and aligned API setup | Recheck payload against importer after P0 contracts fix |
| `guides/workflows/executions.md` | Full read; corrected queued/run state distinction | Verify Desktop-only start flow |
| `guides/workflows/first-run.md` | Full read; API submission and Desktop inspection are distinguished | Verify complete Desktop-only path |
| `guides/workflows/planning.md` | Full read; simplified candidate language | Verify Desktop-only planning flow |
| `installation.md` | Full read; public bootstrap checks verified in `security.go` and tests | Verify complete first-user path and cross-platform installation |
| `internal/workflow-spec.md` | Opening and purpose scanned | Check contracts against current handlers and schemas |
| `modules.md` | Full read; removed incidental polling default | Recheck claims against daemon composition |
| `reference/api-overview.md` | Edited in this pass | Check contracts against current handlers and schemas |
| `reference/environment-yaml.md` | Edited in this pass | Check contracts against current handlers and schemas |
| `reference/execution-scopes-and-topologies.md` | Edited in this pass | Check contracts against current handlers and schemas |
| `reference/feature-coverage.md` | Full read; route coverage remains a reference checklist | Check contracts against current handlers and schemas |
| `reference/planning-and-execution-states.md` | Full read; state limits are explicit | Check contracts against current handlers and schemas |
| `runtimes.md` | Full read; replaced duplicate user procedure with task-guide links | Recheck provider claims against adapters |
| `showcase/edge-cloud-simulation.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/index.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/kubernetes-real-execution.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/local-direct-execution.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/network-fanout.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/parallel-50-core.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/slurm-local-fixture.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `tutorials/api-access.md` | Full read; API prerequisites and token limits are explicit | Recheck commands after P0 contracts fix |
| `tutorials/connect-cloud.md` | Full read; AWS caveat and unvalidated GCP steps are explicit | Validate with disposable GCP account |
| `tutorials/register-hpc.md` | Full read; placeholders and remote-run limit are explicit | Validate on an approved institutional cluster |

## Verification in this pass

- `npm run typecheck`, `npm run build`, `npm run check:links`, and `git diff --check` passed on the editorial branch.
- The link check covered 337 local links/assets and 53 showcase downloads from the `v1.0.8` Git tag.
- Headless Chromium at 390 × 844 loaded Getting Started, Core concepts, and the console guide without page errors or horizontal overflow; the mobile menu opened to full viewport height.
- These checks establish site integrity for this pass. They do not prove tutorial execution, provider support, or completion of the editorial gate.
