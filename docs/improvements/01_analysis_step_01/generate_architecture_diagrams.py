#!/usr/bin/env python3
"""
Generate architecture diagrams for the HelixCode Integration Project.
Outputs: overall_architecture.png, integration_phases.png,
         feature_gap_matrix.png, dependency_graph.png
"""

import os
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import matplotlib.patheffects as pe
import numpy as np

plt.rcParams['font.family'] = 'DejaVu Sans'
plt.rcParams['axes.unicode_minus'] = False

OUTPUT_DIR = '/home/z/my-project/download'
DPI = 150


# ═══════════════════════════════════════════════════════════════════
# COLOR PALETTE  (professional, cohesive, no rainbow)
# ═══════════════════════════════════════════════════════════════════
COLORS = {
    'center':       '#1B2A4A',   # dark navy (HelixCode core)
    'inner_ring':   '#2E86AB',   # steel blue  (primary repos)
    'outer_ring':   '#A23B72',   # muted plum  (supporting repos)
    'arrow_dep':    '#5C6B73',   # slate gray
    'phase0':       '#264653',
    'phase1':       '#2A9D8F',
    'phase2':       '#E9C46A',
    'phase3':       '#F4A261',
    'phase4':       '#E76F51',
    'phase5':       '#E63946',
    'heatmap_full': '#2A9D8F',
    'heatmap_part': '#E9C46A',
    'heatmap_none': '#E8E8E8',
    'node_core':    '#1B2A4A',
    'node_primary': '#2E86AB',
    'node_support': '#A23B72',
    'node_tool':    '#F4A261',
    'bg':           '#FAFAFA',
    'text_dark':    '#1A1A2E',
    'text_light':   '#FFFFFF',
}


def draw_arrow(ax, start, end, color='#5C6B73', lw=1.5, style='->', head_w=12, head_l=10, alpha=0.7):
    """Draw a curved arrow between two points."""
    ax.annotate(
        '', xy=end, xytext=start,
        arrowprops=dict(
            arrowstyle=style,
            color=color,
            lw=lw,
            connectionstyle='arc3,rad=0.08',
            alpha=alpha,
            mutation_scale=15,
        )
    )


def draw_box(ax, x, y, w, h, label, facecolor, textcolor='#FFFFFF',
             fontsize=9, edgecolor=None, alpha=0.92, bold=False, sublabel=None):
    """Draw a rounded rectangle with centered text."""
    if edgecolor is None:
        edgecolor = facecolor
    box = mpatches.FancyBboxPatch(
        (x - w / 2, y - h / 2), w, h,
        boxstyle="round,pad=0.12",
        facecolor=facecolor, edgecolor=edgecolor,
        alpha=alpha, linewidth=1.5,
        zorder=3
    )
    ax.add_patch(box)
    weight = 'bold' if bold else 'normal'
    if sublabel:
        ax.text(x, y + 0.12, label, ha='center', va='center',
                fontsize=fontsize, color=textcolor, fontweight=weight, zorder=4)
        ax.text(x, y - 0.15, sublabel, ha='center', va='center',
                fontsize=fontsize - 2, color=textcolor, alpha=0.85, zorder=4)
    else:
        ax.text(x, y, label, ha='center', va='center',
                fontsize=fontsize, color=textcolor, fontweight=weight, zorder=4)
    return (x, y, w, h)


