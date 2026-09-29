#!/usr/bin/env python3
"""Render the cross-coverage blind-versus-aware degradation figure."""

from __future__ import annotations

import argparse
import html
import json
from pathlib import Path


INK = "#111827"
MUTED = "#6b7280"
GRID = "#d1d5db"
BLIND = "#6b7280"
AWARE = "#2563eb"
PLANNED = "#059669"


def esc(value: object) -> str:
    return html.escape(str(value))


def point(x: float, y: float, color: str, label: str) -> str:
    return (
        f'<circle cx="{x:.1f}" cy="{y:.1f}" r="5" fill="white" '
        f'stroke="{color}" stroke-width="3"><title>{esc(label)}</title></circle>'
    )


def polyline(points: list[tuple[float, float]], color: str, dashed: bool = False) -> str:
    values = " ".join(f"{x:.1f},{y:.1f}" for x, y in points)
    dash = ' stroke-dasharray="9 7"' if dashed else ""
    return (
        f'<polyline points="{values}" fill="none" stroke="{color}" '
        f'stroke-width="3" stroke-linejoin="round" stroke-linecap="round"{dash}/>'
    )


def render(analyses: list[dict], output: Path) -> None:
    rows = []
    for analysis in analyses:
        by_scenario = {row["scenario"]: row for row in analysis["runRows"]}
        blind = by_scenario["blind"]
        aware = by_scenario["aware"]
        rows.append(
            {
                "coverage": int(blind.get("executionTruthCoveragePercent") or 0),
                "blind": float(blind["observedMakespanSeconds"]),
                "aware": float(aware["observedMakespanSeconds"]),
                "awarePlanned": float(aware["predicted"]["makespanSeconds"]),
            }
        )
    rows.sort(key=lambda row: row["coverage"])
    baseline = rows[0]["blind"]
    for row in rows:
        row["blindMultiplier"] = row["blind"] / baseline
        row["awareMultiplier"] = row["aware"] / baseline
        row["awareGain"] = (
            (row["blind"] - row["aware"]) / row["blind"] * 100
            if row["blind"]
            else 0
        )

    width, height = 1240, 820
    left, right = 105, 1190
    top_a, bottom_a = 100, 405
    top_b, bottom_b = 505, 750
    xs = {
        row["coverage"]: left + (right - left) * row["coverage"] / 100
        for row in rows
    }
    max_makespan = max(max(row["blind"], row["awarePlanned"]) for row in rows) * 1.08
    max_multiplier = max(row["blindMultiplier"] for row in rows) * 1.08
    y_a = lambda value: bottom_a - (bottom_a - top_a) * value / max_makespan
    y_b = lambda value: bottom_b - (bottom_b - top_b) * value / max_multiplier

    body = [
        '<rect width="100%" height="100%" fill="white"/>',
        '<style>text{font-family:Inter,Arial,sans-serif;fill:#111827}.title{font-size:24px;font-weight:700}.subtitle{font-size:13px;fill:#4b5563}.axis{font-size:12px;fill:#4b5563}.value{font-size:11px;font-weight:600}</style>',
        '<text x="48" y="40" class="title">Degradação com o aumento da interferência</text>',
        '<text x="48" y="65" class="subtitle">Montage 6.448 · PRISM Time · slowdown aditivo 2× · seed 1</text>',
        '<text x="105" y="88" font-size="14" font-weight="700">Makespan observado e projeção aware</text>',
        '<text x="105" y="487" font-size="14" font-weight="700">Multiplicador de degradação relativo ao cenário de 0%</text>',
    ]

    for tick in range(6):
        value = max_makespan * tick / 5
        y = y_a(value)
        body.append(f'<line x1="{left}" y1="{y:.1f}" x2="{right}" y2="{y:.1f}" stroke="{GRID}"/>')
        body.append(f'<text x="{left-12}" y="{y+4:.1f}" text-anchor="end" class="axis">{value:.0f}s</text>')
    for tick in range(8):
        value = max_multiplier * tick / 7
        y = y_b(value)
        body.append(f'<line x1="{left}" y1="{y:.1f}" x2="{right}" y2="{y:.1f}" stroke="{GRID}"/>')
        body.append(f'<text x="{left-12}" y="{y+4:.1f}" text-anchor="end" class="axis">{value:.0f}×</text>')

    for row in rows:
        x = xs[row["coverage"]]
        body.append(f'<line x1="{x:.1f}" y1="{top_a}" x2="{x:.1f}" y2="{bottom_a}" stroke="{GRID}" stroke-dasharray="3 5"/>')
        body.append(f'<line x1="{x:.1f}" y1="{top_b}" x2="{x:.1f}" y2="{bottom_b}" stroke="{GRID}" stroke-dasharray="3 5"/>')
        body.append(f'<text x="{x:.1f}" y="{bottom_b+28}" text-anchor="middle" class="axis">{row["coverage"]}%</text>')

    series = (
        ("blind", BLIND, False),
        ("aware", AWARE, False),
        ("awarePlanned", PLANNED, True),
    )
    for key, color, dashed in series:
        values = [(xs[row["coverage"]], y_a(row[key])) for row in rows]
        body.append(polyline(values, color, dashed))
        for row, (x, y) in zip(rows, values):
            body.append(point(x, y, color, f'{row["coverage"]}%: {row[key]:.2f} s'))
            if key != "awarePlanned":
                dy = -11 if key == "blind" else 18
                body.append(f'<text x="{x:.1f}" y="{y+dy:.1f}" text-anchor="middle" class="value" fill="{color}">{row[key]:.0f}s</text>')

    for key, color in (("blindMultiplier", BLIND), ("awareMultiplier", AWARE)):
        values = [(xs[row["coverage"]], y_b(row[key])) for row in rows]
        body.append(polyline(values, color))
        for row, (x, y) in zip(rows, values):
            body.append(point(x, y, color, f'{row["coverage"]}%: {row[key]:.2f}×'))
            dy = -11 if key == "blindMultiplier" else 18
            body.append(f'<text x="{x:.1f}" y="{y+dy:.1f}" text-anchor="middle" class="value">{row[key]:.1f}×</text>')
        
    for row in rows[1:]:
        x = xs[row["coverage"]]
        y = min(y_b(row["blindMultiplier"]), y_b(row["awareMultiplier"])) - 31
        body.append(f'<text x="{x:.1f}" y="{y:.1f}" text-anchor="middle" class="value" fill="{AWARE}">aware −{row["awareGain"]:.1f}%</text>')

    body.extend(
        [
            f'<line x1="760" y1="36" x2="795" y2="36" stroke="{BLIND}" stroke-width="3"/><text x="805" y="41" class="axis">blind observado</text>',
            f'<line x1="930" y1="36" x2="965" y2="36" stroke="{AWARE}" stroke-width="3"/><text x="975" y="41" class="axis">aware observado</text>',
            f'<line x1="760" y1="58" x2="795" y2="58" stroke="{PLANNED}" stroke-width="3" stroke-dasharray="9 7"/><text x="805" y="63" class="axis">aware planejado</text>',
            f'<text x="{(left+right)/2:.1f}" y="802" text-anchor="middle" class="axis">Cobertura da matriz de interferência</text>',
        ]
    )
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">'
        + "".join(body)
        + "</svg>",
        encoding="utf-8",
    )


