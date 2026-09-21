#!/usr/bin/env python3
"""Create fixed workflow-SLA planning sessions across the baseline environments."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import urllib.error
import urllib.request
from pathlib import Path


def get_json(base_url: str, token: str, path: str):
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        headers={"Authorization": "Bearer " + token},
    )
    with urllib.request.urlopen(request, timeout=60) as response:
        return json.load(response)


def post_json(base_url: str, token: str, path: str, payload: dict):
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        data=json.dumps(payload).encode("utf-8"),
        method="POST",
        headers={
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"POST {path} returned {error.code}: {body}") from error


def factor_slug(factor: float) -> str:
    return f"f{round(factor * 100):03d}"


def build_session_definitions(
    campaign_prefix: str,
    baseline: dict,
    thresholds: dict,
) -> list[dict]:
    baseline_sessions = []
    for record in baseline.get("records", []):
        detail = record.get("session", {})
        session = detail.get("session", detail)
        if session:
            baseline_sessions.append(session)

    by_workflow = {}
    for session in baseline_sessions:
        by_workflow.setdefault(session["workflowVersionId"], []).append(session)
    definitions = []
    for threshold in thresholds.get("thresholds", []):
        workflow = threshold["workflowVersionId"]
        environments = by_workflow.get(workflow, [])
        if not environments:
            raise RuntimeError(f"baseline has no environment sessions for {workflow}")
        for source in sorted(environments, key=lambda item: item["executionScopeId"]):
            scope_slug = source["executionScopeId"].removeprefix("scheduler-")
            scope_slug = scope_slug.removesuffix("-scope-v1").replace("_", "-")
            identifier = (
                f"{campaign_prefix}-{factor_slug(threshold['factor'])}-"
                f"{workflow}-{scope_slug}"
            )
            definitions.append(
                {
                    "id": identifier,
                    "workflowVersionId": workflow,
                    "executionScopeId": source["executionScopeId"],
                    "networkTopologyId": source["networkTopologyId"],
                    "algorithms": [
                        {
                            "id": "prism-time",
                            "configuration": {"beamWidth": 20, "optionCount": 25},
                        },
                        {
                            "id": "prism-cost",
                            "configuration": {"beamWidth": 20, "optionCount": 25},
                        },
                        {"id": "heft"},
                    ],
                    "deadlineSeconds": threshold["deadlineSeconds"],
                    "budget": threshold["budget"],
                    "configuration": {
                        "campaign": campaign_prefix,
                        "experiment": "1B-fixed-sla",
                        "slaFactor": threshold["factor"],
                        "referenceAlgorithm": "heft",
                        "referenceEvaluator": "simgrid-common-simulator",
                        "referenceScopeId": threshold["referenceScopeId"],
                        "referenceExecutionRunId": threshold["executionRunId"],
                        "referenceMakespanSeconds": threshold[
                            "referenceMakespanSeconds"
                        ],
                        "referenceCost": threshold["referenceCost"],
                    },
                }
            )
    identifiers = [definition["id"] for definition in definitions]
    if len(identifiers) != len(set(identifiers)):
        raise RuntimeError("generated duplicate SLA session identifiers")
    return definitions


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--campaign-prefix", required=True)
    parser.add_argument("--baseline-snapshot", required=True, type=Path)
    parser.add_argument("--thresholds", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--submit", action="store_true")
    parser.add_argument("--expected-sessions", type=int, default=147)
    parser.add_argument(
        "--base-url",
        default="http://127.0.0.1:8080/akoflow-api",
    )
    args = parser.parse_args()

    baseline = json.loads(args.baseline_snapshot.read_text(encoding="utf-8"))
    thresholds = json.loads(args.thresholds.read_text(encoding="utf-8"))
    definitions = build_session_definitions(
        args.campaign_prefix,
        baseline,
        thresholds,
    )
    if len(definitions) != args.expected_sessions:
        raise RuntimeError(
            f"expected {args.expected_sessions} SLA sessions, generated {len(definitions)}"
        )

    actions = []
    if args.submit:
        token = os.environ.get("AKOFLOW_API_TOKEN")
        if not token:
            raise SystemExit("AKOFLOW_API_TOKEN is required")
        existing = {
            session["id"]: session
            for session in get_json(args.base_url, token, "/planning-sessions/")
        }
        for definition in definitions:
            found = existing.get(definition["id"])
            if found:
                if (
                    abs(
                        float(found.get("deadlineSeconds", 0))
                        - float(definition["deadlineSeconds"])
                    )
                    > 1e-9
                    or abs(float(found.get("budget", 0)) - float(definition["budget"]))
                    > 1e-9
                ):
                    raise RuntimeError(
                        f"existing session {definition['id']} has different SLA values"
                    )
                actions.append({"sessionId": definition["id"], "action": "existing"})
                continue
            created = post_json(
                args.base_url,
                token,
                "/planning-sessions/",
                definition,
            )
            actions.append({"sessionId": created["id"], "action": "submitted"})
    else:
        actions = [
            {"sessionId": definition["id"], "action": "planned"}
            for definition in definitions
        ]

    manifest = {
        "schemaVersion": "1",
        "campaignPrefix": args.campaign_prefix,
        "mode": "submit" if args.submit else "dry-run",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "sourceBaselineSnapshot": str(args.baseline_snapshot),
        "sourceThresholds": str(args.thresholds),
        "sessionCount": len(definitions),
        "sessions": definitions,
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
                "sessions": len(definitions),
                "output": str(args.output),
            }
        )
    )


if __name__ == "__main__":
    main()
