#!/usr/bin/env python3
"""Deterministic, dependency-free executable for an AkôFlow showcase image."""
from __future__ import annotations
import csv, hashlib, json, pickle, sys
from pathlib import Path

root = Path("/app")
config = json.loads((root / "scenario.json").read_text())
out = Path(sys.argv[1] if len(sys.argv) > 1 else "/outputs")
out.mkdir(parents=True, exist_ok=True)
digest = hashlib.sha256(json.dumps(config, sort_keys=True).encode()).hexdigest()
record = {"showcase": config["slug"], "digest": digest, "stages": config["stages"], "status": "success"}
for name in config["artifacts"]:
    path = out / name
    if name.endswith(".json"):
        path.write_text(json.dumps({**record, "artifact": name}, indent=2) + "\n")
    elif name.endswith(".csv"):
        with path.open("w", newline="") as f:
            writer = csv.DictWriter(f, fieldnames=["candidate", "score", "selected"])
            writer.writeheader()
            writer.writerow({"candidate": config["slug"], "score": "1.000", "selected": "true"})
    elif name.endswith(".pkl"):
        path.write_bytes(pickle.dumps({**record, "artifact": name}, protocol=4))
    elif name.endswith(".html"):
        path.write_text("<!doctype html><title>AkôFlow report</title><h1>" + config["title"] + "</h1><pre>" + json.dumps(record, indent=2) + "</pre>")
    elif name.endswith(".md"):
        path.write_text("# " + config["title"] + "\n\n" + json.dumps(record, indent=2) + "\n")
    else:
        path.write_text(json.dumps({**record, "artifact": name}) + "\n")
(out / "manifest.json").write_text(json.dumps({**record, "artifacts": config["artifacts"]}, indent=2) + "\n")

