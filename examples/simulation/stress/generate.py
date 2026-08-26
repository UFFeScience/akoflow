#!/usr/bin/env python3
import argparse
import json
import os
import urllib.error
import urllib.request


def post(api, token, endpoint, payload):
    request = urllib.request.Request(
        f"{api}/{endpoint}/",
        data=json.dumps(payload, separators=(",", ":")).encode(),
        method="POST",
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=180) as response:
            response.read()
            return response.status
    except urllib.error.HTTPError as error:
        detail = error.read().decode(errors="replace")
        raise RuntimeError(f"{endpoint}: HTTP {error.code}: {detail[:1000]}") from error


def build(total, cores):
    workers = total - 2
    prefix = f"simgrid-{total}-activities-{cores}-cores"
    workflow_id = f"{prefix}-workflow"
    version_id = f"{workflow_id}-v1"
    environment_id = f"{prefix}-environment"
    environment_version = f"{environment_id}-v1"
    runtime_id = f"{prefix}-runtime"
    scope_id = f"{prefix}-scope"
    topology_id = f"{prefix}-network"
    plan_id = f"{workflow_id}-plan-v1"
    run_id = f"{workflow_id}-run-v1"

    def activity(name, priority):
        return {
            "id": f"{workflow_id}-{name}", "workflowVersionId": version_id,
            "activityTypeId": f"{workflow_id}-activity", "externalId": name,
            "name": name, "kind": "task", "capabilities": ["simulation"],
            "command": {}, "resources": {"cpu": 1, "memoryBytes": 1073741824, "storageBytes": 0},
            "simulation": {"model": "deterministic", "durationSeconds": 10},
            "policy": {"maxAttempts": 1}, "priority": priority,
        }

    activities = [activity("source", 100)]
    activities += [activity(f"worker-{number:04d}", 50) for number in range(1, workers + 1)]
    activities += [activity("sink", 10)]
    dependencies, data_dependencies = [], []
    for number in range(1, workers + 1):
        name = f"worker-{number:04d}"
        worker_id = f"{workflow_id}-{name}"
        dependencies += [
            {"activityId": worker_id, "dependsOnActivityId": f"{workflow_id}-source", "type": "control"},
            {"activityId": f"{workflow_id}-sink", "dependsOnActivityId": worker_id, "type": "control"},
        ]
        data_dependencies += [
            {"producerActivityId": f"{workflow_id}-source", "consumerActivityId": worker_id,
             "logicalName": f"source-to-{name}.bin", "sizeBytes": 10_000_000_000},
            {"producerActivityId": worker_id, "consumerActivityId": f"{workflow_id}-sink",
             "logicalName": f"{name}-to-sink.bin", "sizeBytes": 10_000_000_000},
        ]
    workflow = {"id": version_id, "workflowId": workflow_id, "version": 1,
                "definitionHash": version_id, "activities": activities,
                "dependencies": dependencies, "dataDependencies": data_dependencies}

    resources = []
    for machine, machine_cores in (("m1", 1), ("m2", cores), ("m3", 1)):
        resources.append({
            "id": f"{prefix}-{machine}", "environmentVersionId": environment_version,
            "executionTarget": "batch", "type": "local_machine", "name": machine.upper(),
            "providerId": f"{prefix}-{machine}", "tier": "compute", "cpuCores": machine_cores,
            "cpuCapacity": machine_cores, "memoryBytes": 274877906944, "storageBytes": 1099511627776,
            "computeSpeedup": 1, "pricePerSecond": 0, "schedulable": True,
        })
    runtime = {"environmentVersionId": environment_version, "id": runtime_id,
               "name": f"SimGrid {total}", "driver": "simgrid", "mode": "simulation",
               "role": "simulation", "capabilities": {"simulation": True}}
    bindings = [{"resourceId": resource["id"], "runtimeId": runtime_id, "enabled": True}
                for resource in resources]
    environment = {
        "environment": {"id": environment_id, "name": f"SimGrid {total} activities",
                        "description": f"{total} activities over {cores} M2 cores", "status": "ready"},
        "version": {"id": environment_version, "environmentId": environment_id, "version": 1,
                    "status": "published", "networkModel": "static-links",
                    "interferenceModel": "none", "costModel": "per-second",
                    "configurationHash": environment_version},
        "runtimes": [runtime], "resources": resources, "storages": [], "connections": [],
        "activityResourceProfiles": [], "resourceRuntimeBindings": bindings,
    }
    scope = {"id": scope_id, "name": f"SimGrid {total} activities scope",
             "environmentVersionIds": [environment_version]}
    topology = {"id": topology_id, "name": f"{total}-activity shared network", "version": 1,
                "executionScopeId": scope_id, "links": []}
    for source, target in (("m1", "m2"), ("m2", "m3")):
        topology["links"].append({
            "id": f"{prefix}-{source}-{target}", "topologyId": topology_id,
            "sourceResourceId": f"{prefix}-{source}", "targetResourceId": f"{prefix}-{target}",
            "bandwidthBitsPerSecond": 80_000_000_000, "latencySeconds": 0, "pricePerByte": 0,
            "bidirectional": True, "sharingPolicy": "shared", "maxConcurrentTransfers": workers,
        })

    assignments = [{
        "id": f"{prefix}-plan-source", "planId": plan_id,
        "activityId": f"{workflow_id}-source", "resourceId": f"{prefix}-m1",
        "coreId": "core-0", "slotId": "default", "orderOnResource": 1, "priority": 100,
        "predictedReadyAt": 0, "predictedStartAt": 0, "predictedFinishAt": 10,
        "predictedRuntimeSeconds": 10,
    }]
    fanout_finish = 10 + workers
    for index in range(workers):
        number, wave = index + 1, index // cores
        start = fanout_finish + wave * 10
        assignments.append({
            "id": f"{prefix}-plan-worker-{number:04d}", "planId": plan_id,
            "activityId": f"{workflow_id}-worker-{number:04d}", "resourceId": f"{prefix}-m2",
            "coreId": f"core-{index % cores}", "slotId": "default", "orderOnResource": wave + 1,
            "priority": 50, "predictedReadyAt": fanout_finish, "predictedStartAt": start,
            "predictedFinishAt": start + 10, "predictedRuntimeSeconds": 10,
            "predictedTransferSeconds": workers,
        })
    waves = (workers + cores - 1) // cores
    sink_start = fanout_finish + waves * 10 + workers
    makespan = sink_start + 10
    assignments.append({
        "id": f"{prefix}-plan-sink", "planId": plan_id, "activityId": f"{workflow_id}-sink",
        "resourceId": f"{prefix}-m3", "coreId": "core-0", "slotId": "default",
        "orderOnResource": 1, "priority": 10, "predictedReadyAt": sink_start,
        "predictedStartAt": sink_start, "predictedFinishAt": makespan,
        "predictedRuntimeSeconds": 10, "predictedTransferSeconds": workers,
    })
    plan = {"id": plan_id, "workflowVersionId": version_id, "executionScopeId": scope_id,
            "networkTopologyId": topology_id, "source": "imported", "algorithm": "manual-stress",
            "algorithmVersion": "1", "objective": "makespan", "deadlineSeconds": makespan,
            "budget": 0, "predicted": {"makespanSeconds": makespan, "cost": 0, "feasible": True},
            "assignments": assignments}
    definition_activities = []
    for item in activities:
        name = item["externalId"]
        definition = {"name": name, "runtime": runtime_id, "cpuLimit": "1", "memoryLimit": "1Gi",
                      "simulation": {"model": "deterministic", "durationSeconds": 10}}
        if name.startswith("worker-"):
            definition["dependsOn"] = ["source"]
        elif name == "sink":
            definition["dependsOn"] = [f"worker-{number:04d}" for number in range(1, workers + 1)]
        definition_activities.append(definition)
    definition = {"name": workflow_id, "spec": {"namespace": "examples",
                  "activities": definition_activities,
                  "dataDependencies": [{"producerActivity": item["producerActivityId"].removeprefix(workflow_id + "-"),
                                        "consumerActivity": item["consumerActivityId"].removeprefix(workflow_id + "-"),
                                        "logicalName": item["logicalName"], "sizeBytes": item["sizeBytes"]}
                                       for item in data_dependencies]}}
    plan_create = {"plan": plan, "workflow": workflow, "resources": resources,
                   "executionScope": scope, "networkTopology": topology}
    plan_request = {**plan_create, "activityProfiles": [], "runtimes": [runtime],
                    "runtimeBindings": bindings}
    run_request = {"run": {"id": run_id, "schedulePlanId": plan_id, "mode": "simulation",
                            "seed": total, "status": "created"}, **plan_request}
    return environment, scope, topology, definition, plan_create, run_request, run_id, makespan


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("totals", nargs="+", type=int)
    parser.add_argument("--api", default=os.getenv("AKOFLOW_API_URL", "http://localhost:8080/akoflow-api"))
    parser.add_argument("--token", default=os.getenv("AKOFLOW_API_TOKEN", ""))
    parser.add_argument("--cores", type=int, help="M2 cores; defaults to the activity total")
    parser.add_argument("--start-at", choices=("environments", "execution-scopes", "network-topologies",
                                               "workflow-definitions", "schedule-plans", "execution-runs"),
                        default="environments")
    args = parser.parse_args()
    if not args.token:
        parser.error("--token or AKOFLOW_API_TOKEN is required")
    for total in args.totals:
        if total < 3:
            parser.error("each total must be at least 3")
        cores = args.cores or total
        if cores < 1:
            parser.error("--cores must be positive")
        environment, scope, topology, definition, plan, run, run_id, makespan = build(total, cores)
        requests = (("environments", environment), ("execution-scopes", scope),
                    ("network-topologies", topology), ("workflow-definitions", definition),
                    ("schedule-plans", plan), ("execution-runs", run))
        start = next(index for index, item in enumerate(requests) if item[0] == args.start_at)
        for endpoint, payload in requests[start:]:
            print(f"{total}: {endpoint} HTTP {post(args.api, args.token, endpoint, payload)}")
        print(f"{total}: run={run_id} predictedMakespan={makespan}s")


if __name__ == "__main__":
    main()
