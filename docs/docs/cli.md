---
id: cli
title: Legacy CLI
sidebar_label: Legacy CLI
description: Current status and source-level contract of the experimental akoflow command.
---

:::caution Not the installation interface
The current user-facing installation and lifecycle flow is **AkôFlow Desktop** plus the versioned Docker Compose bundle. The source-tree `akoflow` command does not install, start, stop, restart, update, or reset the packaged stack.
:::

The experimental Go command currently exposes one source-level operation:

```bash
akoflow run -file workflow.yaml -host localhost -port 8080
```

It validates the file, host, and port, Base64-encodes the file, and sends the legacy request to:

```text
POST http://<host>:<port>/akoflow-server/workflow/
```

That legacy path is not part of the current control-plane router under `/akoflow-api/`. Do not use this command for new automation unless the target deployment explicitly provides the compatibility endpoint.

For supported workflows:

- install and operate the packaged stack through [Installation](installation);
- create workflows through [AkôFlow Desktop or the current API](guides/workflows/definitions);
- use the [API overview](reference/api-overview) for scripts and integrations.
