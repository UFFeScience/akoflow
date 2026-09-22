#!/usr/bin/env python3
import json
import math
from collections import defaultdict
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
from matplotlib.colors import TwoSlopeNorm
from matplotlib.ticker import FuncFormatter

ROOT = Path(__file__).resolve().parents[2]
DATA = ROOT / "outputs" / "prism-paper-experiments"
OUT = DATA / "paper-figures"
OUT.mkdir(parents=True, exist_ok=True)

with open(DATA / "baseline-analysis.json") as f:
    baseline = json.load(f)
with open(DATA / "sla-analysis.json") as f:
    sla = json.load(f)
with open(DATA / "interference-analysis-r2.json") as f:
    interference = json.load(f)
with open(DATA / "beam-analysis-r2.json") as f:
    beam = json.load(f)
with open(DATA / "reference-frontier-r2-analysis.json") as f:
    frontier = json.load(f)

COLORS = {
    "heft": "#667085",
    "prism-cost": "#0072B2",
    "prism-time": "#D55E00",
    "accent": "#009E73",
    "ink": "#172B4D",
    "grid": "#D9E2EC",
    "warm": "#F0E442",
}
LABELS = {"heft": "HEFT", "prism-cost": "PRISM Cost", "prism-time": "PRISM Time"}
ORDER = ["heft", "prism-cost", "prism-time"]

plt.rcParams.update({
    "font.family": "DejaVu Sans",
    "font.size": 9,
    "axes.titlesize": 11,
    "axes.labelsize": 9,
    "axes.edgecolor": "#AAB7C4",
    "axes.linewidth": 0.7,
    "axes.titleweight": "bold",
    "figure.facecolor": "white",
    "axes.facecolor": "white",
    "savefig.facecolor": "white",
    "savefig.bbox": "tight",
    "legend.frameon": False,
})


def style(ax, grid="y"):
    ax.spines[["top", "right"]].set_visible(False)
    ax.grid(True, axis=grid, color=COLORS["grid"], linewidth=0.6, alpha=0.75)
    ax.set_axisbelow(True)


def save(fig, stem):
    fig.savefig(OUT / f"{stem}.png", dpi=320)
    fig.savefig(OUT / f"{stem}.pdf")
    fig.savefig(OUT / f"{stem}.svg")
    plt.close(fig)


# 1. Basic overview: winner credits and SLA satisfaction.
fig, axes = plt.subplots(1, 2, figsize=(10.4, 3.8), constrained_layout=True)
metrics = ["Makespan", "Cost"]
x = np.arange(2)
width = 0.24
for i, alg in enumerate(ORDER):
    values = [baseline["winnerCounts"]["makespan"][alg], baseline["winnerCounts"]["cost"][alg]]
    bars = axes[0].bar(x + (i - 1) * width, values, width, color=COLORS[alg], label=LABELS[alg])
    axes[0].bar_label(bars, fmt="%.1f", padding=2, fontsize=8)
axes[0].set_xticks(x, metrics)
axes[0].set_ylabel("Winner credits (49 cases)")
axes[0].set_title("A. Objective specialization")
axes[0].legend(ncol=3, loc="upper center", bbox_to_anchor=(0.5, -0.12))
style(axes[0])

sla_counts = [sla["winnerCounts"]["sla"][alg] for alg in ORDER]
bars = axes[1].bar([LABELS[a] for a in ORDER], sla_counts, color=[COLORS[a] for a in ORDER], width=0.62)
axes[1].axhline(147, color=COLORS["ink"], linewidth=1, linestyle="--")
axes[1].bar_label(bars, labels=[f"{v}/147" for v in sla_counts], padding=3, fontsize=9)
axes[1].set_ylim(0, 157)
axes[1].set_ylabel("SLA-compliant schedules")
axes[1].set_title("B. Feasibility under deadline and budget")
style(axes[1])
fig.suptitle("PRISM separates time and cost goals while satisfying more SLAs", fontsize=14, fontweight="bold", color=COLORS["ink"])
save(fig, "fig01-overview")


