#!/usr/bin/env python3
"""Join planning and simulation evidence and compute campaign-level metrics."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import math
import statistics
from collections import Counter, defaultdict
from pathlib import Path


def elapsed_seconds(start: str | None, finish: str | None) -> float | None:
    if not start or not finish:
        return None
    return (
        dt.datetime.fromisoformat(finish.replace("Z", "+00:00"))
        - dt.datetime.fromisoformat(start.replace("Z", "+00:00"))
    ).total_seconds()


def numeric_summary(values: list[float | None]) -> dict:
    finite = [float(value) for value in values if value is not None and math.isfinite(float(value))]
    if not finite:
        return {"count": 0, "mean": None, "median": None, "minimum": None, "maximum": None, "stdev": None}
    return {
        "count": len(finite),
        "mean": statistics.fmean(finite),
        "median": statistics.median(finite),
        "minimum": min(finite),
        "maximum": max(finite),
        "stdev": statistics.stdev(finite) if len(finite) > 1 else 0.0,
    }


def first_defined(*values):
    return next((value for value in values if value is not None), None)


def analyze_campaign(planning: dict, simulations: dict) -> dict:
    sessions = {}
    algorithm_runs = {}
    for record in planning.get("records", []):
        detail = record.get("session") or {}
        session = detail.get("session", detail)
        sessions[session["id"]] = session
        for run in detail.get("algorithmRuns") or []:
            algorithm_runs[(session["id"], run["algorithm"])] = run

    rows = []
    for simulation in simulations.get("records", []):
        session = sessions.get(simulation["sessionId"], {})
        algorithm_run = algorithm_runs.get(
            (simulation["sessionId"], simulation["algorithm"]), {}
        )
        deadline = float(session.get("deadlineSeconds") or 0)
        budget = float(session.get("budget") or 0)
        observed_time = simulation.get("observedMakespanSeconds")
        observed_cost = simulation.get("observedCost")
        experiment = session.get("configuration") or {}
        rows.append(
            {
                **simulation,
                "experiment": first_defined(
                    simulation.get("experiment"), experiment.get("experiment")
                ),
                "selectionSeed": first_defined(
                    simulation.get("selectionSeed"), experiment.get("selectionSeed")
                ),
                "coveragePercent": first_defined(
                    simulation.get("coveragePercent"), experiment.get("coveragePercent")
                ),
                "slowdownFactor": first_defined(
                    simulation.get("slowdownFactor"), experiment.get("slowdownFactor")
                ),
                "slaFactor": experiment.get("slaFactor"),
                "deadlineSeconds": deadline,
                "budget": budget,
                "deadlineSatisfied": deadline <= 0
                or (observed_time is not None and float(observed_time) <= deadline),
                "budgetSatisfied": budget <= 0
                or (observed_cost is not None and float(observed_cost) <= budget),
                "slaSatisfied": (
                    deadline <= 0
                    or (observed_time is not None and float(observed_time) <= deadline)
                )
                and (
                    budget <= 0
                    or (observed_cost is not None and float(observed_cost) <= budget)
                ),
                "planningStatus": algorithm_run.get("status"),
                "planningElapsedSeconds": elapsed_seconds(
                    algorithm_run.get("startedAt"), algorithm_run.get("completedAt")
                ),
                "planningCandidateCount": algorithm_run.get("candidateCount"),
                "beamWidth": (algorithm_run.get("configuration") or {}).get("beamWidth"),
                "optionCount": (algorithm_run.get("configuration") or {}).get("optionCount"),
            }
        )

    interference_controls = {
        (
            row.get("workflowVersionId"),
            row.get("executionScopeId"),
            row.get("algorithm"),
            row.get("selectionSeed"),
        ): row
        for row in rows
        if row.get("status") == "completed" and row.get("coveragePercent") == 0
    }
    for row in rows:
        control = interference_controls.get(
            (
                row.get("workflowVersionId"),
                row.get("executionScopeId"),
                row.get("algorithm"),
                row.get("selectionSeed"),
            )
        )
        observed_time = row.get("observedMakespanSeconds")
        control_time = control.get("observedMakespanSeconds") if control else None
        observed_cost = row.get("observedCost")
        control_cost = control.get("observedCost") if control else None
        row["makespanDegradationPercent"] = (
            (float(observed_time) / float(control_time) - 1) * 100
            if observed_time is not None and control_time not in (None, 0)
            else None
        )
        row["costDegradationPercent"] = (
            (float(observed_cost) / float(control_cost) - 1) * 100
            if observed_cost is not None and control_cost not in (None, 0)
            else None
        )
        row["observedInterferenceSeconds"] = (
            row.get("breakdown") or {}
        ).get("interferenceSeconds")

    by_algorithm: dict[str, list[dict]] = defaultdict(list)
    by_instance: dict[str, list[dict]] = defaultdict(list)
    for row in rows:
        by_algorithm[row["algorithm"]].append(row)
        by_instance[row["sessionId"]].append(row)

    winner_counts = {
        "makespan": Counter(),
        "cost": Counter(),
        "sla": Counter(),
    }
    instance_comparisons = []
    for session_id, instance_rows in sorted(by_instance.items()):
        completed = [row for row in instance_rows if row.get("status") == "completed"]
        if not completed:
            continue
        workflow_id = instance_rows[0]["workflowVersionId"]
        scope_id = instance_rows[0]["executionScopeId"]
        best_time = min(float(row["observedMakespanSeconds"]) for row in completed)
        best_cost = min(float(row["observedCost"]) for row in completed)
        time_winners = sorted(
            row["algorithm"]
            for row in completed
            if abs(float(row["observedMakespanSeconds"]) - best_time) <= 1e-9
        )
        cost_winners = sorted(
            row["algorithm"]
            for row in completed
            if abs(float(row["observedCost"]) - best_cost) <= 1e-9
        )
        for algorithm in time_winners:
            winner_counts["makespan"][algorithm] += 1 / len(time_winners)
        for algorithm in cost_winners:
            winner_counts["cost"][algorithm] += 1 / len(cost_winners)
        for row in completed:
            if row["slaSatisfied"]:
                winner_counts["sla"][row["algorithm"]] += 1
        instance_comparisons.append(
            {
                "sessionId": session_id,
                "workflowVersionId": workflow_id,
                "executionScopeId": scope_id,
                "slaFactor": instance_rows[0].get("slaFactor"),
                "bestMakespanSeconds": best_time,
                "bestCost": best_cost,
                "makespanWinners": time_winners,
                "costWinners": cost_winners,
                "runs": [
                    {
                        "algorithm": row["algorithm"],
                        "observedMakespanSeconds": row["observedMakespanSeconds"],
                        "observedCost": row["observedCost"],
                        "makespanRatioToBest": float(row["observedMakespanSeconds"])
                        / max(best_time, 1e-12),
                        "costRatioToBest": float(row["observedCost"])
                        / max(best_cost, 1e-12)
                        if best_cost > 0
                        else (1.0 if float(row["observedCost"]) == 0 else None),
                        "slaSatisfied": row["slaSatisfied"],
                        "candidateFeasible": row.get("candidateFeasible"),
                        "selectionPolicy": row.get("selectionPolicy"),
                    }
                    for row in sorted(completed, key=lambda item: item["algorithm"])
                ],
            }
        )

    algorithm_summaries = []
    for algorithm, algorithm_rows in sorted(by_algorithm.items()):
        completed = [row for row in algorithm_rows if row.get("status") == "completed"]
        algorithm_summaries.append(
            {
                "algorithm": algorithm,
                "runCount": len(algorithm_rows),
                "completedCount": len(completed),
                "failedCount": len(algorithm_rows) - len(completed),
                "predictedFeasibleCount": sum(
                    row.get("candidateFeasible") is True for row in algorithm_rows
                ),
                "predictedInfeasibleCount": sum(
                    row.get("candidateFeasible") is False for row in algorithm_rows
                ),
                "slaSatisfiedCount": sum(bool(row["slaSatisfied"]) for row in completed),
                "planningElapsedSeconds": numeric_summary(
                    [row["planningElapsedSeconds"] for row in algorithm_rows]
                ),
                "observedMakespanSeconds": numeric_summary(
                    [row.get("observedMakespanSeconds") for row in completed]
                ),
                "observedCost": numeric_summary(
                    [row.get("observedCost") for row in completed]
                ),
                "predictionErrorMakespan": numeric_summary(
                    [row.get("predictionErrorMakespan") for row in completed]
                ),
                "predictionErrorCost": numeric_summary(
                    [row.get("predictionErrorCost") for row in completed]
                ),
                "makespanWinnerCredits": winner_counts["makespan"][algorithm],
                "costWinnerCredits": winner_counts["cost"][algorithm],
            }
        )

    interference_groups: dict[tuple[str, float], list[dict]] = defaultdict(list)
    for row in rows:
        coverage = row.get("coveragePercent")
        if coverage is not None:
            interference_groups[(row["algorithm"], float(coverage))].append(row)
    interference_summaries = []
    for (algorithm, coverage), group_rows in sorted(interference_groups.items()):
        completed = [row for row in group_rows if row.get("status") == "completed"]
        interference_summaries.append(
            {
                "algorithm": algorithm,
                "coveragePercent": coverage,
                "runCount": len(group_rows),
                "completedCount": len(completed),
                "failedCount": len(group_rows) - len(completed),
                "seedCount": len(
                    {
                        row.get("selectionSeed")
                        for row in completed
                        if row.get("selectionSeed") is not None
                    }
                ),
                "slaSatisfiedCount": sum(bool(row["slaSatisfied"]) for row in completed),
                "slaSatisfiedRate": (
                    sum(bool(row["slaSatisfied"]) for row in completed) / len(completed)
                    if completed
                    else None
                ),
                "planningElapsedSeconds": numeric_summary(
                    [row.get("planningElapsedSeconds") for row in completed]
                ),
                "observedMakespanSeconds": numeric_summary(
                    [row.get("observedMakespanSeconds") for row in completed]
                ),
                "observedCost": numeric_summary(
                    [row.get("observedCost") for row in completed]
                ),
                "observedInterferenceSeconds": numeric_summary(
                    [row.get("observedInterferenceSeconds") for row in completed]
                ),
                "makespanDegradationPercent": numeric_summary(
                    [row.get("makespanDegradationPercent") for row in completed]
                ),
                "costDegradationPercent": numeric_summary(
                    [row.get("costDegradationPercent") for row in completed]
                ),
            }
        )
    return {
        "runRows": rows,
        "algorithmSummaries": algorithm_summaries,
        "instanceComparisons": instance_comparisons,
        "interferenceSummaries": interference_summaries,
        "winnerCounts": {name: dict(counts) for name, counts in winner_counts.items()},
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--planning", required=True, type=Path)
    parser.add_argument("--simulations", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    planning = json.loads(args.planning.read_text(encoding="utf-8"))
    simulations = json.loads(args.simulations.read_text(encoding="utf-8"))
    result = {
        "schemaVersion": "1",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "sourcePlanning": str(args.planning),
        "sourceSimulations": str(args.simulations),
        **analyze_campaign(planning, simulations),
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8")
    print(
        json.dumps(
            {
                "runs": len(result["runRows"]),
                "algorithms": len(result["algorithmSummaries"]),
                "instances": len(result["instanceComparisons"]),
            }
        )
    )


if __name__ == "__main__":
    main()