# ═══════════════════════════════════════════════════════════════════
# 1.  OVERALL ARCHITECTURE  (hub-and-spoke)
# ═══════════════════════════════════════════════════════════════════
def generate_overall_architecture():
    fig, ax = plt.subplots(figsize=(16, 12))
    fig.patch.set_facecolor(COLORS['bg'])
    ax.set_facecolor(COLORS['bg'])
    ax.set_xlim(-7.5, 7.5)
    ax.set_ylim(-6, 6)
    ax.set_aspect('equal')
    ax.axis('off')

    # Title
    ax.text(0, 5.5, 'HelixCode Integration — Overall Architecture',
            ha='center', va='center', fontsize=18, fontweight='bold',
            color=COLORS['text_dark'])
    ax.text(0, 5.0, 'Hub-and-Spoke Dependency Layout',
            ha='center', va='center', fontsize=11, color='#666666')

    # Center: HelixCode Core
    draw_box(ax, 0, 0, 3.2, 1.3, 'HelixCode', COLORS['center'],
             fontsize=16, bold=True, sublabel='Integration Core')

    # Inner ring – Primary repos (radius ~3.2)
    inner_items = [
        ('HelixAgent',  'Agent Runtime'),
        ('HelixML',     'ML Pipeline'),
        ('HelixOps',    'DevOps / CI-CD'),
        ('HelixSDK',    'SDK / API Layer'),
        ('HelixUI',     'Web Dashboard'),
        ('HelixDB',     'Data Store'),
    ]
    inner_radius = 3.2
    inner_positions = {}
    n_inner = len(inner_items)
    for i, (name, sub) in enumerate(inner_items):
        angle = np.pi / 2 + 2 * np.pi * i / n_inner
        x = inner_radius * np.cos(angle)
        y = inner_radius * np.sin(angle)
        draw_box(ax, x, y, 2.0, 0.9, name, COLORS['inner_ring'],
                 fontsize=10, bold=True, sublabel=sub)
        inner_positions[name] = (x, y)

    # Outer ring – Supporting repos (radius ~5.5)
    outer_items = [
        ('HelixCLI',    'CLI Interface'),
        ('HelixTest',   'Test Framework'),
        ('HelixConfig', 'Config Mgmt'),
        ('HelixAuth',   'Auth / Identity'),
        ('HelixMonitor','Monitoring'),
        ('HelixDocs',   'Documentation'),
        ('HelixProto',  'Protobuf Schemas'),
        ('HelixCache',  'Cache Layer'),
    ]
    outer_radius = 5.5
    outer_positions = {}
    n_outer = len(outer_items)
    for i, (name, sub) in enumerate(outer_items):
        angle = 2 * np.pi * i / n_outer + np.pi / n_outer  # offset
        x = outer_radius * np.cos(angle)
        y = outer_radius * np.sin(angle)
        # Keep in view
        x = np.clip(x, -6.5, 6.5)
        y = np.clip(y, -5.2, 5.2)
        draw_box(ax, x, y, 1.8, 0.8, name, COLORS['outer_ring'],
                 fontsize=9, bold=True, sublabel=sub)
        outer_positions[name] = (x, y)

    # Arrows: Center -> Inner ring
    for name, (ix, iy) in inner_positions.items():
        # from center edge to inner box edge (approximate)
        dx, dy = ix, iy
        length = np.sqrt(dx**2 + dy**2)
        ux, uy = dx / length, dy / length
        sx, sy = ux * 1.6, uy * 0.65
        ex, ey = ix - ux * 1.0, iy - uy * 0.45
        draw_arrow(ax, (sx, sy), (ex, ey), color=COLORS['arrow_dep'], lw=2.0, alpha=0.6)

    # Arrows: Inner -> Outer (selected dependencies)
    dep_links = [
        ('HelixAgent', 'HelixCLI'),
        ('HelixAgent', 'HelixAuth'),
        ('HelixML',    'HelixProto'),
        ('HelixML',    'HelixCache'),
        ('HelixOps',   'HelixMonitor'),
        ('HelixOps',   'HelixTest'),
        ('HelixSDK',   'HelixDocs'),
        ('HelixSDK',   'HelixConfig'),
        ('HelixUI',    'HelixAuth'),
        ('HelixDB',    'HelixCache'),
        ('HelixDB',    'HelixProto'),
        ('HelixUI',    'HelixMonitor'),
    ]
    for src, dst in dep_links:
        sx, sy = inner_positions[src]
        dx_, dy_ = outer_positions[dst]
        ddx, ddy = dx_ - sx, dy_ - sy
        length = np.sqrt(ddx**2 + ddy**2)
        ux, uy = ddx / length, ddy / length
        start = (sx + ux * 1.05, sy + uy * 0.45)
        end   = (dx_ - ux * 0.95, dy_ - uy * 0.42)
        draw_arrow(ax, start, end, color=COLORS['arrow_dep'], lw=1.2, alpha=0.45)

    # Legend
    legend_items = [
        mpatches.Patch(facecolor=COLORS['center'],     label='Integration Core'),
        mpatches.Patch(facecolor=COLORS['inner_ring'],  label='Primary Repositories'),
        mpatches.Patch(facecolor=COLORS['outer_ring'],  label='Supporting Repositories'),
    ]
    ax.legend(handles=legend_items, loc='lower right', fontsize=9,
              framealpha=0.9, edgecolor='#CCCCCC')

    fig.tight_layout()
    path = os.path.join(OUTPUT_DIR, 'overall_architecture.png')
    fig.savefig(path, dpi=DPI, bbox_inches='tight', facecolor=fig.get_facecolor())
    plt.close(fig)
    print(f'[OK] {path}')


