# Frozen Figure 8 campaign (2026-11-01)

Run `freeze_guard_campaign.go` once after the six final native simulations.
The tool rejects changed scientific inputs within an environment, mismatched
native trace identities, incomplete simulations and overwriting an existing
capsule. The manifest hashes the complete request, plan, planning request and
native trace. None of the new real observations may update these calibrations
before their prediction errors are evaluated.

The equivalence groups compare resource, explicit slot, order and priority,
bijective homogeneous-core labels and every lane predecessor. An equivalent
physical deployment can supply one shared real observation; retain separate
algorithm forecasts and label the shared observation. Historical deployments
must retain their historical protocol/version and their equivalence proof.

Run each distinct deployment sequentially. Audit it with
`cmd/montage-order-audit --case hpc|medium`. Preserve rejected and aborted attempts;
never turn them into completed measurements or restart an active run merely
because an observation request timed out.

Run `collect_guard_products.go RUN ENV DIRECTORY` beside each deployment to
archive the four final Montage PNGs. It verifies size, SHA256 and PNG structure
against the published workspace manifest. For cloud runs execute it inside the
daemon container with `FIG8_EXPORT_LEASES=1` and `FIG8_API` pointing to that
daemon. This creates actual pending export-consumer leases before publication,
so native consumer-safe teardown cannot delete an unexported final product.
Release each lease only after verified local export; restart the collector with
the same arguments after an observation failure. Retain leases if export fails.
After successful export, retry normal resource teardown and verify the provider
inventory. Preserve base data, images and credentials; remove only campaign
workspaces, instances, disks, firewall rules and temporary credential files.

The operational SQLite path is isolated and schema recreation disabled. A
catalog recovery must preserve the original backup and its checksum. Recovery
of a historical catalog does not restore newer execution rows: use their raw
exports as historical evidence without inventing replacement completed runs.

Export with `export_guard_campaign.go ROOT DEPLOYMENTS_JSON` only after all
three fresh deployments pass the final order and product audits. The source map
uses frozen equivalence-group keys and explicit run/request paths. A historical
PRISM HPC source additionally requires its zero-resource/slot/order-change
proof. The exporter rejects incomplete runs, changed capsules, mismatched plan
IDs and real durations inconsistent with their 58 completed task timestamps.

The plotted common duration is first activity start before launch overhead to
final process completion. For scheduler assignments remove fixed launch
components from compute-start timestamps; remove modeled final workspace
publication from native finish timestamps. Retain the original end-clock
predictions and wall duration in separate CSV/JSON fields. Report absolute
prediction error as `100 * abs(prediction - real) / real`. The gray interval is
only the minimum and maximum of the three observations; its annotation is
`max - min` and `100 * (max - min) / planned`, never a confidence interval.

Transfer exports separate workflow network counters from executable
materialization, using the recorded artifact IDs. They also retain the
engine's original aggregate byte/time counters. These are model/runtime
counters, not packet capture: a same-filesystem operation may report zero
network bytes while still spending time preparing its workspace. Costs use the
frozen resource prices and retain scheduler/execution estimates separately;
they must not be described as provider invoices.