def render_simple(analyses: list[dict], output: Path) -> None:
    rows = []
    for analysis in analyses:
        by_scenario = {row["scenario"]: row for row in analysis["runRows"]}
        blind = by_scenario["blind"]
        aware = by_scenario["aware"]
        rows.append(
            {
                "coverage": int(blind.get("executionTruthCoveragePercent") or 0),
                "blind": float(blind["observedMakespanSeconds"]),
                "aware": float(aware["observedMakespanSeconds"]),
            }
        )
    rows.sort(key=lambda row: row["coverage"])

    width, height = 1080, 650
    left, right, top, bottom = 105, 1025, 105, 555
    maximum = max(row["blind"] for row in rows) * 1.08
    xs = {
        row["coverage"]: left + (right - left) * row["coverage"] / 100
        for row in rows
    }
    y = lambda value: bottom - (bottom - top) * value / maximum
    body = [
        '<rect width="100%" height="100%" fill="white"/>',
        '<style>text{font-family:Inter,Arial,sans-serif;fill:#111827}.title{font-size:24px;font-weight:700}.subtitle{font-size:13px;fill:#4b5563}.axis{font-size:12px;fill:#4b5563}.value{font-size:11px;font-weight:700}</style>',
        '<text x="48" y="40" class="title">Makespan por cobertura de interferência</text>',
        '<text x="48" y="65" class="subtitle">Montage 6.448 · PRISM Time · execução no SimGrid · seed 1</text>',
    ]
    for tick in range(6):
        value = maximum * tick / 5
        tick_y = y(value)
        body.append(f'<line x1="{left}" y1="{tick_y:.1f}" x2="{right}" y2="{tick_y:.1f}" stroke="{GRID}"/>')
        body.append(f'<text x="{left-12}" y="{tick_y+4:.1f}" text-anchor="end" class="axis">{value:.0f}s</text>')
    for row in rows:
        x = xs[row["coverage"]]
        body.append(f'<line x1="{x:.1f}" y1="{top}" x2="{x:.1f}" y2="{bottom}" stroke="{GRID}" stroke-dasharray="3 5"/>')
        body.append(f'<text x="{x:.1f}" y="{bottom+28}" text-anchor="middle" class="axis">{row["coverage"]}%</text>')
    for key, color, label in (("blind", BLIND, "Blind"), ("aware", AWARE, "Aware")):
        points = [(xs[row["coverage"]], y(row[key])) for row in rows]
        body.append(polyline(points, color))
        for row, (x, point_y) in zip(rows, points):
            body.append(point(x, point_y, color, f'{label} · {row["coverage"]}% · {row[key]:.2f} s'))
            offset = -13 if key == "blind" else 22
            body.append(f'<text x="{x:.1f}" y="{point_y+offset:.1f}" text-anchor="middle" class="value">{row[key]:.0f}s</text>')
    body.extend(
        [
            f'<line x1="760" y1="38" x2="800" y2="38" stroke="{BLIND}" stroke-width="3"/><text x="812" y="43" class="axis">Blind</text>',
            f'<line x1="885" y1="38" x2="925" y2="38" stroke="{AWARE}" stroke-width="3"/><text x="937" y="43" class="axis">Aware</text>',
            f'<text x="{(left+right)/2:.1f}" y="625" text-anchor="middle" class="axis">Cobertura da interferência</text>',
            f'<text x="24" y="{(top+bottom)/2:.1f}" transform="rotate(-90 24 {(top+bottom)/2:.1f})" text-anchor="middle" class="axis">Makespan observado (segundos)</text>',
        ]
    )
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">'
        + "".join(body)
        + "</svg>",
        encoding="utf-8",
    )


