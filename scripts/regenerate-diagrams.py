#!/usr/bin/env python3
"""scripts/regenerate-diagrams.py

Reads docs/improvements/canonical/topology.yaml, emits four PNGs to
docs/improvements/06_diagrams_real/. Per Phase 0 P0-16.

Dependencies: matplotlib, pyyaml, networkx. Install with:
    pip install --user matplotlib pyyaml networkx
"""
from __future__ import annotations
import sys
import os
from pathlib import Path

# Use non-interactive backend so this script works headlessly (no display required)
os.environ.setdefault("MPLBACKEND", "Agg")

try:
    import yaml
    import matplotlib.pyplot as plt
    import matplotlib.patches as mpatches
    import networkx as nx
except ImportError as e:
    print(f"ERROR: missing dependency — {e}", file=sys.stderr)
    print("Install with: pip install --user matplotlib pyyaml networkx", file=sys.stderr)
    sys.exit(2)


REPO_ROOT = Path(__file__).resolve().parent.parent
TOPOLOGY = REPO_ROOT / "docs/improvements/canonical/topology.yaml"
OUT_DIR = REPO_ROOT / "docs/improvements/06_diagrams_real"


def load_topology():
    with TOPOLOGY.open() as f:
        return yaml.safe_load(f)


def emit_overall_architecture(t, out: Path):
    fig, ax = plt.subplots(figsize=(12, 9))
    ax.set_axis_off()
    ax.set_title("HelixCode — Overall Architecture (Real Submodule Topology)",
                 fontsize=14, weight="bold")

    # Hub
    ax.add_patch(mpatches.FancyBboxPatch((0.42, 0.42), 0.16, 0.16,
                                          boxstyle="round,pad=0.02",
                                          linewidth=2, edgecolor="navy",
                                          facecolor="lightblue"))
    ax.text(0.5, 0.5, "HelixCode\n(meta-repo)", ha="center", va="center",
            fontsize=11, weight="bold")

    # Substrate
    ax.add_patch(mpatches.FancyBboxPatch((0.30, 0.65), 0.18, 0.10,
                                          boxstyle="round,pad=0.01",
                                          facecolor="khaki"))
    ax.text(0.39, 0.70, "HelixAgent\n(substrate)", ha="center", va="center",
            fontsize=9)

    libs = [(0.10, 0.75, "HelixLLM"),
            (0.10, 0.55, "HelixMemory"),
            (0.10, 0.35, "HelixSpecifier"),
            (0.10, 0.15, "LLMsVerifier")]
    for x, y, name in libs:
        ax.add_patch(mpatches.FancyBboxPatch((x, y), 0.16, 0.08,
                                              boxstyle="round,pad=0.01",
                                              facecolor="lightgreen"))
        ax.text(x + 0.08, y + 0.04, name, ha="center", va="center", fontsize=9)

    apps = [(0.74, 0.75, "HelixQA"),
            (0.74, 0.55, "Challenges"),
            (0.74, 0.35, "Containers"),
            (0.74, 0.15, "Security")]
    for x, y, name in apps:
        ax.add_patch(mpatches.FancyBboxPatch((x, y), 0.16, 0.08,
                                              boxstyle="round,pad=0.01",
                                              facecolor="lightcoral"))
        ax.text(x + 0.08, y + 0.04, name, ha="center", va="center", fontsize=9)

    ax.add_patch(mpatches.FancyBboxPatch((0.30, 0.05), 0.40, 0.08,
                                          boxstyle="round,pad=0.01",
                                          facecolor="lavender"))
    ax.text(0.50, 0.09,
            f"cli_agents/  ({t['modules']['cli_agent_count']} agents — canonical source; 47 populated, 13 Phase-2-deferred)",
            ha="center", va="center", fontsize=8, style="italic")

    plt.tight_layout()
    plt.savefig(out, dpi=120, bbox_inches="tight")
    plt.close()
    print(f"OK: {out}")


