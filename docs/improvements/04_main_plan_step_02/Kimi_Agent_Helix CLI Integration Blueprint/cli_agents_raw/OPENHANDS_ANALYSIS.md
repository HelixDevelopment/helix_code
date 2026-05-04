# OpenHands CLI Agent Analysis

> **Tier 1 Agent** - Open source autonomous software engineer
> **Source**: https://github.com/All-Hands-AI/OpenHands
> **Language**: Python
> **License**: MIT

## Overview

OpenHands is a community-driven AI-powered software development platform. It provides multiple interfaces including SDK, CLI, GUI, Cloud, and Enterprise editions.

**Key Stats:**
- SWE-bench score: 77.6%
- Multi-language support
- Active open source community

## Product Editions

### 1. OpenHands SDK
- Composable Python library
- Core agentic technology
- Scalable from local to cloud

### 2. OpenHands CLI
- Familiar CLI experience (like Claude Code/Codex)
- Works with Claude, GPT, and other LLMs

### 3. OpenHands Local GUI
- REST API + React SPA
- Devin/Jules-like experience
- Runs locally

### 4. OpenHands Cloud
- Hosted infrastructure
- Free tier with Minimax model
- GitHub/GitLab integration

### 5. OpenHands Enterprise
- Self-hosted in VPC
- Kubernetes deployment
- RBAC and permissions
- Slack/Jira/Linear integrations

## Core Architecture

```
openhands/
├── openhands/           # Core Python package
│   ├── agenthub/        # Agent implementations
│   ├── controller/      # Agent controller
│   ├── events/          # Event system
│   ├── runtime/         # Runtime environment
│   ├── server/          # REST API server
│   └── ...
├── frontend/            # React frontend
├── evaluation/          # Evaluation framework
└── enterprise/          # Enterprise features
```

## Key Features

### 1. Agent System
- Multiple agent implementations
- AgentHub for agent management
- Configurable agent behaviors

### 2. Runtime Environment
- Docker-based sandboxing
- Local runtime support
- Remote runtime support
- E2B integration

### 3. Event-Driven Architecture
- Event system for actions
- Streamable observation system
- Action-observation loop

### 4. Evaluation Framework
- SWE-bench integration
- Custom evaluation metrics
- Benchmark infrastructure

### 5. Multi-Modal Support
- Browser automation (headless)
- Image understanding
- Screenshot capabilities

### 6. Theory of Mind Module
- Research project for agent cognition
- Available separately

## Enterprise Features

- **Integrations**: Slack, Jira, Linear
- **Multi-user**: Team collaboration
- **RBAC**: Role-based access control
- **Kubernetes**: Enterprise deployment
- **Source-available**: Code visible, license required

## Documentation

- Docs: https://docs.openhands.dev
- SDK: https://docs.openhands.dev/sdk
- CLI: https://docs.openhands.dev/openhands/usage/run-openhands/cli-mode
- Paper: https://arxiv.org/abs/2511.03690

## HelixAgent Integration Points

| OpenHands Feature | HelixAgent Implementation |
|------------------|---------------------------|
| Agent System | SubAgent system |
| Runtime | Container adapter |
| Event System | EventBus module |
| Evaluation | Benchmark module |
| Browser | Browser automation tools |
| Multi-user | Team management |

## Porting Priority: HIGH

OpenHands' agent system, evaluation framework, and event-driven architecture are valuable for HelixAgent.