# 2. Elaborate heatmap: PRISM Time speedup over HEFT across workflows/environments.
rows = baseline["runRows"]
groups = defaultdict(dict)
for row in rows:
    groups[(row["workflowVersionId"], row["executionScopeId"])][row["algorithm"]] = row
workflows = sorted({key[0] for key in groups})
envs = sorted({key[1] for key in groups})
wf_names = {
    "montage-58-v1": "Montage 58",
    "montage-6448-v1": "Montage 6448",
    "wfcommons-1000genome-902-v1": "1000Genome",
    "wfcommons-cycles-6543-v1": "Cycles",
    "wfcommons-epigenomics-1695-v1": "Epigenomics",
    "wfcommons-seismology-1101-v1": "Seismology",
    "wfcommons-srasearch-104-v1": "SRA Search",
}
env_names = {
    "scheduler-cloud_hetero-scope-v1": "Cloud\nhetero",
    "scheduler-cloud_homo-scope-v1": "Cloud\nhomo",
    "scheduler-cluster_hetero-scope-v1": "Cluster\nhetero",
    "scheduler-cluster_homo-scope-v1": "Cluster\nhomo",
    "scheduler-hybrid_hetero-scope-v1": "Hybrid\nhetero",
    "scheduler-hybrid_homo-scope-v1": "Hybrid\nhomo",
    "scheduler-network_fog_hpc_cloud-scope-v1": "Fog–HPC–\ncloud",
}
matrix = np.empty((len(workflows), len(envs)))
for i, wf in enumerate(workflows):
    for j, env in enumerate(envs):
        cell = groups[(wf, env)]
        matrix[i, j] = cell["heft"]["observedMakespanSeconds"] / cell["prism-time"]["observedMakespanSeconds"]
log_matrix = np.log2(matrix)
lim = max(abs(log_matrix.min()), abs(log_matrix.max()))
fig, ax = plt.subplots(figsize=(11.4, 5.3), constrained_layout=True)
im = ax.imshow(log_matrix, cmap="RdYlBu", norm=TwoSlopeNorm(vmin=-lim, vcenter=0, vmax=lim), aspect="auto")
for i in range(matrix.shape[0]):
    for j in range(matrix.shape[1]):
        value = matrix[i, j]
        text = f"{value:.1f}×" if value >= 1 else f"{value:.2f}×"
        ax.text(j, i, text, ha="center", va="center", fontsize=8, color="black" if abs(log_matrix[i, j]) < lim * 0.62 else "white", fontweight="bold" if value >= 10 or value < 1 else "normal")
ax.set_xticks(np.arange(len(envs)), [env_names[e] for e in envs])
ax.set_yticks(np.arange(len(workflows)), [wf_names[w] for w in workflows])
ax.set_xlabel("Execution environment")
ax.set_ylabel("Workflow")
ax.set_title("PRISM Time speedup over HEFT (executed makespan)", fontsize=14, color=COLORS["ink"], pad=14)
cbar = fig.colorbar(im, ax=ax, fraction=0.025, pad=0.025)
cbar.set_label("log₂ speedup; blue favors PRISM Time, red favors HEFT")
for spine in ax.spines.values():
    spine.set_visible(False)
save(fig, "fig02-environment-speedup-heatmap")


# 3. Planned vs executed parity plot.
fig, ax = plt.subplots(figsize=(7.2, 5.6), constrained_layout=True)
all_values = []
markers = {"heft": "o", "prism-cost": "s", "prism-time": "^"}
for alg in ORDER:
    subset = [r for r in rows if r["algorithm"] == alg and r["predicted"]["makespanSeconds"] > 0 and r["observedMakespanSeconds"] > 0]
    xvals = np.array([r["predicted"]["makespanSeconds"] for r in subset])
    yvals = np.array([r["observedMakespanSeconds"] for r in subset])
    all_values.extend(xvals.tolist() + yvals.tolist())
    ax.scatter(xvals, yvals, s=38, marker=markers[alg], color=COLORS[alg], alpha=0.76, edgecolor="white", linewidth=0.45, label=LABELS[alg])
