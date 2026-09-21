#!/usr/bin/env python3
"""Collect a reproducible AkôFlow planning campaign snapshot through the API."""

from __future__ import annotations

import argparse
import datetime as dt
import json
import os
import time
import subprocess
import urllib.error
import urllib.request
from pathlib import Path


def get_json(base_url: str, token: str, path: str, attempts: int = 5):
    for attempt in range(attempts):
        request = urllib.request.Request(
            base_url.rstrip("/") + path,
            headers={"Authorization": "Bearer " + token},
        )
        try:
            with urllib.request.urlopen(request, timeout=60) as response:
                return json.load(response)
        except (
            ConnectionError,
            TimeoutError,
            urllib.error.URLError,
        ):
            if attempt + 1 == attempts:
                raise
            time.sleep(2**attempt)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--prefix", required=True)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument(
        "--runtime-revision",
        default="",
        help="Git revision of the deployed AkôFlow runtime that executed the campaign",
    )
    parser.add_argument(
        "--base-url",
        default="http://127.0.0.1:8080/akoflow-api",
    )
    args = parser.parse_args()

    token = os.environ.get("AKOFLOW_API_TOKEN")
    if not token:
        raise SystemExit("AKOFLOW_API_TOKEN is required")

    listed = get_json(args.base_url, token, "/planning-sessions/")
    sessions = listed if isinstance(listed, list) else listed.get("results", [])
    sessions = [
        session
        for session in sessions
        if str(session.get("id", "")).startswith(args.prefix)
    ]

    records = []
    for summary in sorted(sessions, key=lambda item: item["id"]):
        session_id = summary["id"]
        detail = get_json(
            args.base_url,
            token,
            f"/planning-sessions/{session_id}/",
        )
        candidates = get_json(
            args.base_url,
            token,
            f"/planning-sessions/{session_id}/candidates/",
        )
        records.append(
            {
                "session": detail,
                "candidates": candidates,
            }
        )

    try:
        revision = subprocess.check_output(
            ["git", "rev-parse", "HEAD"], text=True
        ).strip()
    except (OSError, subprocess.CalledProcessError):
        revision = ""

    payload = {
        "schemaVersion": "1",
        "campaignPrefix": args.prefix,
        "capturedAt": dt.datetime.now(dt.timezone.utc).isoformat(),
        "collectorRevision": revision,
        "runtimeRevision": args.runtime_revision or revision,
        "sessionCount": len(records),
        "records": records,
    }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )


if __name__ == "__main__":
    main()
