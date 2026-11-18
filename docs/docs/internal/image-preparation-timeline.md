---
title: Image preparation timeline
---

The execution timeline exposes `image-pull`, `image-build`, and
`image-cache-hit` phases. The daemon associates each observation with the run,
activity, and attempt. An active phase carries elapsed time, and a terminal
phase preserves its outcome and completion time. Runs created before these
observations were introduced may have no image preparation phases.

Local Docker inspects the requested platform before starting a container.
When it confirms the image is available, the timeline records a cache hit.
Otherwise, an explicit pull records the command's elapsed time and outcome.
Activities in the same run waiting for an in-flight pull share its operation
ID. A portable Dockerfile reproduction measures the BuildKit build and the
subsequent Docker image load together.

The pull and build totals measure accumulated work. The timeline computes
wall-clock coverage from merged intervals and keeps the existing compute
makespan separate. An observed preparation interval is removed from inferred
queue and launch time for that attempt.

Remote Docker and Kubernetes launches currently provide no reliable,
attempt-scoped pull timing to this timeline. A remote launch can pull an
image internally, and Kubernetes can reuse an image on the chosen node.
The daemon leaves these cases unclassified unless it observes an explicit
preparation command itself. Pull and build progress percentages are optional.
