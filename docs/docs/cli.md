---
id: cli
title: CLI Reference
sidebar_label: CLI (akoflow)
---

The `akoflow` command-line tool is the fastest way to get AkôFlow running on your machine.  
It manages a local server (Docker container) with simple commands — no YAML, no configuration files.

---

## Installation

**Requires:** [Docker](https://docs.docker.com/get-docker/) installed and running.

```bash
curl -fsSL https://akoflow.com/run | bash
```

This downloads and installs the `akoflow` binary to your system.  
Run `akoflow --help` to confirm the installation.

:::tip macOS / Linux without sudo
If you prefer not to install system-wide, the script also supports a local install. Follow the prompts after running the command.
:::

---

## Starting the server

```bash
akoflow
```

On the first run, the CLI pulls the `akoflow/akoflow` Docker image and starts the server.  
You will see:

```
  ➜  ~ akoflow     

   █████╗ ██╗  ██╗ ██████╗ ███████╗██╗      ██████╗ ██╗    ██╗
  ██╔══██╗██║ ██╔╝██╔═══██╗██╔════╝██║     ██╔═══██╗██║    ██║
  ███████║█████╔╝ ██║   ██║█████╗  ██║     ██║   ██║██║ █╗ ██║
  ██╔══██║██╔═██╗ ██║   ██║██╔══╝  ██║     ██║   ██║██║███╗██║
  ██║  ██║██║  ██╗╚██████╔╝██║     ███████╗╚██████╔╝╚███╔███╔╝
  ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚═╝     ╚══════╝ ╚═════╝  ╚══╝╚══╝
  ...

  AkôFlow is running

  Access      http://localhost:8080
  Container   akoflow  (904b103f882f)
  Image       akoflow/akoflow

  akoflow stop      stop the server
  akoflow restart   restart the container
  akoflow logs      stream live logs
  akoflow reset     remove and start fresh
  akoflow --help    show all commands
```

Open **http://localhost:8080** in your browser to access the dashboard.

---

## Commands

### `akoflow`

Starts the AkôFlow server. If the container already exists, resumes it. If the image is not present, pulls it first.

```bash
akoflow
```

---

### `akoflow stop`

Stops the running container without removing it. Data is preserved.

```bash
akoflow stop
```

To start again, run `akoflow`.

---

### `akoflow restart`

Stops and immediately restarts the container.

```bash
akoflow restart
```

---

### `akoflow logs`

Streams live logs from the running container. Press `Ctrl+C` to stop.

```bash
akoflow logs
```

Useful for debugging workflows or checking what the server is doing.

---

### `akoflow reset`

Stops and removes the container and its data, then starts fresh.

```bash
akoflow reset
```

:::warning
This deletes all local workflow data. Use it when you want a clean slate or to upgrade to a new image version.
:::

---

### `akoflow --help`

Shows all available commands and options.

```bash
akoflow --help
```

---

## Update

To update to the latest version of AkôFlow, run reset — the CLI will pull the newest image:

```bash
akoflow reset
```

Or pull the image manually and restart:

```bash
docker pull akoflow/akoflow:latest
akoflow restart
```

---

## Uninstall

```bash
akoflow reset          # remove container and data
docker rmi akoflow/akoflow  # remove the image
sudo rm $(which akoflow)    # remove the CLI binary
```

---

## Next steps

- [Modules](modules) — understand what's running inside the server
- [Downloads](downloads) — Desktop App, Engine binaries, and Docker images
- [User Guide](user-guide) — create your first workflow
