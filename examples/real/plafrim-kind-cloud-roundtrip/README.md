# PlaFRIM → Kind → GCP → PlaFRIM

This real-execution fixture has four ordered activities and three cross-environment
workspace transfers. The PlaFRIM source writes `payload.txt` and its SHA-256;
Kind verifies both and adds `kind.receipt`; GCP verifies those files and adds
`cloud.receipt`; the final PlaFRIM activity verifies all three and writes
`final.receipt`. A completed run therefore proves the content survived the
entire route, not just that the activities started.

The scope and topology refer to the connected `hpc-environment`, `kind-akoflow`
and `goal-gcp-worker` versions used by this fixture. Review their IDs, credentials,
and cloud capacity target before importing the files. The `testing` PlaFRIM
partition is used instead of computing on the login node. The OCI BusyBox
executable is materialized as SIF for Slurm; Kind and GCP use the OCI image.

Import `scope.json`, `network.json`, `workflow.yaml`, then `plan.yaml` through
the corresponding API endpoints. Set `AKOFLOW_API_TOKEN` and run:

```sh
sh examples/real/plafrim-kind-cloud-roundtrip/submit-run.sh roundtrip-run-1
```

The execution request includes the PlaFRIM entrypoint resource because its
discovered `homeDirectory` is needed to resolve workspace transfer paths.
The cloud allocator must not prewarm at run submission: it provisions when
`cloud-relay` becomes ready after Kind. It destroys the VM after the run.
The manual plan's predicted timestamps are an illustrative lifecycle schedule,
not a calibrated estimate of PlaFRIM queue or VM setup time.

Verified on 2026-09-13: `plafrim-kind-cloud-roundtrip-run-v4` completed 4/4
activities, transferred 116 B PlaFRIM → Kind, 130 B Kind → GCP, and 145 B
GCP → PlaFRIM (391 B total). Observed makespan was 545.536 s. The GCP instance
was `destroyed` after the run. The Kind service-account token had expired before
this test and was renewed through `POST /kubernetes-tokens/`.
