#!/usr/bin/env python3
"""Create the Montage-6448 interference-knowledge ablation campaign.

Execution truth is fixed at full-workflow interference.  Only the matrix shown
to PRISM changes, so observed differences measure knowledge rather than a
different workload.
"""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
from pathlib import Path

from create_interference_campaign import (
    DEFAULT_COVERAGES,
    DEFAULT_SEEDS,
    request_json,
    selected_activity_ids,
)


def matrix(
    group_id: str,
    activity_ids: list[str],
    slowdown_factor: float,
    aggregation: str = "maximum",
) -> dict:
    groups = []
    if activity_ids:
        groups.append(
            {
                "id": group_id,
                "activityIds": sorted(activity_ids),
                "slowdownFactor": slowdown_factor,
            }
        )
    return {
        "schemaVersion": "4" if aggregation == "additive-excess" else "3",
        "model": "pairwise-slowdown",
        "aggregation": aggregation,
        "entries": [],
        "groups": groups,
    }


def full_truth_matrix(workflow: dict, slowdown_factor: float) -> dict:
    activity_types = sorted(
        {
            activity.get("activityTypeId", "")
            for activity in workflow.get("activities", [])
            if activity.get("activityTypeId")
        }
    )
    if len(activity_types) == 1:
        return {
            "schemaVersion": "2",
            "model": "pairwise-slowdown",
            "aggregation": "maximum",
            "entries": [],
            "rules": [
                {
                    "affectedActivityTypeId": activity_types[0],
                    "interferingActivityTypeId": activity_types[0],
                    "slowdownFactor": slowdown_factor,
                }
            ],
        }
    return matrix(
        "execution-truth-full-workflow",
        [activity["id"] for activity in workflow.get("activities", [])],
        slowdown_factor,
    )


def build_knowledge_sessions(
    campaign_prefix: str,
    workflow: dict,
    threshold: dict,
    reference_scope_id: str,
    reference_topology_id: str,
    slowdown_factor: float,
    coverages: tuple[int, ...] = DEFAULT_COVERAGES,
    seeds: tuple[int, ...] = DEFAULT_SEEDS,
) -> list[dict]:
    workflow_id = workflow["id"]
    activity_ids = [activity["id"] for activity in workflow.get("activities", [])]
    if not activity_ids:
        raise RuntimeError(f"workflow {workflow_id} has no activities")
    truth = full_truth_matrix(workflow, slowdown_factor)
    definitions = []
    for seed in seeds:
        for coverage in coverages:
            known = selected_activity_ids(workflow_id, activity_ids, seed, coverage)
            knowledge = matrix(
                f"knowledge-s{seed:03d}-c{coverage:03d}",
                known,
                slowdown_factor,
            )
            identifier = f"{campaign_prefix}-s{seed:03d}-k{coverage:03d}-{workflow_id}"
            definitions.append(
                {
                    "id": identifier,
                    "workflowVersionId": workflow_id,
                    "executionScopeId": reference_scope_id,
                    "networkTopologyId": reference_topology_id,
                    "algorithms": [
                        {
                            "id": "prism-time",
                            "configuration": {
                                "beamWidth": 20,
                                "optionCount": 25,
                                "interferenceAware": True,
                            },
                        },
                        {
                            "id": "prism-cost",
                            "configuration": {
                                "beamWidth": 20,
                                "optionCount": 25,
                                "interferenceAware": True,
                            },
                        },
                        {"id": "heft"},
                    ],
                    "deadlineSeconds": threshold["deadlineSeconds"],
                    "budget": threshold["budget"],
                    "configuration": {
                        "campaign": campaign_prefix,
                        "experiment": "3-fixed-truth-progressive-knowledge",
                        "selectionSeed": seed,
                        "knowledgeCoveragePercent": coverage,
                        "knownActivityCount": len(known),
                        "totalActivityCount": len(activity_ids),
                        "executionTruthCoveragePercent": 100,
                        "slowdownFactor": slowdown_factor,
                        "slaFactor": threshold["factor"],
                        "referenceExecutionRunId": threshold["executionRunId"],
                        "interferenceMatrix": truth,
                        "interferenceKnowledgeMatrix": knowledge,
                    },
                }
            )
    return definitions


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--campaign-prefix", required=True)
    parser.add_argument("--thresholds", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--submit", action="store_true")
    parser.add_argument("--expected-sessions", type=int, default=30)
    parser.add_argument("--workflow", default="montage-6448-v1")
    parser.add_argument("--sla-factor", type=float, default=1.2)
    parser.add_argument("--slowdown-factor", type=float, default=1.5)
    parser.add_argument("--seeds", default="1,2,3,4,5")
    parser.add_argument("--coverages", default="0,10,20,50,80,100")
    parser.add_argument("--reference-scope", default="scheduler-hybrid_hetero-scope-v1")
    parser.add_argument("--reference-topology", default="scheduler-hybrid_hetero-network-v1")
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/akoflow-api")
    args = parser.parse_args()

    token = os.environ.get("AKOFLOW_API_TOKEN")
    if not token:
        raise SystemExit("AKOFLOW_API_TOKEN is required")
    thresholds = json.loads(args.thresholds.read_text(encoding="utf-8"))
    matches = [
        item
        for item in thresholds.get("thresholds", [])
        if item["workflowVersionId"] == args.workflow
        and abs(float(item["factor"]) - args.sla_factor) <= 1e-12
    ]
    if len(matches) != 1:
        raise RuntimeError(
            f"expected one threshold for {args.workflow} at factor {args.sla_factor}, found {len(matches)}"
        )
    definitions = request_json(args.base_url, token, "GET", "/workflow-definitions/")
    workflows = [
        definition["version"]
        for definition in definitions
        if definition.get("version", {}).get("id") == args.workflow
    ]
    if len(workflows) != 1:
        raise RuntimeError(f"expected one workflow definition for {args.workflow}")
    seeds = tuple(int(item) for item in args.seeds.split(",") if item)
    coverages = tuple(int(item) for item in args.coverages.split(",") if item)
    sessions = build_knowledge_sessions(
        args.campaign_prefix,
        workflows[0],
        matches[0],
        args.reference_scope,
        args.reference_topology,
        args.slowdown_factor,
        coverages,
        seeds,
    )
    if len(sessions) != args.expected_sessions:
        raise RuntimeError(f"expected {args.expected_sessions} sessions, generated {len(sessions)}")

    existing = {}
    if args.submit:
        existing = {
            session["id"]: session
            for session in request_json(args.base_url, token, "GET", "/planning-sessions/")
        }
    actions = []
    for session in sessions:
        if not args.submit:
            action = "planned"
        elif session["id"] in existing:
            action = "existing"
        else:
            request_json(args.base_url, token, "POST", "/planning-sessions/", session)
            action = "submitted"
        actions.append({"sessionId": session["id"], "action": action})

    manifest = {
        "schemaVersion": "3",
        "campaignPrefix": args.campaign_prefix,
        "mode": "submit" if args.submit else "dry-run",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "workflowVersionId": args.workflow,
        "experiment": "fixed execution truth, progressive PRISM knowledge",
        "executionTruthCoveragePercent": 100,
        "slowdownFactor": args.slowdown_factor,
        "knowledgeCoverages": list(coverages),
        "seeds": list(seeds),
        "sessionCount": len(sessions),
        "expectedSimulationCount": sum(len(item["algorithms"]) for item in sessions),
        "sessions": sessions,
        "actions": actions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"mode": manifest["mode"], "sessions": len(sessions), "simulations": manifest["expectedSimulationCount"], "output": str(args.output)}))


if __name__ == "__main__":
    main()
