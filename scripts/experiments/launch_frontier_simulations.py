#!/usr/bin/env python3
"""Simulate a bounded, predicted-nondominated candidate set for reference fronts."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import math
import os
import urllib.parse
from pathlib import Path

from launch_campaign_simulations import (
    Client,
    existing_execution_run_ids,
    unwrap_list,
)


def predicted_point(candidate: dict) -> tuple[float, float]:
    predicted = candidate.get("predicted") or {}
    return float(predicted.get("makespanSeconds", math.inf)), float(
        predicted.get("cost", math.inf)
    )


def nondominated_candidates(candidates: list[dict]) -> list[dict]:
    feasible = []
    fingerprints = set()
    for candidate in candidates:
        time, cost = predicted_point(candidate)
        fingerprint = candidate.get("fingerprint") or candidate["id"]
        if (
            candidate.get("feasible")
            and math.isfinite(time)
            and math.isfinite(cost)
            and time >= 0
            and cost >= 0
            and fingerprint not in fingerprints
        ):
            feasible.append(candidate)
            fingerprints.add(fingerprint)
    nondominated = []
    for candidate in feasible:
        time, cost = predicted_point(candidate)
        dominated = any(
            other["id"] != candidate["id"]
            and predicted_point(other)[0] <= time
            and predicted_point(other)[1] <= cost
            and predicted_point(other) != (time, cost)
            for other in feasible
        )
        if not dominated:
            nondominated.append(candidate)
    return sorted(nondominated, key=lambda item: (*predicted_point(item), item["id"]))


def bounded_candidates(candidates: list[dict], maximum: int) -> list[dict]:
    frontier = nondominated_candidates(candidates)
    if maximum < 1:
        raise RuntimeError("maximum candidates per session must be positive")
    if len(frontier) <= maximum:
        return frontier
    if maximum == 1:
        return [frontier[0]]
    indexes = {
        round(index * (len(frontier) - 1) / (maximum - 1))
        for index in range(maximum)
    }
    return [frontier[index] for index in sorted(indexes)]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--campaign-prefix", required=True)
    parser.add_argument("--run-prefix", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--expected-sessions", type=int, required=True)
    parser.add_argument("--max-candidates-per-session", type=int, default=100)
    parser.add_argument("--submit", action="store_true")
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/akoflow-api")
    args = parser.parse_args()

    token = os.environ.get("AKOFLOW_API_TOKEN")
    if not token:
        raise SystemExit("AKOFLOW_API_TOKEN is required")
    client = Client(args.base_url, token)
    sessions = sorted(
        [
            session
            for session in unwrap_list(client.get("/planning-sessions/"))
            if str(session.get("id", "")).startswith(args.campaign_prefix)
        ],
        key=lambda item: item["id"],
    )
    if len(sessions) != args.expected_sessions:
        raise RuntimeError(
            f"expected {args.expected_sessions} sessions, found {len(sessions)}"
        )
    incomplete = [session for session in sessions if session.get("status") != "completed"]
    if incomplete:
        raise RuntimeError(f"{len(incomplete)} reference sessions are not completed")

    environments = {
        item["version"]["id"]: item for item in unwrap_list(client.get("/environments/"))
    }
    workflows = {
        item["version"]["id"]: item["version"]
        for item in unwrap_list(client.get("/workflow-definitions/"))
    }
    existing_run_ids = existing_execution_run_ids(client) if args.submit else set()
    manifest = {
        "schemaVersion": "1",
        "campaignPrefix": args.campaign_prefix,
        "runPrefix": args.run_prefix,
        "mode": "submit" if args.submit else "dry-run",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "maximumCandidatesPerSession": args.max_candidates_per_session,
        "records": [],
    }
    for session in sessions:
        session_id = session["id"]
        candidates = unwrap_list(
            client.get(f"/planning-sessions/{urllib.parse.quote(session_id)}/candidates/")
        )
        selected = bounded_candidates(candidates, args.max_candidates_per_session)
        scope = client.get(
            f"/execution-scopes/{urllib.parse.quote(session['executionScopeId'])}/"
        )
        topology = client.get(
            f"/network-topologies/{urllib.parse.quote(session['networkTopologyId'])}/"
        )
        environment_definitions = [
            environments[version_id] for version_id in scope["environmentVersionIds"]
        ]
        resources = [
            resource
            for environment in environment_definitions
            for resource in environment.get("resources", [])
        ]
        runtimes = [
            runtime
            for environment in environment_definitions
            for runtime in environment.get("runtimes", [])
        ]
        bindings = [
            binding
            for environment in environment_definitions
            for binding in environment.get("resourceRuntimeBindings", [])
        ]
        for index, candidate in enumerate(selected, start=1):
            candidate_id = candidate["id"]
            detail = client.get(
                f"/planning-sessions/{urllib.parse.quote(session_id)}/candidates/"
                f"{urllib.parse.quote(candidate_id)}/"
            )
            plan = detail["plan"]
            run_id = (
                f"{args.run_prefix}-{session_id}-p{index:03d}-{candidate['algorithm']}"
            ).replace("_", "-")
            record = {
                "sessionId": session_id,
                "workflowVersionId": session["workflowVersionId"],
                "executionScopeId": session["executionScopeId"],
                "algorithm": candidate["algorithm"],
                "candidateId": candidate_id,
                "candidateRank": candidate.get("rank"),
                "planId": plan["id"],
                "executionRunId": run_id,
                "predicted": candidate.get("predicted") or {},
                "frontierSampleIndex": index,
                "predictedFrontierCount": len(nondominated_candidates(candidates)),
                "action": "planned",
            }
            if args.submit:
                if run_id in existing_run_ids:
                    record["action"] = "existing"
                else:
                    client.post_discard(
                        f"/planning-sessions/{urllib.parse.quote(session_id)}/candidates/"
                        f"{urllib.parse.quote(candidate_id)}/select/"
                    )
                    payload = {
                        "run": {
                            "id": run_id,
                            "schedulePlanId": plan["id"],
                            "mode": "simulation",
                            "seed": 1,
                            "status": "created",
                            "kind": "workflow",
                            "title": f"Reference frontier {candidate['algorithm']}: {session['workflowVersionId']}",
                        },
                        "plan": plan,
                        "workflow": workflows[session["workflowVersionId"]],
                        "resources": resources,
                        "executionScope": scope,
                        "runtimes": runtimes,
                        "runtimeBindings": bindings,
                        "networkTopology": topology,
                        "activityProfiles": [],
                        "preparationRequirementsByActivity": {},
                        "runtimeAllocations": {},
                    }
                    job = client.post("/execution-runs/", payload)
                    existing_run_ids.add(run_id)
                    record["action"] = "submitted"
                    record["queueJobId"] = job.get("id")
            manifest["records"].append(record)
    manifest["recordCount"] = len(manifest["records"])
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
    print(json.dumps({"mode": manifest["mode"], "records": manifest["recordCount"]}))


if __name__ == "__main__":
    main()
