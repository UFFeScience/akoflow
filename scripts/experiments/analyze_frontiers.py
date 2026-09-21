#!/usr/bin/env python3
"""Compute common-simulator Pareto and quality metrics."""

from __future__ import annotations

import argparse
import datetime as dt
import gzip
import json
import math
import statistics
from collections import defaultdict
from pathlib import Path


def point(record: dict) -> tuple[float, float]:
    return float(record["observedMakespanSeconds"]), float(record["observedCost"])


def valid_records(records: list[dict]) -> list[dict]:
    return [
        record
        for record in records
        if record.get("status") == "completed"
        and record.get("observedMakespanSeconds") is not None
        and record.get("observedCost") is not None
        and math.isfinite(point(record)[0])
        and math.isfinite(point(record)[1])
    ]


def pareto_records(records: list[dict]) -> list[dict]:
    valid = valid_records(records)
    frontier = []
    for record in valid:
        time, cost = point(record)
        dominated = any(
            other is not record
            and point(other)[0] <= time
            and point(other)[1] <= cost
            and point(other) != (time, cost)
            for other in valid
        )
        if not dominated:
            frontier.append(record)
    return sorted(frontier, key=lambda item: (*point(item), item["executionRunId"]))


def hypervolume(records: list[dict], reference: tuple[float, float]) -> float:
    frontier = pareto_records(records)
    previous_cost = reference[1]
    volume = 0.0
    for record in frontier:
        time, cost = point(record)
        if time <= reference[0] and cost < previous_cost:
            volume += (reference[0] - time) * (previous_cost - cost)
            previous_cost = cost
    return volume


def multiplicative_epsilon(approximation: list[dict], reference: list[dict]) -> float | None:
    if not approximation or not reference:
        return None
    values = []
    for reference_record in reference:
        reference_time, reference_cost = point(reference_record)
        values.append(
            min(
                max(
                    point(candidate)[0] / max(reference_time, 1e-12),
                    point(candidate)[1] / max(reference_cost, 1e-12),
                )
                for candidate in approximation
            )
        )
    return max(values)


def normalized_frontier_metrics(
    approximation: list[dict], reference: list[dict]
) -> dict:
    if not approximation or not reference:
        return {
            "meanDistanceToReference": None,
            "maximumDistanceToReference": None,
            "frontierSpan": None,
            "frontierSpacing": None,
        }
    reference_points = [point(record) for record in reference]
    time_values = [value[0] for value in reference_points]
    cost_values = [value[1] for value in reference_points]
    time_scale = max(max(time_values) - min(time_values), max(time_values), 1e-12)
    cost_scale = max(max(cost_values) - min(cost_values), max(cost_values), 1e-12)

    def distance(left: tuple[float, float], right: tuple[float, float]) -> float:
        return math.hypot(
            (left[0] - right[0]) / time_scale,
            (left[1] - right[1]) / cost_scale,
        )

    approximation_points = [point(record) for record in approximation]
    distances = [
        min(distance(candidate, reference_point) for reference_point in reference_points)
        for candidate in approximation_points
    ]
    ordered = sorted(approximation_points)
    consecutive = [
        distance(ordered[index - 1], ordered[index])
        for index in range(1, len(ordered))
    ]
    return {
        "meanDistanceToReference": statistics.fmean(distances),
        "maximumDistanceToReference": max(distances),
        "frontierSpan": distance(ordered[0], ordered[-1]),
        "frontierSpacing": (
            statistics.stdev(consecutive) if len(consecutive) > 1 else 0.0
        ),
    }


def analyze(records: list[dict]) -> dict:
    by_workflow: dict[str, list[dict]] = defaultdict(list)
    for record in records:
        by_workflow[record["workflowVersionId"]].append(record)
    workflow_results = []
    metric_rows = []
    for workflow_id in sorted(by_workflow):
        workflow_records = valid_records(by_workflow[workflow_id])
        if not workflow_records:
            continue
        reference_frontier = pareto_records(workflow_records)
        maximum_time = max(point(record)[0] for record in workflow_records)
        maximum_cost = max(point(record)[1] for record in workflow_records)
        best_reference_time = min(point(record)[0] for record in reference_frontier)
        best_reference_cost = min(point(record)[1] for record in reference_frontier)
        reference_point = (maximum_time * 1.05, max(maximum_cost * 1.05, 1e-9))
        reference_hv = hypervolume(reference_frontier, reference_point)
        workflow_results.append(
            {
                "workflowVersionId": workflow_id,
                "referencePoint": {"makespanSeconds": reference_point[0], "cost": reference_point[1]},
                "referenceHypervolume": reference_hv,
                "frontier": reference_frontier,
            }
        )
        algorithms = sorted({record["algorithm"] for record in workflow_records})
        for algorithm in algorithms:
            algorithm_records = [
                record for record in workflow_records if record["algorithm"] == algorithm
            ]
            algorithm_frontier = pareto_records(algorithm_records)
            algorithm_hv = hypervolume(algorithm_frontier, reference_point)
            best_time = min(point(record)[0] for record in algorithm_records)
            best_cost = min(point(record)[1] for record in algorithm_records)
            metric_rows.append(
                {
                    "workflowVersionId": workflow_id,
                    "algorithm": algorithm,
                    "evaluatedCount": len(algorithm_records),
                    "nondominatedCount": len(algorithm_frontier),
                    "bestMakespanSeconds": best_time,
                    "bestCost": best_cost,
                    "bestMakespanGapPercent": (
                        best_time / max(best_reference_time, 1e-12) - 1
                    )
                    * 100,
                    "bestCostGapPercent": (
                        (best_cost / best_reference_cost - 1) * 100
                        if best_reference_cost > 0
                        else (0.0 if best_cost == 0 else None)
                    ),
                    "hypervolume": algorithm_hv,
                    "hypervolumeRatio": algorithm_hv / reference_hv if reference_hv else None,
                    "multiplicativeEpsilon": multiplicative_epsilon(
                        algorithm_frontier, reference_frontier
                    ),
                    **normalized_frontier_metrics(
                        algorithm_frontier, reference_frontier
                    ),
                }
            )
    return {"workflows": workflow_results, "metrics": metric_rows}


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--results", required=True, type=Path, nargs="+")
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    records = []
    for path in args.results:
        if path.suffix == ".gz":
            with gzip.open(path, "rt", encoding="utf-8") as source:
                payload = json.load(source)
        else:
            payload = json.loads(path.read_text(encoding="utf-8"))
        records.extend(payload.get("records", []))
    result = {
        "schemaVersion": "1",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "sourceResults": [str(path) for path in args.results],
        **analyze(records),
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"workflows": len(result["workflows"]), "metricRows": len(result["metrics"])}))


if __name__ == "__main__":
    main()
