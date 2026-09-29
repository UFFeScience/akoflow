#!/usr/bin/env python3
"""Create the controlled blind-versus-aware interference ablation campaign."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
from pathlib import Path

from create_interference_campaign import request_json, selected_activity_ids
from create_interference_knowledge_campaign import matrix


SCENARIOS = (
    ("control", False, False),
    ("blind", True, False),
    ("aware", True, True),
)
ALGORITHM_IDS = ("prism-time", "prism-cost", "heft")


def empty_matrix(aggregation: str = "maximum") -> dict:
    return {
        "schemaVersion": "4" if aggregation == "additive-excess" else "3",
        "model": "pairwise-slowdown",
        "aggregation": aggregation,
        "entries": [],
        "groups": [],
    }


def interference_activity_ids(workflow: dict, activity_family: str | None) -> list[str]:
    activities = workflow.get("activities", [])
    if not activity_family:
        return [activity["id"] for activity in activities]
    prefix = activity_family.casefold()
    selected = [
        activity["id"]
        for activity in activities
        if str(activity.get("externalId") or activity.get("name") or "")
        .casefold()
        .startswith(prefix)
    ]
    if not selected:
        raise RuntimeError(
            f"workflow {workflow['id']} has no activities in family {activity_family!r}"
        )
    return selected


def build_ablation_sessions(
    campaign_prefix: str,
    workflow: dict,
    threshold: dict,
    scope_id: str,
    topology_id: str,
    slowdown_factor: float,
    seeds: tuple[int, ...] = (1, 2, 3, 4, 5),
    coverage_percent: int = 100,
    scenario_names: tuple[str, ...] = ("control", "blind", "aware"),
    beam_width: int = 20,
    activity_family: str | None = None,
    algorithm_ids: tuple[str, ...] = ALGORITHM_IDS,
    aggregation: str = "maximum",
) -> list[dict]:
    if slowdown_factor < 1:
        raise RuntimeError("slowdown factor must be at least one")
    if not workflow.get("activities"):
        raise RuntimeError(f"workflow {workflow['id']} has no activities")
    if aggregation not in ("maximum", "additive-excess"):
        raise RuntimeError(f"unsupported slowdown aggregation: {aggregation}")
    none = empty_matrix(aggregation)
    activity_ids = interference_activity_ids(workflow, activity_family)
    scenarios = [item for item in SCENARIOS if item[0] in scenario_names]
    if len(scenarios) != len(scenario_names):
        raise RuntimeError(f"unsupported scenarios: {scenario_names}")
    sessions = []
    for seed in seeds:
        selected = selected_activity_ids(
            workflow["id"], activity_ids, seed, coverage_percent
        )
        truth = matrix(
            f"execution-truth-s{seed:03d}-c{coverage_percent:03d}",
            selected,
            slowdown_factor,
            aggregation,
        )
        for scenario, execution_has_interference, planner_is_aware in scenarios:
            execution_matrix = truth if execution_has_interference else none
            knowledge_matrix = truth if planner_is_aware else none
            sessions.append(
                {
                    "id": (
                        f"{campaign_prefix}-s{seed:03d}-{scenario}-"
                        f"{workflow['id']}"
                    ),
                    "workflowVersionId": workflow["id"],
                    "executionScopeId": scope_id,
                    "networkTopologyId": topology_id,
                    "algorithms": [
                        {
                            "id": algorithm,
                            **(
                                {
                                    "configuration": {
                                        "beamWidth": beam_width,
                                        "optionCount": 25,
                                        "interferenceAware": planner_is_aware,
                                    }
                                }
                                if algorithm.startswith("prism-")
                                else {}
                            ),
                        }
                        for algorithm in algorithm_ids
                    ],
                    "deadlineSeconds": threshold["deadlineSeconds"],
                    "budget": threshold["budget"],
                    "configuration": {
                        "campaign": campaign_prefix,
                        "experiment": "interference-blind-aware-ablation",
                        "scenario": scenario,
                        "selectionSeed": seed,
                        "executionHasInterference": execution_has_interference,
                        "plannerHasInterferenceKnowledge": planner_is_aware,
                        "executionTruthCoveragePercent": (
                            coverage_percent if execution_has_interference else 0
                        ),
                        "knowledgeCoveragePercent": (
                            coverage_percent if planner_is_aware else 0
                        ),
                        "slowdownFactor": slowdown_factor,
                        "interferenceAggregation": aggregation,
                        "interferenceActivityFamily": activity_family or "all",
                        "interferenceEligibleActivityCount": len(activity_ids),
                        "interferenceSelectedActivityCount": len(selected),
                        "slaFactor": threshold["factor"],
                        "referenceExecutionRunId": threshold["executionRunId"],
                        "interferenceMatrix": execution_matrix,
                        "interferenceKnowledgeMatrix": knowledge_matrix,
                    },
                }
            )
    return sessions


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--campaign-prefix", required=True)
    parser.add_argument("--thresholds", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--submit", action="store_true")
    parser.add_argument("--expected-sessions", type=int, default=15)
    parser.add_argument("--workflow", default="montage-6448-v1")
    parser.add_argument("--sla-factor", type=float, default=1.2)
    parser.add_argument("--slowdown-factor", type=float, default=1.5)
    parser.add_argument("--seeds", default="1,2,3,4,5")
    parser.add_argument("--coverage-percent", type=int, default=100)
    parser.add_argument("--scenarios", default="control,blind,aware")
    parser.add_argument("--beam-width", type=int, default=20)
    parser.add_argument(
        "--activity-family",
        help="Restrict the interference matrix to activities whose externalId or name starts with this value.",
    )
    parser.add_argument("--algorithms", default=",".join(ALGORITHM_IDS))
    parser.add_argument(
        "--aggregation", choices=("maximum", "additive-excess"), default="maximum"
    )
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
            f"expected one threshold for {args.workflow} at factor "
            f"{args.sla_factor}, found {len(matches)}"
        )
    definitions = request_json(args.base_url, token, "GET", "/workflow-definitions/")
    workflows = [
        definition["version"]
        for definition in definitions
        if definition.get("version", {}).get("id") == args.workflow
    ]
    if len(workflows) != 1:
        raise RuntimeError(f"expected one workflow definition for {args.workflow}")
    seeds = tuple(int(value) for value in args.seeds.split(",") if value)
    scenario_names = tuple(value for value in args.scenarios.split(",") if value)
    algorithm_ids = tuple(value for value in args.algorithms.split(",") if value)
    unsupported_algorithms = set(algorithm_ids) - set(ALGORITHM_IDS)
    if unsupported_algorithms:
        raise RuntimeError(f"unsupported algorithms: {sorted(unsupported_algorithms)}")
    sessions = build_ablation_sessions(
        args.campaign_prefix,
        workflows[0],
        matches[0],
        args.reference_scope,
        args.reference_topology,
        args.slowdown_factor,
        seeds,
        args.coverage_percent,
        scenario_names,
        args.beam_width,
        args.activity_family,
        algorithm_ids,
        args.aggregation,
    )
    if len(sessions) != args.expected_sessions:
        raise RuntimeError(
            f"expected {args.expected_sessions} sessions, generated {len(sessions)}"
        )

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

    scope = request_json(
        args.base_url,
        token,
        "GET",
        f"/execution-scopes/{args.reference_scope}/",
    )
    environments = request_json(args.base_url, token, "GET", "/environments/")
    by_version = {item["version"]["id"]: item for item in environments}
    resources = [
        {"id": resource["id"], "cpuCores": resource.get("cpuCores", 1)}
        for version_id in scope["environmentVersionIds"]
        for resource in by_version[version_id].get("resources", [])
    ]
    manifest = {
        "schemaVersion": "1",
        "campaignPrefix": args.campaign_prefix,
        "mode": "submit" if args.submit else "dry-run",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "workflowVersionId": args.workflow,
        "experiment": "controlled blind-versus-aware interference ablation",
        "scenarios": list(scenario_names),
        "algorithms": list(algorithm_ids),
        "seeds": list(seeds),
        "coveragePercent": args.coverage_percent,
        "slowdownFactor": args.slowdown_factor,
        "interferenceAggregation": args.aggregation,
        "interferenceActivityFamily": args.activity_family or "all",
        "interferenceEligibleActivityCount": len(
            interference_activity_ids(workflows[0], args.activity_family)
        ),
        "sessionCount": len(sessions),
        "expectedSimulationCount": sum(len(item["algorithms"]) for item in sessions),
        "resourceCapacities": resources,
        "workflowModel": {
            "activities": [
                {
                    "id": activity["id"],
                    "activityTypeId": activity.get("activityTypeId", ""),
                    "cpu": (activity.get("resources") or {}).get("cpu", 1),
                }
                for activity in workflows[0].get("activities", [])
            ],
            "dependencies": [
                {
                    "activityId": dependency["activityId"],
                    "dependsOnActivityId": dependency["dependsOnActivityId"],
                }
                for dependency in workflows[0].get("dependencies", [])
            ],
        },
        "sessions": sessions,
        "actions": actions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(manifest, ensure_ascii=False, indent=2))
    print(
        json.dumps(
            {
                "mode": manifest["mode"],
                "sessions": len(sessions),
                "simulations": manifest["expectedSimulationCount"],
                "output": str(args.output),
            }
        )
    )


if __name__ == "__main__":
    main()