# ═══════════════════════════════════════════════════════════════════
# 2.  INTEGRATION PHASES  (Gantt-chart style)
# ═══════════════════════════════════════════════════════════════════
def generate_integration_phases():
    phases = [
        {'name': 'Phase 0', 'label': 'Discovery & Audit',
         'start': 0, 'end': 3, 'color': COLORS['phase0'],
         'deliverables': 'Repo inventory, gap analysis, tech debt report'},
        {'name': 'Phase 1', 'label': 'Foundation & Scaffolding',
         'start': 3, 'end': 7, 'color': COLORS['phase1'],
         'deliverables': 'Monorepo scaffold, CI pipelines, shared configs'},
        {'name': 'Phase 2', 'label': 'Core Integration',
         'start': 7, 'end': 14, 'color': COLORS['phase2'],
         'deliverables': 'Agent runtime, ML pipeline, SDK v1'},
        {'name': 'Phase 3', 'label': 'Feature Parity & Migration',
         'start': 14, 'end': 20, 'color': COLORS['phase3'],
         'deliverables': 'CLI parity, auth migration, DB migration'},
        {'name': 'Phase 4', 'label': 'Hardening & Optimization',
         'start': 20, 'end': 24, 'color': COLORS['phase4'],
         'deliverables': 'Perf benchmarks, chaos tests, security audit'},
        {'name': 'Phase 5', 'label': 'Launch & Handoff',
         'start': 24, 'end': 26, 'color': COLORS['phase5'],
         'deliverables': 'Go-live, runbooks, team training'},
    ]

    n = len(phases)
    fig, ax = plt.subplots(figsize=(16, 8))
    fig.patch.set_facecolor(COLORS['bg'])
    ax.set_facecolor(COLORS['bg'])

    bar_height = 0.6
    y_positions = np.arange(n)[::-1]  # top-to-bottom

    for i, phase in enumerate(phases):
        y = y_positions[i]
        duration = phase['end'] - phase['start']
        # Main bar
        bar = ax.barh(y, duration, left=phase['start'], height=bar_height,
                       color=phase['color'], edgecolor='white', linewidth=1.2,
                       alpha=0.92, zorder=3)
        # Phase label inside bar
        mid = phase['start'] + duration / 2
        ax.text(mid, y, f"{phase['name']}: {phase['label']}",
                ha='center', va='center', fontsize=10, fontweight='bold',
                color='#FFFFFF', zorder=4)
        # Week range
        ax.text(phase['end'] + 0.3, y + 0.15,
                f"Wk {phase['start']}-{phase['end']}",
                ha='left', va='center', fontsize=8, color='#555555', zorder=4)
        # Deliverables
        ax.text(phase['end'] + 0.3, y - 0.15,
                phase['deliverables'],
                ha='left', va='center', fontsize=7.5, color='#888888',
                fontstyle='italic', zorder=4)

    # Milestone diamonds
    milestones = [
        (3,  'Audit\nComplete'),
        (7,  'Scaffold\nReady'),
        (14, 'Core\nIntegrated'),
        (20, 'Parity\nReached'),
        (24, 'Hardened'),
        (26, 'Go-Live'),
    ]
    for mx, mlabel in milestones:
        ax.plot(mx, -1.0, marker='D', markersize=10, color=COLORS['center'],
                zorder=5, markeredgecolor='white', markeredgewidth=1.2)
        ax.text(mx, -1.55, mlabel, ha='center', va='top', fontsize=7,
                color=COLORS['text_dark'], fontweight='bold')

    # Dashed line for "now" marker (example at week 10)
    ax.axvline(x=10, color='#E63946', linewidth=1.2, linestyle='--', alpha=0.5, zorder=2)
    ax.text(10, n - 0.2, 'Current\n(Wk 10)', ha='center', va='bottom',
            fontsize=7.5, color='#E63946', fontweight='bold')

    ax.set_xlim(-1, 34)
    ax.set_ylim(-2.2, n + 0.3)
    ax.set_xlabel('Project Weeks', fontsize=12, color=COLORS['text_dark'])
    ax.set_yticks([])
    ax.set_xticks(range(0, 28, 2))
    ax.set_xticklabels([f'Wk {w}' for w in range(0, 28, 2)], fontsize=8)
    ax.spines['top'].set_visible(False)
    ax.spines['right'].set_visible(False)
    ax.spines['left'].set_visible(False)
    ax.grid(axis='x', alpha=0.2, linestyle='--')

    ax.set_title('HelixCode Integration — Phased Timeline',
                 fontsize=16, fontweight='bold', color=COLORS['text_dark'], pad=20)

    fig.tight_layout()
    path = os.path.join(OUTPUT_DIR, 'integration_phases.png')
    fig.savefig(path, dpi=DPI, bbox_inches='tight', facecolor=fig.get_facecolor())
    plt.close(fig)
    print(f'[OK] {path}')


