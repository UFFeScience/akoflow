#!/usr/bin/env python3
"""Derive workflow SLA thresholds from common-simulator HEFT references."""

from __future__ import annotations

import argparse
import datetime as dt
import json
from pathlib import Path


def derive_thresholds(results: dict, reference_scope: str, factors: list[float]) -> list[dict]:
    references = {}
    for record in results.get("records", []):
        if (
            record.get("algorithm") == "heft"
            and record.get("executionScopeId") == reference_scope
            and record.get("status") == "completed"
        ):
            workflow_id = record["workflowVersionId"]
            if workflow_id in references:
                raise RuntimeError(f"duplicate HEFT reference for {workflow_id}")
            makespan = record.get("observedMakespanSeconds")
            cost = record.get("observedCost")
            if makespan is None or float(makespan) <= 0:
                raise RuntimeError(f"invalid HEFT makespan reference for {workflow_id}")
            if cost is None or float(cost) <= 0:
                raise RuntimeError(
                    f"invalid HEFT cost reference for {workflow_id}; zero would disable budget enforcement"
                )
            references[workflow_id] = {
                "workflowVersionId": workflow_id,
                "executionRunId": record["executionRunId"],
                "referenceScopeId": reference_scope,
                "referenceMakespanSeconds": float(makespan),
                "referenceCost": float(cost),
            }

    workflows = sorted(
        {record["workflowVersionId"] for record in results.get("records", [])}
    )
    missing = [workflow for workflow in workflows if workflow not in references]
    if missing:
        raise RuntimeError(
            "missing completed hybrid HEFT references: " + ", ".join(missing)
        )

    thresholds = []
    for workflow in workflows:
        reference = references[workflow]
        for factor in factors:
            thresholds.append(
                {
                    **reference,
                    "factor": factor,
                    "deadlineSeconds": factor
                    * reference["referenceMakespanSeconds"],
                    "budget": factor * reference["referenceCost"],
                }
            )
    return thresholds


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--results", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument(
        "--reference-scope",
        default="scheduler-hybrid_hetero-scope-v1",
    )
    parser.add_argument(
        "--factors",
        default="1.1,1.2,1.5",
        help="Comma-separated sensitivity factors",
    )
    args = parser.parse_args()

    results = json.loads(args.results.read_text(encoding="utf-8"))
    factors = [float(value) for value in args.factors.split(",")]
    if not factors or any(factor <= 1 for factor in factors):
        raise RuntimeError("all SLA factors must be greater than one")

    thresholds = derive_thresholds(results, args.reference_scope, factors)
    workflows = sorted({item["workflowVersionId"] for item in thresholds})

    payload = {
        "schemaVersion": "1",
        "sourceResults": str(args.results),
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "referenceAlgorithm": "heft",
        "referenceEvaluator": "simgrid-common-simulator",
        "referenceScopeId": args.reference_scope,
        "factors": factors,
        "workflowCount": len(workflows),
        "thresholds": thresholds,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(
        json.dumps(
            {
                "workflows": len(workflows),
                "factors": factors,
                "thresholds": len(thresholds),
            }
        )
    )


if __name__ == "__main__":
    main()