def emit_dependency_graph(t, out: Path):
    fig, ax = plt.subplots(figsize=(12, 8))
    ax.set_axis_off()
    ax.set_title("HelixCode — Module Dependency Graph (Real)",
                 fontsize=14, weight="bold")

    G = nx.DiGraph()
    G.add_node("HelixCode (meta)")
    G.add_node("HelixCode (Go app)")
    G.add_node("HelixAgent")
    for lib in ["HelixLLM", "HelixMemory", "HelixSpecifier", "LLMsVerifier"]:
        G.add_node(lib)
        G.add_edge("HelixAgent", lib)
        G.add_edge("HelixCode (Go app)", lib)
    G.add_edge("HelixCode (meta)", "HelixCode (Go app)")
    G.add_edge("HelixCode (meta)", "HelixAgent")
    for app in ["HelixQA", "Challenges", "Containers", "Security"]:
        G.add_node(app)
        G.add_edge("HelixCode (meta)", app)

    pos = nx.spring_layout(G, seed=7)
    nx.draw(G, pos, ax=ax, with_labels=True, node_color="lightblue",
            node_size=2000, font_size=8, arrows=True)

    plt.tight_layout()
    plt.savefig(out, dpi=120, bbox_inches="tight")
    plt.close()
    print(f"OK: {out}")


def emit_feature_gap_matrix(t, out: Path):
    fig, ax = plt.subplots(figsize=(14, 10))
    features = t["features"]
    modules = ["HelixCode", "HelixAgent", "HelixLLM", "HelixMemory",
               "HelixSpecifier", "LLMsVerifier", "HelixQA", "Challenges"]

    ax.set_xticks(range(len(modules)))
    ax.set_yticks(range(len(features)))
    ax.set_xticklabels(modules, rotation=30, ha="right")
    ax.set_yticklabels(features)
    ax.set_title("HelixCode — Feature Gap Matrix (TBP — Phase 4 will populate from runtime evidence)",
                 fontsize=12, weight="bold")
    ax.set_xlim(-0.5, len(modules) - 0.5)
    ax.set_ylim(-0.5, len(features) - 0.5)
    ax.grid(True, alpha=0.3)

    for i in range(len(features)):
        for j in range(len(modules)):
            ax.text(j, i, "?", ha="center", va="center", fontsize=10,
                    color="gray")

    plt.tight_layout()
    plt.savefig(out, dpi=120, bbox_inches="tight")
    plt.close()
    print(f"OK: {out}")


def emit_integration_phases(t, out: Path):
    fig, ax = plt.subplots(figsize=(14, 6))
    phases = t["phases"]
    colors = {"active": "khaki", "pending": "lightgray",
              "pending (parallelisable)": "lightyellow", "done": "lightgreen"}

    for i, p in enumerate(phases):
        weeks = p["weeks"].split("-")
        start, end = int(weeks[0]), int(weeks[1])
        ax.barh(p["id"], end - start, left=start,
                color=colors.get(p["state"], "white"), edgecolor="black")
        ax.text((start + end) / 2, p["id"], p["name"],
                ha="center", va="center", fontsize=9)

    ax.set_xlabel("Project Weeks")
    ax.set_title("HelixCode — Integration Phases Timeline (Real)",
                 fontsize=14, weight="bold")
    ax.invert_yaxis()
    plt.tight_layout()
    plt.savefig(out, dpi=120, bbox_inches="tight")
    plt.close()
    print(f"OK: {out}")


def main() -> int:
    t = load_topology()
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    emit_overall_architecture(t, OUT_DIR / "overall_architecture.png")
    emit_dependency_graph(t, OUT_DIR / "dependency_graph.png")
    emit_feature_gap_matrix(t, OUT_DIR / "feature_gap_matrix.png")
    emit_integration_phases(t, OUT_DIR / "integration_phases.png")
    return 0


if __name__ == "__main__":
    sys.exit(main())
