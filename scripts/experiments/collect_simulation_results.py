#!/usr/bin/env python3
"""Collect execution, activity, transfer, handle, and event evidence for a manifest."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter
from pathlib import Path


def get_json(base_url: str, token: str, path: str):
    request = urllib.request.Request(
        base_url.rstrip("/") + path,
        headers={"Authorization": "Bearer " + token},
    )
    try:
        with urllib.request.urlopen(request, timeout=300) as response:
            return json.load(response)
    except urllib.error.HTTPError as error:
        if error.code == 404:
            return None
        body = error.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"GET {path} returned {error.code}: {body}") from error


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument(
        "--base-url",
        default="http://127.0.0.1:8080/akoflow-api",
    )
    args = parser.parse_args()

    token = os.environ.get("AKOFLOW_API_TOKEN")
    if not token:
        raise SystemExit("AKOFLOW_API_TOKEN is required")
    manifest = json.loads(args.manifest.read_text(encoding="utf-8"))

    records = []
    activity_records = []
    transfer_records = []
    handle_records = []
    event_records = []
    status_counts: Counter[str] = Counter()
    for planned in manifest.get("records", []):
        run_id = planned["executionRunId"]
        detail = get_json(
            args.base_url,
            token,
            f"/execution-runs/{urllib.parse.quote(run_id)}/",
        )
        if detail is None:
            status = "not-created"
            run = {}
            detail = {}
        else:
            run = detail.get("run", detail)
            status = run.get("status", "unknown")
        status_counts[status] += 1
        predicted = planned.get("predicted", {})
        observed_makespan = run.get("makespanSeconds")
        observed_cost = run.get("cost")
        predicted_makespan = predicted.get("makespanSeconds")
        predicted_cost = predicted.get("cost")

        def relative_error(predicted_value, observed_value):
            if predicted_value is None or observed_value in (None, 0):
                return None
            return abs(float(predicted_value) - float(observed_value)) / abs(
                float(observed_value)
            )

        records.append(
            {
                **planned,
                "status": status,
                "mode": run.get("mode"),
                "seed": run.get("seed"),
                "startedAt": run.get("startedAt"),
                "finishedAt": run.get("finishedAt"),
                "observedMakespanSeconds": observed_makespan,
                "observedCost": observed_cost,
                "predictionErrorMakespan": relative_error(
                    predicted_makespan, observed_makespan
                ),
                "predictionErrorCost": relative_error(predicted_cost, observed_cost),
                "breakdown": run.get("breakdown", {}),
                "transferredBytes": run.get("transferredBytes"),
                "activityCount": run.get("activityCount"),
                "completedActivityCount": run.get("completedActivityCount"),
                "failureReason": run.get("failureReason", ""),
            }
        )
        for key, target in (
            ("activities", activity_records),
            ("dataTransfers", transfer_records),
            ("handles", handle_records),
            ("events", event_records),
        ):
            for item in detail.get(key, []):
                target.append(
                    {
                        "executionRunId": run_id,
                        "sessionId": planned.get("sessionId"),
                        "workflowVersionId": planned.get("workflowVersionId"),
                        "executionScopeId": planned.get("executionScopeId"),
                        "algorithm": planned.get("algorithm"),
                        **item,
                    }
                )

    result = {
        "schemaVersion": "1",
        "sourceManifest": str(args.manifest),
        "campaignPrefix": manifest.get("campaignPrefix"),
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "statusCounts": dict(status_counts),
        "records": records,
        "activities": activity_records,
        "dataTransfers": transfer_records,
        "handles": handle_records,
        "events": event_records,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(result, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(
        json.dumps(
            {
                "runs": len(records),
                "statuses": dict(status_counts),
                "activities": len(activity_records),
                "transfers": len(transfer_records),
                "handles": len(handle_records),
                "events": len(event_records),
            }
        )
    )


if __name__ == "__main__":
    main()
