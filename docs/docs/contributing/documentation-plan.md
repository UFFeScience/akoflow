---
id: documentation-plan
title: Documentation production plan
sidebar_label: Production plan
description: Source-of-truth, media, and review rules for AkôFlow documentation.
---

This plan keeps the documentation aligned with the shipping daemon and Desktop application. It is also the contract for parallel documentation work.

## Documentation principles

1. Teach complete user tasks instead of listing screens in isolation.
2. Present **AkôFlow Desktop** and **API** as equivalent paths whenever both exist.
3. Derive behavior from code, tests, and checked-in examples; never infer an endpoint or field from a label alone.
4. Use screenshots to explain spatial relationships and short videos to explain motion or multi-step transitions.
5. Keep a text equivalent for every visual procedure.
6. Mark compatibility behavior and experimental functionality explicitly.

## Sources of truth

| Subject                        | Primary source                                                      |
| ------------------------------ | ------------------------------------------------------------------- |
| Desktop navigation             | `akoflow-admin/src/App.jsx` and `src/components/AppShell.jsx`       |
| Desktop operations             | Page, form, and provider components in `akoflow-admin/src`          |
| HTTP methods and paths         | `akoflow/internal/api/httpserver/httpserver.go`                     |
| Request and response contracts | HTTP handlers, application services, and `internal/domain`          |
| Runnable scenarios             | `akoflow/examples` and integration tests                            |
| Packaged installation          | Root README, `releases/`, Electron bootstrap, and release workflows |

Generated site output and old copied Markdown files are not sources of truth.

## Production waves

### Wave 1 — foundation

- Establish the information architecture and sidebar.
- Add reusable screenshot, video, and Desktop/API components.
- Build a feature coverage matrix.
- Define stable demo data and redact all secrets from captures.

### Wave 2 — task guides

- Infrastructure and execution scopes.
- Workflow definition, planning, and execution.
- Artifacts, storage, provenance, and audit.
- Installation, instance management, and troubleshooting.

Independent guide groups may be authored in parallel after their source inventory is complete. Each group owns separate files.

### Wave 3 — reference

- Replace the legacy workflow specification with the current versioned model.
- Document API conventions and endpoint groups.
- Document runtime capabilities, lifecycle states, and compatibility rules.

### Wave 4 — media

- Load a deterministic demonstration instance.
- Capture a fixed desktop viewport in the light theme.
- Add numbered callouts and restrained directional arrows.
- Record one operation per video.
- Prefer WebM for the site; create an optimized GIF only when a fallback is useful.

### Wave 5 — verification

- Verify every field against the Go contract.
- Verify every route against the HTTP mux.
- Run or validate checked-in examples.
- Build and type-check Docusaurus.
- Review screenshots for secrets, hostnames, tokens, usernames, and unstable identifiers.
- Search for removed terminology and stale fixed-port instructions.

## Link verification

Run the repository-owned link check after a documentation build:

```bash
npm run build --prefix docs
npm run check:links --prefix docs
```

The check rejects a missing internal documentation route, a missing file below
`docs/static/`, and a Showcase download that no longer has its checked-in
counterpart under `examples/`. It intentionally does not make network requests
or judge third-party URLs: availability of external services belongs to the
reader's environment, while these three classes are artifacts maintained in
this repository. GitHub Actions runs the same type-check, build, and link check
for documentation or example changes.

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

### Canonical image lifecycle

- Capture Electron Desktop screenshots at `1280 × 860`, device scale factor
  `1`, with reduced motion. A larger Xvfb screen may surround the Electron
  window, but it does not change the canonical output size. Browser-only
  documentation captures keep their scripted `1440 × 900` viewport. Keep the
  original PNG resolution; optimize without resizing.
- Existing reviewed captures from before this convention are an explicit legacy
  exception: keep their original pixels while the depicted task remains
  accurate, and replace them at `1280 × 860` when that screen is next
  recaptured. Do not resize old evidence merely to satisfy the convention.
- Name files `<area>-<task>-<two-digit-step>-<description>.png` for a sequence,
  or `<area>-<stable-view>.png` for a single durable view. Dark-theme variants
  use the `-dark` suffix before the extension.
- Commit only reviewed files below `docs/static/img/interface/<area>/`. A path
  below a Playwright report, test-results directory, browser profile, system
  temporary directory, or `PAPERCLIP_*_SCRATCH_DIR` is never canonical and must
  not be copied into Git without the naming and review steps above.
- Review the rendered page at desktop and narrow widths. Confirm that the image
  matches the current release, contains no token, username, host, credential,
  transient notification, or unstable identifier, and has useful alt text.
- To update an image, reproduce the same deterministic dataset and viewport,
  replace the canonical file in place, and review its Git diff. Rename only
  when the documented task changes, updating all references in the same commit.
- Remove an image when the related UI/task no longer exists or the text is
  clearer without it. Delete the canonical file and every documentation
  reference together; `npm run check:links --prefix docs` verifies no local
  reference was left behind.

## Architecture-diagram system

Use the black, white, and neutral-gray visual system established by [`akoflow-control-plane.svg`](../../static/img/architecture/akoflow-control-plane.svg) for new architecture, lifecycle, and relationship diagrams. It is a reusable visual reference, not a claim that every diagram has the same topology.

- Use a white page or card, black rules, light-gray responsibility groups, and restrained rounded corners.
- Use the checked-in AkôFlow logo rather than recreating or tracing it.
- Group by responsibility, runtime boundary, or lifecycle stage; do not draw application-service cards as separate microservices unless they are independently deployed and evidenced as such.
- Draw only directional arrows that communicate a real request, dispatch, observation, or ownership relationship. Keep labels beside the line rather than on top of the arrowhead.
- Keep every diagram as a versioned SVG below `static/img/architecture/`, include a `<title>` and `<desc>`, and provide equivalent explanatory text in the page.
- Review each new diagram in the rendered documentation at desktop and narrow widths before merging.

## Definition of done for a guide

- The task has prerequisites, Desktop steps, API steps, expected result, and next steps.
- Screenshot placeholders or final captures cover only moments where the visual adds information.
- API examples include authentication and use the current `/akoflow-api` prefix.
- Identifiers in examples are visibly placeholders or come from a documented demo dataset.
- Links resolve and the documentation build succeeds.
- Another reviewer has compared the page with both the UI and daemon implementation.

## Generated API reference

Run `npm run generate:api` to rebuild the endpoint catalog from `internal/api/httpserver/httpserver.go`. The Docusaurus `prestart` and `prebuild` hooks run this automatically. Generated pages are intentionally ignored by Git; changes to method, path, or handler appear on the next documentation build without copying the router by hand.

Each generated endpoint page includes its HTTP method, registered path, path parameters, authentication example, request-body indication, owning handler, and a copyable cURL command. Domain guides remain responsible for semantic explanations and complete payload examples.

## Reproducible media capture

With the AkôFlow Desktop development server on port `5173` and the documentation server on port `3000`, run:

```bash
npm run capture:media
```

The script opens an isolated headless Chrome profile, applies a fixed `1440 × 900` viewport and reduced-motion preference, waits for each page, and stores captures below `static/img/interface/`. It can read the local development API token without printing or embedding it; the temporary browser profile is removed after capture. Use `AKOFLOW_CAPTURE_TOKEN`, `AKOFLOW_DESKTOP_URL`, or `AKOFLOW_DOCS_ENDPOINT_URL` to override local defaults.
