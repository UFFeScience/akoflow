## Objective
Publish every documentation showcase under an official AkôFlow Hub account, with immutable workflow versions, explanatory READMEs, images, genuine execution results, and unique import URIs. Update documentation to link to the published versions.

## Execution plan
1. Reconcile live documentation, local showcase pages, 28 validation contracts, source workflows and existing daemon records. Include simple infrastructure/simulation examples and newer showcases missing from the live index.
2. Create or reuse the official AkôFlow publisher account after confirming its email; use one publication per showcase. Complete normal email verification and administrator approval.
3. Recover source bundles, input datasets and pinned container images. Validate portability and document runtime/infrastructure prerequisites; provide a local runnable profile wherever practical.
4. For each showcase, prepare an English README covering purpose, DAG, inputs and their provenance/license, prerequisites, import/run instructions, outputs, screenshots, observed results and validation limits.
5. Recover completed executions or run missing examples. Preserve actual statuses, logs, metrics, hashes and artifacts; never represent old contract metadata as freshly verified results. Montage currently has an old running contract and needs inspection.
6. Export native experiment packages, review redactions, and publish each workflow/version with its README and real results. Record the canonical public URL and the exact import URI returned by Hub.
7. Update showcase pages and the catalog from a manifest mapping documentation routes to Hub publications, immutable versions, import URIs and evidence.
8. Validate anonymous access, downloads, version identity, fresh Desktop/daemon import, execution prerequisites, visible results and all documentation links. Open linked PRs for repository changes.

## Acceptance criteria
- Every inventoried showcase has an accessible public Hub publication and version.
- Anonymous visitors can inspect README/images/results and obtain the import URI.
- Imports resolve to the intended immutable workflow version; required input data is obtainable.
- Real runs and simulations are clearly identified, and unsupported prerequisites are explicit.
- Documentation links match the published manifest and no pending execution is claimed as successful.

## Initial evidence
Local docs contain 28 showcase contracts (27 recorded passed, one recorded running), plus simple showcases outside that contract file. Public docs still describe several AI workflows as patterns. The local server checkout no longer contains the examples directory referenced by those docs, so bundle recovery is part of this task. Hub responds over HTTPS and supports native experiment package import; package binaries/datasets are excluded and require separate accessible references.
