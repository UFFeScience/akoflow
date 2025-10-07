---
title: 2 - Installation
description: Here you will find instructions on how to install AkôFlow in different environments, including local clusters and cloud platforms.
layout: page
---

## Software Requirements

- **Operating System:** Linux, macOS or WSL2 (Windows Subsystem for Linux)
- **Docker:** [Install Docker](https://docs.docker.com/get-docker/)
- **kubectl:** [Install kubectl](https://kubernetes.io/docs/tasks/tools/)
- **Kubernetes Cluster:** One of the following:
  - [Kind](https://kind.sigs.k8s.io/) (local)
  - Docker Desktop Kubernetes (enable Kubernetes in settings)
  - Cloud providers (e.g., EKS, GKE, AKS)


## Quick Installation

Run the following command to install AkôFlow:
```bash
curl -fsSL https://akoflow.com/run | bash
```

AkôFlow will be available at `http://localhost:8080`.

