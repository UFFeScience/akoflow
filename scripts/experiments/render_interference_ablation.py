#!/usr/bin/env python3
"""Render dependency-free SVG figures for the interference ablation."""

from __future__ import annotations

import argparse
import html
import json
from collections import defaultdict
from pathlib import Path

from analyze_campaign_results import read_json


GREY = "#6b7280"
BLUE = "#2563eb"
GREEN = "#059669"
RED = "#dc2626"
INK = "#111827"
GRID = "#d1d5db"


def esc(value) -> str:
    return html.escape(str(value))


def svg_document(width: int, height: int, body: list[str]) -> str:
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" '
        f'viewBox="0 0 {width} {height}">'
        '<rect width="100%" height="100%" fill="white"/>'
        '<style>text{font-family:Inter,Arial,sans-serif;fill:#111827}'
        '.title{font-size:20px;font-weight:700}.label{font-size:12px}'
        '.small{font-size:10px;fill:#4b5563}</style>'
        + "".join(body)
        + "</svg>"
    )


def write_svg(path: Path, width: int, height: int, body: list[str]) -> None:
    path.write_text(svg_document(width, height, body), encoding="utf-8")


def paired_bars(analysis: dict, path: Path) -> None:
    rows = [
        row
        for row in analysis["pairedComparisons"]
        if row["algorithm"] in ("prism-time", "prism-cost")
    ]
    width, height = 1100, 620
    body = ['<text x="40" y="36" class="title">Observed makespan: blind vs aware</text>']
    algorithms = [
        algorithm
        for algorithm in ("prism-time", "prism-cost")
        if any(row["algorithm"] == algorithm for row in rows)
    ]
    panel_width = 900 if len(algorithms) == 1 else 430
    panels = [
        (algorithm, 90 + index * 500)
        for index, algorithm in enumerate(algorithms)
    ]
    for algorithm, x0 in panels:
        items = [row for row in rows if row["algorithm"] == algorithm]
        maximum = max(
            max(row["blindMakespanSeconds"], row["awareMakespanSeconds"])
            for row in items
        )
        body.append(f'<text x="{x0}" y="75" font-weight="700">{esc(algorithm)}</text>')
        chart_h = 440
        for tick in range(6):
            y = 520 - chart_h * tick / 5
            value = maximum * tick / 5
            body.append(f'<line x1="{x0}" y1="{y}" x2="{x0+panel_width}" y2="{y}" stroke="{GRID}"/>')
            body.append(f'<text x="{x0-8}" y="{y+4}" text-anchor="end" class="small">{value:.0f}s</text>')
        group_w = 78
        for index, row in enumerate(items):
            gx = x0 + 28 + index * group_w
            for offset, key, color in (
                (0, "blindMakespanSeconds", GREY),
                (24, "awareMakespanSeconds", BLUE),
            ):
                value = float(row[key])
                h = chart_h * value / maximum
                body.append(f'<rect x="{gx+offset}" y="{520-h}" width="20" height="{h}" fill="{color}" rx="2"/>')
            body.append(f'<text x="{gx+22}" y="540" text-anchor="middle" class="label">s{row["seed"]}</text>')
    body.extend(
        [
            f'<rect x="430" y="575" width="14" height="14" fill="{GREY}"/><text x="450" y="587" class="label">blind</text>',
            f'<rect x="520" y="575" width="14" height="14" fill="{BLUE}"/><text x="540" y="587" class="label">aware</text>',
        ]
    )
    write_svg(path, width, height, body)


def predicted_observed(analysis: dict, path: Path) -> None:
    rows = [
        row
        for row in analysis["runRows"]
        if row["algorithm"] == "prism-time" and row.get("scenario") in ("blind", "aware")
    ]
    width, height = 1050, 620
    maximum = max(
        max(
            float((row.get("predicted") or {}).get("makespanSeconds") or 0),
            float(row["observedMakespanSeconds"]),
        )
        for row in rows
    )
    body = ['<text x="40" y="36" class="title">PRISM Time: planned vs observed</text>']
    chart_h = 450
    for tick in range(6):
        y = 520 - chart_h * tick / 5
        value = maximum * tick / 5
        body.append(f'<line x1="75" y1="{y}" x2="1010" y2="{y}" stroke="{GRID}"/>')
        body.append(f'<text x="67" y="{y+4}" text-anchor="end" class="small">{value:.0f}s</text>')
    for index, row in enumerate(sorted(rows, key=lambda item: (item["selectionSeed"], item["scenario"]))):
        x = 95 + index * 90
        planned = float((row.get("predicted") or {}).get("makespanSeconds") or 0)
        observed = float(row["observedMakespanSeconds"])
        for offset, value, color in ((0, planned, GREEN), (28, observed, BLUE)):
            h = chart_h * value / maximum
            body.append(f'<rect x="{x+offset}" y="{520-h}" width="24" height="{h}" fill="{color}" rx="2"/>')
        body.append(f'<text x="{x+25}" y="540" text-anchor="middle" class="small">s{row["selectionSeed"]} {row["scenario"][0]}</text>')
    body.extend(
        [
            f'<rect x="430" y="575" width="14" height="14" fill="{GREEN}"/><text x="450" y="587" class="label">planned</text>',
            f'<rect x="530" y="575" width="14" height="14" fill="{BLUE}"/><text x="550" y="587" class="label">observed</text>',
        ]
    )
    write_svg(path, width, height, body)


