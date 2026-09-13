---
id: documentation-plan
title: Documentation production plan
sidebar_label: Production plan
description: Source-of-truth, media, and review rules for AkôFlow documentation.
---

This page defines the editorial and verification rules for the documentation. Apply them whenever a page, example, screenshot, API route, or supported capability changes.

## Editorial contract

The documentation should show how AkôFlow simplifies scientific workflow execution, not display the complexity of its implementation.

1. Explain the task and expected result first. Give each page one main job and reveal details only when the reader needs them.
2. Use workflow, environment, plan, run, artifacts, and provenance in user paths. Put supervisors, handlers, adapters, and persistence in developer architecture pages unless a task requires them.
3. Make support claims only when code and appropriate evidence support them. Label partial features and distinguish code review, local fixtures, and real-environment validation.
4. Prefer a concrete example over a list of capabilities. Remove repeated caveats and text that does not help a reader act or decide.
5. Check whether each page quickly answers what it is for, when to use it, how to use it, and what to expect.

For each editorial pass, classify passages as **KEEP**, **SIMPLIFY**, **MOVE**, **DELETE**, or **VERIFY**. Resolve P0 (false claims and broken instructions), then P1 (confusing paths and misplaced concepts), then P2 (length and repetition), then P3 (presentation). Repeat audit → edit → build → link check → claim check → first-time-reader review until a full pass finds no P0 or P1 issues. A successful build alone is not the finish line.

The completion gate is a new user running a first workflow without undocumented knowledge, support claims matching implementation and validation, implementation details outside the basic path, and no P0/P1 findings in the final audit.

## Documentation principles

1. Teach complete user tasks instead of listing screens in isolation.
2. Present **AkôFlow Desktop** and **API** paths only where each procedure is documented and verified; state extra prerequisites instead of calling them equivalent by default.
3. Derive behavior from code, tests, and checked-in examples; never infer an endpoint or field from a label alone.
4. Use screenshots to explain spatial relationships and short videos to explain motion or multi-step transitions.
5. Keep a text equivalent for every visual procedure.
6. Mark compatibility behavior and experimental functionality explicitly.

## Sources of truth

| Subject | Primary source |
|---|---|
| Desktop navigation | `akoflow-desktop/src/App.jsx` and `akoflow-desktop/src/components/AppShell.jsx` in the Desktop repository |
| Desktop operations | Page, form, and provider components in `akoflow-desktop/src` |
| HTTP methods and paths | `internal/api/httpserver/httpserver.go` in this repository |
| Request and response contracts | HTTP handlers, application services, and `internal/domain` |
| Runnable scenarios | `examples/` and integration tests in this repository |
| Packaged installation | Root README, `releases/`, the Desktop repository's `electron/` bootstrap, and release workflows |

Generated site output and old copied Markdown files are not sources of truth.

## Review a change

1. Check page purpose, audience, order of concepts, and whether the example solves a concrete task.
2. Compare affected claims and payloads with current handlers, Desktop behavior, tests, and checked-in examples.
3. Distinguish local fixtures from real-provider validation, and update support limits when evidence changes.
4. Check screenshots for secrets, hostnames, tokens, usernames, and unstable identifiers.
5. Run the documentation type-check, build, and link check; then read the rendered path at desktop and mobile widths.
6. Record unresolved P0/P1 findings and repeat the pass after corrections.

## Link verification

Run the repository-owned link check after a documentation build:

```bash
npm run build --prefix docs
npm run check:links --prefix docs
npm run check:shell --prefix docs
```

The check rejects a missing internal documentation route, a missing file below
`docs/static/`, and a Showcase download that no longer has its checked-in
counterpart under `examples/`. It intentionally does not make network requests
or judge third-party URLs: availability of external services belongs to the
reader's environment, while these three classes are artifacts maintained in
this repository. The shell check parses fenced Bash/sh examples and Showcase JSX
command blocks without running them. It requires `curl` examples to fail on HTTP
errors, but cannot validate named files or API behavior. GitHub Actions runs the
type-check, build, link check, and shell check for
documentation or example changes.

## Media naming

Store static screenshots under `static/img/interface/<area>/` and walkthroughs under `static/media/tutorials/`.

Use task and sequence names:

```text
environment-real-01-open-form.png
environment-real-02-test-connection.png
environment-real-03-review-discovery.png
environment-real-create.webm
```

Every media item must have descriptive alternative text. Videos need a written procedure that can be completed without watching them.

## Architecture-diagram system

Use the black, white, and neutral-gray visual system established by [`akoflow-control-plane.svg`](../../static/img/architecture/akoflow-control-plane.svg) for new architecture, lifecycle, and relationship diagrams. It is a reusable visual reference, not a claim that every diagram has the same topology.

- Use a white page or card, black rules, light-gray responsibility groups, and restrained rounded corners.
- Use the checked-in AkôFlow logo rather than recreating or tracing it.
- Group by responsibility, runtime boundary, or lifecycle stage; do not draw application-service cards as separate microservices unless they are independently deployed and evidenced as such.
- Draw only directional arrows that communicate a real request, dispatch, observation, or ownership relationship. Keep labels beside the line rather than on top of the arrowhead.
- Keep every diagram as a versioned SVG below `static/img/architecture/`, include a `<title>` and `<desc>`, and provide equivalent explanatory text in the page.
- Review each new diagram in the rendered documentation at desktop and narrow widths before merging.

## Definition of done for a guide

- The task has prerequisites, a verified procedure for its stated interface, an expected result, and next steps. Add a second interface only when its path has been checked.
- Screenshot placeholders or final captures cover only moments where the visual adds information.
- API examples include authentication and use the current `/akoflow-api` prefix.
- Identifiers in examples are visibly placeholders or come from a documented demo dataset.
- Links resolve and the documentation build succeeds.
- Another reviewer has compared the page with both the UI and daemon implementation.

## Generated API reference

Run `npm run generate:api` to rebuild the endpoint catalog from `internal/api/httpserver/httpserver.go`. The Docusaurus `prestart` and `prebuild` hooks run this automatically. Generated pages are intentionally ignored by Git; changes to method, path, or handler appear on the next documentation build without copying the router by hand.

Each generated endpoint page shows the registered method and path, parameters, owning handler, and request-body indication. HTTP routes include a cURL command or template; the console stream shows a WebSocket connection instead. A template still needs valid IDs and, for a body, a prepared request file.

The generator shows a request body only when it has a checked example. Otherwise, use the handler-checked notes and linked guide to prepare one. Response JSON shapes are illustrative and may omit fields or show placeholder values. Check a route against its handler and a real response before treating a field-level example as verified.

## Reproducible media capture

With the AkôFlow Desktop development server on port `5173` and the documentation server on port `3000`, run:

```bash
npm run capture:media
```

The script opens an isolated headless Chrome profile, applies a fixed `1440 × 900` viewport and reduced-motion preference, waits for each page, and stores captures below `static/img/interface/`. It can read the local development API token without printing or embedding it; the temporary browser profile is removed after capture. Use `AKOFLOW_CAPTURE_TOKEN`, `AKOFLOW_DESKTOP_URL`, or `AKOFLOW_DOCS_ENDPOINT_URL` to override local defaults.
