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