def overlap_scatter(analysis: dict, path: Path) -> None:
    rows = [
        row for row in analysis["runRows"] if row.get("scenario") in ("blind", "aware")
    ]
    width, height = 1000, 620
    max_x = max(float(row.get("interferingOverlapSeconds") or 0) for row in rows) or 1
    deltas = [abs(float(row.get("plannedObservedDeltaSeconds") or 0)) for row in rows]
    max_y = max(deltas) or 1
    body = [
        '<text x="40" y="36" class="title">Interfering overlap vs planning degradation</text>',
        '<line x1="90" y1="530" x2="960" y2="530" stroke="#111827"/>',
        '<line x1="90" y1="70" x2="90" y2="530" stroke="#111827"/>',
        '<text x="525" y="585" text-anchor="middle" class="label">pairwise overlap time (s)</text>',
        '<text x="20" y="300" transform="rotate(-90 20 300)" text-anchor="middle" class="label">|observed - planned| (s)</text>',
    ]
    for row in rows:
        x = 90 + 870 * float(row.get("interferingOverlapSeconds") or 0) / max_x
        y = 530 - 460 * abs(float(row.get("plannedObservedDeltaSeconds") or 0)) / max_y
        cost = max(0.0, float(row.get("observedCost") or 0))
        radius = min(14, 4 + math_sqrt(cost))
        color = GREY if row["scenario"] == "blind" else BLUE
        body.append(f'<circle cx="{x}" cy="{y}" r="{radius}" fill="{color}" fill-opacity="0.7"><title>{esc(row["algorithm"])} seed {row["selectionSeed"]}</title></circle>')
    body.extend(
        [
            f'<circle cx="760" cy="570" r="6" fill="{GREY}"/><text x="774" y="574" class="label">blind</text>',
            f'<circle cx="850" cy="570" r="6" fill="{BLUE}"/><text x="864" y="574" class="label">aware</text>',
        ]
    )
    write_svg(path, width, height, body)


def math_sqrt(value: float) -> float:
    return value**0.5


def gantt(analysis: dict, simulations: dict, path: Path) -> None:
    pairs = [row for row in analysis["pairedComparisons"] if row["algorithm"] == "prism-time"]
    representative = sorted(pairs, key=lambda row: row["blindPenaltyVsAwarePercent"])[len(pairs) // 2]
    run_ids = {"blind": representative["blindRunId"], "aware": representative["awareRunId"]}
    by_run: dict[str, list[dict]] = defaultdict(list)
    for task in simulations.get("activities", []):
        if task["executionRunId"] in run_ids.values():
            by_run[task["executionRunId"]].append(task)
    width, height = 1200, 780
    body = [
        f'<text x="40" y="36" class="title">Representative Gantt · seed {representative["seed"]} · PRISM Time</text>'
    ]
    for panel, scenario in enumerate(("blind", "aware")):
        x0 = 70 + panel * 570
        tasks = sorted(
            by_run[run_ids[scenario]],
            key=lambda item: (float(item.get("startedAt") or 0), str(item["activityId"])),
        )[:60]
        maximum = max((float(task.get("finishedAt") or 0) for task in tasks), default=1)
        body.append(f'<text x="{x0}" y="75" font-weight="700">{scenario}</text>')
        for index, task in enumerate(tasks):
            y = 92 + index * 10
            start = float(task.get("startedAt") or 0)
            finish = float(task.get("finishedAt") or start)
            x = x0 + 90 + 420 * start / maximum
            bar_w = max(1, 420 * (finish - start) / maximum)
            color = RED if float(task.get("interferenceSeconds") or 0) > 0 else BLUE
            body.append(f'<rect x="{x}" y="{y}" width="{bar_w}" height="7" fill="{color}"><title>{esc(task["activityId"])}</title></rect>')
        body.append(f'<line x1="{x0+90}" y1="700" x2="{x0+510}" y2="700" stroke="{INK}"/>')
        body.append(f'<text x="{x0+90}" y="720" class="small">0 s</text><text x="{x0+510}" y="720" text-anchor="end" class="small">{maximum:.1f} s</text>')
    body.extend(
        [
            f'<rect x="440" y="750" width="14" height="8" fill="{RED}"/><text x="462" y="758" class="label">interference affected</text>',
            f'<rect x="640" y="750" width="14" height="8" fill="{BLUE}"/><text x="662" y="758" class="label">not affected</text>',
        ]
    )
    write_svg(path, width, height, body)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--analysis", required=True, type=Path)
    parser.add_argument("--simulations", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path)
    args = parser.parse_args()
    analysis = read_json(args.analysis)
    simulations = read_json(args.simulations)
    args.output_dir.mkdir(parents=True, exist_ok=True)
    paired_bars(analysis, args.output_dir / "blind-aware-makespan.svg")
    predicted_observed(analysis, args.output_dir / "planned-observed.svg")
    overlap_scatter(analysis, args.output_dir / "overlap-degradation.svg")
    gantt(analysis, simulations, args.output_dir / "representative-gantt.svg")
    print(json.dumps({"figures": 4, "outputDir": str(args.output_dir)}))


if __name__ == "__main__":
    main()
