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