lo, hi = min(all_values), max(all_values)
ax.plot([lo, hi], [lo, hi], color=COLORS["ink"], linestyle="--", linewidth=1.1, label="Perfect prediction")
ax.set_xscale("log")
ax.set_yscale("log")
ax.set_xlim(lo * 0.7, hi * 1.5)
ax.set_ylim(lo * 0.7, hi * 1.5)
ax.set_xlabel("Planned makespan (s, log scale)")
ax.set_ylabel("Executed makespan (s, log scale)")
ax.set_title("Prediction fidelity across 147 baseline executions", fontsize=14, color=COLORS["ink"], pad=12)
ax.legend(ncol=2, loc="upper left")
style(ax, grid="both")
ax.annotate("HEFT underestimation\nin distributed environments", xy=(80, 16000), xytext=(3, 22000), arrowprops=dict(arrowstyle="->", color=COLORS["heft"], lw=1), color=COLORS["ink"], fontsize=9)
save(fig, "fig03-planned-vs-executed")


# 4. Interference narrative: absolute performance and relative degradation.
coverages = [0, 10, 20, 50, 80, 100]
fig, axes = plt.subplots(1, 2, figsize=(11.5, 4.4), constrained_layout=True)
for alg in ORDER:
    means, lows, highs, degradation = [], [], [], []
    for cov in coverages:
        subset = [r for r in interference["runRows"] if r["algorithm"] == alg and r["coveragePercent"] == cov]
        observed = np.array([r["observedMakespanSeconds"] for r in subset])
        means.append(observed.mean())
        lows.append(observed.min())
        highs.append(observed.max())
        degradation.append(np.mean([r["makespanDegradationPercent"] for r in subset]))
    axes[0].plot(coverages, means, marker={"heft":"o","prism-cost":"s","prism-time":"^"}[alg], color=COLORS[alg], linewidth=2, label=LABELS[alg])
    axes[0].fill_between(coverages, lows, highs, color=COLORS[alg], alpha=0.12)
    axes[1].plot(coverages, degradation, marker={"heft":"o","prism-cost":"s","prism-time":"^"}[alg], color=COLORS[alg], linewidth=2, label=LABELS[alg])
axes[0].set_yscale("log")
axes[0].set_xlabel("Activities affected by slowdown (%)")
axes[0].set_ylabel("Executed makespan (s, log scale)")
axes[0].set_title("A. Absolute performance remains separated")
axes[0].legend(ncol=1, loc="center left", bbox_to_anchor=(0.02, 0.48))
style(axes[0], grid="both")
axes[0].annotate("PRISM Time at 100%: 145 s\nHEFT at 0%: 15,988 s", xy=(100, 145.31), xytext=(48, 540), arrowprops=dict(arrowstyle="->", color=COLORS["prism-time"], lw=1), color=COLORS["ink"], fontsize=9)
axes[1].set_xlabel("Activities affected by slowdown (%)")
axes[1].set_ylabel("Makespan degradation from own baseline (%)")
axes[1].set_title("B. Relative sensitivity tells a different story")
style(axes[1])
fig.suptitle("Interference changes relative sensitivity, not the winner", fontsize=14, fontweight="bold", color=COLORS["ink"])
save(fig, "fig04-interference-story")


