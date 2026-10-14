# Offline planning and simulation export

Export an existing simulation request without submitting it:

```sh
go run ./cmd/simgrid-case --request private-request.json --output new-case-dir
```

Generate experimental PRISM-time candidates first, requiring measured compute
profiles, then export the first time-ranked candidate for native SimGrid:

```sh
go run ./cmd/simgrid-case --prism-time --request private-request.json --output new-case-dir
```

Input must be simulation-only. The caller supplies the complete resource scope,
topology, evidenced materialization profiles, compute profiles and runtimes.
Missing materialization route profiles do not acquire guessed rates. Both the
planner and simulation exporter consume the same request model. Timeout is ten
minutes; beam width and candidate count use PRISM defaults (120 and 25).
Private exports include candidates, selected request, platform and native input.
The output directory must not exist. This tool does not contact the daemon,
access the database, provision cloud resources or start a real execution.

Model agreement is not independent real validation. This command does not
estimate missing physical interference or validate unobserved route classes.
