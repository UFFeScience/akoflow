# Scheduling algorithm library

The daemon includes HEFT, PRISM Time and PRISM Cost as native schedulers. PRISM
implementations are marked as AkôFlow exclusive. `algorithms.Sources` embeds the
actual implementation and shared helpers for inspection and export.

A custom scheduler is a Go source file with `package main`, optional standard
library imports, and this function:

```go
func Schedule(input Input) ([]Plan, error)
```

The generated `contract.go` defines Input, Request, Workflow, Resource, Plan,
Assignment and Metrics. The request is the same frozen planning input used by
native schedulers. The function returns complete plans with activity/resource
IDs, scope and workflow IDs, predicted intervals and metrics. The coordinator
validates candidates through its existing validator before storing them.

`GET /akoflow-api/scheduling-algorithms/template/` returns the template and complete
contract. The collection supports GET and POST, while `/{algorithmId}/` supports
GET and PUT. GET detail is also the portable JSON package export. Native IDs
cannot be overwritten. Package schema is `akoflow.scheduler/v1`.

Source is stored in the instance's SQLite database in `scheduler_plugin_sources`.
The latest edit and last successfully compiled source are preserved independently.
A failed build returns the saved edit and compiler diagnostics without replacing
the active scheduler. Successful edits atomically replace the registry entry;
existing runs keep their original scheduler object and immutable executable.
Compile artifacts are content-addressed under `scheduler-plugins/` beside the
database. Missing executables are rebuilt from the active database source at
startup. Instance exports include the database source; binary artifacts are
rebuildable and platform-specific.

The compiler uses no shell, disables module downloads and CGO, and has a 60-second
deadline. Execution uses a separate executable with JSON stdin/stdout, a two-minute
deadline, bounded output and cancellation through the planning context. Plugins
run with the daemon's filesystem and system access; only install trusted code.
The child environment does not inherit daemon credentials. This is a portable Go
function plugin, not a Go `-buildmode=plugin` shared object: different revisions
can be compiled and loaded without restarting the engine or Go ABI coupling.

The production image includes the compiler toolchain. Native algorithms remain
available if a custom function fails compilation. Run
`go test ./internal/planning/library ./internal/planning/algorithms` to exercise
compilation, JSON execution, persistence, failed-edit retention and native-ID
protection.

The editor also accepts just the `Schedule` function (and optional import
statements); the daemon supplies `package main` when omitted. Revision payloads
are preserved in `scheduler_plugin_revisions`. Candidate plans record the active
revision and source hash in `algorithmVersion` for later reproducibility.
