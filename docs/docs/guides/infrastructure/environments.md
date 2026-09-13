---
title: Create and inspect environments
---

An environment describes where AkôFlow can plan or run work. A **real environment** has an execution runtime such as local, SSH, Kubernetes, SLURM, or cloud. A **simulation environment** uses the SimGrid runtime and models resources without connecting to physical infrastructure.

Environment definitions are versioned. Execution scopes and plans refer to an environment **version**, so changing an environment does not silently change existing planning inputs.

## Create an environment

### Using AkôFlow Desktop

1. Open **Infrastructure → Environments** and select **Connect environment** for a real target or **Create simulation** for a modeled platform.
2. Choose the environment type. Use a simulation environment when you need modeled resources only; use a real environment when AkôFlow must connect to infrastructure.
3. Enter the environment name and the fields shown for the selected runtime.
4. For a real remote environment, configure its connection and credential reference. Secrets are stored by the daemon; the environment keeps a reference rather than the secret value.
5. Test the connection when the form offers the action, then save the environment.

<img src={require('@site/static/img/interface/infrastructure/environments.png').default} alt="AkôFlow Desktop Environments catalog showing the execution and simulation tabs, environment health, connection state, runtime type and inventory summary." />

*The catalog separates connection-backed targets from SimGrid models. Each card makes its health, connection or cloud-access state, runtime and discovered inventory visible before you open it. Use **Connect environment** for a real target and **Create simulation** for a modeled platform; the form that follows determines the appropriate fields.*

Simulation creation collects a SimGrid platform model and can also define an execution host used by the simulation engine. It does not validate a remote SSH or Kubernetes endpoint.

### Using the API

Complete [API connection setup](../../tutorials/api-access) before running these commands.

The creation body needs more than an environment name. This local example includes a version, runtime, resource, and runtime binding:

```bash
curl --fail-with-body \
  -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -H 'Content-Type: application/json' \
  -X POST "$AKOFLOW_API_URL/environments/" \
  -d '{
    "environment":{"id":"local-lab","name":"Local lab","status":"defined"},
    "version":{"id":"local-lab-v1","environmentId":"local-lab","version":1,"status":"published"},
    "runtimes":[{"environmentVersionId":"local-lab-v1","id":"local-lab-local","name":"Local execution","driver":"local","mode":"execution","capabilities":{"container":true}}],
    "resources":[{"id":"local-lab-machine","environmentVersionId":"local-lab-v1","executionTarget":"direct","type":"local_machine","name":"Local machine","cpuCores":4,"memoryBytes":8589934592}],
    "resourceRuntimeBindings":[{"resourceId":"local-lab-machine","runtimeId":"local-lab-local"}]
  }'
```

For a remote connection, follow the complete [HPC registration](../../tutorials/register-hpc) or [Google Cloud connection](../../tutorials/connect-cloud) tutorial. Each shows how to obtain a credential reference, test the connection, and save the environment.

## Validate health and discover infrastructure

### Using AkôFlow Desktop

1. Open the environment detail page.
2. In **Connections**, run the health check for the connection you want to verify.
3. Run discovery to refresh observed resources and storage.
4. Open **Inventory** to review the resulting resources and host-observed filesystems.

Health and discovery are different operations: health verifies access; discovery collects snapshots and updates the infrastructure AkôFlow can expose. Discovery results depend on the connection and runtime driver.

### Using the API

Use the ID of a connection already saved in the environment. For the HPC tutorial's template, that ID is `research-hpc-connection`.

```bash
AKOFLOW_CONNECTION_ID='research-hpc-connection'
# Check the connection
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -X POST "$AKOFLOW_API_URL/environment-connections/$AKOFLOW_CONNECTION_ID/health/"

# Discovery through that connection
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  -X POST "$AKOFLOW_API_URL/environment-connections/$AKOFLOW_CONNECTION_ID/discover/"

# Recent health history
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environment-connections/$AKOFLOW_CONNECTION_ID/history/?limit=20"
```

Discovery returns a `snapshots` array. A successful request does not imply that every possible resource type was found; inspect the returned snapshots and the environment inventory.

## Inspect and navigate an environment

### Using AkôFlow Desktop

The detail page is the hub for the environment map, version, runtimes, connections, resources, and storage. Cloud environments also expose **Cloud capacity** and **Provisioning** tabs. Use **Inventory** for discovered compute details and **Storage** to browse approved roots.

### Using the API

```bash
curl --fail-with-body -H "Authorization: Bearer $AKOFLOW_API_TOKEN" \
  "$AKOFLOW_API_URL/environments/local-lab/"
```

The response is the full definition, including the current version and related runtimes, resources, connections, and discovered storage when present.
