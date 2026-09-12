---
title: Capturing Desktop documentation
description: Reproducible Electron screenshots using the shared QA environment and an operational Core.
---

# Capturing Desktop documentation

Desktop screenshots are generated with tools installed in the shared QA
environment. Playwright, Chromium, Xvfb, browser profiles, fixtures, traces, and
videos are not repository dependencies and must not be added to either
`package.json` or lockfile.

The selected installation image below was captured on 2026-09-12 from the real
Electron window against Core `caac328` and Desktop `157fed5`, with the renderer
served at `http://127.0.0.1:5174`. The Core `/akoflow-api/instance/` endpoint
returned HTTP 200. The image is 1280 × 860 pixels; the wrapper ran Electron
43.4.1 via Playwright 1.63.0 and Xvfb 21.1.22 with a 1440 × 1000 × 24 virtual
screen. The shared browser-only tool used Chromium 153.0.8010.12 at the time of
verification; Electron capture uses Electron's bundled Chromium instead.

![AkôFlow Desktop welcome step connected to an operational local Core, before environment configuration.](/img/interface/desktop/installation-welcome.png)

## Capture procedure

Use an isolated Core database and a Desktop checkout with its locked
dependencies already installed. Start the Core and renderer using values that
are local to the capture session:

```bash
AKOFLOW_DATABASE_PATH="$PAPERCLIP_RUN_SCRATCH_DIR/akoflow.db" \
AKOFLOW_HTTP_ADDRESS=127.0.0.1:8080 \
AKOFLOW_API_TOKEN=<temporary-token> \
AKOFLOW_API_ALLOWED_ORIGINS=http://127.0.0.1:5174 \
/usr/local/bin/go run ./cmd/server

AKOFLOW_ENGINE_URL=http://127.0.0.1:8080 \
AKOFLOW_API_TOKEN=<temporary-token> \
npm run dev -- --host 127.0.0.1 --port 5174
```

Confirm readiness, then capture to a run-owned temporary path:

```bash
curl --fail http://127.0.0.1:8080/akoflow-api/instance/
akoflow-electron-screenshot \
  <checkout-desktop> \
  "$PAPERCLIP_RUN_SCRATCH_DIR/candidate.png" \
  http://127.0.0.1:5174
```

For a browser-only documentation page, use
`akoflow-web-screenshot <url> <output.png>`. These commands use Xvfb
automatically. Keep all candidates, traces, videos, and browser profiles outside
the repositories.

## Approval checklist

Before copying a candidate to `docs/static/img/interface/<area>/`:

1. Confirm it is an Electron window, not a browser substitute.
2. Confirm the Core endpoint returns success and the UI is complete. Reject
   `502`, “Instance identity unavailable”, loading, or partial states.
3. Inspect the image at original resolution. Reject credentials, tokens, real
   hostnames, usernames, transient notifications, and runtime-generated IDs.
4. Use a lowercase task-oriented filename, preserve its resolution, and add
   descriptive alternative text where it is referenced.
5. Run the documentation typecheck, build, link check, and `git diff --check`.

An otherwise healthy Overview capture from this verification was rejected
because its footer exposed the daemon identity generated from the machine
hostname. It remains temporary and is not documentation. This is expected QA
behavior: a working screen is not canonical until it is stable and redacted by
the product or avoided by the selected documentation moment.

Replace a canonical file in place when the documented state is unchanged.
Rename it only when the task changes, updating every reference in the same
commit. Remove the image and references together when the screen no longer
exists. Never keep stale captures as an in-repository archive.
