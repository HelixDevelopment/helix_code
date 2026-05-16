#!/usr/bin/env python3
"""
HelixCode Comprehensive Integration Technical Documentation Generator
Generates a professional PDF with all analysis, plans, and documentation
"""

import os
from reportlab.lib import colors
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.enums import TA_LEFT, TA_CENTER, TA_JUSTIFY
from reportlab.lib.units import inch, cm
from reportlab.platypus import (
    Paragraph, Spacer, Table, TableStyle, PageBreak, KeepTogether, CondPageBreak
)
from reportlab.platypus.tableofcontents import TableOfContents
from reportlab.platypus import SimpleDocTemplate
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.pdfbase.pdfmetrics import registerFontFamily
import hashlib

# ━━ Color Palette ━━
ACCENT       = colors.HexColor('#a74c2e')
TEXT_PRIMARY  = colors.HexColor('#21201e')
TEXT_MUTED    = colors.HexColor('#817c75')
BG_SURFACE   = colors.HexColor('#e0dcd7')
BG_PAGE      = colors.HexColor('#efeeeb')
TABLE_HEADER_COLOR = ACCENT
TABLE_HEADER_TEXT  = colors.white
TABLE_ROW_EVEN     = colors.white
TABLE_ROW_ODD      = BG_SURFACE

# ━━ Font Registration ━━
pdfmetrics.registerFont(TTFont('Carlito', '/usr/share/fonts/truetype/english/Carlito-Regular.ttf'))
pdfmetrics.registerFont(TTFont('Carlito-Bold', '/usr/share/fonts/truetype/english/Carlito-Bold.ttf'))
pdfmetrics.registerFont(TTFont('DejaVuSans', '/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf'))
pdfmetrics.registerFont(TTFont('DejaVuSans-Bold', '/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf'))
registerFontFamily('Carlito', normal='Carlito', bold='Carlito-Bold')
registerFontFamily('DejaVuSans', normal='DejaVuSans', bold='DejaVuSans-Bold')

# ━━ Page Setup ━━
PAGE_W, PAGE_H = A4
LEFT_MARGIN = 1.0 * inch
RIGHT_MARGIN = 1.0 * inch
TOP_MARGIN = 0.8 * inch
BOTTOM_MARGIN = 0.8 * inch
AVAILABLE_WIDTH = PAGE_W - LEFT_MARGIN - RIGHT_MARGIN

# ━━ Styles ━━
styles = getSampleStyleSheet()

title_style = ParagraphStyle(
    'DocTitle', fontName='Carlito', fontSize=28, leading=36,
    alignment=TA_CENTER, textColor=ACCENT, spaceAfter=12
)
h1_style = ParagraphStyle(
    'H1', fontName='Carlito', fontSize=20, leading=28,
    textColor=ACCENT, spaceBefore=18, spaceAfter=10
)
h2_style = ParagraphStyle(
    'H2', fontName='Carlito', fontSize=16, leading=22,
    textColor=TEXT_PRIMARY, spaceBefore=14, spaceAfter=8
)
h3_style = ParagraphStyle(
    'H3', fontName='Carlito', fontSize=13, leading=18,
    textColor=TEXT_PRIMARY, spaceBefore=10, spaceAfter=6
)
body_style = ParagraphStyle(
    'Body', fontName='Carlito', fontSize=10.5, leading=17,
    alignment=TA_JUSTIFY, textColor=TEXT_PRIMARY, spaceAfter=6
)
body_left = ParagraphStyle(
    'BodyLeft', fontName='Carlito', fontSize=10.5, leading=17,
    alignment=TA_LEFT, textColor=TEXT_PRIMARY, spaceAfter=4
)
bullet_style = ParagraphStyle(
    'Bullet', fontName='Carlito', fontSize=10.5, leading=17,
    alignment=TA_LEFT, textColor=TEXT_PRIMARY, spaceAfter=3,
    leftIndent=18, bulletIndent=6
)
code_style = ParagraphStyle(
    'Code', fontName='DejaVuSans', fontSize=9, leading=14,
    alignment=TA_LEFT, textColor=TEXT_PRIMARY, spaceAfter=4,
    leftIndent=12, backColor=BG_SURFACE
)
caption_style = ParagraphStyle(
    'Caption', fontName='Carlito', fontSize=9.5, leading=14,
    alignment=TA_CENTER, textColor=TEXT_MUTED, spaceBefore=3, spaceAfter=6
)
toc_h1 = ParagraphStyle('TOCH1', fontName='Carlito', fontSize=13, leftIndent=20)
toc_h2 = ParagraphStyle('TOCH2', fontName='Carlito', fontSize=11, leftIndent=40)

# Table styles
header_p_style = ParagraphStyle('TH', fontName='Carlito', fontSize=10, leading=14, textColor=colors.white, alignment=TA_CENTER)
cell_p_style = ParagraphStyle('TC', fontName='Carlito', fontSize=9.5, leading=14, textColor=TEXT_PRIMARY, alignment=TA_LEFT)
cell_center = ParagraphStyle('TCC', fontName='Carlito', fontSize=9.5, leading=14, textColor=TEXT_PRIMARY, alignment=TA_CENTER)

def make_table(data, col_ratios=None):
    """Create a professionally styled table."""
    if col_ratios:
        col_widths = [r * AVAILABLE_WIDTH for r in col_ratios]
    else:
        col_widths = None
    t = Table(data, colWidths=col_widths, hAlign='CENTER')
    style_cmds = [
        ('BACKGROUND', (0, 0), (-1, 0), TABLE_HEADER_COLOR),
        ('TEXTCOLOR', (0, 0), (-1, 0), TABLE_HEADER_TEXT),
        ('GRID', (0, 0), (-1, -1), 0.5, TEXT_MUTED),
        ('VALIGN', (0, 0), (-1, -1), 'MIDDLE'),
        ('LEFTPADDING', (0, 0), (-1, -1), 6),
        ('RIGHTPADDING', (0, 0), (-1, -1), 6),
        ('TOPPADDING', (0, 0), (-1, -1), 5),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 5),
    ]
    for i in range(1, len(data)):
        bg = TABLE_ROW_EVEN if i % 2 == 1 else TABLE_ROW_ODD
        style_cmds.append(('BACKGROUND', (0, i), (-1, i), bg))
    t.setStyle(TableStyle(style_cmds))
    return t

def P(text, style=body_style):
    return Paragraph(text, style)

def H1(text):
    p = Paragraph(f'<b>{text}</b>', h1_style)
    p.bookmark_name = text
    p.bookmark_level = 0
    p.bookmark_text = text
    key = 'h1_' + hashlib.md5(text.encode()).hexdigest()[:8]
    p.bookmark_key = key
    return [CondPageBreak(AVAILABLE_WIDTH * 0.15), Paragraph(f'<a name="{key}"/><b>{text}</b>', h1_style)]

def H2(text):
    p = Paragraph(f'<b>{text}</b>', h2_style)
    p.bookmark_name = text
    p.bookmark_level = 1
    p.bookmark_text = text
    key = 'h2_' + hashlib.md5(text.encode()).hexdigest()[:8]
    p.bookmark_key = key
    return [Paragraph(f'<a name="{key}"/><b>{text}</b>', h2_style)]

def H3(text):
    return [Paragraph(f'<b>{text}</b>', h3_style)]

def TH(text):
    return Paragraph(f'<b>{text}</b>', header_p_style)

def TC(text):
    return Paragraph(text, cell_p_style)

def TCC(text):
    return Paragraph(text, cell_center)

def bullet(text):
    return Paragraph(f'<bullet>&bull;</bullet> {text}', bullet_style)

class TocDocTemplate(SimpleDocTemplate):
    def afterFlowable(self, flowable):
        if hasattr(flowable, 'bookmark_name'):
            level = getattr(flowable, 'bookmark_level', 0)
            text = getattr(flowable, 'bookmark_text', '')
            key = getattr(flowable, 'bookmark_key', '')
            self.notify('TOCEntry', (level, text, self.page, key))

# ━━ Build Document ━━
output_path = '/home/z/my-project/download/HelixCode_Integration_Technical_Documentation.pdf'
doc = TocDocTemplate(
    output_path, pagesize=A4,
    leftMargin=LEFT_MARGIN, rightMargin=RIGHT_MARGIN,
    topMargin=TOP_MARGIN, bottomMargin=BOTTOM_MARGIN
)

story = []

# ━━ TABLE OF CONTENTS ━━
story.append(Paragraph('<b>HelixCode Comprehensive Integration Technical Documentation</b>', title_style))
story.append(Spacer(1, 12))
story.append(Paragraph('Deep Analysis, CLI Agent Porting Guide, Integration Plan, Testing Strategy, and Implementation Roadmap', ParagraphStyle('Sub', fontName='Carlito', fontSize=12, leading=18, alignment=TA_CENTER, textColor=TEXT_MUTED)))
story.append(Spacer(1, 6))
story.append(Paragraph('Version 1.0 | May 2026', ParagraphStyle('Meta', fontName='Carlito', fontSize=10, leading=14, alignment=TA_CENTER, textColor=TEXT_MUTED)))
story.append(Spacer(1, 24))

