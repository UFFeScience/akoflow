# Workspace archive relay

Independent dependencies with an SSH destination are prepared concurrently,
with eight workers by default and a maximum of eight. Set
`AKOFLOW_WORKSPACE_TRANSFER_PARALLELISM` (1–8) on the daemon to select a smaller
limit; `WorkspaceRsync.Parallelism` controls the same bound internally. This is
a per-consumer preparation limit, not a global process or CPU-count limit.
Eight workers can increase contention on shared disks, links and small VMs;
it is a configured experiment limit, not a measured optimal value.
Proven shared NFS/Lustre/BeeGFS routes remain internal
and are not archived or counted as relay network traffic.

Cloud-to-HPC and HPC-to-cloud dependencies always use the relay. Proven
HPC-to-HPC shared storage uses internal copy/merge. The same concrete cloud
instance uses runtime-local copy; distinct eligible cloud instances may use
authenticated private peer transfer, with an explicit relay fallback when the
required generation trust or connectivity cannot be proven. Route selection is
per dependency, including mixed-origin fan-in.

Relay routes pack a complete gzip-compressed tar snapshot, receive it through a
daemon-local temporary file, and upload it into a unique remote preparation
directory. The daemon rejects traversal, duplicate entries, links and special
files before uploading. The receiver verifies the archive SHA-256 before
extraction. No archive is extracted over the consumer workspace.

Every preparation must complete before aggregate path/content conflict checking,
merge and final comparison. Workers are joined on cancellation before their
temporary directories are cleaned. Producer workspaces are never modified.
Recovery repeats isolated preparation and reuses identical published consumer
files through the existing merge contract. Eligible private cloud peers may
reuse a sealed source snapshot only with its source manifest and generation
revalidated. Transfer preparation itself does not launch activity commands.

Transfer route metadata exposes `packaging: tar-gzip`,
`preparationStartedAt`, `preparationFinishedAt` and `preparationSeconds`.
`durationSeconds` includes the final aggregate barrier and is not additive across
parallel dependencies. Network bytes count archive payload received by the relay
plus rsync bytes uploaded; they do not represent unique logical output bytes.

This is not streaming forwarding: receipt of an individual archive precedes its
upload, while different dependencies overlap. Full snapshots can transmit more
bytes than incremental rsync when much of the destination already exists. No
live-network speedup is claimed by local concurrency/integrity tests.

Deployment requires a safe execution boundary. Changing this checkout does not
change an already running daemon; do not restart it during valid active jobs.

Tracked by issue #101.
