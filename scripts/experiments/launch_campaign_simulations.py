#!/usr/bin/env python3
"""Persist one plan per campaign algorithm and launch idempotent simulations."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path


TERMINAL_PLANNING = {"completed", "failed", "cancelled"}
ALGORITHMS = ("heft", "prism-time", "prism-cost")


class Client:
    def __init__(self, base_url: str, token: str) -> None:
        self.base_url = base_url.rstrip("/")
        self.headers = {
            "Authorization": "Bearer " + token,
            "Content-Type": "application/json",
        }

    def request(self, method: str, path: str, payload=None):
        data = None if payload is None else json.dumps(payload).encode("utf-8")
        request = urllib.request.Request(
            self.base_url + path,
            data=data,
            method=method,
            headers=self.headers,
        )
        try:
            with urllib.request.urlopen(request, timeout=300) as response:
                if response.status == 204:
                    return None
                return json.load(response)
        except urllib.error.HTTPError as error:
            body = error.read().decode("utf-8", errors="replace")
            raise RuntimeError(
                f"{method} {path} returned {error.code}: {body}"
            ) from error

    def get(self, path: str):
        return self.request("GET", path)

    def post(self, path: str, payload=None):
        return self.request("POST", path, payload)

    def exists(self, path: str) -> bool:
        request = urllib.request.Request(
            self.base_url + path,
            method="GET",
            headers=self.headers,
        )
        try:
            with urllib.request.urlopen(request, timeout=60):
                return True
        except urllib.error.HTTPError as error:
            if error.code == 404:
                return False
            raise


def unwrap_list(value):
    if isinstance(value, list):
        return value
    return value.get("results", value.get("items", []))


def choose_candidate(candidates: list[dict], algorithm: str) -> dict:
    feasible = [
        candidate
        for candidate in candidates
        if candidate.get("algorithm") == algorithm and candidate.get("feasible")
    ]
    if not feasible:
        raise RuntimeError(f"no feasible candidate for {algorithm}")

    def number(candidate: dict, key: str) -> float:
        return float(candidate.get("predicted", {}).get(key, float("inf")))

    if algorithm == "prism-cost":
        key = lambda candidate: (
            number(candidate, "cost"),
            number(candidate, "makespanSeconds"),
            candidate.get("rank", 1_000_000),
            candidate["id"],
        )
    else:
        key = lambda candidate: (
            number(candidate, "makespanSeconds"),
            number(candidate, "cost"),
            candidate.get("rank", 1_000_000),
            candidate["id"],
        )
    return min(feasible, key=key)


def run_id(prefix: str, session: dict, algorithm: str) -> str:
    workflow = session["workflowVersionId"].replace("_", "-")
    scope = session["executionScopeId"].removeprefix("scheduler-")
    scope = scope.removesuffix("-scope-v1").replace("_", "-")
    return f"{prefix}-{workflow}-{scope}-{algorithm}"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--campaign-prefix", required=True)
    parser.add_argument("--run-prefix", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--submit", action="store_true")
    parser.add_argument("--allow-incomplete", action="store_true")
    parser.add_argument(
        "--base-url",
        default="http://127.0.0.1:8080/akoflow-api",
    )
    args = parser.parse_args()

    token = os.environ.get("AKOFLOW_API_TOKEN")
    if not token:
        raise SystemExit("AKOFLOW_API_TOKEN is required")
    client = Client(args.base_url, token)

    sessions = [
        session
        for session in unwrap_list(client.get("/planning-sessions/"))
        if str(session.get("id", "")).startswith(args.campaign_prefix)
    ]
    sessions.sort(key=lambda session: session["id"])
    if len(sessions) != 49:
        raise RuntimeError(f"expected 49 campaign sessions, found {len(sessions)}")

    nonterminal = [
        session for session in sessions if session.get("status") not in TERMINAL_PLANNING
    ]
    unsuccessful = [
        session for session in sessions if session.get("status") != "completed"
    ]
    if args.submit and nonterminal:
        raise RuntimeError(
            f"refusing submission while {len(nonterminal)} planning sessions are active"
        )
    if args.submit and unsuccessful:
        raise RuntimeError(
            f"refusing submission because {len(unsuccessful)} sessions did not complete"
        )
    if nonterminal and not args.allow_incomplete:
        raise RuntimeError(
            f"{len(nonterminal)} planning sessions are active; use --allow-incomplete for a dry run"
        )

    environment_definitions = unwrap_list(client.get("/environments/"))
    environment_by_version = {
        item["version"]["id"]: item for item in environment_definitions
    }
    workflow_definitions = unwrap_list(client.get("/workflow-definitions/"))
    workflow_by_version = {
        item["version"]["id"]: item["version"]
        for item in workflow_definitions
    }

    manifest = {
        "schemaVersion": "1",
        "campaignPrefix": args.campaign_prefix,
        "runPrefix": args.run_prefix,
        "mode": "submit" if args.submit else "dry-run",
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "records": [],
    }

    eligible_sessions = [
        session for session in sessions if session.get("status") == "completed"
    ]
    for session in eligible_sessions:
        session_id = session["id"]
        candidates = unwrap_list(
            client.get(
                f"/planning-sessions/{urllib.parse.quote(session_id)}/candidates/"
            )
        )
        scope = client.get(
            f"/execution-scopes/{urllib.parse.quote(session['executionScopeId'])}/"
        )
        topology = client.get(
            f"/network-topologies/{urllib.parse.quote(session['networkTopologyId'])}/"
        )
        workflow = workflow_by_version[session["workflowVersionId"]]
        environments = [
            environment_by_version[version_id]
            for version_id in scope["environmentVersionIds"]
        ]
        resources = [
            resource
            for environment in environments
            for resource in environment.get("resources", [])
        ]
        runtimes = [
            runtime
            for environment in environments
            for runtime in environment.get("runtimes", [])
        ]
        bindings = [
            binding
            for environment in environments
            for binding in environment.get("resourceRuntimeBindings", [])
        ]
        if not resources or not runtimes or not bindings:
            raise RuntimeError(f"incomplete execution inventory for {session_id}")

        for algorithm in ALGORITHMS:
            candidate = choose_candidate(candidates, algorithm)
            candidate_id = candidate["id"]
            detail = client.get(
                f"/planning-sessions/{urllib.parse.quote(session_id)}/candidates/"
                f"{urllib.parse.quote(candidate_id)}/"
            )
            plan = detail["plan"]
            expected_run_id = run_id(args.run_prefix, session, algorithm)
            record = {
                "sessionId": session_id,
                "workflowVersionId": session["workflowVersionId"],
                "executionScopeId": session["executionScopeId"],
                "algorithm": algorithm,
                "candidateId": candidate_id,
                "candidateRank": candidate.get("rank"),
                "candidateParetoOptimal": candidate.get("paretoOptimal"),
                "planId": plan["id"],
                "executionRunId": expected_run_id,
                "predicted": candidate.get("predicted", {}),
                "action": "planned",
            }
            if args.submit:
                if client.exists(
                    f"/execution-runs/{urllib.parse.quote(expected_run_id)}/"
                ):
                    record["action"] = "existing"
                else:
                    persisted = client.post(
                        f"/planning-sessions/{urllib.parse.quote(session_id)}/candidates/"
                        f"{urllib.parse.quote(candidate_id)}/select/"
                    )
                    if persisted["id"] != plan["id"]:
                        raise RuntimeError(
                            f"selected plan ID mismatch for {candidate_id}: "
                            f"{persisted['id']} != {plan['id']}"
                        )
                    payload = {
                        "run": {
                            "id": expected_run_id,
                            "schedulePlanId": plan["id"],
                            "mode": "simulation",
                            "seed": 1,
                            "status": "created",
                            "kind": "workflow",
                            "title": f"Baseline {algorithm}: {session['workflowVersionId']} / {session['executionScopeId']}",
                        },
                        "plan": persisted,
                        "workflow": workflow,
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
                    record["action"] = "submitted"
                    record["queueJobId"] = job.get("id")
            manifest["records"].append(record)

    manifest["recordCount"] = len(manifest["records"])
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(
        json.dumps(
            {
                "mode": manifest["mode"],
                "eligibleSessions": len(eligible_sessions),
                "records": manifest["recordCount"],
                "output": str(args.output),
            }
        )
    )


if __name__ == "__main__":
    main()