def render_degradation(analyses: list[dict], output: Path) -> None:
    rows = []
    for analysis in analyses:
        by_scenario = {row["scenario"]: row for row in analysis["runRows"]}
        blind = by_scenario["blind"]
        aware = by_scenario["aware"]
        rows.append(
            {
                "coverage": int(blind.get("executionTruthCoveragePercent") or 0),
                "blindDegradation": (
                    abs(float(blind["observedMakespanSeconds"]) - float(blind["predicted"]["makespanSeconds"]))
                    / float(blind["predicted"]["makespanSeconds"])
                    * 100
                ),
                "awareDegradation": (
                    abs(float(aware["observedMakespanSeconds"]) - float(aware["predicted"]["makespanSeconds"]))
                    / float(aware["predicted"]["makespanSeconds"])
                    * 100
                ),
            }
        )
    rows.sort(key=lambda row: row["coverage"])

    width, height = 1080, 650
    left, right, top, bottom = 115, 1025, 105, 555
    values = [
        row[key]
        for row in rows
        for key in ("blindDegradation", "awareDegradation")
    ]
    minimum = min(0.0, min(values))
    maximum = max(0.0, max(values))
    padding = max((maximum - minimum) * 0.08, 5.0)
    axis_min = minimum - padding if minimum < 0 else 0.0
    axis_max = maximum + padding
    xs = {
        row["coverage"]: left + (right - left) * row["coverage"] / 100
        for row in rows
    }
    y = lambda value: bottom - (bottom - top) * (value - axis_min) / (axis_max - axis_min)
    body = [
        '<rect width="100%" height="100%" fill="white"/>',
        '<style>text{font-family:Inter,Arial,sans-serif;fill:#111827}.title{font-size:24px;font-weight:700}.subtitle{font-size:13px;fill:#4b5563}.axis{font-size:12px;fill:#4b5563}.value{font-size:11px;font-weight:700}</style>',
        '<text x="48" y="40" class="title">Erro absoluto em relação ao makespan planejado</text>',
        '<text x="48" y="65" class="subtitle">|executado − planejado| / planejado × 100</text>',
    ]
    for tick in range(6):
        value = axis_min + (axis_max - axis_min) * tick / 5
        tick_y = y(value)
        body.append(f'<line x1="{left}" y1="{tick_y:.1f}" x2="{right}" y2="{tick_y:.1f}" stroke="{GRID}"/>')
        body.append(f'<text x="{left-12}" y="{tick_y+4:.1f}" text-anchor="end" class="axis">{value:.0f}%</text>')
    if axis_min < 0:
        zero_y = y(0)
        body.append(f'<line x1="{left}" y1="{zero_y:.1f}" x2="{right}" y2="{zero_y:.1f}" stroke="{INK}" stroke-width="1.5"/>')
    for row in rows:
        x = xs[row["coverage"]]
        body.append(f'<line x1="{x:.1f}" y1="{top}" x2="{x:.1f}" y2="{bottom}" stroke="{GRID}" stroke-dasharray="3 5"/>')
        body.append(f'<text x="{x:.1f}" y="{bottom+28}" text-anchor="middle" class="axis">{row["coverage"]}%</text>')
    for key, color, label in (
        ("blindDegradation", BLIND, "Blind"),
        ("awareDegradation", AWARE, "Aware"),
    ):
        points = [(xs[row["coverage"]], y(row[key])) for row in rows]
        body.append(polyline(points, color))
        for row, (x, point_y) in zip(rows, points):
            body.append(point(x, point_y, color, f'{label} · cobertura {row["coverage"]}% · erro absoluto {row[key]:.2f}%'))
            offset = -13 if key == "blindDegradation" else 22
            body.append(f'<text x="{x:.1f}" y="{point_y+offset:.1f}" text-anchor="middle" class="value">{row[key]:.1f}%</text>')
    body.extend(
        [
            f'<line x1="760" y1="38" x2="800" y2="38" stroke="{BLIND}" stroke-width="3"/><text x="812" y="43" class="axis">Blind</text>',
            f'<line x1="885" y1="38" x2="925" y2="38" stroke="{AWARE}" stroke-width="3"/><text x="937" y="43" class="axis">Aware</text>',
            f'<text x="{(left+right)/2:.1f}" y="625" text-anchor="middle" class="axis">Cobertura da interferência</text>',
            f'<text x="24" y="{(top+bottom)/2:.1f}" transform="rotate(-90 24 {(top+bottom)/2:.1f})" text-anchor="middle" class="axis">Erro absoluto sobre o planejado (%)</text>',
        ]
    )
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(
        f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="0 0 {width} {height}">'
        + "".join(body)
        + "</svg>",
        encoding="utf-8",
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--analysis", required=True, nargs="+", type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--metric", choices=("makespan", "degradation"), default="makespan")
    args = parser.parse_args()
    analyses = [json.loads(path.read_text()) for path in args.analysis]
    if args.metric == "degradation":
        render_degradation(analyses, args.output)
    else:
        render_simple(analyses, args.output)
    print(json.dumps({"figure": str(args.output), "analyses": len(args.analysis)}))


if __name__ == "__main__":
    main()