toc = TableOfContents()
toc.levelStyles = [toc_h1, toc_h2]
story.append(toc)
story.append(PageBreak())

# ═══════════════════════════════════════════════════════════
# SECTION 1: EXECUTIVE SUMMARY
# ═══════════════════════════════════════════════════════════
story.extend(H1('1. Executive Summary'))
story.append(P('This document provides an exhaustive technical analysis and integration plan for the HelixCode AI CLI agent ecosystem. It covers deep codebase analysis of all 9 primary repositories and 10 supporting submodules, a comparative feature gap analysis against 60+ CLI agent implementations, a comprehensive phased integration plan, and a rigorous testing strategy leveraging Challenges, HelixQA, and LLMsVerifier. The goal is to port every meaningful power feature, innovation, API, workaround, and optimization from all CLI agents into the HelixCode and HelixAgent codebases while ensuring complete test coverage and documentation.'))
story.append(Spacer(1, 8))
story.append(P('The HelixCode project represents a massive Go-based multi-agent AI coding system with 376 production source files, 258 test files, 87 git submodules, 30+ LLM providers, and 5 specialized agent types. However, critical gaps exist: all 87 git submodules remain uninitialized, key HelixAgent and HelixSpecifier integrations are missing, and several high-value features from CLI agents have only stub implementations. This document addresses every gap with concrete, actionable implementation tasks.'))
story.append(Spacer(1, 8))

# Key metrics table
story.extend(H2('1.1 Key Metrics'))
data = [
    [TH('Metric'), TH('Value')],
    [TC('Total Repositories Analyzed'), TC('19 (9 primary + 10 supporting)')],
    [TC('CLI Agents Analyzed'), TC('60+ (10 Tier 1, 9 Tier 2, 41 Tier 3-5)')],
    [TC('HelixCode Source Files'), TC('376 production + 258 test Go files')],
    [TC('HelixAgent Source Files'), TC('1,609 Go source files (~1M LOC)')],
    [TC('HelixLLM Source Files'), TC('100+ Go source files')],
    [TC('HelixQA Test Banks'), TC('126 test banks (~106K lines)')],
    [TC('LLMsVerifier Provider Adapters'), TC('25+ adapters, 48 CLI agent generators')],
    [TC('Features to Port'), TC('50+ unique capabilities identified')],
    [TC('Already Implemented'), TC('~15 features (30%)')],
    [TC('Remaining to Port'), TC('~35 features (70%)')],
]
story.append(make_table(data, [0.55, 0.45]))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 2: REPOSITORY INVENTORY
# ═══════════════════════════════════════════════════════════
story.extend(H1('2. Repository Inventory and Current Integration Status'))
story.append(P('This section catalogs all repositories in the Helix ecosystem, their current integration status with HelixCode, and identifies which submodules require initialization, wiring, or new integration paths.'))

story.extend(H2('2.1 Primary Repositories'))
data = [
    [TH('Repository'), TH('Go Module'), TH('Integration Status'), TH('Criticality')],
    [TC('HelixCode'), TC('dev.helix.code'), TCC('MAIN PROJECT'), TCC('CRITICAL')],
    [TC('HelixAgent'), TC('N/A (not imported)'), TCC('NOT INTEGRATED'), TCC('CRITICAL')],
    [TC('HelixLLM'), TC('N/A (not imported)'), TCC('NOT INTEGRATED'), TCC('HIGH')],
    [TC('LLMsVerifier'), TC('digital.vasic.*'), TCC('Adapter only, submodule empty'), TCC('CRITICAL')],
    [TC('HelixMemory'), TC('N/A (not imported)'), TCC('NOT INTEGRATED'), TCC('HIGH')],
    [TC('HelixSpecifier'), TC('N/A (not imported)'), TCC('NOT INTEGRATED'), TCC('MEDIUM')],
    [TC('HelixQA'), TC('digital.vasic.helixqa'), TCC('Code exists, submodule empty'), TCC('CRITICAL')],
    [TC('Challenges'), TC('digital.vasic.challenges'), TCC('Code exists, submodule empty'), TCC('HIGH')],
    [TC('Containers'), TC('digital.vasic.containers'), TCC('Code exists, submodule empty'), TCC('HIGH')],
]
story.append(make_table(data, [0.18, 0.22, 0.30, 0.15]))
story.append(Spacer(1, 12))

story.extend(H2('2.2 Supporting Submodules'))
data = [
    [TH('Repository'), TH('Purpose'), TH('Integration Status')],
    [TC('Agentic'), TC('Multi-agent orchestration framework'), TCC('Not integrated')],
    [TC('TOON'), TC('Task orchestration and optimization network'), TCC('Not integrated (placeholder)')],
    [TC('ToolSchema'), TC('Tool schema definitions and validation'), TCC('Not integrated')],
    [TC('VectorDB'), TC('Vector database abstraction layer'), TCC('Not integrated (stubs)')],
    [TC('Watcher'), TC('File and resource watching system'), TCC('Not integrated')],
    [TC('conversation'), TC('Conversation state management'), TCC('Not integrated (missing dep)')],
    [TC('Security'), TC('SSRF guard and security utilities'), TCC('Referenced, submodule empty')],
    [TC('Panoptic'), TC('Vision and image analysis'), TCC('Not integrated (pixel-heuristic)')],
    [TC('EventBus'), TC('Event-driven communication'), TCC('Not integrated')],
    [TC('Concurrency'), TC('Concurrency-safe data structures'), TCC('Integrated via safe.Slice/Store')],
]
story.append(make_table(data, [0.18, 0.42, 0.25]))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 3: DEEP CODEBASE ANALYSIS
# ═══════════════════════════════════════════════════════════
story.extend(H1('3. Deep Codebase Analysis'))
story.append(P('This section provides an in-depth architectural analysis of each primary repository, identifying strengths, weaknesses, implementation status, and integration opportunities.'))

story.extend(H2('3.1 HelixCode Architecture'))
story.append(P('HelixCode is a Go-based multi-agent AI coding system built on a layered architecture. The system provides 8 entry points (CLI, TUI, REST API, WebSocket, Desktop GUI, Aurora OS, Harmony OS, Mobile), a Gin-based HTTP server with 40+ REST endpoints, and a modular internal package structure with 45+ packages. The core architecture follows a layered design: Client Interfaces (CLI/TUI/API/WS) at the top, followed by the Cobra Command Layer, the Gin HTTP Server with route groups, Core Services (Auth, Worker, Task, Project, Session, LLM, Notification, Memory, Workflow), and the Data Layer (PostgreSQL, Redis, Cognee, Memory Providers).'))
story.append(Spacer(1, 6))
story.append(P('The LLM Provider Layer is the most extensive package, implementing a unified Provider interface supporting 30+ AI providers including 18 cloud providers (OpenAI, Anthropic with prompt caching and extended thinking, Gemini, Azure, Bedrock, VertexAI, Groq, OpenRouter, Qwen, XAI, Copilot, KoboldAI) and 11+ local providers (Ollama, llama.cpp, vLLM, LocalAI, LM Studio, and generic OpenAI-compatible). The ModelManager implements multi-criteria scoring with 60% LLMsVerifier / 40% local heuristic blended scores, hardware-aware model selection, and automatic provider discovery.'))
story.append(Spacer(1, 6))

story.extend(H3('3.1.1 Critical Gaps in HelixCode'))
story.append(bullet('ALL 87 git submodules are UNINITIALIZED - build WILL FAIL for any code importing digital.vasic.* packages'))
story.append(bullet('Dual go.mod confusion: root declares Go 1.25.2, HelixCode/ declares Go 1.26, both use module dev.helix.code'))
story.append(bullet('HelixAgent is NOT integrated - no code references, no submodule entry'))
story.append(bullet('HelixSpecifier is NOT integrated - no code references, no submodule entry'))
story.append(bullet('Redis/Memcached memory providers use local maps instead of real connections (stubs)'))
story.append(bullet('Some API handlers return 501 Not Implemented'))
story.append(bullet('Collaboration voting is "first result as consensus" placeholder'))
story.append(bullet('MCP WebSocket origin check returns true for all origins'))
story.append(bullet('Project/workflow routes have no authentication'))
story.append(Spacer(1, 10))

story.extend(H2('3.2 HelixAgent Architecture'))
story.append(P('HelixAgent is a massive Go monorepo with 1,609 Go source files totaling approximately 1 million lines of code, 51 LLM providers, 50+ MCP adapters, 8 application binaries, and 31 constitutional rules governing all development. It incorporates features ported from 60+ CLI agent codebases, organized in the cli_agents/ directory. The internal structure includes 65+ packages covering agents, tools, features, browser, memory, planning, background services, MCP, containers, challenges, verifier, and much more.'))
story.append(Spacer(1, 6))
story.append(P('Key already-implemented systems include KAIROS (always-on background assistant with tick-based decision making), Dream System (memory consolidation with 3-gate triggers), Swarm (multi-agent coordination with XML messaging and shared scratchpad), YOLO Classifier (7-rule heuristic auto-approval with history-based risk assessment), EditBlock (Aider-style search/replace editing), RepoMap (basic repository mapping), and Sandbox (Docker/Podman execution).'))
story.append(Spacer(1, 6))

