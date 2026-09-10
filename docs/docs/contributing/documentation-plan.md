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

| Subject | Primary source |
|---|---|
| Desktop navigation | `akoflow-admin/src/App.jsx` and `src/components/AppShell.jsx` |
| Desktop operations | Page, form, and provider components in `akoflow-admin/src` |
| HTTP methods and paths | `akoflow/internal/api/httpserver/httpserver.go` |
| Request and response contracts | HTTP handlers, application services, and `internal/domain` |
| Runnable scenarios | `akoflow/examples` and integration tests |
| Packaged installation | Root README, `releases/`, Electron bootstrap, and release workflows |

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

## Definition of done for a guide

- The task has prerequisites, Desktop steps, API steps, expected result, and next steps.
- Screenshot placeholders or final captures cover only moments where the visual adds information.
- API examples include authentication and use the current `/akoflow-api` prefix.
- Identifiers in examples are visibly placeholders or come from a documented demo dataset.
- Links resolve and the documentation build succeeds.
- Another reviewer has compared the page with both the UI and daemon implementation.
