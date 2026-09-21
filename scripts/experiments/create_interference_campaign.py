#!/usr/bin/env python3
"""Create informed/uninformed PRISM slowdown experiment sessions."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import urllib.error
import urllib.request
from pathlib import Path


DEFAULT_LAMBDAS = (0.0, 0.25, 0.5, 1.0, 1.5, 2.0)


def request_json(base_url: str, token: str, method: str, path: str, payload=None):
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=data,
        method=method,
        headers={
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=300) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(
            f"{method} {path} returned {error.code}: {body}"
        ) from error


def lambda_slug(value: float) -> str:
    return f"l{round(value * 100):03d}"


def build_interference_sessions(
    campaign_prefix: str,
    workflows: dict[str, dict],
    thresholds: dict,
    reference_scope_id: str,
    reference_topology_id: str,
    sla_factor: float,
    base_slowdown: float,
    lambdas: tuple[float, ...] = DEFAULT_LAMBDAS,
) -> list[dict]:
    selected_thresholds = {
        item["workflowVersionId"]: item
        for item in thresholds.get("thresholds", [])
        if abs(float(item["factor"]) - sla_factor) <= 1e-12
    }
    if set(workflows) != set(selected_thresholds):
        missing = sorted(set(workflows) - set(selected_thresholds))
        extra = sorted(set(selected_thresholds) - set(workflows))
        raise RuntimeError(f"threshold/workflow mismatch; missing={missing}, extra={extra}")
    if base_slowdown < 1:
        raise RuntimeError("base slowdown must be at least one")

    definitions = []
    for workflow_id in sorted(workflows):
        workflow = workflows[workflow_id]
        activity_types = sorted(
            {activity["activityTypeId"] for activity in workflow.get("activities", [])}
        )
        if not activity_types:
            raise RuntimeError(f"workflow {workflow_id} has no activity types")
        threshold = selected_thresholds[workflow_id]
        for lambda_value in lambdas:
            effective = 1 + lambda_value * (base_slowdown - 1)
            rules = [
                {
                    "affectedActivityTypeId": affected,
                    "interferingActivityTypeId": interferer,
                    "slowdownFactor": effective,
                }
                for affected in activity_types
                for interferer in activity_types
            ]
            matrix = {
                "schemaVersion": "2",
                "model": "pairwise-slowdown",
                "aggregation": "maximum",
                "entries": [],
                "rules": rules,
            }
            for awareness in ("informed", "uninformed"):
                aware = awareness == "informed"
                algorithms = [
                    {
                        "id": "prism-time",
                        "configuration": {
                            "beamWidth": 20,
                            "optionCount": 25,
                            "interferenceAware": aware,
                        },
                    },
                    {
                        "id": "prism-cost",
                        "configuration": {
                            "beamWidth": 20,
                            "optionCount": 25,
                            "interferenceAware": aware,
                        },
                    },
                ]
                if not aware:
                    algorithms.append({"id": "heft"})
                identifier = (
                    f"{campaign_prefix}-{lambda_slug(lambda_value)}-"
                    f"{awareness}-{workflow_id}"
                )
                definitions.append(
                    {
                        "id": identifier,
                        "workflowVersionId": workflow_id,
                        "executionScopeId": reference_scope_id,
                        "networkTopologyId": reference_topology_id,
                        "algorithms": algorithms,
                        "deadlineSeconds": threshold["deadlineSeconds"],
                        "budget": threshold["budget"],
                        "configuration": {
                            "campaign": campaign_prefix,
                            "experiment": "2-pairwise-slowdown",
                            "knowledgeCondition": awareness,
                            "lambda": lambda_value,
                            "baseSlowdown": base_slowdown,
                            "effectiveSlowdown": effective,
                            "slaFactor": sla_factor,
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
    parser.add_argument("--expected-sessions", type=int, default=84)
    parser.add_argument("--sla-factor", type=float, default=1.2)
    parser.add_argument("--base-slowdown", type=float, default=1.5)
    parser.add_argument(
        "--reference-scope",
        default="scheduler-hybrid_hetero-scope-v1",
    )
    parser.add_argument(
        "--reference-topology",
        default="scheduler-hybrid_hetero-network-v1",
    )
    parser.add_argument(
        "--base-url",
        default="http://127.0.0.1:8080/akoflow-api",
    )
    args = parser.parse_args()

    token = os.environ.get("AKOFLOW_API_TOKEN")
    if not token:
        raise SystemExit("AKOFLOW_API_TOKEN is required")
    thresholds = json.loads(args.thresholds.read_text(encoding="utf-8"))
    definitions = request_json(
        args.base_url, token, "GET", "/workflow-definitions/"
    )
    workflows = {
        definition["version"]["id"]: definition["version"]
        for definition in definitions
        if definition.get("version", {}).get("id")
        in {item["workflowVersionId"] for item in thresholds.get("thresholds", [])}
    }
    sessions = build_interference_sessions(
        args.campaign_prefix,
        workflows,
        thresholds,
        args.reference_scope,
        args.reference_topology,
        args.sla_factor,
        args.base_slowdown,
    )
    if len(sessions) != args.expected_sessions:
        raise RuntimeError(
            f"expected {args.expected_sessions} sessions, generated {len(sessions)}"
        )

    actions = []
    if args.submit:
        existing = {
            session["id"]: session
            for session in request_json(
                args.base_url, token, "GET", "/planning-sessions/"
            )
        }
        for session in sessions:
            if session["id"] in existing:
                actions.append({"sessionId": session["id"], "action": "existing"})
                continue
            created = request_json(
                args.base_url,
                token,
                "POST",
                "/planning-sessions/",
                session,
            )
            actions.append({"sessionId": created["id"], "action": "submitted"})
    else:
        actions = [
            {"sessionId": session["id"], "action": "planned"}
            for session in sessions
        ]

    manifest = {
        "schemaVersion": "1",
        "campaignPrefix": args.campaign_prefix,
        "mode": "submit" if args.submit else "dry-run",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "sourceThresholds": str(args.thresholds),
        "baseSlowdown": args.base_slowdown,
        "lambdas": list(DEFAULT_LAMBDAS),
        "sessionCount": len(sessions),
        "expectedSimulationCount": sum(
            len(session["algorithms"]) for session in sessions
        ),
        "sessions": sessions,
        "actions": actions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8"
    )
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
