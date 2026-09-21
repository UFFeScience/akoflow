#!/usr/bin/env python3
"""Compute common-simulator Pareto and quality metrics."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import math
from collections import defaultdict
from pathlib import Path


def point(record: dict) -> tuple[float, float]:
    return float(record["observedMakespanSeconds"]), float(record["observedCost"])


def pareto_records(records: list[dict]) -> list[dict]:
    valid = [
        record
        for record in records
        if record.get("status") == "completed"
        and record.get("observedMakespanSeconds") is not None
        and record.get("observedCost") is not None
        and math.isfinite(point(record)[0])
        and math.isfinite(point(record)[1])
    ]
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


def analyze(records: list[dict]) -> dict:
    by_workflow: dict[str, list[dict]] = defaultdict(list)
    for record in records:
        by_workflow[record["workflowVersionId"]].append(record)
    workflow_results = []
    metric_rows = []
    for workflow_id in sorted(by_workflow):
        workflow_records = by_workflow[workflow_id]
        reference_frontier = pareto_records(workflow_records)
        maximum_time = max(point(record)[0] for record in workflow_records if record.get("status") == "completed")
        maximum_cost = max(point(record)[1] for record in workflow_records if record.get("status") == "completed")
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
            metric_rows.append(
                {
                    "workflowVersionId": workflow_id,
                    "algorithm": algorithm,
                    "evaluatedCount": len(algorithm_records),
                    "nondominatedCount": len(algorithm_frontier),
                    "bestMakespanSeconds": min(point(record)[0] for record in algorithm_records),
                    "bestCost": min(point(record)[1] for record in algorithm_records),
                    "hypervolume": algorithm_hv,
                    "hypervolumeRatio": algorithm_hv / reference_hv if reference_hv else None,
                    "multiplicativeEpsilon": multiplicative_epsilon(
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
