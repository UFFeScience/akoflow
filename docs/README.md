# docs akoflow

## Visual assets

Documentation diagrams are repository-owned SVG files under `static/img/architecture`.
Follow the established AkôFlow visual system: a white background, `#151515`
border/text/arrows, `#f2f2f2` structural cards, and `#555` only for secondary
annotations. Do not introduce colored fills, gradients, or a separate diagram
theme. Use `system-ui, sans-serif`, rounded outer frames and cards, concise
English labels, and an accessible `<title>` and `<desc>` in every SVG.

## External example catalog

Runnable examples are owned by
[`UFFeScience/akoflow-examples`](https://github.com/UFFeScience/akoflow-examples),
not copied into this repository. Documentation CI checks out that repository at
the commit received through `examples-updated` (or `main` during scheduled and
ordinary builds) and exposes it through `AKOFLOW_EXAMPLES_DIR`. The link checker
then verifies every Showcase download and source link against the real checkout.

Example validation and image publication are implemented once in
`.github/workflows/examples-ci.yml`. The examples repository calls that reusable
workflow with the exact source commit. This keeps example files independently
versioned while CI/CD and documentation deployment remain controlled by AkôFlow.

## Dependency security

Install dependencies with `npm ci` from this directory. Do not use
`--ignore-scripts`: the postinstall step applies the reviewed, version-bound
`patches/braces+3.0.3.patch` and fails if the patch cannot be applied.

The upstream advisory
[GHSA-vfj7-8cjw-p6xm](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm)
has no published patched version. The local patch bounds parser nesting and
all recursive AST walkers at 128 levels, including direct AST entry points.
It preserves the original package version; `npm audit` therefore still reports
this advisory and its transitive dependants. This is a documented local
mitigation, not a claim that the upstream advisory disappeared.

Run `npm run test:dependency-security` after installation. CI checks malicious
patterns below the existing input-length limit, excessive direct AST depth,
normal nested patterns, ranges, and micromatch compatibility before building
the site. Other advisories must be fixed independently; this patch is not a
blanket audit exemption. Remove it only after an upstream depth-safe release
is available and the regression tests still pass.

The version-bound `patches/deepmerge+4.3.1.patch` mitigates
[GHSA-gcqx-w48m-w2h4](https://github.com/advisories/GHSA-gcqx-w48m-w2h4),
which has no published patched version. Both CommonJS and UMD entry points use
own data properties when copying or merging keys, so an own `__proto__` key
cannot invoke the inherited prototype setter. Regression tests cover malicious
target and source objects, null-prototype targets, nested merges, arrays and
symbols. The exact advisory remains visible in npm audit; every other advisory
continues to fail CI. Remove the patch and its exact advisory exception after a
fixed upstream release passes the same tests.

## Automatic project changelog

GitHub Releases is the source of truth for `/docs/changelog`. Publish or edit release notes there to update the changelog, including screenshots and download files. The page follows every pagination link and includes stable releases and public pre-releases; drafts are excluded.

The Go command `go run ./cmd/docs-release-sync` writes the public build snapshot to `docs/src/data/releases.generated.json`. The documentation build runs this command automatically through `npm run generate:releases`; Go 1.25 and Node.js 20 or later are required. `GITHUB_TOKEN`, when provided by CI, is used only by this build command and is never written to the browser snapshot. A valid previous snapshot remains available during a GitHub outage. The browser also refreshes the full public history on entry and through **Refresh**.

Release publication, edits, deletion or unpublication request a documentation workflow on `main`, preserving the existing Pages deployment branch policy. The existing daily schedule refreshes the saved history as well. Pull request checks use separate concurrency groups from production Pages deployment.

Validate the generator with `go test ./cmd/docs-release-sync` and the browser pagination/normalization with `npm run test:releases --prefix docs`. Runtime release Markdown is parsed and sanitized before rendering. Screenshots in release notes should use an immutable raw GitHub commit URL; the normal documentation guides reference their repository-owned static image paths.
