# HelixCode User Manual - Quick Index

**Version**: 2.1 | **Date**: May 8, 2026 | **Total**: 7,549 lines

---

## 📚 Main Documentation

### [README.md](README.md) - 3,027 lines
**Comprehensive user manual covering all HelixCode features**

**Quick Navigation**:
- [Introduction](#1-introduction) - What is HelixCode, architecture, use cases
- [Installation](#2-installation) - Quick start, production, Docker, Kubernetes
- [First Project](#3-first-project) - Create your first application
- [Configuration](#4-configuration) - YAML config, environment variables
- [LLM Providers](#5-llm-providers-overview) - All 14+ providers documented
  - [Anthropic Claude](#61-anthropic-claude) - Extended thinking, caching
  - [Google Gemini](#62-google-gemini) - 2M token context
  - [OpenAI](#63-openai) - GPT-4.1, O1 reasoning
  - [AWS Bedrock](#64-aws-bedrock) - Enterprise multi-model
  - [Azure OpenAI](#65-azure-openai) - Entra ID, compliance
  - [VertexAI](#66-google-vertexai) - GCP unified platform
  - [Groq](#67-groq) - Ultra-fast 500+ tok/s
  - [Mistral](#68-mistral) - EU data residency
  - [XAI Grok](#71-xai-grok) - Free tier
  - [OpenRouter](#72-openrouter) - 100+ models
  - [GitHub Copilot](#73-github-copilot) - Free with subscription
  - [Qwen](#74-qwen) - 2K free requests/day
  - [Ollama](#75-ollama) - 100% local
  - [Llama.cpp](#76-llamacpp) - Direct inference
- [Core Tools](#part-iv-core-tools)
  - [File System](#8-file-system-tools)
  - [Shell](#9-shell-execution)
  - [Browser](#10-browser-automation)
  - [Web](#11-web-tools)
  - [Voice](#12-voice-to-code)
  - [Codebase Mapping](#13-codebase-mapping)
- [Advanced Workflows](#part-v-advanced-workflows)
  - [Plan Mode](#14-plan-mode)
  - [Multi-File Editing](#15-multi-file-editing)
  - [Git Auto-Commit](#16-git-auto-commit)
  - [Context Compression](#17-context-compression)
  - [Tool Confirmation](#18-tool-confirmation)
  - [Snapshots](#19-checkpoint-snapshots)
  - [Autonomy Modes](#20-autonomy-modes)
  - [Vision Auto-Switch](#21-vision-auto-switch)
- [Distributed Computing](#part-vi-distributed-computing)
- [Memory System](#35-memory-system)
- [Sandbox Configuration](#95-sandbox-configuration)
- [Search Provider](#114-search-provider-configuration)
- [Security](#29-security-best-practices)
- [Troubleshooting](#30-troubleshooting)
  - [Common Issues](#301-common-issues)
  - [SSH Worker Issues](#302-ssh-worker-connection-issues)
  - [LLM Connection Issues](#303-llm-connection-issues)
  - [Rate Limit Handling](#304-rate-limit-handling)
- [CLI Reference](#31-cli-command-reference)
- [API Reference](#32-api-reference)
- [FAQ](#33-faq)

---

## 🎓 Tutorials (10 Comprehensive Guides) - 2,514 lines

### Beginner Level

#### [Tutorial 1: Building a Web App from Scratch](tutorials/Tutorial_1_Building_Web_App.md) - 1,245 lines
⏱️ **45-60 minutes**
- Complete REST API with authentication
- PostgreSQL database design
- Three-layer architecture
- Docker deployment
- **Output**: Working Task Management API

#### [Tutorial 3: Using Multiple AI Providers](tutorials/Tutorial_3_Multiple_AI_Providers.md) - 110 lines
⏱️ **20 minutes**
- Provider selection strategy
- Cost optimization
- Automatic fallback
- **Outcome**: 70% cost reduction

#### [Tutorial 5: Voice-to-Code Workflow](tutorials/Tutorial_5_Voice_to_Code.md) - 118 lines
⏱️ **15 minutes**
- Hands-free coding
- Voice commands
- Whisper transcription
- **Use Case**: Accessibility, multitasking

### Intermediate Level

#### [Tutorial 2: Refactoring a Large Codebase](tutorials/Tutorial_2_Refactoring_Large_Codebase.md) - 133 lines
⏱️ **30-45 minutes**
- Codebase mapping (247 files)
- Multi-file atomic edits
- Checkpoint snapshots
- **Result**: 95% test coverage in 30 min

#### [Tutorial 4: Browser Automation for Testing](tutorials/Tutorial_4_Browser_Automation.md) - 149 lines
⏱️ **30 minutes**
- E2E test automation
- Visual regression testing
- Web scraping
- **Technology**: Chrome/Chromium

#### [Tutorial 6: Multi-File Atomic Edits](tutorials/Tutorial_6_Multi_File_Atomic_Edits.md) - 134 lines
⏱️ **20 minutes**
- Transaction-based editing
- Atomic commits with rollback
- Cross-file refactoring
- **Safety**: All-or-nothing

### Advanced Level

#### [Tutorial 7: Distributed Development with Workers](tutorials/Tutorial_7_Distributed_Development.md) - 165 lines
⏱️ **30 minutes**
- SSH worker setup
- Auto-installation
- Task distribution
- **Performance**: 3x speedup

#### [Tutorial 8: Using Plan Mode for Complex Projects](tutorials/Tutorial_8_Using_Plan_Mode.md) - 301 lines
⏱️ **45 minutes**
- Two-phase planning
- Option generation
- Step-by-step execution
- **Example**: Real-time chat app (15K LOC in 9 hours)

### Advanced Level (continued)

#### [Tutorial 9: Creating a Custom LLM Provider](tutorials/Tutorial_9_Custom_Provider.md) - 79 lines
⏱️ **30 minutes**
- Implement the `Provider` interface
- Provider scaffold (Name, Generate, GenerateStream)
- Configuration & registration
- **Skill**: Extending HelixCode

#### [Tutorial 10: Adding a New Tool via MCP SDK](tutorials/Tutorial_10_Adding_a_Tool.md) - 80 lines
⏱️ **30 minutes**
- Define tool schema with JSON-RPC
- Implement Execute() business logic
- Register in MCP registry
- **Skill**: MCP server extension

---

## ⚙️ Configuration Examples - 740 lines

### [basic_config.yaml](examples/basic_config.yaml) - 57 lines
**Purpose**: Development and small projects
- Local Ollama provider
- Minimal dependencies
- Debug logging
- **Best For**: Learning, prototyping

### [provider_configs.yaml](examples/provider_configs.yaml) - 174 lines
**Purpose**: Complete reference for all AI providers
- 14+ provider configurations
- API key setup
- Advanced features
- **Best For**: Multi-provider setups

### [enterprise_setup.yaml](examples/enterprise_setup.yaml) - 248 lines
**Purpose**: Production enterprise deployment
- High availability
- Security (SSO, encryption, audit)
- Monitoring (Prometheus, Grafana)
- Auto-scaling
- Compliance (GDPR, SOX, HIPAA)
- **Best For**: Large organizations

### [multi_worker_setup.yaml](examples/multi_worker_setup.yaml) - 261 lines
**Purpose**: Distributed worker orchestration
- 3 worker pools (build, GPU, test)
- 10+ workers (x86, ARM, macOS)
- Auto-scaling with cloud
- Cost optimization
- **Best For**: Large-scale parallel development

---

## 📊 Coverage Matrix

### Providers Documented (14+)
| Provider | Free Tier | Context | Speed | Best For |
|----------|-----------|---------|-------|----------|
| Anthropic Claude ⭐ | ❌ | 200K | Medium | Complex reasoning |
| Google Gemini ⭐ | ❌ | 2M | Fast | Large codebases |
| OpenAI | ❌ | 128K-1M | Medium | General purpose |
| AWS Bedrock | ❌ | Varies | Medium | Enterprise |
| Azure OpenAI | ❌ | 128K | Medium | Microsoft stack |
| VertexAI | ❌ | 2M | Fast | GCP integration |
| Groq | ❌ | 32K | **Ultra** | Fast iterations |
| Mistral | ❌ | 128K | Fast | EU compliance |
| XAI Grok | ✅ | 128K | Medium | Experimentation |
| OpenRouter | ✅ | Varies | Varies | Model variety |
| GitHub Copilot | ✅* | 128K | Fast | GitHub users |
| Qwen | ✅** | 32K | Fast | Chinese language |
| Ollama | ✅ | Varies | Medium | Privacy |
| Llama.cpp | ✅ | Varies | Fast | Performance |

*Free with GitHub subscription
**2,000 free requests/day

### Tools Covered (6)
- ✅ File System (read, write, edit, search)
- ✅ Shell Execution (sandboxed)
- ✅ Browser Automation
- ✅ Web Tools (search, fetch)
- ✅ Voice-to-Code
- ✅ Codebase Mapping (30+ languages)

### Workflows Covered (8)
- ✅ Plan Mode
- ✅ Multi-File Editing
- ✅ Git Auto-Commit
- ✅ Context Compression
- ✅ Tool Confirmation
- ✅ Checkpoint Snapshots
- ✅ Autonomy Modes (5 levels)
- ✅ Vision Auto-Switch

---

## 🚀 Quick Start Paths

### Path 1: Get Started in 5 Minutes
1. Read [README: Introduction](#1-introduction)
2. Follow [README: Quick Start Installation](#22-quick-start-installation)
3. Complete [Tutorial 3: Multiple AI Providers](tutorials/Tutorial_3_Multiple_AI_Providers.md)

### Path 2: Build Your First App (1 Hour)
1. Setup [basic_config.yaml](examples/basic_config.yaml)
2. Follow [Tutorial 1: Building a Web App](tutorials/Tutorial_1_Building_Web_App.md)
3. Deploy with Docker

### Path 3: Master Advanced Features (3 Hours)
1. Complete [Tutorial 2: Refactoring](tutorials/Tutorial_2_Refactoring_Large_Codebase.md)
2. Learn [Tutorial 6: Multi-File Edits](tutorials/Tutorial_6_Multi_File_Atomic_Edits.md)
3. Master [Tutorial 8: Plan Mode](tutorials/Tutorial_8_Using_Plan_Mode.md)

### Path 4: Enterprise Deployment (1 Day)
1. Review [README: Security Best Practices](#29-security-best-practices)
2. Configure [enterprise_setup.yaml](examples/enterprise_setup.yaml)
3. Setup [Tutorial 7: Distributed Workers](tutorials/Tutorial_7_Distributed_Development.md)
4. Deploy to Kubernetes

---

## 📖 Documentation Structure

```
docs/user_manual/
├── README.md                     # 3,027 lines - Comprehensive manual
├── SUMMARY.md                    # 632 lines - Creation summary
├── INDEX.md                      # This file - Quick navigation
│
├── tutorials/                    # 2,514 lines - 10 step-by-step guides
│   ├── Tutorial_1_Building_Web_App.md                (1,245 lines)
│   ├── Tutorial_2_Refactoring_Large_Codebase.md     (133 lines)
│   ├── Tutorial_3_Multiple_AI_Providers.md          (110 lines)
│   ├── Tutorial_4_Browser_Automation.md             (149 lines)
│   ├── Tutorial_5_Voice_to_Code.md                  (118 lines)
│   ├── Tutorial_6_Multi_File_Atomic_Edits.md        (134 lines)
│   ├── Tutorial_7_Distributed_Development.md        (165 lines)
│   ├── Tutorial_8_Using_Plan_Mode.md                (301 lines)
│   ├── Tutorial_9_Custom_Provider.md                (79 lines)
│   └── Tutorial_10_Adding_a_Tool.md                 (80 lines)
│
├── examples/                     # 740 lines - 4 production configs
│   ├── basic_config.yaml                            (57 lines)
│   ├── provider_configs.yaml                        (174 lines)
│   ├── enterprise_setup.yaml                        (248 lines)
│   └── multi_worker_setup.yaml                      (261 lines)
│
├── chapters/                     # Reserved for future expansion
└── images/                       # Reserved for diagrams

Total: 7,549 lines of comprehensive documentation
```

---

## 🎯 Use Case Index

### By Role

#### **Software Developer**
- [Tutorial 1: Building Web App](tutorials/Tutorial_1_Building_Web_App.md)
- [Tutorial 6: Multi-File Edits](tutorials/Tutorial_6_Multi_File_Atomic_Edits.md)
- [README: Core Tools](#part-iv-core-tools)

#### **DevOps Engineer**
- [Tutorial 7: Distributed Workers](tutorials/Tutorial_7_Distributed_Development.md)
- [enterprise_setup.yaml](examples/enterprise_setup.yaml)
- [multi_worker_setup.yaml](examples/multi_worker_setup.yaml)

#### **QA/Test Engineer**
- [Tutorial 4: Browser Automation](tutorials/Tutorial_4_Browser_Automation.md)
- [README: Browser Automation](#10-browser-automation)

#### **Tech Lead/Architect**
- [Tutorial 8: Plan Mode](tutorials/Tutorial_8_Using_Plan_Mode.md)
- [Tutorial 2: Refactoring](tutorials/Tutorial_2_Refactoring_Large_Codebase.md)
- [README: Architecture Overview](#13-architecture-overview)

#### **Accessibility Specialist**
- [Tutorial 5: Voice-to-Code](tutorials/Tutorial_5_Voice_to_Code.md)
- [README: Voice-to-Code](#12-voice-to-code)

#### **Enterprise Admin**
- [enterprise_setup.yaml](examples/enterprise_setup.yaml)
- [README: Security Best Practices](#29-security-best-practices)
- [README: Distributed Computing](#part-vi-distributed-computing)

### By Task

#### **Code Generation**
- [Tutorial 1: Building Web App](tutorials/Tutorial_1_Building_Web_App.md)
- [README: LLM Providers](#5-llm-providers-overview)

#### **Refactoring**
- [Tutorial 2: Refactoring](tutorials/Tutorial_2_Refactoring_Large_Codebase.md)
- [Tutorial 6: Multi-File Edits](tutorials/Tutorial_6_Multi_File_Atomic_Edits.md)

#### **Testing**
- [Tutorial 4: Browser Automation](tutorials/Tutorial_4_Browser_Automation.md)
- [README: Browser Automation](#10-browser-automation)

#### **Deployment**
- [enterprise_setup.yaml](examples/enterprise_setup.yaml)
- [README: Docker Deployment](#24-docker-deployment)
- [README: Kubernetes Deployment](#25-kubernetes-deployment)

#### **Cost Optimization**
- [Tutorial 3: Multiple Providers](tutorials/Tutorial_3_Multiple_AI_Providers.md)
- [README: Cost Comparison](#52-cost-comparison)

---

## 📞 Support Resources

### Documentation
- **Main Manual**: [README.md](README.md)
- **Creation Summary**: [SUMMARY.md](SUMMARY.md)
- **This Index**: INDEX.md

### External Resources
- **GitHub**: https://github.com/your-org/helixcode
- **Documentation**: https://docs.helixcode.dev
- **Community**: https://community.helixcode.dev
- **Issues**: https://github.com/your-org/helixcode/issues

### Getting Help
1. Check [FAQ](#33-faq) in main README
2. Search [Troubleshooting](#30-troubleshooting) section
3. Review relevant tutorial
4. Check example configurations
5. Post in community forum
6. Create GitHub issue

---

## 🔄 Version History

### Version 2.0 (November 6, 2025)
- ✅ Complete rewrite with 6,754 lines
- ✅ 14+ AI providers documented
- ✅ 8 comprehensive tutorials
- ✅ 4 production-ready configurations
- ✅ All features covered

### Version 2.1 (May 8, 2026)
- ✅ Expanded to 7,549 lines
- ✅ 10 tutorials (added Custom Provider, MCP Tool)
- ✅ Memory System documentation
- ✅ Sandbox & Search Provider configuration
- ✅ Enhanced troubleshooting coverage

### Planned Updates
- Video tutorials
- Architecture diagrams
- Interactive examples
- Case studies
- Performance benchmarks

---

## 📝 Quick Reference

### Most Common Commands
```bash
# Setup
helixcode llm provider set anthropic
helixcode llm provider set gemini --model gemini-2.5-flash

# Development
helixcode plan "Create REST API"
helixcode generate "Add user authentication"
helixcode test --coverage
helixcode commit --auto

# Advanced
helixcode snapshot create "Before refactoring"
helixcode edit transaction start
helixcode worker add --host worker1.example.com

# Troubleshooting
helixcode health
helixcode llm providers
helixcode worker list
```

### Most Useful Configurations
- **Development**: [basic_config.yaml](examples/basic_config.yaml)
- **All Providers**: [provider_configs.yaml](examples/provider_configs.yaml)
- **Production**: [enterprise_setup.yaml](examples/enterprise_setup.yaml)
- **Distributed**: [multi_worker_setup.yaml](examples/multi_worker_setup.yaml)

---

**Last Updated**: May 8, 2026
**Documentation Version**: 2.1
**Total Lines**: 7,549
**Status**: Complete ✅
