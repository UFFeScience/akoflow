# Editorial version closeout

Date: 2026-09-13. Branch: `docs/editorial-consistency`. This closes the current editorial iteration, not the external-user completion gate in `quality-plan.md`.

## What changed

- All 61 authored pages received an editorial inventory and at least one pass. The entry path now leads from workflow and environment to plan, run, and result. Task guides put Desktop or API steps before internal architecture; the developer pages retain implementation details.
- Support language distinguishes verified local, SimGrid, and Kind runs from a local SLURM adapter fixture and unverified institutional HPC and live cloud paths. The Cloud support matrix states the current GCP, AWS EC2, S3, and `gs://` limits.
- A Linux v1.0.8 Desktop package completed the documented first local workflow and produced a recorded output file and checksum. The package was extracted and launched under Xvfb with Docker; this does not establish clean-host package-manager installation or other-platform launch.
- The generator builds 125 endpoint pages. All 61 mutating routes have handler-checked notes or versioned runnable SimGrid requests. It no longer presents inferred request bodies as runnable examples, labels illustrative response shapes, and fails on unknown success statuses.
- Documentation navigation uses stable `/docs/...` routes. The checker rejects route-relative documentation links. The mobile menu fix and a 390 px browser pass are recorded in the audit ledger; one internal link from each authored page reached its compiled destination.
- `documentation-plan.md` now holds the ongoing editorial contract. `editorial-audit-2026-09-12.md` records individual corrections and evidence.

## Final assessment of this iteration

The basic narrative, verified Linux first-run path, site navigation, and documented support boundaries are substantially improved. Current build and link checks establish that the site compiles and its checked routes resolve. They do not establish that every API field example is correct or that every procedure works on every claimed host and provider.

The full completion gate remains **open**. `quality-plan.md` retains the specific work: clean-host and other-platform installation, real GCP and S3 account validation, institutional SLURM validation, field-level verification across all 125 endpoint contracts, and a final plain-language and claim pass after those corrections. The audit ledger explicitly records these as unresolved. No claim of zero P0/P1 findings is made for this version.

## Handoff

Keep the pull request in draft until the completion gate is evidenced. Resume with the open items in `quality-plan.md`, prioritize false or unusable instructions, and update the support matrix and page inventory as each external validation is completed. Re-run typecheck, production build, link and shell checks, then inspect the rendered first-user paths at mobile and desktop widths before marking the documentation ready.