# ═══════════════════════════════════════════════════════════════════
# 3.  FEATURE GAP MATRIX  (heatmap style)
# ═══════════════════════════════════════════════════════════════════
def generate_feature_gap_matrix():
    # Feature support: 2 = Full, 1 = Partial, 0 = None
    agents = [
        'HelixAgent',
        'HelixML',
        'HelixOps',
        'HelixSDK',
        'HelixUI',
        'HelixDB',
        'HelixCLI',
        'HelixTest',
    ]
    features = [
        'Multi-LLM Support',
        'Tool Calling',
        'RAG Pipeline',
        'Streaming Output',
        'Auth / RBAC',
        'CLI Interface',
        'API Gateway',
        'Observability',
        'Hot-Reload',
        'Plugin System',
        'Batch Mode',
        'Cost Tracking',
    ]

    # Data matrix  (features x agents)
    data = np.array([
        [2, 2, 0, 1, 0, 0, 1, 0],   # Multi-LLM
        [2, 1, 0, 2, 0, 0, 2, 0],   # Tool Calling
        [1, 2, 0, 1, 0, 1, 0, 0],   # RAG Pipeline
        [2, 1, 0, 2, 1, 0, 2, 0],   # Streaming
        [1, 0, 1, 2, 2, 1, 1, 0],   # Auth / RBAC
        [2, 0, 1, 1, 0, 0, 2, 1],   # CLI Interface
        [1, 1, 2, 2, 2, 2, 0, 1],   # API Gateway
        [1, 1, 2, 1, 1, 1, 0, 2],   # Observability
        [2, 1, 1, 0, 0, 0, 1, 2],   # Hot-Reload
        [2, 0, 0, 1, 1, 0, 2, 0],   # Plugin System
        [1, 2, 1, 1, 0, 1, 2, 0],   # Batch Mode
        [1, 0, 0, 1, 2, 0, 1, 0],   # Cost Tracking
    ])

    fig, ax = plt.subplots(figsize=(14, 9))
    fig.patch.set_facecolor(COLORS['bg'])
    ax.set_facecolor(COLORS['bg'])

    # Custom colormap: None -> light gray, Partial -> gold, Full -> teal
    from matplotlib.colors import ListedColormap
    cmap = ListedColormap([COLORS['heatmap_none'], COLORS['heatmap_part'], COLORS['heatmap_full']])
    bounds = [-0.5, 0.5, 1.5, 2.5]
    norm = plt.matplotlib.colors.BoundaryNorm(bounds, cmap.N)

    im = ax.imshow(data, cmap=cmap, norm=norm, aspect='auto')

    # Cell labels
    label_map = {2: 'Full', 1: 'Partial', 0: '-'}
    text_color_map = {2: '#FFFFFF', 1: '#1A1A2E', 0: '#AAAAAA'}
    for i in range(len(features)):
        for j in range(len(agents)):
            val = data[i, j]
            ax.text(j, i, label_map[val], ha='center', va='center',
                    fontsize=8, fontweight='bold', color=text_color_map[val])

    ax.set_xticks(range(len(agents)))
    ax.set_xticklabels(agents, rotation=35, ha='right', fontsize=9)
    ax.set_yticks(range(len(features)))
    ax.set_yticklabels(features, fontsize=9)
    ax.tick_params(axis='both', length=0)

    # Grid lines
    for i in range(len(features) + 1):
        ax.axhline(i - 0.5, color='white', linewidth=2)
    for j in range(len(agents) + 1):
        ax.axvline(j - 0.5, color='white', linewidth=2)

    ax.set_title('HelixCode Integration — Feature Gap Matrix',
                 fontsize=16, fontweight='bold', color=COLORS['text_dark'], pad=20)

    # Colorbar / Legend
    legend_patches = [
        mpatches.Patch(facecolor=COLORS['heatmap_full'],  label='Full Support'),
        mpatches.Patch(facecolor=COLORS['heatmap_part'],  label='Partial Support'),
        mpatches.Patch(facecolor=COLORS['heatmap_none'],  label='Not Available'),
    ]
    ax.legend(handles=legend_patches, loc='upper right', fontsize=9,
              framealpha=0.9, edgecolor='#CCCCCC',
              bbox_to_anchor=(1.0, -0.02))

    fig.tight_layout()
    path = os.path.join(OUTPUT_DIR, 'feature_gap_matrix.png')
    fig.savefig(path, dpi=DPI, bbox_inches='tight', facecolor=fig.get_facecolor())
    plt.close(fig)
    print(f'[OK] {path}')


