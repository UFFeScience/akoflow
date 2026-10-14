# Montage tooling in Go

Run from the repository root:

```sh
go run ./cmd/montage-tools <command> <arguments>
go test -race ./internal/montagetools ./cmd/montage-tools
```

Alternatively, build once with `go build -o /tmp/akoflow-montage-tools ./cmd/montage-tools`.

The migrated commands preserve the positional arguments of the former scripts;
use the former basename without `.rb` as the command:

- `audit_montage_results`
- `build_montage_execution_calibration`
- `build_montage_workspace_projection`
- `build_montage_work_features`
- `extract_montage_compute_work`
- `freeze_montage_compute_profiles`
- `freeze_montage_materialization`
- `prepare_montage_measured_requests`
- `supplement_montage_root_compute`
- `validate_montage_transfer_model`
- `export_cloud_workspace`

Missing or invalid arguments produce a usage error and nonzero exit status.
Validation modes are `calibrate`, `supplement`, `request`, `compare`,
`shared-capacity`, and `report`. Request generation accepts simulation requests
only; it does not submit a real execution.

Scientific JSON exports are private (0600). Historical evidence and predictions
are not rewritten. Newly serialized JSON may have a different SHA256 because Go
orders object keys deterministically; source-file digests remain provenance of
the original input bytes. Numerical equivalence tests allow only floating-point
rounding tolerance, not changes to routes, volumes, or model contracts.

Only the explicitly requested cloud workspace export calls the daemon API,
using `AKOFLOW_API_TOKEN` without printing it. It verifies chunk lengths and the
archive checksum and refuses to overwrite an existing archive. Compute-input
extraction reads an existing container image via Docker; it does not execute a
workflow. None of these tools accesses the database directly or restarts the
daemon. Ruby is no longer required for these repository tools or their tests.
