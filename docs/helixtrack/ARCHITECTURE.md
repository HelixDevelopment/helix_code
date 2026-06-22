# HelixTrack Architecture Overview

## What is HelixTrack?

HelixTrack is a comprehensive, modern, open-source JIRA alternative for the free world. It's a multi-platform project management and issue tracking system with a microservices architecture.

**Repository:** git@github.com:Helix-Track/Everything.git
**Status:** V1, V2, V3 Production Ready

---

## System Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    HELIXTRACK ECOSYSTEM                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐   │
│  │   Web    │  │ Desktop  │  │ Android  │  │   iOS    │   │
│  │ Angular  │  │  Tauri   │  │  Kotlin  │  │ SwiftUI  │   │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘   │
│       │              │              │              │         │
│       └──────────────┴──────────────┴──────────────┘         │
│                          │                                   │
│                    REST API / WebSocket                       │
│                          │                                   │
│  ┌───────────────────────────────────────────────────────┐   │
│  │              CORE BACKEND (Go + Gin)                   │   │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐            │   │
│  │  │   Auth   │  │ Tickets  │  │ Projects │  ...        │   │
│  │  │  (JWT)   │  │  (CRUD)  │  │  (CRUD)  │            │   │
│  │  └──────────┘  └──────────┘  └──────────┘            │   │
│  │                                                       │   │
│  │  ┌──────────────────────────────────────────────┐     │   │
│  │  │         Database (SQLite / PostgreSQL)        │     │   │
│  │  │              71 Tables (V1-V3)                │     │   │
│  │  └──────────────────────────────────────────────┘     │   │
│  └───────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Component Inventory

### Core Backend (Go)
- **Location:** `core/Application/`
- **Framework:** Gin Gonic
- **Database:** SQLite (dev) / PostgreSQL (prod)
- **Tables:** 71 (V1-V3)
- **Auth:** JWT + RBAC
- **Features:** REST API, WebSocket, File attachments, Localization
- **Status:** Production Ready

### Web Client (Angular 19)
- **Location:** `web_client/`
- **Framework:** Angular 19 + Angular Material
- **Theme:** Lime Green #BCE63B, Teal #7AA590, Mint #B2E3C2
- **Features:**
  - Dashboard
  - Boards (Kanban)
  - Tickets (CRUD, workflows, relationships)
  - Chat
  - Documents
  - Projects
  - Teams
  - Users
  - Settings
  - Reports
  - Localization management
- **Testing:** Cypress, Playwright, AI QA

### Desktop Client (Tauri + Angular)
- **Location:** `desktop_client/`
- **Framework:** Tauri + Angular
- **Platforms:** macOS, Windows, Linux
- **Features:** Native menus, system tray, window management

### Android Client (Kotlin)
- **Location:** `android_client/`
- **Framework:** Kotlin + Jetpack Compose
- **SDK:** compileSdk 35, minSdk 29
- **DI:** Hilt
- **Navigation:** Navigation component

### iOS Client (SwiftUI)
- **Location:** `ios_client/`
- **Framework:** SwiftUI
- **Dependencies:** GRDB, Lottie, Markdown UI
- **Platforms:** iOS 16+, macOS 13+

---

## Submodules (50+)

HelixTrack incorporates 50+ submodules from vasic-digital and HelixDevelopment organizations:

### Core Services
- `auth` — Authentication service
- `database` — Database abstraction
- `storage` — File storage
- `messaging` — Messaging service
- `event_bus` — Event system
- `cache` — Caching layer

### AI/ML Services
- `llm_orchestrator` — LLM orchestration
- `llm_provider` — LLM provider abstraction
- `llms_verifier` — LLM verification
- `vision_engine` — Vision AI
- `embeddings` — Embedding service
- `rag` — RAG pipeline

### Infrastructure
- `containers` — Docker/container management
- `security` — Security tools
- `observability` — Monitoring
- `optimization` — Performance optimization

### Development Tools
- `helix_qa` — QA automation
- `challenges` — Test challenges
- `docs_chain` — Documentation sync
- `doc_processor` — Document processing

---

## Database Schema (71 Tables)

### V1 Core (57 tables)
- `project` — Projects/workspaces
- `ticket` — Issues/tasks
- `user` — Users
- `team` — Teams
- `board` — Kanban boards
- `comment` — Comments
- `attachment` — File attachments
- ... and 50 more

### V2 Extensions (14 tables)
- Enhanced features
- Workflow management
- Reporting

### V3 Advanced
- AI integration
- Advanced analytics

---

## API Design

### REST Endpoints
- `/api/v1/projects` — Project CRUD
- `/api/v1/tickets` — Ticket CRUD
- `/api/v1/users` — User management
- `/api/v1/teams` — Team management
- `/api/v1/boards` — Board management
- `/do` — Generic action endpoint

### WebSocket
- Real-time updates
- Chat
- Notifications

---

## Theme System

### Brand Colors
- **Primary:** Lime Green #BCE63B
- **Secondary:** Teal #7AA590
- **Accent:** Mint #B2E3C2

### CSS Custom Properties
```css
--helixtrack-primary: #BCE63B;
--helixtrack-secondary: #7AA590;
--helixtrack-accent: #B2E3C2;
--helixtrack-background: #FFFFFF;
--helixtrack-surface: #F8F9FA;
--helixtrack-text-primary: #1A1A1A;
```

---

## Integration Points

### CodeGraph
- Local SQLite code-knowledge-graph
- MCP server for CLI agents
- Indexes all source code

### OpenDesign
- UI design system
- Design tokens and themes
- MCP server for design generation

### Media Validator
- Validates media artifacts
- OCR/vision verification
- Evidence-based testing

---

## Cross-references
- [HelixTrack README](/Volumes/T7/Projects/helix_track/README.md)
- [Core CLAUDE.md](/Volumes/T7/Projects/helix_track/core/CLAUDE.md)
- [Web Client Guide](/Volumes/T7/Projects/helix_track/web_client/WEB_CLIENT_IMPLEMENTATION_GUIDE.md)
- [Desktop Client Guide](/Volumes/T7/Projects/helix_track/desktop_client/DESKTOP_IMPLEMENTATION_COMPLETE.md)

## Sources verified 2026-06-22: /Volumes/T7/Projects/helix_track/ (sibling HelixTrack source tree) , https://github.com/Helix-Track/Everything

REPO-STATE-DERIVED (per §11.4.99 the sources are the cross-referenced HelixTrack
source tree + upstream umbrella, following the `docs/ARCHITECTURE.md` precedent).
Cross-referenced on 2026-06-22:
- **Client roster CONFIRMED** against the upstream umbrella repo
  `github.com/Helix-Track/Everything` (Web / Desktop / Android / iOS / Aurora OS /
  Harmony OS / Core) AND the present sibling tree (`helix_track/{core,web_client,
  desktop_client,android_client,aurora_os_client,auth,Database}` all present).
- **Microservices/AI components CONFIRMED** by the umbrella's submodule set
  (Auth, Cache, storage, VectorDB, LLMOrchestrator, LLMProvider, embeddings, RAG).
- **Negative finding:** the umbrella README describes itself as a "wrapper /
  monorepo umbrella repo" and does NOT explicitly state "JIRA alternative for the
  free world" — that framing in this doc comes from the project's own component
  docs, not the umbrella landing page. The `git@github.com:Helix-Track/Everything.git`
  remote in the header is correct.
