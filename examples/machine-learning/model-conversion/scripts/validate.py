#!/usr/bin/env python3
import json, sys
from pathlib import Path
root = Path(__file__).resolve().parents[1]
expected = json.loads((root / "expected" / "manifest.json").read_text())
actual_path = root / "outputs" / "manifest.json"
if not actual_path.exists():
    sys.exit("missing outputs/manifest.json; run ./run.sh first")
actual = json.loads(actual_path.read_text())
if actual["showcase"] != expected["showcase"] or actual["artifacts"] != expected["artifacts"] or actual["status"] != "success":
    sys.exit("manifest does not match expected contract")
missing = [name for name in expected["artifacts"] if not (root / "outputs" / name).is_file()]
if missing:
    sys.exit("missing artifacts: " + ", ".join(missing))
missing = [name for name in ("workflow.png", "execution.png", "outputs.png") if not (root / "screenshots" / name).is_file()]
if missing:
    sys.exit("missing screenshots: " + ", ".join(missing))
print("validated " + expected["showcase"])