# 5. Beam elbow: observed quality and planning cost.
fig, axes = plt.subplots(1, 2, figsize=(11.5, 4.4), constrained_layout=True)
for ax, wf in zip(axes, ["montage-58-v1", "montage-6448-v1"]):
    subset = sorted([r for r in beam["beamComparisons"] if r["workflowVersionId"] == wf and r["algorithm"] == "prism-time"], key=lambda r: r["beamWidth"])
    xs = np.array([r["beamWidth"] for r in subset])
    ms = np.array([r["observedMakespanSeconds"] for r in subset])
    planning = np.array([r["planningElapsedSeconds"] for r in subset])
    ax.plot(xs, ms, color=COLORS["prism-time"], marker="o", linewidth=2, label="Executed makespan")
    ax.set_xscale("log", base=2)
    ax.set_xticks(xs, [str(x) for x in xs])
    ax.set_xlabel("Beam width")
    ax.set_ylabel("Executed makespan (s)", color=COLORS["prism-time"])
    ax.tick_params(axis="y", labelcolor=COLORS["prism-time"])
    ax.axvspan(4, 8, color=COLORS["accent"], alpha=0.12)
    ax2 = ax.twinx()
    ax2.plot(xs, planning, color=COLORS["heft"], marker="s", linestyle="--", linewidth=1.5, label="Planning time")
    ax2.set_ylabel("Planning time (s)", color=COLORS["heft"])
    ax2.tick_params(axis="y", labelcolor=COLORS["heft"])
    ax.set_title(wf_names[wf])
    ax.grid(True, axis="x", color=COLORS["grid"], linewidth=0.6)
    ax.spines["top"].set_visible(False)
    ax2.spines["top"].set_visible(False)
    if wf == "montage-6448-v1":
        ax.annotate("Elbow", xy=(4, ms[list(xs).index(4)]), xytext=(5.2, ms.max() * 0.72), arrowprops=dict(arrowstyle="->", color=COLORS["accent"]), color=COLORS["accent"], fontweight="bold")
fig.suptitle("Most beam-search value is captured by widths 4–8", fontsize=14, fontweight="bold", color=COLORS["ink"])
fig.text(0.5, -0.01, "Green band marks the recommended operating region; beam 120 adds planning cost without consistent quality gain.", ha="center", color=COLORS["ink"], fontsize=9)
save(fig, "fig05-beam-elbow")


# 6. Reference frontier: complementary operating regions on Montage 6448.
wf_frontier = next(w for w in frontier["workflows"] if w["workflowVersionId"] == "montage-6448-v1")
fig, ax = plt.subplots(figsize=(8.1, 5.4), constrained_layout=True)
for alg in ["prism-cost", "prism-time"]:
    subset = [r for r in wf_frontier["frontier"] if r["algorithm"] == alg]
    xvals = np.array([r["observedMakespanSeconds"] for r in subset])
    yvals = np.array([r["observedCost"] for r in subset])
    ax.scatter(xvals, yvals, s=45, marker="s" if alg == "prism-cost" else "^", color=COLORS[alg], alpha=0.72, edgecolor="white", linewidth=0.45, label=LABELS[alg])
ax.set_xscale("log")
ax.set_yscale("symlog", linthresh=0.002)
ax.set_xlabel("Executed makespan (s, log scale)")
ax.set_ylabel("Executed cost (symlog; zero retained)")
ax.set_title("PRISM Time and Cost occupy complementary frontier regions", fontsize=14, color=COLORS["ink"], pad=12)
ax.legend(loc="upper center", ncol=2)
style(ax, grid="both")
ax.annotate("Fast extreme", xy=(72.25, 0.1576), xytext=(115, 0.11), arrowprops=dict(arrowstyle="->", color=COLORS["prism-time"]), color=COLORS["prism-time"], fontweight="bold")
ax.annotate("Zero-cost extreme", xy=(4755, 0), xytext=(1100, 0.006), arrowprops=dict(arrowstyle="->", color=COLORS["prism-cost"]), color=COLORS["prism-cost"], fontweight="bold")
save(fig, "fig06-reference-frontier")


manifest = {
    "figures": [
        {"file": "fig01-overview", "purpose": "Basic summary of objective wins and SLA compliance"},
        {"file": "fig02-environment-speedup-heatmap", "purpose": "Workflow-by-environment speedup map"},
        {"file": "fig03-planned-vs-executed", "purpose": "Prediction fidelity and HEFT outliers"},
        {"file": "fig04-interference-story", "purpose": "Absolute versus relative interference response"},
        {"file": "fig05-beam-elbow", "purpose": "Beam quality-planning trade-off"},
        {"file": "fig06-reference-frontier", "purpose": "Time-cost frontier complementarity"},
    ],
    "formats": ["png", "pdf", "svg"],
}
with open(OUT / "manifest.json", "w") as f:
    json.dump(manifest, f, indent=2)
print(OUT)
