#!/usr/bin/env python3
"""Create the Montage beam-width calibration campaign."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import urllib.error
import urllib.request
from pathlib import Path


DEFAULT_BEAMS = (1, 4, 8, 16, 20, 32, 64, 120)
DEFAULT_WORKFLOWS = ("montage-58-v1", "montage-6448-v1")


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


def build_beam_sessions(
    campaign_prefix: str,
    thresholds: dict,
    workflow_ids: tuple[str, ...] = DEFAULT_WORKFLOWS,
    beams: tuple[int, ...] = DEFAULT_BEAMS,
    sla_factor: float = 1.2,
    scope_id: str = "scheduler-hybrid_hetero-scope-v1",
    topology_id: str = "scheduler-hybrid_hetero-network-v1",
) -> list[dict]:
    selected = {
        item["workflowVersionId"]: item
        for item in thresholds.get("thresholds", [])
        if item["workflowVersionId"] in workflow_ids
        and abs(float(item["factor"]) - sla_factor) <= 1e-12
    }
    if set(selected) != set(workflow_ids):
        raise RuntimeError(
            "missing beam thresholds: " + ", ".join(sorted(set(workflow_ids) - set(selected)))
        )
    sessions = []
    for workflow_id in workflow_ids:
        threshold = selected[workflow_id]
        for beam in beams:
            if beam < 1:
                raise RuntimeError("beam widths must be positive")
            sessions.append(
                {
                    "id": f"{campaign_prefix}-b{beam:04d}-{workflow_id}",
                    "workflowVersionId": workflow_id,
                    "executionScopeId": scope_id,
                    "networkTopologyId": topology_id,
                    "algorithms": [
                        {
                            "id": "prism-time",
                            "configuration": {
                                "beamWidth": beam,
                                "optionCount": 25,
                                "readyBranchLimit": 3,
                            },
                        },
                        {
                            "id": "prism-cost",
                            "configuration": {
                                "beamWidth": beam,
                                "optionCount": 25,
                                "readyBranchLimit": 3,
                            },
                        },
                    ],
                    "deadlineSeconds": threshold["deadlineSeconds"],
                    "budget": threshold["budget"],
                    "configuration": {
                        "campaign": campaign_prefix,
                        "experiment": "4-beam-calibration",
                        "beamWidth": beam,
                        "slaFactor": sla_factor,
                        "referenceExecutionRunId": threshold["executionRunId"],
                        "referenceFrontier": "union-of-all-evaluated-candidates",
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
    parser.add_argument("--expected-sessions", type=int, default=16)
    parser.add_argument("--workflows", default=",".join(DEFAULT_WORKFLOWS))
    parser.add_argument("--beams", default=",".join(str(value) for value in DEFAULT_BEAMS))
    parser.add_argument("--sla-factor", type=float, default=1.2)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/akoflow-api")
    args = parser.parse_args()

    token = os.environ.get("AKOFLOW_API_TOKEN")
    if not token:
        raise SystemExit("AKOFLOW_API_TOKEN is required")
    thresholds = json.loads(args.thresholds.read_text(encoding="utf-8"))
    workflows = tuple(item for item in args.workflows.split(",") if item)
    beams = tuple(int(item) for item in args.beams.split(",") if item)
    sessions = build_beam_sessions(
        args.campaign_prefix,
        thresholds,
        workflows,
        beams,
        args.sla_factor,
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
        "schemaVersion": "1",
        "campaignPrefix": args.campaign_prefix,
        "mode": "submit" if args.submit else "dry-run",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "sourceThresholds": str(args.thresholds),
        "slaFactor": args.sla_factor,
        "workflows": list(workflows),
        "beams": list(beams),
        "sessionCount": len(sessions),
        "expectedSimulationCount": sum(len(session["algorithms"]) for session in sessions),
        "sessions": sessions,
        "actions": actions,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"mode": manifest["mode"], "sessions": len(sessions), "simulations": manifest["expectedSimulationCount"]}))


if __name__ == "__main__":
    main()
