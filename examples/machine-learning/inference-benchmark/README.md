# Inference Benchmark

A reproducible AkôFlow showcase. It executes a real Docker image locally, writes deterministic artifacts, captures three browser screenshots, and validates its output contract.

## Run

```sh
./run.sh
```

Requires Docker, Node.js, and a Playwright-compatible Chromium installation (the screenshot command provisions it when needed). The workflow graph is in [workflow.yaml](./workflow.yaml); the expected contract is in [expected/manifest.json](./expected/manifest.json).

Published image: `ghcr.io/uffescience/akoflow-showcase-inference-benchmark:v1`.

