# Editorial audit ledger

Date: 2026-09-12. Scope: authored documentation pages in the current branch; generated API endpoint pages are reviewed through their generator and component.

This is an iteration ledger, not a completion certificate. The first pass read all 51 authored pages and corrected confirmed narrative, terminology, API convention, example, and support-claim findings. A Desktop first-workflow path has since been run through the v1.0.8 Linux package and documented as page 52. A second full audit, priority endpoint contracts, and external-provider validation remain open.

## Open P0/P1 findings

1. **P0 resolved — Desktop first workflow:** the v1.0.8 Linux package created a local environment, workflow, scope, and manual plan through Desktop. Run `run-1789254029488` completed with 1/1 activities, exit code 0, and generated `result.txt` (20 B, SHA-256 `8dcc517ee6ace065324746726b7f6b5cd67b8c22956ec75bd8b713a0aeaca070`). The new `guides/workflows/first-local-run.md` documents that UI path. The package was extracted and launched under Xvfb with Docker access; this run does not validate a clean package-manager install or other platforms.
2. **P0 — Generated request contracts:** ZIP instance import is corrected and inferred JSON is labeled. Six first-run POST pages link the verified, versioned SimGrid payloads and their submission order. Twenty-seven more pages carry handler/service-checked notes for credentials, cloud provisioning, planning, storage, artifacts, console, connection tests, and machine configuration. The planning-session page uses a concise request example with the actual required fields. Other priority endpoints still need handler-checked required fields and runnable payloads.
3. **P1 — Provider evidence:** GCP and S3 procedures still need disposable-account validation. The SLURM fixture is local adapter evidence, not an institutional batch run.
4. **P1 — Full plain-language review:** after those corrections, re-read every authored page and the generated template as a new user, then repeat the audit until no new P0/P1 issue appears.
5. **P0 resolved in this pass — AWS/S3 narrative:** the former guide described a Desktop storage-creation flow and implied saved AWS credentials enabled S3 operations. Current code exposes a browsing screen without creation and wires the transfer connector to server environment credentials. The guide and support matrix now state that limit; a live S3 procedure still needs validation.
6. **P0 resolved in this pass — GCS claim:** the former support matrix and runtime explanations listed GCS as implemented transfer. The current `gs://` connector returns an unavailable error. The matrix, runtime pages, and schema reference now distinguish accepted `gcs` values from a working transfer path.
7. **P1 resolved in this pass — scope/topology UI:** the scope form creates an empty topology. A separate link-creation form exists at `/network/new` in Desktop source, but current sidebar and scope detail do not link to it. The documented, navigable path for registering links is the API; the SimGrid and scope guides now say so.
8. **P1 resolved in this pass — cloud target language:** saving a capacity target was described as making a provisioned resource. The guide now states that no VM is created at that step.
9. **P1 resolved in this pass — execution state language:** the workflow guide said normal runs move through `created`, though the supervisor creates them as `running` after queue acceptance. The guide now matches the state reference.
10. **P1 resolved in this pass — core concepts:** the concept page conflated executable artifacts with observed scientific files. It now distinguishes executable artifacts from scientific data, matching the data guide.
11. **P1 resolved in this pass — Showcase Desktop claims:** six Showcase tabs described unverified Desktop-only submissions; some asked users to add network links through a Desktop path not exposed in current navigation. The tabs now identify API submission as the verified path and limit Desktop steps to inspection on the same server.
12. **P0 resolved in this pass — GCP Desktop route:** the detailed guide directed readers to a separate Settings credential form and a “New environment” action that do not match the current cloud onboarding. It now follows the tested form path in the connection tutorial.
13. **P0 resolved in the second pass — credential examples:** the SSH import example wrote a private key into a predictable `/tmp` JSON file without restrictive creation permissions. It now streams JSON directly from the existing protected key file to the API, without a temporary payload. The Kubernetes token example likewise reads a protected file into a streamed request instead of asking readers to put a token in a shell command. Both Bash blocks passed syntax checking.
14. **P0 resolved in the second pass — cloud target/API examples:** the API overview put a literal token in a shell `export`; it now uses the shared secret prompt. The capacity-target example omitted `sshSourceRanges`, which lets the current Terraform target default SSH ingress to `0.0.0.0/0`, and attached a configuration-version ID before the guide created it. The example now requires an approved CIDR and leaves optional machine configuration out of the base request.
15. **P0/P1 resolved in the second pass — Showcase setup:** four Showcase API blocks and the SimGrid guide put a token placeholder in a shell `export`; they now use the shared API setup. Six Showcase procedures assumed an unprovided repository checkout; each now shows how to enter the v1.0.8 files before running its script or inline requests. The edge–cloud page uses the checked-in submission script instead of repeating six HTTP calls. The Showcase index no longer calls a single 50-core run a scheduler scalability test.
16. **P0/P1 resolved in the second pass — core narrative and runtime support:** Core concepts now leads to the verified local Desktop run. `serverless` was listed alongside runtime drivers with an ambiguous mode; current source has a domain enum but no built-in adapter, so the runtime table now marks it unavailable. The planning and timing explanations removed repeated implementation detail while preserving prediction-versus-observation limits.
17. **P0/P1 resolved in the second pass — storage operation contracts:** five generated endpoint pages now state the storage and path preconditions plus the observed completion semantics for download, checksum, copy, archive, and index. The storage guide no longer groups immediate file downloads with queued archives. The `202` index route currently completes its scan before responding; the note makes that behavior explicit.
18. **P0/P1 resolved in the second pass — artifact examples and contracts:** the artifact guide no longer supplies nonexistent lineage/project IDs or leaves `BUILD_ID` undefined. Five generated endpoint pages now distinguish upload from metadata registration, build specification from started run, and stored materialization records from verified byte transfers.
19. **P0 resolved in the second pass — console outcome:** the one-shot console guide wrongly treated runner failure as HTTP `422`. The service waits for the runner and returns a `201` command record with `status: failed`; the guide and generated endpoint now tell readers to inspect status and failure. A successful session creation returns `connected`, not a `starting` session. Seven more generated endpoint pages now state checked console, connection-test, machine-configuration, and GCP-only catalog-validation limits.