story.extend(H3('3.2.1 Stub and Incomplete Implementations'))
story.append(bullet('SubAgent executeTask() is a SIMULATION - returns hardcoded results, does not call LLMs'))
story.append(bullet('Tree-sitter NOT integrated - RepoMap uses basic regex, not AST parsing'))
story.append(bullet('Dream phases 2-3 are no-ops - gather and consolidation just log messages'))
story.append(bullet('KAIROS context is STUB - getGitBranch and getModifiedFiles return empty'))
story.append(bullet('Voice package is EMPTY - no speech recognition implementation'))
story.append(bullet('Seatbelt (macOS sandbox) is a STUB - no actual sandbox-exec integration'))
story.append(bullet('No TUI implementation exists'))
story.append(bullet('No evaluation/benchmark framework'))
story.append(bullet('loadObservations in KAIROS is placeholder'))
story.append(Spacer(1, 10))

story.extend(H2('3.3 HelixLLM Architecture'))
story.append(P('HelixLLM implements a sophisticated single-binary mode system with 6 operating modes (full, gateway, brain, knowledge, agents, control) and clean layer separation: Gateway to FallbackChain to Brain to Knowledge to Agents to Control. It supports 10 LLM provider integrations (llama.cpp, OpenAI, Anthropic, Chutes, OpenRouter, HuggingFace, Nvidia, Cerebras, SambaNova, Together), 22+ built-in agent tools, full RAG pipeline with hybrid BM25+semantic search and MMR reranking, and 80+ test files covering unit, integration, e2e, stress, benchmark, security, and chaos testing.'))
story.append(Spacer(1, 6))
story.append(P('Power features include intelligent multi-provider fallback with LLMsVerifier scoring and circuit breakers, small model optimization (system prompt replacement, tool compression, request orchestration), HTTP/3 + HTTP/2 dual-stack with TLS 1.3, 3-tier agent memory (working/episodic/semantic), and multi-agent coordination with a 4-phase pipeline. However, critical gaps include: PostgreSQL, Kafka, and ClickHouse are configured but not wired, no persistent conversation storage (in-memory only), distributed mode is scaffolding not implemented, and tool call extraction uses fragile string-matching heuristics.'))
story.append(Spacer(1, 10))

story.extend(H2('3.4 HelixQA Architecture'))
story.append(P('HelixQA v0.2.0 is the quality assurance engine with 383 source files, 350 test files, and 126 test banks containing approximately 106K lines of test definitions. Its flagship feature is the autonomous 4-phase LLM-driven QA session (Setup, Doc-Driven, Curiosity, Report). Key capabilities include a dual-model vision architecture, Anti-Bluff system (CONST-035), cognitive memory layer, cheaper vision with 5 adapters, PELT change-point detection, and 13 structured action types. Critical gaps include inability to build standalone (requires sibling dependencies), 4K+ prose actions in test banks that need migration to structured types, Playwright runtime pending, and iOS executor missing.'))
story.append(Spacer(1, 10))

story.extend(H2('3.5 LLMsVerifier Architecture'))
story.append(P('LLMsVerifier v2.0 implements a strategy pattern with circuit breaker, verification pipeline, and scoring engine. Its flagship feature is the mandatory "Do you see my code?" verification with 3-stage testing (meaningful response check, debate prompt, code visibility verification). It provides 48 CLI agent config generators, models.dev integration, (llmsvd) branding suffix, self-improvement feedback loop, and 18+ containerized MCP servers. Critical gaps include 80+ markdown document noise, API key exposure in git history, and 10+ Python scripts that should be Go.'))
story.append(Spacer(1, 10))

story.extend(H2('3.6 HelixMemory Architecture'))
story.append(P('HelixMemory implements an orchestration layer fusing 4 memory backends (Mem0, Cognee, Letta, Graphiti) via a 3-stage fusion engine (collect, deduplicate, rerank). It supports 7 memory types (fact, graph, core, temporal, episodic, procedural, semantic) with auto-classification, 4 full REST API clients with circuit breakers, and production infrastructure with Docker Compose (7 services), PostgreSQL schema (6 tables, 4 views, triggers), and Prometheus metrics. Key gaps include Letta search not passing queries, no write fan-out for redundancy, and trivial consolidation logic.'))
story.append(Spacer(1, 10))

story.extend(H2('3.7 HelixSpecifier Architecture'))
story.append(P('HelixSpecifier implements a 3-pillar fusion engine (SpecKit + Superpowers + GSD) with adaptive ceremony scaling. It provides a 7-phase SDD (Constitution, Specify, Clarify, Plan, Tasks, Analyze, Implement) with phases selected by ceremony level, 10 pre-configured CLI adapters, and 835+ tests across 7 test suites, all race-detector clean. Critical gaps include no persistence (SpecMemory is in-memory only), task execution is stubbed (Superpowers returns hardcoded success), placeholder output without injected DebateFunc, and no spec validation. The highest-value integration opportunity is bridging SpecMemory to HelixMemory for persistent storage and semantic search.'))
story.append(Spacer(1, 10))

story.extend(H2('3.8 Challenges and Containers (Special Attention)'))
story.append(P('The Challenges module is the most architecturally sophisticated supporting module, featuring a constitutional anti-bluff system with no bypass possible, 16 built-in assertion evaluators, a multi-platform userflow automation framework with 21 adapters and 19 challenge templates, topological dependency ordering using Kahn algorithm, liveness monitoring with progress reporters, and direct integration with the Containers module via callback adapters. It contains 209+ tests in the userflow package alone.'))
story.append(Spacer(1, 6))
story.append(P('The Containers module is the most feature-rich supporting module, supporting 7 container runtime backends (Docker, Podman, Kubernetes, LXD, nerdctl, CRI-O, plus RemoteRuntime over SSH), GPU-aware scheduling with 6 strategies, lifecycle management (lazy boot, idle shutdown, concurrency semaphores), a ctop TUI, remote distribution with failover, and 20 event types with aggressive test coverage and multiple coverage escalation files. Both modules use the CLI-shim pattern (shelling out to CLI tools rather than native SDKs), and the Containers module should be upgraded with native Go SDK clients for Docker and Kubernetes.'))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 4: CLI AGENT COMPARATIVE ANALYSIS
# ═══════════════════════════════════════════════════════════
story.extend(H1('4. CLI Agent Comparative Analysis'))
story.append(P('This section provides a detailed comparative analysis of what each CLI agent does better than the current HelixCode/HelixAgent codebase, identifying missing features, performance advantages, and game-changing innovations that must be ported.'))