# ═══════════════════════════════════════════════════════════════════
# 4.  DEPENDENCY GRAPH  (topological order, DAG)
# ═══════════════════════════════════════════════════════════════════
def generate_dependency_graph():
    # Nodes with (x, y) positions arranged in topological layers
    # Layer 0: no dependencies, Layer 1: depends on layer 0, etc.
    nodes = {
        # Layer 0 — foundations
        'HelixProto':   (1.5, 8.5),
        'HelixConfig':  (4.5, 8.5),
        'HelixDocs':    (7.5, 8.5),
        # Layer 1 — core infra
        'HelixAuth':    (0.5, 6.5),
        'HelixDB':      (3.5, 6.5),
        'HelixCache':   (6.5, 6.5),
        'HelixTest':    (9.5, 6.5),
        # Layer 2 — services
        'HelixSDK':     (2.0, 4.5),
        'HelixMonitor': (5.5, 4.5),
        'HelixOps':     (8.5, 4.5),
        # Layer 3 — applications
        'HelixAgent':   (1.5, 2.5),
        'HelixML':      (5.0, 2.5),
        'HelixCLI':     (8.5, 2.5),
        # Layer 4 — integration core
        'HelixUI':      (3.5, 0.5),
        'HelixCode':    (6.5, 0.5),
    }

    # Category coloring
    node_categories = {
        'HelixProto': 'foundation', 'HelixConfig': 'foundation', 'HelixDocs': 'foundation',
        'HelixAuth': 'infra', 'HelixDB': 'infra', 'HelixCache': 'infra', 'HelixTest': 'infra',
        'HelixSDK': 'service', 'HelixMonitor': 'service', 'HelixOps': 'service',
        'HelixAgent': 'app', 'HelixML': 'app', 'HelixCLI': 'app',
        'HelixUI': 'integration', 'HelixCode': 'integration',
    }
    cat_colors = {
        'foundation':  '#264653',
        'infra':       '#2A9D8F',
        'service':     '#2E86AB',
        'app':         '#E76F51',
        'integration': '#1B2A4A',
    }

    # Edges: (source, target)  source -> target means "target depends on source"
    edges = [
        ('HelixProto', 'HelixDB'),
        ('HelixProto', 'HelixAuth'),
        ('HelixProto', 'HelixSDK'),
        ('HelixConfig', 'HelixSDK'),
        ('HelixConfig', 'HelixOps'),
        ('HelixConfig', 'HelixMonitor'),
        ('HelixDocs', 'HelixOps'),
        ('HelixAuth', 'HelixSDK'),
        ('HelixAuth', 'HelixAgent'),
        ('HelixDB', 'HelixSDK'),
        ('HelixDB', 'HelixCache'),
        ('HelixDB', 'HelixML'),
        ('HelixCache', 'HelixSDK'),
        ('HelixCache', 'HelixML'),
        ('HelixTest', 'HelixOps'),
        ('HelixSDK', 'HelixAgent'),
        ('HelixSDK', 'HelixML'),
        ('HelixSDK', 'HelixCLI'),
        ('HelixMonitor', 'HelixOps'),
        ('HelixOps', 'HelixCLI'),
        ('HelixAgent', 'HelixCode'),
        ('HelixML', 'HelixCode'),
        ('HelixCLI', 'HelixUI'),
        ('HelixAgent', 'HelixUI'),
        ('HelixSDK', 'HelixUI'),
        ('HelixOps', 'HelixCode'),
    ]

    fig, ax = plt.subplots(figsize=(14, 11))
    fig.patch.set_facecolor(COLORS['bg'])
    ax.set_facecolor(COLORS['bg'])
    ax.set_xlim(-0.5, 11)
    ax.set_ylim(-0.5, 10)
    ax.axis('off')

    # Title
    ax.text(5.25, 9.7, 'HelixCode Integration — Module Dependency Graph',
            ha='center', va='center', fontsize=16, fontweight='bold',
            color=COLORS['text_dark'])
    ax.text(5.25, 9.25, 'Topological dependency order (bottom = highest-level consumer)',
            ha='center', va='center', fontsize=10, color='#666666')

    # Draw edges first (behind nodes)
    for src, dst in edges:
        sx, sy = nodes[src]
        dx_, dy_ = nodes[dst]
        # Shorten arrows to not overlap boxes
        ddx, ddy = dx_ - sx, dy_ - sy
        length = np.sqrt(ddx**2 + ddy**2)
        if length == 0:
            continue
        ux, uy = ddx / length, ddy / length
        # Start from edge of source box, end at edge of dest box
        start_x = sx + ux * 0.85
        start_y = sy + uy * 0.35
        end_x = dx_ - ux * 0.85
        end_y = dy_ - uy * 0.35

        ax.annotate(
            '', xy=(end_x, end_y), xytext=(start_x, start_y),
            arrowprops=dict(
                arrowstyle='->', color='#888888', lw=1.3,
                connectionstyle='arc3,rad=0.05', alpha=0.55,
                mutation_scale=12,
            ),
            zorder=1
        )

    # Draw nodes
    for name, (x, y) in nodes.items():
        cat = node_categories[name]
        fc = cat_colors[cat]
        bw, bh = 1.5, 0.55
        box = mpatches.FancyBboxPatch(
            (x - bw / 2, y - bh / 2), bw, bh,
            boxstyle="round,pad=0.08",
            facecolor=fc, edgecolor='white', linewidth=1.5,
            alpha=0.92, zorder=3
        )
        ax.add_patch(box)
        ax.text(x, y, name, ha='center', va='center',
                fontsize=8.5, fontweight='bold', color='#FFFFFF', zorder=4)

    # Layer labels
    layer_labels = [
        (9.3, 'Layer 0: Foundations'),
        (7.3, 'Layer 1: Infrastructure'),
        (5.3, 'Layer 2: Services'),
        (3.3, 'Layer 3: Applications'),
        (1.3, 'Layer 4: Integration'),
    ]
    for ly, label in layer_labels:
        ax.text(-0.3, ly, label, ha='left', va='center',
                fontsize=8, color='#888888', fontstyle='italic')

    # Horizontal separators
    for sep_y in [7.5, 5.5, 3.5, 1.5]:
        ax.axhline(sep_y, color='#DDDDDD', linewidth=0.8, linestyle=':', zorder=0)

    # Legend
    legend_patches = [
        mpatches.Patch(facecolor=cat_colors['foundation'],  label='Foundation Layer'),
        mpatches.Patch(facecolor=cat_colors['infra'],       label='Infrastructure Layer'),
        mpatches.Patch(facecolor=cat_colors['service'],     label='Service Layer'),
        mpatches.Patch(facecolor=cat_colors['app'],         label='Application Layer'),
        mpatches.Patch(facecolor=cat_colors['integration'], label='Integration Layer'),
    ]
    ax.legend(handles=legend_patches, loc='lower right', fontsize=8,
              framealpha=0.9, edgecolor='#CCCCCC')

    fig.tight_layout()
    path = os.path.join(OUTPUT_DIR, 'dependency_graph.png')
    fig.savefig(path, dpi=DPI, bbox_inches='tight', facecolor=fig.get_facecolor())
    plt.close(fig)
    print(f'[OK] {path}')


# ═══════════════════════════════════════════════════════════════════
# MAIN
# ═══════════════════════════════════════════════════════════════════
if __name__ == '__main__':
    os.makedirs(OUTPUT_DIR, exist_ok=True)
    print('Generating architecture diagrams...')
    generate_overall_architecture()
    generate_integration_phases()
    generate_feature_gap_matrix()
    generate_dependency_graph()
    print('\nAll diagrams generated successfully!')