## Page inventory

| Page | Current pass | Next review |
| --- | --- | --- |
| `guides/workflows/first-local-run.md` | New; end-to-end Linux package UI path verified through generated file evidence; built page and open mobile menu inspected at 390 × 844 | Repeat on a clean supported host and other platforms |
| `concepts.md` | Second read; corrected executable/data distinction and made verified Desktop first run the next step | Recheck complete new-user path |
| `contributing/documentation-plan.md` | Full read; replaced historical production waves with recurring review contract | Recheck contract at final audit |
| `downloads.md` | Full read; platform limits and verification are explicit | Recheck release links when version changes |
| `engine.md` | Second read; implementation details remain in the developer section | Recheck claims against event-loop code |
| `explanations/evidence-and-provenance.md` | Second read; plan predictions and optional observations remain distinct | Recheck against execution evidence after P0 fixes |
| `explanations/network-modeling.md` | Second read; shortened transfer strategy list and clarified data-versus-order dependency | Recheck transfer claims against execution paths |
| `explanations/observed-timing.md` | Second read; removed repeated evaluator detail while preserving optional observed-field limits | Recheck metrics against trace aggregation |
| `explanations/planning.md` | Second read; linked scheduler comparison and removed abstract separation language | Recheck planning claims against coordinator |
| `explanations/prism-and-heft.md` | Second read; removed Go request type and condensed bounded-search detail | Recheck algorithm claims against source |
| `getting-started.md` | Full read; now leads to the verified Desktop local run before the API simulation | Recheck the entry flow on a clean supported host |
| `guides/data/artifacts.md` | Second read; removed fictitious lineage/project IDs, made build ID acquisition explicit, and aligned storage operation states | Recheck API payloads after P0 contracts fix |
| `guides/data/provenance-and-audit.md` | Full read; detailed screen tables serve investigation tasks | Recheck API payloads after P0 contracts fix |
| `guides/infrastructure/aws.md` | Full read after code audit; partial S3 path and absent EC2 path are explicit | Check provider claims against disposable-bucket evidence |
| `guides/infrastructure/cloud-capacity.md` | Second read; target example now states CIDR and configuration-version prerequisites; target versus VM remains explicit | Validate with disposable GCP account |
| `guides/infrastructure/environments.md` | Full read; aligned Desktop actions and shared API setup | Recheck API payloads after P0 contracts fix |
| `guides/infrastructure/execution-scopes.md` | Full read; moved reciprocal IDs to API procedure | Recheck API payloads after P0 contracts fix |
| `guides/infrastructure/gcp.md` | Full read; corrected Desktop onboarding and aligned API setup | Check provider claims against disposable-project evidence |
| `guides/infrastructure/hpc-slurm.md` | Full read; aligned API setup and retained site-specific limits | Check provider claims against real cluster evidence |
| `guides/infrastructure/kubernetes.md` | Full read; aligned API setup and task-first opening | Recheck procedure against Kind bundle |
| `guides/infrastructure/simgrid.md` | Second read; corrected unlinked Desktop topology path and now uses shared API setup with a versioned checkout | Recheck procedure against pinned bundle |
| `guides/infrastructure/storage.md` | Second read; Desktop capability gates match current browser; API IDs and paths are examples, promotion omits fictitious lineage IDs, and operation states match the storage coordinator | Recheck storage payloads against a real registered storage |
| `guides/interface-tour.mdx` | Full read; removed API routes, shortened search explanation, and linked the verified Desktop first run | Recheck rendered page at mobile width |
| `guides/operations/credentials-and-ssh.md` | Second read; streamed private key and token payloads from protected files; aligned reference wording with connection fields | Recheck current Desktop form and key lifecycle claims |
| `guides/operations/instance-management.md` | Full read; aligned shared API setup and removed internal snapshot detail | Recheck import/export contracts against handlers |
| `guides/operations/interactive-console.md` | Second read; corrected synchronous command failure and successful session creation states against controllers | Recheck session behavior against Desktop |
| `guides/operations/search-and-notifications.md` | Full read; removed debounce/polling details and aligned API setup | Recheck UI behavior against Desktop |
| `guides/operations/server-instance.md` | Full read; prerequisites, verification, and removal are explicit | Recheck release assets when version changes |
| `guides/operations/troubleshooting.md` | Full read; starts with user-visible failure and uses server terminology | Recheck diagnostic claims after P0 fixes |
| `guides/workflows/definitions.md` | Full read; simplified opening and aligned API setup | Recheck payload against importer after P0 contracts fix |
| `guides/workflows/executions.md` | Full read; corrected queued/run state distinction; packaged Desktop start flow verified with a local real run | Recheck other runtimes against their own guides |
| `guides/workflows/first-run.md` | Full read; API submission and Desktop inspection are distinguished | Keep the SimGrid API example aligned with its versioned bundle |
| `guides/workflows/planning.md` | Full read; simplified candidate language and made the API example's registration prerequisite explicit | Verify automatic Desktop planning flow |
| `installation.md` | Full read; public bootstrap checks verified in `security.go` and tests; clarified what Connected proves | Verify clean-host and cross-platform installation |
| `internal/workflow-spec.md` | Full read; compatibility and simulation-only import limits are explicit | Check contracts against current importer |
| `modules.md` | Second read; source map and implementation detail remain in the developer section | Recheck claims against daemon composition |
| `reference/api-overview.md` | Second read; removed token-in-shell-history setup and an unprovided `workflow.json` example; linked the task guide and versioned tutorial | Check contracts against current handlers and schemas |
| `reference/environment-yaml.md` | Full read; field-level detail belongs in reference | Check contracts against current handlers and schemas |
| `reference/execution-scopes-and-topologies.md` | Full read; creation order, units, and validation limits are explicit | Check contracts against current handlers and schemas |
| `reference/feature-coverage.md` | Second read; now distinguishes registered but unlinked Desktop network routes from navigable task paths | Check contracts against current handlers and schemas |
| `reference/planning-and-execution-states.md` | Full read; state limits are explicit | Check contracts against current handlers and schemas |
| `runtimes.md` | Second read; marked serverless schema value unavailable because no adapter is registered | Recheck other provider claims against adapters |
| `showcase/edge-cloud-simulation.mdx` | Second read; uses versioned checkout and submission script, shared API setup, and same-server Desktop inspection | Check result against pinned bundle |
| `showcase/index.mdx` | Second read; instructions fit script and inline API examples; removed unsupported scalability language | Recheck cards after Showcase audit |
| `showcase/kubernetes-real-execution.mdx` | Second read; checkout precedes Kind setup and inline API submission; Desktop is same-server inspection | Check result against pinned bundle |
| `showcase/local-direct-execution.mdx` | Second read; uses versioned checkout and shared API setup; removed unsupported cancellation link | Check result against pinned bundle |
| `showcase/network-fanout.mdx` | Second read; uses versioned checkout and shared API setup; network link creation stays in API | Check result against pinned bundle |
| `showcase/parallel-50-core.mdx` | Second read; uses versioned checkout and shared API setup; timing explanation links to correct page | Check result against pinned bundle |
| `showcase/slurm-local-fixture.mdx` | Second read; both terminals use the versioned checkout; Desktop path remains optional inspection | Check result against pinned bundle |
| `tutorials/api-access.md` | Full read; API prerequisites and token limits are explicit | Recheck commands after P0 contracts fix |
| `tutorials/connect-cloud.md` | Full read; AWS caveat and unvalidated GCP steps are explicit | Validate with disposable GCP account |
| `tutorials/register-hpc.md` | Full read; placeholders and remote-run limit are explicit | Validate on an approved institutional cluster |

## Verification in this pass

- `npm run typecheck`, `npm run build`, `npm run check:links`, and `git diff --check` passed on the editorial branch.
- The latest link check covered 382 local links/assets and 54 showcase downloads from the `v1.0.8` Git tag.
- Headless Chromium at 390 × 844 loaded Getting Started, Core concepts, the console guide, the new Desktop first-run page, and all seven Showcase pages without page errors or horizontal overflow. Every Showcase API tab displayed its v1.0.8 checkout command; the first-run page's mobile menu opened and showed its tutorial link.
- These checks establish site integrity for this pass. They do not prove tutorial execution, provider support, or completion of the editorial gate.
