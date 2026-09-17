# Activity resource telemetry

Real Kubernetes containers, Slurm batch jobs, and local/remote Docker
containers can publish CPU, memory and block-I/O measurements for each
activity attempt. The Kubernetes and Slurm collectors read Linux
cgroup v2 counters every 10 seconds by default (configurable from 5 to 60
seconds with the activity environment variable `AKOFLOW_METRIC_INTERVAL_SECONDS`). It takes an initial and final
sample, so short activities can still report a peak memory observation.

The Slurm adapter writes `<job-sentinel>.metrics.tsv` next to its status
sentinel, outside the artifact workspace, and imports new rows while the job
runs. It declines to collect if the job does not have a non-root cgroup path,
because a host-wide counter must not be attributed to one activity. The
Kubernetes adapter writes a temporary file outside the artifact workspace and
publishes its rows through marked pod-log lines, allowing live retrieval and
post-completion recovery. Metric markers are removed from user-visible logs.
The Docker adapters make a bounded `docker stats` call at most once per 10
seconds while a container is running. They do not change the activity image or
command and remain optional if Docker does not expose stats. Docker CPU time is
integrated from its percentage and is therefore approximate, unlike cgroup
CPU time.

Measurements are best-effort: missing cgroup v2 support, a metrics read error
or a database write error must not fail the scientific activity. Samples are
stored in an additive SQLite table; this does not change the canonical schema
checksum or require database recreation. Older read-only snapshots without
that table simply return no measurements.

`GET /akoflow-api/execution-runs/{runId}/activity-metrics/` returns compact
summaries. `GET /akoflow-api/execution-runs/{runId}/activities/{activityId}/metrics/?attempt=1`
returns a bounded time series (at most approximately 1,200 points). The
`.../activities/{activityId}/metrics/summary/?attempt=1` endpoint reads only
the selected activity's aggregate for its detail page. The
normal run-detail response never embeds the series. CPU and I/O values are
cumulative counters; memory is an instantaneous gauge. Summary CPU cores and
I/O bytes are calculated from counter deltas, and peak memory from all stored
samples, not from the downsampled chart.

The first release does not claim per-activity network or GPU usage. Bare local
processes without a container also remain uninstrumented. The UI reports
telemetry as unavailable for these cases rather than showing a misleading zero.