story.extend(H2('4.1 Tier 1 CLI Agent Feature Matrix'))
data = [
    [TH('Feature'), TH('claude-code'), TH('aider'), TH('codex'), TH('openhands'), TH('cline'), TH('continue'), TH('roo-code'), TH('swe-agent')],
    [TC('Git Integration'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes')],
    [TC('Browser Automation'), TCC('No'), TCC('No'), TCC('No'), TCC('Yes'), TCC('Yes'), TCC('No'), TCC('No'), TCC('No')],
    [TC('Sandbox/Security'), TCC('Yes'), TCC('No'), TCC('Yes'), TCC('Yes'), TCC('No'), TCC('No'), TCC('No'), TCC('Yes')],
    [TC('Voice Commands'), TCC('No'), TCC('Yes'), TCC('No'), TCC('No'), TCC('No'), TCC('No'), TCC('No'), TCC('No')],
    [TC('Multi-file Edit'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes')],
    [TC('Repo Mapping (tree-sitter)'), TCC('Yes'), TCC('Yes'), TCC('No'), TCC('Yes'), TCC('No'), TCC('Yes'), TCC('No'), TCC('Yes')],
    [TC('Evaluation Framework'), TCC('No'), TCC('No'), TCC('No'), TCC('Yes'), TCC('No'), TCC('No'), TCC('No'), TCC('Yes')],
    [TC('Team/Swarm'), TCC('Yes'), TCC('No'), TCC('No'), TCC('Yes'), TCC('No'), TCC('No'), TCC('No'), TCC('No')],
    [TC('Plan Mode'), TCC('Yes'), TCC('No'), TCC('No'), TCC('No'), TCC('No'), TCC('No'), TCC('No'), TCC('No')],
    [TC('Auto-commit'), TCC('No'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('Yes'), TCC('No'), TCC('Yes'), TCC('No')],
    [TC('Context Providers'), TCC('No'), TCC('No'), TCC('No'), TCC('No'), TCC('No'), TCC('Yes'), TCC('No'), TCC('No')],
    [TC('Agent Modes'), TCC('No'), TCC('Yes'), TCC('No'), TCC('No'), TCC('No'), TCC('No'), TCC('Yes'), TCC('No')],
]
story.append(make_table(data, [0.20, 0.10, 0.10, 0.10, 0.10, 0.10, 0.10, 0.10, 0.10]))
story.append(Spacer(1, 12))

story.extend(H2('4.2 Claude Code Source - Deep Analysis (Priority #1)'))
story.append(P('Claude Code Source is the highest-priority CLI agent for analysis and porting. As the internal reference implementation that inspired KAIROS, Dream, Teams, and YOLO features in HelixAgent, it contains the canonical implementations that HelixAgent has partially ported but with critical stubs and gaps. The key innovations include the KAIROS always-on assistant (HelixAgent port has stub context gathering), the Dream System for memory consolidation (HelixAgent port has no-op gather/consolidation phases), Team Management with XML communication (HelixAgent port works but lacks consensus voting), and the YOLO Classifier (HelixAgent port has 7 heuristic rules but lacks ML-based classification). Additionally, the Plan Mode with verification gates, the 4-layer Permission System, and the EditBlock format are all features where the original implementation has nuances and optimizations not yet captured in the HelixAgent port.'))
story.append(Spacer(1, 6))

story.extend(H3('4.2.1 Power Features to Extract from Claude Code'))
story.append(bullet('<b>Prompt Caching</b> - Anthropic-style cache control with cache_control markers reducing token usage by 40-60%'))
story.append(bullet('<b>Extended Thinking</b> - Automatic chain-of-thought activation for complex tasks with budget tokens'))
story.append(bullet('<b>SubAgent Lifecycle</b> - Complete task delegation with real LLM calls (not simulation)'))
story.append(bullet('<b>Memory Consolidation</b> - 4-phase dream system with actual signal gathering and pattern extraction'))
story.append(bullet('<b>Context Window Engineering</b> - Dynamic context selection and compression to maximize relevant information'))
story.append(bullet('<b>Auto-Approval with Learning</b> - ML-based YOLO classifier that improves from execution history'))
story.append(bullet('<b>Git-Native Workflow</b> - Automatic commit message generation, branch awareness, diff-based reviews'))
story.append(bullet('<b>Verification Gates</b> - Multi-step plan mode with verification checkpoints between phases'))
story.append(Spacer(1, 10))

story.extend(H2('4.3 Aider - Git-Native Pair Programming'))
story.append(P('Aider is the most popular open-source AI pair programming tool with 5.7M+ PyPI installs and 15B+ tokens processed weekly. Its key differentiators that HelixAgent/HelixCode must incorporate include tree-sitter based Repository Mapping (repomap.py, 867 lines) supporting 100+ programming languages with intelligent file context selection, the EditBlock Format for surgical code modifications with search/replace blocks and minimal diff generation, Voice-to-Code integration (voice.py, 187 lines) for hands-free coding, and its Git-Native Workflow with automatic commits, diff reviews, and lint/test integration loops. HelixAgent already has a basic RepoMap using regex instead of tree-sitter, and EditBlock is implemented but lacks the flexible matching algorithms from Aider.'))
story.append(Spacer(1, 10))

story.extend(H2('4.4 OpenAI Codex - Sandboxed Execution'))
story.append(P('Codex provides the definitive implementation of sandboxed command execution that HelixCode must adopt. Key features include macOS Seatbelt integration (/usr/bin/sandbox-exec) for OS-level sandboxing, network-disabled execution by default, a multi-level approval system (full autonomy, interactive, approval-required modes), a rich TUI built with ratatui in Rust, and a JSON-RPC Lite protocol for structured communication with thread history management and conversation summaries. The TUI is particularly important as HelixCode currently lacks a proper terminal UI. The approval system maps directly to HelixAgent YOLO Classifier but provides a more sophisticated multi-gate architecture.'))
story.append(Spacer(1, 10))

story.extend(H2('4.5 OpenHands - Autonomous SWE'))
story.append(P('OpenHands achieves a 77.6% SWE-bench score and provides the most complete autonomous software engineering implementation. Its agent system with AgentHub for pluggable agent architectures maps to HelixAgent SubAgent system but provides more sophisticated agent types. The evaluation framework with SWE-bench integration is critical for measuring HelixCode performance objectively. The event-driven Action-Observation loop provides a cleaner execution model than current HelixAgent implementations. Docker-based runtime sandboxing with E2B integration provides the execution isolation that HelixCode needs. The Theory of Mind module and browser automation for headless web navigation are unique features not present in any other agent.'))
story.append(Spacer(1, 10))

story.extend(H2('4.6 Cline - Browser Automation'))
story.append(P('Cline provides the most complete browser automation implementation among CLI agents. Its headless browser integration enables web navigation, screenshot analysis, API documentation reading, and autonomous task execution with self-correction. The multi-provider support, file context management with gitignore-aware operations, and diff-based editing with a minimal change principle are all valuable features. The autonomous task planning with multi-step workflow execution and self-correction on errors is a key capability that maps to HelixAgent SubAgent but with a more robust iteration loop.'))
story.append(Spacer(1, 10))

story.extend(H2('4.7 Continue - Universal IDE Support'))
story.append(P('Continue provides the most extensible context provider system with @file, @url, @docs, and @codebase context sources, tab-based code completion with multi-line suggestions, a slash command action system (/edit, /comment, /doc, /test), and universal IDE support across VS Code, JetBrains, and any LSP-compatible editor. The modular context provider system is the key innovation - it allows dynamic context injection from arbitrary sources, which would significantly improve HelixCode context building. The action framework maps to HelixCode slash commands but provides a more composable architecture.'))
story.append(Spacer(1, 10))

story.extend(H2('4.8 Feature Gap Summary'))
data = [
    [TH('Feature'), TH('Source Agent'), TH('Current Status in Helix'), TH('Porting Priority')],
    [TC('Tree-sitter Repository Mapping'), TC('Aider'), TC('Basic regex-based only'), TCC('CRITICAL')],
    [TC('Sandboxed Execution (Seatbelt)'), TC('Codex'), TC('Docker/Podman only'), TCC('CRITICAL')],
    [TC('SubAgent Real LLM Calls'), TC('Claude Code'), TC('Simulation/hardcoded'), TCC('CRITICAL')],
    [TC('Dream System Consolidation'), TC('Claude Code'), TC('No-op phases 2-3'), TCC('CRITICAL')],
    [TC('Browser Automation (Headless)'), TC('Cline/OpenHands'), TC('HelixCode has it; HelixAgent partial'), TCC('HIGH')],
    [TC('Evaluation/SWE-bench Framework'), TC('OpenHands/SWE-agent'), TC('Not implemented'), TCC('HIGH')],
    [TC('Voice Commands'), TC('Aider'), TC('Empty package'), TCC('HIGH')],
    [TC('Auto-Commit with Message Gen'), TC('Aider/Codex'), TC('HelixCode has it; HelixAgent basic'), TCC('HIGH')],
    [TC('Context Providers (@file/@url/@docs)'), TC('Continue'), TC('Mentions parser exists'), TCC('HIGH')],
    [TC('Agent Modes (Code/Architect/Ask)'), TC('Roo Code'), TC('Not implemented'), TCC('MEDIUM')],
    [TC('TUI (Terminal User Interface)'), TC('Codex (ratatui)'), TC('Not implemented'), TCC('MEDIUM')],
    [TC('Lint/Test Fix Iteration Loop'), TC('Aider'), TC('Not implemented'), TCC('MEDIUM')],
    [TC('ML-Based YOLO Classifier'), TC('Claude Code'), TC('7-rule heuristic only'), TCC('MEDIUM')],
    [TC('JSON-RPC Protocol'), TC('Codex'), TC('Not implemented'), TCC('LOW')],
    [TC('Event-Driven Action Loop'), TC('OpenHands'), TC('Not implemented'), TCC('LOW')],
]
story.append(make_table(data, [0.28, 0.16, 0.28, 0.14]))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 5: GIT SUBMODULE INTEGRATION
# ═══════════════════════════════════════════════════════════
story.extend(H1('5. Git Submodule Integration via SSH'))
story.append(P('All submodules MUST be integrated using Git via SSH (not HTTPS) as specified. This section provides the step-by-step procedure for initializing, wiring, and verifying all submodules in both HelixCode and HelixAgent projects.'))

story.extend(H2('5.1 Submodule Initialization Procedure'))
story.append(P('The following steps must be executed in order for both the HelixCode and HelixAgent repositories. Each step includes verification commands to ensure proper initialization. Before beginning, ensure SSH keys are configured for both github.com and gitlab.com remotes, and that the SSH config includes entries for both HelixDevelopment and vasic-digital organizations.'))
story.append(Spacer(1, 6))

story.extend(H3('5.1.1 Convert Existing HTTPS Submodules to SSH'))
story.append(P('For each repository, convert all .gitmodules entries from HTTPS to SSH URLs. This is critical because the projects currently reference submodules with git@github.com: SSH URLs in their .gitmodules files, but the clone operations in this environment used HTTPS as a fallback. The canonical SSH format must be maintained going forward.'))
story.append(Spacer(1, 6))

story.extend(H3('5.1.2 Add Missing Submodules to HelixCode'))
story.append(P('HelixCode currently lacks submodule entries for HelixAgent, HelixSpecifier, and HelixMemory. These must be added as proper Git submodules with SSH URLs and fully integrated into the go.mod replace directives. The following submodules need to be added:'))
story.append(Spacer(1, 4))
story.append(bullet('HelixAgent: git@github.com:HelixDevelopment/HelixAgent.git - Path: HelixAgent/'))
story.append(bullet('HelixSpecifier: git@github.com:HelixDevelopment/HelixSpecifier.git - Path: HelixSpecifier/'))
story.append(bullet('HelixMemory: git@github.com:HelixDevelopment/HelixMemory.git - Path: HelixMemory/'))
story.append(bullet('HelixLLM: git@github.com:HelixDevelopment/HelixLLM.git - Path: HelixLLM/'))
story.append(Spacer(1, 6))

story.extend(H3('5.1.3 Initialize All Submodules Recursively'))
story.append(P('Execute the following command sequence to recursively initialize and update all submodules across both repositories. This must be done on a machine with SSH access configured. The recursive flag is essential because HelixAgent, HelixLLM, and Challenges all have their own nested submodules that must also be initialized.'))
story.append(Spacer(1, 4))
story.append(P('git submodule update --init --recursive', code_style))
story.append(Spacer(1, 6))

story.extend(H2('5.2 go.mod Replace Directives'))
story.append(P('After adding submodules, the go.mod file must be updated with replace directives for each submodule. The following table shows the required additions for HelixCode:'))
data = [
    [TH('Module Path'), TH('Local Path'), TH('Status')],
    [TC('digital.vasic.containers'), TC('../Containers'), TCC('Exists, needs init')],
    [TC('digital.vasic.helixqa'), TC('../HelixQA'), TCC('Exists, needs init')],
    [TC('digital.vasic.challenges'), TC('../Challenges'), TCC('Exists, needs init')],
    [TC('digital.vasic.security'), TC('../Security'), TCC('Exists, needs init')],
    [TC('dev.helix.agent'), TC('../HelixAgent'), TCC('NEW - must add')],
    [TC('dev.helix.specifier'), TC('../HelixSpecifier'), TCC('NEW - must add')],
    [TC('dev.helix.memory'), TC('../HelixMemory'), TCC('NEW - must add')],
    [TC('dev.helix.llm'), TC('../HelixLLM'), TCC('NEW - must add')],
]
story.append(make_table(data, [0.35, 0.25, 0.25]))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 6: COMPREHENSIVE INTEGRATION PLAN
# ═══════════════════════════════════════════════════════════
story.extend(H1('6. Comprehensive Phased Integration Plan'))
story.append(P('This plan is divided into 6 phases spanning 12-16 weeks. Each phase contains fine-grained tasks with specific deliverables, dependencies, and verification criteria. No feature, optimization, or workaround from any CLI agent may be skipped, omitted, or relativized.'))

story.extend(H2('6.1 Phase 0: Foundation Repair (Week 1-2)'))
story.append(P('Before any feature porting can begin, the foundation must be solid. This phase addresses all critical infrastructure issues that currently block development and testing.'))
story.append(Spacer(1, 6))

data = [
    [TH('Task ID'), TH('Task'), TH('Deliverable'), TH('Priority')],
    [TC('0.1'), TC('Initialize ALL git submodules recursively via SSH'), TC('All 87+ submodules checked out and building'), TCC('CRITICAL')],
    [TC('0.2'), TC('Fix dual go.mod - remove root go.mod, keep HelixCode/go.mod'), TC('Single go.mod with consistent Go version'), TCC('CRITICAL')],
    [TC('0.3'), TC('Add missing submodules (HelixAgent, HelixSpecifier, HelixMemory, HelixLLM)'), TC('git submodule add entries with SSH URLs'), TCC('CRITICAL')],
    [TC('0.4'), TC('Wire go.mod replace directives for all new submodules'), TC('Replace directives for dev.helix.agent, specifier, memory, llm'), TCC('CRITICAL')],
    [TC('0.5'), TC('Verify build succeeds with all submodules initialized'), TC('go build ./... passes without errors'), TCC('CRITICAL')],
    [TC('0.6'), TC('Resolve digital.vasic.messaging missing dependency'), TC('Import path resolved or stub created'), TCC('HIGH')],
    [TC('0.7'), TC('Clean up 80+ contradictory markdown status files'), TC('Archive to docs/archive/, single STATUS.md'), TCC('MEDIUM')],
    [TC('0.8'), TC('Secure public project/workflow API routes'), TC('JWT auth middleware on all /api/v1/projects/* routes'), TCC('HIGH')],
]
story.append(make_table(data, [0.07, 0.38, 0.35, 0.12]))
story.append(Spacer(1, 10))

story.extend(H2('6.2 Phase 1: Critical Feature Porting (Week 3-5)'))
story.append(P('This phase ports the most critical features from CLI agents that are currently stubs or missing entirely. These features are required for basic production functionality.'))
story.append(Spacer(1, 6))

data = [
    [TH('Task ID'), TH('Task'), TH('Source Agent'), TH('Deliverable')],
    [TC('1.1'), TC('Implement tree-sitter Repository Mapping'), TC('Aider'), TC('internal/tools/repomap/ with AST parsing, 100+ languages')],
    [TC('1.2'), TC('Implement Seatbelt sandbox for macOS'), TC('Codex'), TC('internal/tools/sandbox/seatbelt.go with /usr/bin/sandbox-exec')],
    [TC('1.3'), TC('Wire SubAgent executeTask() to real LLM calls'), TC('Claude Code'), TC('SubAgent delegates to LLM via provider layer')],
    [TC('1.4'), TC('Implement Dream gather and consolidation phases'), TC('Claude Code'), TC('Phase 2: KAIROS log analysis; Phase 3: LLM-based extraction')],
    [TC('1.5'), TC('Fix KAIROS context gathering (git branch, modified files)'), TC('Claude Code'), TC('Real git command execution replacing stubs')],
    [TC('1.6'), TC('Port Aider EditBlock flexible matching algorithms'), TC('Aider'), TC('Fuzzy matching, whitespace normalization, line-range matching')],
    [TC('1.7'), TC('Implement JSON-RPC Lite protocol adapter'), TC('Codex'), TC('internal/protocol/jsonrpc.go for structured communication')],
    [TC('1.8'), TC('Port OpenHands Action-Observation loop'), TC('OpenHands'), TC('internal/agents/actionloop.go with event stream')],
]
story.append(make_table(data, [0.07, 0.32, 0.16, 0.40]))
story.append(Spacer(1, 10))

story.extend(H2('6.3 Phase 2: High-Priority Features (Week 6-8)'))
story.append(P('This phase ports high-priority features that significantly enhance capabilities but are not blocking for basic operation.'))
story.append(Spacer(1, 6))

data = [
    [TH('Task ID'), TH('Task'), TH('Source Agent'), TH('Deliverable')],
    [TC('2.1'), TC('Port browser automation to HelixAgent'), TC('Cline/OpenHands'), TC('internal/browser/ with headless Chrome, screenshots, DOM')],
    [TC('2.2'), TC('Implement SWE-bench evaluation framework'), TC('OpenHands/SWE-agent'), TC('internal/benchmark/ with SWE-bench runner and metrics')],
    [TC('2.3'), TC('Implement voice command system'), TC('Aider'), TC('internal/agents/voice/ with speech recognition and command routing')],
    [TC('2.4'), TC('Enhance auto-commit with AI message generation'), TC('Aider/Codex'), TC('Diff analysis + LLM commit message + conventional commits')],
    [TC('2.5'), TC('Port Continue context provider system'), TC('Continue'), TC('internal/context/providers/ with @file, @url, @docs, @codebase')],
    [TC('2.6'), TC('Bridge HelixSpecifier SpecMemory to HelixMemory'), TC('HelixSpecifier'), TC('Persistent spec storage with semantic search')],
    [TC('2.7'), TC('Wire HelixLLM as primary LLM provider for HelixCode'), TC('HelixLLM'), TC('Replace direct provider layer with HelixLLM gateway')],
    [TC('2.8'), TC('Implement HelixMemory integration in HelixCode'), TC('HelixMemory'), TC('Memory providers connected to agent memory system')],
    [TC('2.9'), TC('Port lint/test fix iteration loop'), TC('Aider'), TC('Automatic lint/test/fix cycle with error analysis and retry')],
    [TC('2.10'), TC('Add native Go SDK clients for Containers'), TC('Containers'), TC('Replace CLI-shims with Docker/K8s Go SDK calls')],
]
story.append(make_table(data, [0.07, 0.32, 0.16, 0.40]))
story.append(Spacer(1, 10))

story.extend(H2('6.4 Phase 3: Medium-Priority Features (Week 9-11)'))
story.append(Spacer(1, 6))

data = [
    [TH('Task ID'), TH('Task'), TH('Source Agent'), TH('Deliverable')],
    [TC('3.1'), TC('Implement Agent Modes (Code/Architect/Ask/Debug)'), TC('Roo Code'), TC('internal/agents/modes/ with mode-specific behavior')],
    [TC('3.2'), TC('Build TUI with ratatui-style interface'), TC('Codex'), TC('applications/terminal_ui/ with chat, file browser, commands')],
    [TC('3.3'), TC('Implement ML-based YOLO classifier'), TC('Claude Code'), TC('Train model from execution history for auto-approval')],
    [TC('3.4'), TC('Port OpenHands event-driven architecture'), TC('OpenHands'), TC('Event bus with action/observation stream processing')],
    [TC('3.5'), TC('Implement panoptic ML-based vision'), TC('Panoptic'), TC('Replace pixel heuristics with ML model for image analysis')],
    [TC('3.6'), TC('Wire Challenges anti-bluff into HelixCode'), TC('Challenges'), TC('Constitutional anti-bluff verification for all operations')],
    [TC('3.7'), TC('Implement conversation persistent storage'), TC('HelixLLM'), TC('PostgreSQL-backed conversation persistence layer')],
    [TC('3.8'), TC('Add HelixQA autonomous QA session integration'), TC('HelixQA'), TC('4-phase LLM-driven QA with Setup/Doc/Curiosity/Report')],
]
story.append(make_table(data, [0.07, 0.32, 0.16, 0.40]))
story.append(Spacer(1, 10))

story.extend(H2('6.5 Phase 4: UI/WEB/REST/CLI/TUI Coverage (Week 12-13)'))
story.append(P('This phase ensures all applicative components are fully covered across all interface types: UI (Desktop GUI), WEB (REST API + WebSocket), CLI (Command Line), TUI (Terminal UI), and REST (API endpoints). Every feature ported in Phases 1-3 must be accessible from every interface.'))
story.append(Spacer(1, 6))
story.append(bullet('<b>REST API</b>: Add endpoints for all new features (/api/v1/agents/modes, /api/v1/repomap, /api/v1/sandbox, /api/v1/voice, /api/v1/context, /api/v1/benchmark)'))
story.append(bullet('<b>CLI</b>: Add Cobra commands for all new features (helix repomap, helix sandbox run, helix voice, helix benchmark, helix mode)'))
story.append(bullet('<b>TUI</b>: Integrate all features into the terminal UI with panels for repomap, browser, voice, and mode switching'))
story.append(bullet('<b>Desktop GUI</b>: Update Fyne desktop application with new feature panels and controls'))
story.append(bullet('<b>WebSocket</b>: Expose all features via MCP protocol for real-time integration'))
story.append(bullet('<b>Mobile</b>: Update shared mobile core with API bindings for new features'))
story.append(Spacer(1, 10))

story.extend(H2('6.6 Phase 5: Testing and Hardening (Week 14-16)'))
story.append(P('This phase implements the comprehensive testing strategy detailed in Section 7 and performs security hardening, performance optimization, and documentation finalization. Every implementation from Phases 1-4 must achieve 90%+ test coverage and pass all Challenges and HelixQA test suites.'))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 7: TESTING STRATEGY
# ═══════════════════════════════════════════════════════════
story.extend(H1('7. Testing Strategy'))
story.append(P('The testing strategy leverages three pillars: the Challenges framework for constitutional anti-bluff verification, HelixQA for autonomous QA sessions with LLM-driven test generation, and LLMsVerifier for model quality assurance. No change or implementation is considered complete until it passes all three testing pillars with 90%+ coverage.'))
story.append(Spacer(1, 6))

story.extend(H2('7.1 Testing Pyramid'))
data = [
    [TH('Level'), TH('Type'), TH('Tool/Framework'), TH('Coverage Target')],
    [TC('L1'), TC('Unit Tests'), TC('Go testing + testify'), TCC('90%+ line coverage')],
    [TC('L2'), TC('Integration Tests'), TC('Go testing + Docker Compose'), TCC('All module interactions')],
    [TC('L3'), TC('E2E Tests'), TC('Challenges scripts + HelixQA'), TCC('All user workflows')],
    [TC('L4'), TC('QA Sessions'), TC('HelixQA autonomous 4-phase'), TCC('All features verified')],
    [TC('L5'), TC('Model Verification'), TC('LLMsVerifier scoring'), TCC('All LLM providers verified')],
    [TC('L6'), TC('Benchmark Tests'), TC('Go benchmark + SWE-bench'), TCC('Performance baselines')],
    [TC('L7'), TC('Security Tests'), TC('Security module + OWASP'), TCC('All attack vectors')],
    [TC('L8'), TC('Chaos Tests'), TC('HelixLLM chaos suite'), TCC('Failure resilience')],
]
story.append(make_table(data, [0.08, 0.20, 0.32, 0.25]))
story.append(Spacer(1, 10))

story.extend(H2('7.2 Challenges Integration'))
story.append(P('The Challenges framework provides 16 built-in assertion evaluators, 19 challenge templates, and topological dependency ordering. Every feature implementation must have corresponding challenge scripts that verify the feature works correctly in isolation and in combination with other features. The constitutional anti-bluff system (CONST-035 equivalent) must be applied to ensure no feature claims to work without actual verification. Challenge scripts are organized by phase and must pass before any phase is considered complete.'))
story.append(Spacer(1, 6))

story.extend(H3('7.2.1 Required Challenge Scripts by Phase'))
data = [
    [TH('Phase'), TH('Challenge'), TH('Verifies')],
    [TC('Phase 0'), TC('submodule-init-check'), TC('All submodules initialized and buildable')],
    [TC('Phase 0'), TC('go-mod-consistency'), TC('Single go.mod, correct replace directives')],
    [TC('Phase 1'), TC('repomap-tree-sitter'), TC('AST parsing produces correct symbol tables')],
    [TC('Phase 1'), TC('sandbox-seatbelt'), TC('Commands run in isolated macOS sandbox')],
    [TC('Phase 1'), TC('subagent-real-llm'), TC('SubAgent makes actual LLM API calls')],
    [TC('Phase 1'), TC('dream-consolidation'), TC('Dream phases produce real memory entries')],
    [TC('Phase 2'), TC('browser-automation'), TC('Headless browser navigates and captures')],
    [TC('Phase 2'), TC('swe-bench-eval'), TC('SWE-bench test cases pass above threshold')],
    [TC('Phase 2'), TC('context-providers'), TC('All @mention types resolve correctly')],
    [TC('Phase 3'), TC('agent-modes'), TC('Each mode produces mode-appropriate output')],
    [TC('Phase 3'), TC('anti-bluff-verify'), TC('No feature returns bluff/unverified results')],
    [TC('Phase 4'), TC('api-completeness'), TC('All features accessible via REST/CLI/TUI/WS')],
    [TC('Phase 5'), TC('coverage-gate'), TC('90%+ test coverage across all packages')],
]
story.append(make_table(data, [0.12, 0.28, 0.48]))
story.append(Spacer(1, 10))

story.extend(H2('7.3 HelixQA Test Suite Integration'))
story.append(P('HelixQA provides 126 test banks with approximately 106K lines of test definitions and an autonomous 4-phase LLM-driven QA session. Each feature implementation must undergo a HelixQA autonomous session that validates the feature through Setup, Documentation-Driven testing, Curiosity-driven exploration, and comprehensive Report generation. The Anti-Bluff system (CONST-035) ensures that no test can be bypassed and all results are verified against actual execution rather than claimed behavior. Additionally, the Cheaper Vision system with 5 adapters can validate UI components across different rendering contexts.'))
story.append(Spacer(1, 10))

story.extend(H2('7.4 LLMsVerifier Quality Gates'))
story.append(P('LLMsVerifier provides mandatory "Do you see my code?" verification for all LLM interactions. Every feature that involves LLM calls must pass through the 3-stage verification pipeline: (1) meaningful response check, (2) debate prompt to test consistency, and (3) code visibility verification ensuring the LLM has actually seen and understood the codebase. The 48 CLI agent config generators ensure that every supported LLM provider is properly configured, and the scoring engine provides quantitative quality metrics for each provider and model combination. Circuit breakers and fallback models ensure that degraded LLM performance does not result in incorrect feature behavior.'))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 8: API SPECIFICATIONS
# ═══════════════════════════════════════════════════════════
story.extend(H1('8. API Specifications for New Integrations'))
story.append(P('This section defines the API interfaces for all new integration points between HelixCode, HelixAgent, and the supporting submodules.'))

story.extend(H2('8.1 HelixAgent Integration API'))
story.append(P('HelixAgent must be integrated as both a Go module dependency and a runtime service. The module integration provides direct access to HelixAgent internal packages (agents, tools, features, mcp) while the service integration enables remote agent execution via gRPC or REST API.'))
story.append(Spacer(1, 6))

data = [
    [TH('Endpoint/Interface'), TH('Method'), TH('Purpose')],
    [TC('/api/v1/agents/execute'), TC('POST'), TC('Execute a task via HelixAgent SubAgent')],
    [TC('/api/v1/agents/swarm/create'), TC('POST'), TC('Create a multi-agent swarm')],
    [TC('/api/v1/agents/swarm/{id}/assign'), TC('POST'), TC('Assign task to swarm agent')],
    [TC('/api/v1/agents/modes'), TC('GET/PUT'), TC('Get/set agent mode (code/architect/ask/debug)')],
    [TC('/api/v1/agents/dream/trigger'), TC('POST'), TC('Manually trigger Dream consolidation')],
    [TC('/api/v1/agents/kairos/status'), TC('GET'), TC('Get KAIROS observation and action history')],
    [TC('/api/v1/agents/yolo/classify'), TC('POST'), TC('Classify tool execution risk level')],
    [TC('/api/v1/repomap/generate'), TC('POST'), TC('Generate tree-sitter repository map')],
    [TC('/api/v1/sandbox/execute'), TC('POST'), TC('Execute command in sandboxed environment')],
    [TC('/api/v1/voice/command'), TC('POST'), TC('Process voice command via speech recognition')],
    [TC('/api/v1/context/resolve'), TC('POST'), TC('Resolve @mention context providers')],
    [TC('/api/v1/benchmark/run'), TC('POST'), TC('Run evaluation benchmark (SWE-bench, etc.)')],
]
story.append(make_table(data, [0.30, 0.12, 0.48]))
story.append(Spacer(1, 10))

story.extend(H2('8.2 Submodule Interface Contracts'))
story.append(P('Each supporting submodule must expose a clean Go interface that HelixCode and HelixAgent can consume without tight coupling. The following table defines the required interface contracts:'))
story.append(Spacer(1, 6))

data = [
    [TH('Submodule'), TH('Primary Interface'), TH('Key Methods')],
    [TC('Challenges'), TC('Verifier'), TC('RegisterChallenge(), RunChallenge(), GetResults()')],
    [TC('Containers'), TC('RuntimeManager'), TC('CreateContainer(), ExecuteInContainer(), HealthCheck()')],
    [TC('Security'), TC('SSRFGuard'), TC('ValidateURL(), CheckIPAlternative(), AuditRequest()')],
    [TC('Agentic'), TC('Orchestrator'), TC('Plan(), Execute(), Coordinate()')],
    [TC('TOON'), TC('TaskNetwork'), TC('SubmitTask(), GetStatus(), Optimize()')],
    [TC('ToolSchema'), TC('SchemaRegistry'), TC('Register(), Validate(), GenerateSpec()')],
    [TC('VectorDB'), TC('VectorStore'), TC('Insert(), Search(), Delete(), HybridSearch()')],
    [TC('Watcher'), TC('FileWatcher'), TC('Watch(), OnChange(), Stop()')],
    [TC('Conversation'), TC('StateManager'), TC('CreateSession(), AddMessage(), GetContext()')],
    [TC('Panoptic'), TC('VisionAnalyzer'), TC('AnalyzeImage(), ExtractText(), Classify()')],
]
story.append(make_table(data, [0.16, 0.22, 0.52]))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 9: RISK ANALYSIS
# ═══════════════════════════════════════════════════════════
story.extend(H1('9. Risk Analysis and Mitigation'))
story.append(P('This section identifies the key risks in the integration effort and provides mitigation strategies for each.'))

data = [
    [TH('Risk'), TH('Impact'), TH('Probability'), TH('Mitigation')],
    [TC('Submodule initialization reveals broken builds'), TCC('HIGH'), TCC('HIGH'), TC('Phase 0 dedicated to build verification; incremental init')],
    [TC('API incompatibilities between submodules'), TCC('HIGH'), TCC('MEDIUM'), TC('Adapter pattern with versioned interfaces; compatibility shims')],
    [TC('Tree-sitter CGO compilation issues'), TCC('MEDIUM'), TCC('MEDIUM'), TC('Pre-built WASM parsers as fallback; CI matrix testing')],
    [TC('LLM provider API changes break integrations'), TCC('MEDIUM'), TCC('HIGH'), TC('LLMsVerifier circuit breakers; fallback model lists; adapter versioning')],
    [TC('Test coverage targets unrealistic for stub code'), TCC('MEDIUM'), TCC('LOW'), TC('Incremental coverage gates: 60% Phase 1, 75% Phase 3, 90% Phase 5')],
    [TC('HelixMemory backends unavailable in test env'), TCC('LOW'), TCC('MEDIUM'), TC('In-memory providers for testing; Docker Compose for integration')],
    [TC('Cross-platform sandbox (Seatbelt/Linux disparities)'), TCC('MEDIUM'), TCC('HIGH'), TC('Runtime detection with graceful fallback; container-based universal sandbox')],
    [TC('Performance regression from added abstraction layers'), TCC('MEDIUM'), TCC('MEDIUM'), TC('Benchmark gates in CI; profiling at each phase boundary')],
    [TC('Voice recognition accuracy insufficient'), TCC('LOW'), TCC('MEDIUM'), TC('Multiple STT providers with fallback; command confirmation step')],
    [TC('SWE-bench evaluation reveals lower than expected scores'), TCC('HIGH'), TCC('MEDIUM'), TC('Iterative improvement with prompt engineering; multi-agent consensus approaches')],
]
story.append(make_table(data, [0.28, 0.10, 0.12, 0.42]))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 10: IMPLEMENTATION TIMELINE
# ═══════════════════════════════════════════════════════════
story.extend(H1('10. Implementation Timeline'))
story.append(P('The following timeline provides a week-by-week breakdown of the 16-week integration effort with milestones, dependencies, and deliverables.'))

data = [
    [TH('Week'), TH('Phase'), TH('Key Deliverables'), TH('Milestone')],
    [TC('1-2'), TC('Phase 0'), TC('All submodules initialized, build green, go.mod fixed'), TC('Foundation Ready')],
    [TC('3'), TC('Phase 1'), TC('Tree-sitter repomap, Seatbelt sandbox'), TC('Critical Infra')],
    [TC('4'), TC('Phase 1'), TC('SubAgent real LLM, Dream consolidation'), TC('Agent Core')],
    [TC('5'), TC('Phase 1'), TC('KAIROS context, EditBlock matching, JSON-RPC'), TC('CLI Agent Core')],
    [TC('6'), TC('Phase 2'), TC('Browser automation, SWE-bench framework'), TC('Automation Ready')],
    [TC('7'), TC('Phase 2'), TC('Voice commands, auto-commit, context providers'), TC('UX Features')],
    [TC('8'), TC('Phase 2'), TC('HelixMemory bridge, HelixLLM integration, Containers SDK'), TC('Module Integration')],
    [TC('9'), TC('Phase 3'), TC('Agent modes, TUI framework, ML YOLO'), TC('Advanced Features')],
    [TC('10'), TC('Phase 3'), TC('Event-driven arch, panoptic vision, anti-bluff'), TC('Verification Layer')],
    [TC('11'), TC('Phase 3'), TC('Persistent conversations, HelixQA sessions'), TC('QA Integration')],
    [TC('12'), TC('Phase 4'), TC('REST API endpoints for all features'), TC('API Complete')],
    [TC('13'), TC('Phase 4'), TC('CLI commands, TUI panels, WebSocket, Desktop, Mobile'), TC('UI/UX Complete')],
    [TC('14'), TC('Phase 5'), TC('Unit + integration tests, 90% coverage'), TC('Test Coverage')],
    [TC('15'), TC('Phase 5'), TC('Challenges + HelixQA sessions, security hardening'), TC('QA Verified')],
    [TC('16'), TC('Phase 5'), TC('Performance optimization, documentation, release'), TC('RELEASE')],
]
story.append(make_table(data, [0.07, 0.10, 0.48, 0.18]))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 11: CLI AGENT STEP-BY-STEP PORTING GUIDE
# ═══════════════════════════════════════════════════════════
story.extend(H1('11. CLI Agent Step-by-Step Porting Guide'))
story.append(P('This section provides detailed step-by-step guides for incorporating each CLI agent features into HelixAgent and HelixCode, starting with claude-code-source as the priority.'))

story.extend(H2('11.1 Claude Code Source Porting Guide'))
story.append(P('Claude Code Source is the canonical implementation for KAIROS, Dream, Teams, YOLO, and Plan Mode features. The porting process involves analyzing the original TypeScript implementation, extracting the core logic and algorithms, reimplementing in Go with HelixAgent architecture patterns (safe.Slice, safe.Store, atomic operations per CONST-029), and writing comprehensive tests using the Challenges and HelixQA frameworks.'))
story.append(Spacer(1, 6))

story.extend(H3('11.1.1 KAIROS Full Implementation'))
story.append(P('<b>Step 1:</b> Replace getGitBranch() stub with actual git command execution using os/exec to run git rev-parse --abbrev-ref HEAD. <b>Step 2:</b> Replace getModifiedFiles() stub with git status --porcelain parsing. <b>Step 3:</b> Implement loadObservations() to read the last 3 daily log files from the kairos logs directory. <b>Step 4:</b> Wire the onDecision callback to actual LLM calls through the HelixAgent LLM provider layer, providing the TickPrompt as context. <b>Step 5:</b> Implement proactive observation sources: file system watcher for workspace changes, git hook integration for commit/push events, and process monitor for development activity. <b>Step 6:</b> Add KAIROS REST API endpoints for observation history, action history, and manual tick triggering. <b>Step 7:</b> Write Challenge script verifying KAIROS detects and responds to workspace changes.'))
story.append(Spacer(1, 6))

story.extend(H3('11.1.2 Dream System Full Implementation'))
story.append(P('<b>Step 1:</b> Implement gatherPhase to search KAIROS daily logs for patterns, extract session transcripts, and identify drifting memories (accessed but never updated). <b>Step 2:</b> Implement consolidationPhase using LLM calls to extract patterns from gathered signals, identify facts worth preserving, update existing memories with new information, translate relative dates to absolute, and remove disproven facts. <b>Step 3:</b> Enhance cleanupPhase from simple line trimming to intelligent MEMORY.md maintenance with stale pointer removal and contradiction resolution. <b>Step 4:</b> Wire Dream to HelixMemory for persistent storage across restarts. <b>Step 5:</b> Add Dream session API endpoints for triggering, monitoring, and reviewing dream sessions. <b>Step 6:</b> Write HelixQA autonomous session verifying Dream produces meaningful consolidations.'))
story.append(Spacer(1, 10))

story.extend(H2('11.2 Aider Porting Guide'))
story.append(P('<b>Step 1:</b> Port tree-sitter integration from Aider repomap.py. Create internal/tools/repomap/treesitter.go with CGO bindings to tree-sitter C library. Implement language detection and parser selection for 100+ languages. <b>Step 2:</b> Port repository structure analysis - map files to symbols (functions, classes, variables) with signatures, line numbers, and dependencies. <b>Step 3:</b> Port intelligent file context selection - rank files by relevance to current task using symbol dependencies and modification recency. <b>Step 4:</b> Enhance EditBlock with Aider flexible matching: whitespace normalization, line-range fuzzy matching, and multi-block application with rollback. <b>Step 5:</b> Port voice.py integration using Whisper or similar STT service with command routing. <b>Step 6:</b> Port lint/test integration loop: run linter, parse errors, feed to LLM for fixes, iterate until clean. <b>Step 7:</b> Write Challenges verifying each ported feature against Aider test cases.'))
story.append(Spacer(1, 10))

story.extend(H2('11.3 Codex Porting Guide'))
story.append(P('<b>Step 1:</b> Port Seatbelt sandbox: create internal/tools/sandbox/seatbelt.go that generates Seatbelt policy profiles and executes commands via /usr/bin/sandbox-exec. <b>Step 2:</b> Port multi-level approval system: implement ApprovalPolicy with full-autonomy, interactive, and approval-required modes. <b>Step 3:</b> Port TUI: create applications/terminal_ui/ with ratatui-style interface using tview, including chat panel, file browser, command palette, and diff viewer. <b>Step 4:</b> Port JSON-RPC Lite protocol: implement internal/protocol/jsonrpc.go with structured request/response, thread history, and conversation summaries. <b>Step 5:</b> Write Challenges for sandbox isolation verification and approval gate testing.'))
story.append(Spacer(1, 10))

story.extend(H2('11.4 OpenHands Porting Guide'))
story.append(P('<b>Step 1:</b> Port AgentHub pluggable agent architecture: refactor HelixAgent SubAgent to support dynamic agent registration and selection. <b>Step 2:</b> Port evaluation framework: create internal/benchmark/ with SWE-bench runner, custom metrics, and result aggregation. <b>Step 3:</b> Port event-driven Action-Observation loop: implement internal/agents/actionloop.go with event stream processing, action dispatch, and observation collection. <b>Step 4:</b> Port runtime sandboxing with Docker-based execution and E2B integration. <b>Step 5:</b> Write Challenges with SWE-bench test cases and performance thresholds.'))
story.append(Spacer(1, 10))

story.extend(H2('11.5 Cline and Continue Porting Guide'))
story.append(P('<b>Cline Porting:</b> Port headless browser integration to HelixAgent internal/browser/ with Chrome DevTools Protocol. Implement autonomous task planning with multi-step execution and self-correction. Port diff-based editing with minimal change principle. <b>Continue Porting:</b> Port modular context provider system with @file, @url, @docs, @codebase resolvers. Port slash command action system with composable commands. Port tab-based code completion with multi-line suggestions. Both require Challenge scripts for browser navigation verification and context resolution correctness.'))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 12: CROSS-MODULE DEPENDENCY GRAPH
# ═══════════════════════════════════════════════════════════
story.extend(H1('12. Cross-Module Dependency Graph'))
story.append(P('The following table maps the dependencies between all modules and identifies the integration order required to satisfy all dependency chains. Modules must be integrated in topological order to prevent compilation failures.'))
story.append(Spacer(1, 6))

data = [
    [TH('Module'), TH('Depends On'), TH('Required By'), TH('Integration Order')],
    [TC('Concurrency'), TC('(none)'), TC('HelixAgent, HelixLLM, all'), TCC('1')],
    [TC('Security'), TC('(none)'), TC('HelixCode, HelixAgent'), TCC('1')],
    [TC('Containers'), TC('(none)'), TC('HelixCode, HelixAgent'), TCC('2')],
    [TC('Challenges'), TC('Containers'), TC('HelixCode, HelixQA'), TCC('3')],
    [TC('EventBus'), TC('(none)'), TC('HelixAgent, HelixLLM'), TCC('1')],
    [TC('VectorDB'), TC('(none)'), TC('HelixMemory, HelixLLM'), TCC('2')],
    [TC('ToolSchema'), TC('(none)'), TC('HelixAgent, HelixLLM'), TCC('2')],
    [TC('Agentic'), TC('EventBus, Concurrency'), TC('HelixAgent, HelixLLM'), TCC('3')],
    [TC('TOON'), TC('Agentic, Concurrency'), TC('HelixLLM'), TCC('4')],
    [TC('Watcher'), TC('Concurrency'), TC('HelixAgent'), TCC('3')],
    [TC('Conversation'), TC('EventBus, Messaging'), TC('HelixAgent, HelixLLM'), TCC('4')],
    [TC('Panoptic'), TC('(none)'), TC('HelixCode'), TCC('3')],
    [TC('LLMsVerifier'), TC('(none)'), TC('HelixCode, HelixLLM, HelixQA'), TCC('2')],
    [TC('HelixMemory'), TC('VectorDB, Concurrency'), TC('HelixCode, HelixAgent'), TCC('4')],
    [TC('HelixLLM'), TC('Agentic, TOON, VectorDB, Conversation'), TC('HelixCode, HelixAgent'), TCC('5')],
    [TC('HelixQA'), TC('Challenges, LLMsVerifier'), TC('HelixCode'), TCC('5')],
    [TC('HelixSpecifier'), TC('HelixMemory, Concurrency'), TC('HelixCode, HelixAgent'), TCC('5')],
    [TC('HelixAgent'), TC('All above'), TC('HelixCode'), TCC('6')],
]
story.append(make_table(data, [0.14, 0.22, 0.28, 0.14]))
story.append(Spacer(1, 18))

# ═══════════════════════════════════════════════════════════
# SECTION 13: CONCLUSION
# ═══════════════════════════════════════════════════════════
story.extend(H1('13. Conclusion and Next Actions'))
story.append(P('This document provides the comprehensive technical foundation for integrating the entire Helix ecosystem into a unified, fully-tested, production-ready AI CLI agent system. The analysis covers 19 repositories totaling millions of lines of code, identifies 50+ features to port from 60+ CLI agents, and provides a detailed 16-week phased implementation plan with 50+ specific tasks, each with deliverables and verification criteria.'))
story.append(Spacer(1, 6))
story.append(P('The immediate next actions are: (1) Execute Phase 0 on a machine with SSH access to initialize all git submodules recursively, (2) Fix the dual go.mod issue and add missing submodule entries, (3) Verify the complete build succeeds, and (4) Begin Phase 1 with tree-sitter repository mapping and SubAgent real LLM execution. Every feature, optimization, workaround, and innovation identified in this analysis must be ported without exception, and every implementation must pass Challenges, HelixQA, and LLMsVerifier verification before being considered complete.'))
story.append(Spacer(1, 6))
story.append(P('The testing strategy ensures no gaps, bluffing, or incomplete implementations through three independent verification pillars: Challenges for constitutional anti-bluff checks, HelixQA for autonomous LLM-driven QA sessions, and LLMsVerifier for model quality assurance. The 90% coverage gate at Phase 5 completion ensures production readiness. All UI, WEB, REST, CLI, TUI, and other applicative components must be fully covered across all interface types by Phase 4, ensuring no feature is inaccessible from any supported interface.'))

# ━━ Build ━━
doc.multiBuild(story)
print(f"PDF generated: {output_path}")
