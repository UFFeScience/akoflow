#!/usr/bin/env python3
"""Analyze the causal blind-versus-aware interference experiment."""

from __future__ import annotations

import argparse
import json
import math
from collections import defaultdict
from pathlib import Path

from analyze_campaign_results import analyze_campaign, numeric_summary, read_json


def overlap_metrics(
    tasks: list[dict], interfering_activity_ids: set[str] | None = None
) -> tuple[int, float, set[str]]:
    """Count pairwise overlaps on one resource with a sweep-line scan."""
    by_resource: dict[str, list[dict]] = defaultdict(list)
    for task in tasks:
        activity_id = str(task["activityId"])
        if (
            interfering_activity_ids is not None
            and activity_id not in interfering_activity_ids
        ):
            continue
        start = float(task.get("startedAt") or 0)
        finish = float(task.get("finishedAt") or 0)
        resource = task.get("allocatedResourceId") or task.get("plannedResourceId")
        if resource and finish > start:
            by_resource[resource].append(task)
    count = 0
    duration = 0.0
    affected: set[str] = set()
    for resource_tasks in by_resource.values():
        active: list[dict] = []
        for task in sorted(resource_tasks, key=lambda item: float(item["startedAt"])):
            start = float(task["startedAt"])
            finish = float(task["finishedAt"])
            active = [item for item in active if float(item["finishedAt"]) > start]
            for peer in active:
                overlap = min(finish, float(peer["finishedAt"])) - start
                if overlap <= 0:
                    continue
                count += 1
                duration += overlap
                affected.add(str(task["activityId"]))
                affected.add(str(peer["activityId"]))
            active.append(task)
    return count, duration, affected


def critical_path(
    workflow_model: dict,
    tasks: list[dict],
) -> tuple[list[str], float]:
    duration = {
        str(task["activityId"]): float(task.get("runtimeSeconds") or 0)
        for task in tasks
    }
    predecessors: dict[str, list[str]] = defaultdict(list)
    successors: dict[str, list[str]] = defaultdict(list)
    indegree = {str(activity["id"]): 0 for activity in workflow_model["activities"]}
    for dependency in workflow_model["dependencies"]:
        activity = str(dependency["activityId"])
        predecessor = str(dependency["dependsOnActivityId"])
        predecessors[activity].append(predecessor)
        successors[predecessor].append(activity)
        indegree[activity] = indegree.get(activity, 0) + 1
        indegree.setdefault(predecessor, 0)
    ready = sorted(activity for activity, degree in indegree.items() if degree == 0)
    order = []
    while ready:
        activity = ready.pop()
        order.append(activity)
        for successor in successors[activity]:
            indegree[successor] -= 1
            if indegree[successor] == 0:
                ready.append(successor)
    best: dict[str, float] = {}
    parent: dict[str, str] = {}
    for activity in order:
        predecessor = max(
            predecessors[activity],
            key=lambda item: best.get(item, 0),
            default="",
        )
        best[activity] = best.get(predecessor, 0) + duration.get(activity, 0)
        if predecessor:
            parent[activity] = predecessor
    if not best:
        return [], 0.0
    tail = max(best, key=best.get)
    path = []
    while tail:
        path.append(tail)
        tail = parent.get(tail, "")
    return list(reversed(path)), max(best.values())


def resource_metrics(
    tasks: list[dict],
    capacities: dict[str, float],
    makespan: float,
    activity_cpu: dict[str, float],
) -> list[dict]:
    busy: dict[str, float] = defaultdict(float)
    for task in tasks:
        resource = task.get("allocatedResourceId") or task.get("plannedResourceId")
        if resource:
            busy[resource] += float(task.get("runtimeSeconds") or 0) * activity_cpu.get(
                str(task["activityId"]), 1
            )
    rows = []
    for resource, cores in sorted(capacities.items()):
        available = makespan * max(cores, 1)
        utilization = busy.get(resource, 0) / available if available > 0 else 0
        rows.append(
            {
                "resourceId": resource,
                "busyCoreSeconds": busy.get(resource, 0),
                "availableCoreSeconds": available,
                "utilization": utilization,
                "idleFraction": max(0.0, 1 - utilization),
            }
        )
    return rows


