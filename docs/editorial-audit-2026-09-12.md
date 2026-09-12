# Editorial audit ledger

Date: 2026-09-12. Scope: authored documentation pages in the current branch; generated API endpoint pages are reviewed through their generator and component.

This is an iteration ledger, not a completion certificate. The current pass inspected the title, opening, heading structure, and claim language of every authored page. It edited the entry narrative, user-facing terminology, API convention, and versioned example links where findings were confirmed. A full line-by-line read and external-provider validation remain open.

## Open P0/P1 findings

1. **P0 — Desktop first workflow:** installation reaches a connected local environment, but no verified Desktop-only procedure takes a new user through workflow creation, planning, execution, and result inspection. The SimGrid API tutorial has separate prerequisites.
2. **P0 — Generated request contracts:** ZIP instance import is corrected and inferred JSON is labeled, but priority endpoints still need handler-checked required fields and runnable payloads.
3. **P1 — Provider evidence:** GCP and S3 procedures still need disposable-account validation. The SLURM fixture is local adapter evidence, not an institutional batch run.
4. **P1 — Full plain-language review:** after those corrections, re-read every authored page and the generated template as a new user, then repeat the audit until no new P0/P1 issue appears.

## Page inventory

| Page | Current pass | Next review |
| --- | --- | --- |
| `concepts.md` | Edited in this pass | Full read after P0/P1 fixes |
| `contributing/documentation-plan.md` | Edited in this pass | Full read after P0/P1 fixes |
| `downloads.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `engine.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `explanations/evidence-and-provenance.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `explanations/network-modeling.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `explanations/observed-timing.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `explanations/planning.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `explanations/prism-and-heft.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `getting-started.md` | Edited in this pass | Verify complete first-user path |
| `guides/data/artifacts.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/data/provenance-and-audit.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/infrastructure/aws.md` | Opening and purpose scanned | Check provider claims against real-environment evidence |
| `guides/infrastructure/cloud-capacity.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/infrastructure/environments.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/infrastructure/execution-scopes.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/infrastructure/gcp.md` | Edited in this pass | Check provider claims against real-environment evidence |
| `guides/infrastructure/hpc-slurm.md` | Edited in this pass | Check provider claims against real-environment evidence |
| `guides/infrastructure/kubernetes.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/infrastructure/simgrid.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/infrastructure/storage.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/interface-tour.mdx` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/operations/credentials-and-ssh.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/operations/instance-management.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/operations/interactive-console.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/operations/search-and-notifications.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/operations/server-instance.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `guides/operations/troubleshooting.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/workflows/definitions.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/workflows/executions.md` | Edited in this pass | Full read after P0/P1 fixes |
| `guides/workflows/first-run.md` | Edited in this pass | Verify complete first-user path |
| `guides/workflows/planning.md` | Edited in this pass | Full read after P0/P1 fixes |
| `installation.md` | Opening and purpose scanned | Verify complete first-user path |
| `internal/workflow-spec.md` | Opening and purpose scanned | Check contracts against current handlers and schemas |
| `modules.md` | Edited in this pass | Full read after P0/P1 fixes |
| `reference/api-overview.md` | Edited in this pass | Check contracts against current handlers and schemas |
| `reference/environment-yaml.md` | Edited in this pass | Check contracts against current handlers and schemas |
| `reference/execution-scopes-and-topologies.md` | Edited in this pass | Check contracts against current handlers and schemas |
| `reference/feature-coverage.md` | Opening and purpose scanned | Check contracts against current handlers and schemas |
| `reference/planning-and-execution-states.md` | Opening and purpose scanned | Check contracts against current handlers and schemas |
| `runtimes.md` | Edited in this pass | Full read after P0/P1 fixes |
| `showcase/edge-cloud-simulation.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/index.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/kubernetes-real-execution.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/local-direct-execution.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/network-fanout.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/parallel-50-core.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `showcase/slurm-local-fixture.mdx` | Edited in this pass | Check procedure and result against pinned bundle |
| `tutorials/api-access.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `tutorials/connect-cloud.md` | Opening and purpose scanned | Full read after P0/P1 fixes |
| `tutorials/register-hpc.md` | Opening and purpose scanned | Full read after P0/P1 fixes |

## Verification in this pass

- `npm run typecheck`, `npm run build`, `npm run check:links`, and `git diff --check` passed on the editorial branch.
- The link check covered 337 local links/assets and 53 showcase downloads from the `v1.0.8` Git tag.
- Headless Chromium at 390 × 844 loaded Getting Started, Core concepts, and the console guide without page errors or horizontal overflow; the mobile menu opened to full viewport height.
- These checks establish site integrity for this pass. They do not prove tutorial execution, provider support, or completion of the editorial gate.
