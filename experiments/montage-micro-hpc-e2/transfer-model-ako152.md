# AKO-152: independent preparation and relay service

Planning (PRISM) and the native SimGrid exporter consume the same materialization
profile. For gateway routes, two network legs remain mandatory. Before ingress,
source preparation costs `workspaceBytes * snapshotSecondsPerByte`; ingress moves
archive bytes at its own service rate; relay validation follows ingress; upload
and extraction precede the joint consumer merge. Shared topology links account
for concurrent bandwidth demands, and bulk service slots account for admission
wait. Observed admission wait must not enter service coefficients again.

An independently measured ingress profile must set
`ingressIncludesSourcePreparation=false` and provide
`separatedIngressServiceSource`. The freezer accepts this only with both
`source-snapshot` and `archive-ingress` queue-free phase observations. A combined
pack-and-receive profile cannot also charge a source snapshot coefficient.

## What the existing evidence supports

The completed Montage reference and auxiliary diagnostic measure pack-and-receive
as combined service, not independent CPU/disk and network service. They therefore
retain their combined profile. The aggregate logical bottleneck is not a measured
NIC rate. No independent rates, per-file overhead, destination service pool, or
new performance improvement can be inferred from these samples.

The AKO-151 fixed-placement result remains 2835.602 s versus 3150.634 s real, with
large compensating dependency errors. It is not accepted as model validation.
Synthetic PRISM and native tests verify independent preparation adds 2 s to the
same fixture (13 → 15 s), retains 200 network bytes, and does not add queue.
Existing native shared-link and bulk-slot tests verify concurrent sharing and
admission separately. This is a model-contract improvement, not a new calibrated
Montage forecast or a new plan.

## Remaining calibration gate

Before enabling separated coefficients for Montage, obtain independently timed
source snapshot and ingress, queue-free receiver extraction, and evidence of
per-file overhead if file count is to become a predictor. A pipelined pack/send
must not be replaced by serial independent services without proving that timing
contract. Use an independent completed reference for validation where available;
do not tune coefficients to makespan alone. New planning remains blocked by the
dominant transfer residuals, not by the implementation of the profile contract.
No real execution or daemon restart is required for these offline tests.

## Verification

Race-enabled Go tests passed for the shared domain model, planner and SimGrid
provider. Native SimGrid: 12 tests passed. Freezer: 8 tests / 27 assertions;
offline validation: 4 tests / 23 assertions. The 58-task fixed-placement native
regression reproduced all executed aggregates exactly, including makespan
2835.6021967566785 s. Previous forecasts and raw evidence remain unchanged.
