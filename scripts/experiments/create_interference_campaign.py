#!/usr/bin/env python3
"""Create the seeded activity-coverage interference experiment."""

from __future__ import annotations

import argparse
import datetime as dt
import hashlib
import json
import math
import os
import random
import urllib.error
import urllib.request
from pathlib import Path


DEFAULT_COVERAGES = (0, 10, 20, 50, 80, 100)
DEFAULT_SEEDS = (1, 2, 3, 4, 5)


def request_json(base_url: str, token: str, method: str, path: str, payload=None):
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=data,
        method=method,
        headers={"Authorization": "Bearer " + token, "Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=300) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{method} {path} returned {error.code}: {body}") from error


def activity_permutation(workflow_id: str, activity_ids: list[str], seed: int) -> list[str]:
    """Return one stable permutation; coverage levels use nested prefixes."""
    digest = hashlib.sha256(f"{workflow_id}\0{seed}".encode()).digest()
    generator = random.Random(int.from_bytes(digest, "big"))
    ordered = sorted(activity_ids)
    generator.shuffle(ordered)
    return ordered


def selected_activity_ids(
    workflow_id: str,
    activity_ids: list[str],
    seed: int,
    coverage_percent: int,
) -> list[str]:
    if coverage_percent < 0 or coverage_percent > 100:
        raise RuntimeError("coverage percent must be between zero and 100")
    count = math.ceil(len(activity_ids) * coverage_percent / 100)
    return sorted(activity_permutation(workflow_id, activity_ids, seed)[:count])


def build_interference_sessions(
    campaign_prefix: str,
    workflow: dict,
    threshold: dict,
    reference_scope_id: str,
    reference_topology_id: str,
    slowdown_factor: float,
    coverages: tuple[int, ...] = DEFAULT_COVERAGES,
    seeds: tuple[int, ...] = DEFAULT_SEEDS,
) -> list[dict]:
    if slowdown_factor < 1:
        raise RuntimeError("slowdown factor must be at least one")
    workflow_id = workflow["id"]
    activity_ids = [activity["id"] for activity in workflow.get("activities", [])]
    if not activity_ids:
        raise RuntimeError(f"workflow {workflow_id} has no activities")
    if len(activity_ids) != len(set(activity_ids)):
        raise RuntimeError(f"workflow {workflow_id} has duplicate activity ids")

    definitions = []
    for seed in seeds:
        for coverage in coverages:
            selected = selected_activity_ids(workflow_id, activity_ids, seed, coverage)
            groups = []
            if selected:
                groups.append(
                    {
                        "id": f"seed-{seed}-coverage-{coverage}",
                        "activityIds": selected,
                        "slowdownFactor": slowdown_factor,
                    }
                )
            matrix = {
                "schemaVersion": "3",
                "model": "pairwise-slowdown",
                "aggregation": "maximum",
                "entries": [],
                "groups": groups,
            }
            identifier = f"{campaign_prefix}-s{seed:03d}-c{coverage:03d}-{workflow_id}"
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
                        "experiment": "2-seeded-activity-coverage-slowdown",
                        "selectionSeed": seed,
                        "coveragePercent": coverage,
                        "selectedActivityCount": len(selected),
                        "totalActivityCount": len(activity_ids),
                        "slowdownFactor": slowdown_factor,
                        "slaFactor": threshold["factor"],
                        "referenceExecutionRunId": threshold["executionRunId"],
                        "interferenceMatrix": matrix,
                    },
                }
            )
    identifiers = [definition["id"] for definition in definitions]
    if len(identifiers) != len(set(identifiers)):
        raise RuntimeError("generated duplicate interference session identifiers")
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
    matching_thresholds = [
        item
        for item in thresholds.get("thresholds", [])
        if item["workflowVersionId"] == args.workflow
        and abs(float(item["factor"]) - args.sla_factor) <= 1e-12
    ]
    if len(matching_thresholds) != 1:
        raise RuntimeError(
            f"expected one threshold for {args.workflow} at factor {args.sla_factor}, "
            f"found {len(matching_thresholds)}"
        )
    definitions = request_json(args.base_url, token, "GET", "/workflow-definitions/")
    matching_workflows = [
        definition["version"]
        for definition in definitions
        if definition.get("version", {}).get("id") == args.workflow
    ]
    if len(matching_workflows) != 1:
        raise RuntimeError(f"expected one workflow definition for {args.workflow}")
    seeds = tuple(int(item) for item in args.seeds.split(",") if item)
    coverages = tuple(int(item) for item in args.coverages.split(",") if item)
    sessions = build_interference_sessions(
        args.campaign_prefix,
        matching_workflows[0],
        matching_thresholds[0],
        args.reference_scope,
        args.reference_topology,
        args.slowdown_factor,
        coverages,
        seeds,
    )
    if len(sessions) != args.expected_sessions:
        raise RuntimeError(f"expected {args.expected_sessions} sessions, generated {len(sessions)}")

    actions = []
    if args.submit:
        existing = {
            session["id"]: session
            for session in request_json(args.base_url, token, "GET", "/planning-sessions/")
        }
        for session in sessions:
            if session["id"] in existing:
                actions.append({"sessionId": session["id"], "action": "existing"})
                continue
            created = request_json(args.base_url, token, "POST", "/planning-sessions/", session)
            actions.append({"sessionId": created["id"], "action": "submitted"})
    else:
        actions = [{"sessionId": session["id"], "action": "planned"} for session in sessions]

    manifest = {
        "schemaVersion": "2",
        "campaignPrefix": args.campaign_prefix,
        "mode": "submit" if args.submit else "dry-run",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "sourceThresholds": str(args.thresholds),
        "workflowVersionId": args.workflow,
        "slowdownFactor": args.slowdown_factor,
        "coverages": list(coverages),
        "seeds": list(seeds),
        "sessionCount": len(sessions),
        "expectedSimulationCount": sum(len(session["algorithms"]) for session in sessions),
        "sessions": sessions,
        "actions": actions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
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