def analyze_ablation(planning: dict, simulations: dict, campaign: dict) -> dict:
    base = analyze_campaign(planning, simulations)
    activities_by_run: dict[str, list[dict]] = defaultdict(list)
    for activity in simulations.get("activities", []):
        activities_by_run[str(activity["executionRunId"])].append(activity)
    workflow = campaign["workflowModel"]
    activity_cpu = {
        str(activity["id"]): float(activity.get("cpu") or 1)
        for activity in workflow["activities"]
    }
    capacities = {
        str(resource["id"]): float(resource.get("cpuCores") or 1)
        for resource in campaign["resourceCapacities"]
    }
    truth_ids_by_seed: dict[int, set[str]] = {}
    for session in campaign.get("sessions", []):
        configuration = session.get("configuration") or {}
        matrix = configuration.get("interferenceMatrix") or {}
        selected: set[str] = set()
        for group in matrix.get("groups") or []:
            selected.update(str(item) for item in group.get("activityIds") or [])
        for entry in matrix.get("entries") or []:
            selected.add(str(entry.get("activityAId") or ""))
            selected.add(str(entry.get("activityBId") or ""))
        selected.discard("")
        seed = int(configuration.get("selectionSeed") or 0)
        if selected:
            truth_ids_by_seed[seed] = selected

    enriched = []
    for row in base["runRows"]:
        tasks = activities_by_run.get(str(row["executionRunId"]), [])
        truth_ids = truth_ids_by_seed.get(int(row.get("selectionSeed") or 0), set())
        overlaps, overlap_seconds, overlapping = overlap_metrics(tasks, truth_ids)
        critical, critical_seconds = critical_path(workflow, tasks)
        interference_affected = {
            str(task["activityId"])
            for task in tasks
            if float(task.get("interferenceSeconds") or 0) > 0
        }
        observed = float(row.get("observedMakespanSeconds") or 0)
        predicted = float((row.get("predicted") or {}).get("makespanSeconds") or 0)
        row.update(
            {
                "interferingOverlapCount": overlaps
                if row.get("executionHasInterference")
                else 0,
                "interferingOverlapSeconds": overlap_seconds
                if row.get("executionHasInterference")
                else 0,
                "overlappingActivityCount": len(overlapping)
                if row.get("executionHasInterference")
                else 0,
                "criticalPathActivityCount": len(critical),
                "criticalPathRuntimeSeconds": critical_seconds,
                "criticalPathAffectedCount": len(set(critical) & interference_affected),
                "plannedObservedDeltaSeconds": observed - predicted,
                "resourceMetrics": resource_metrics(
                    tasks, capacities, observed, activity_cpu
                ),
            }
        )
        enriched.append(row)

    indexed = {
        (row["algorithm"], int(row["selectionSeed"]), row.get("scenario")): row
        for row in enriched
    }
    paired = []
    for algorithm in sorted({row["algorithm"] for row in enriched}):
        for seed in sorted({int(row["selectionSeed"]) for row in enriched}):
            control = indexed.get((algorithm, seed, "control"))
            blind = indexed.get((algorithm, seed, "blind"))
            aware = indexed.get((algorithm, seed, "aware"))
            if not all((blind, aware)):
                continue
            blind_time = float(blind["observedMakespanSeconds"])
            aware_time = float(aware["observedMakespanSeconds"])
            control_time = (
                float(control["observedMakespanSeconds"]) if control else None
            )
            paired.append(
                {
                    "algorithm": algorithm,
                    "seed": seed,
                    "controlRunId": control["executionRunId"] if control else None,
                    "blindRunId": blind["executionRunId"],
                    "awareRunId": aware["executionRunId"],
                    "controlMakespanSeconds": control_time,
                    "blindMakespanSeconds": blind_time,
                    "awareMakespanSeconds": aware_time,
                    "blindPenaltyVsAwarePercent": (
                        (blind_time - aware_time) / aware_time * 100
                        if aware_time > 0
                        else None
                    ),
                    "blindDegradationVsControlPercent": (
                        (blind_time - control_time) / control_time * 100
                        if control_time is not None and control_time > 0
                        else None
                    ),
                    "awareDegradationVsControlPercent": (
                        (aware_time - control_time) / control_time * 100
                        if control_time is not None and control_time > 0
                        else None
                    ),
                }
            )
    summaries = []
    for algorithm in sorted({row["algorithm"] for row in paired}):
        items = [row for row in paired if row["algorithm"] == algorithm]
        summaries.append(
            {
                "algorithm": algorithm,
                "seedCount": len(items),
                "blindPenaltyVsAwarePercent": numeric_summary(
                    [row["blindPenaltyVsAwarePercent"] for row in items]
                ),
                "blindDegradationVsControlPercent": numeric_summary(
                    [row["blindDegradationVsControlPercent"] for row in items]
                ),
                "awareDegradationVsControlPercent": numeric_summary(
                    [row["awareDegradationVsControlPercent"] for row in items]
                ),
            }
        )
    base["runRows"] = enriched
    base["pairedComparisons"] = paired
    base["pairedSummaries"] = summaries
    return base


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--planning", required=True, type=Path)
    parser.add_argument("--simulations", required=True, type=Path)
    parser.add_argument("--campaign", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    result = analyze_ablation(
        read_json(args.planning), read_json(args.simulations), read_json(args.campaign)
    )
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2))
    print(
        json.dumps(
            {
                "runs": len(result["runRows"]),
                "pairs": len(result["pairedComparisons"]),
                "summaries": len(result["pairedSummaries"]),
            }
        )
    )


if __name__ == "__main__":
    main()
