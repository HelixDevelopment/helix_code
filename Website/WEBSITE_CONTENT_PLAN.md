# HelixCode Website Content Plan
**Status:** Draft
**Location:** `/Website`

## 1. Site Architecture
The website will be a static site generated via Hugo or Docusaurus (TBD), hosted on GitHub Pages or Netlify.

### Structure
*   **Home** (`/`): Hero section, Value Prop, Quick Start, Testimonials.
*   **Features** (`/features`): Deep dive into Distributed Workers, MCP, Memory.
*   **Docs** (`/docs`): Full User Manual & API Reference.
*   **Blog** (`/blog`): Release notes, tutorials, case studies.
*   **Community** (`/community`): Links to Discord, GitHub, Contributing guide.

## 2. Page Content Outlines

### 2.1. Home Page
*   **Hero**: "The Enterprise-Grade Distributed AI Development Platform."
*   **Sub-hero**: "Orchestrate AI agents across local and cloud infrastructure. Secure, Scalable, Open."
*   **CTA**: "Get Started" (Link to Docs) | "View on GitHub".
*   **Key Features Grid**:
    *   Distributed Worker Pools.
    *   Multi-Provider Support (Anthropic, OpenAI, Local).
    *   Long-term Memory (Vector DBs).
    *   Secure Sandboxing.

### 2.2. Features Page
*   **Section 1: The Worker Pool**: Explain SSH-based distribution.
*   **Section 2: The Brain**: Explain the Memory System (Zep, Weaviate).
*   **Section 3: The Tools**: Explain MCP and the Tool Ecosystem.
*   **Section 4: Enterprise Ready**: Security, Auth, Monitoring.

### 2.3. Documentation (Docs)
*   **Getting Started**: Installation, Config.
*   **Core Concepts**: Agents, Tasks, Workflows.
*   **Guides**: "Building your first App", "Deploying to Production".
*   **API Reference**: GoDoc links, REST API Swagger.

## 3. Implementation Steps
1.  Initialize Static Site Generator (e.g., `hugo new site .`).
2.  Select "DocuAPI" or similar technical theme.
3.  Migrate existing Markdown docs from `Documentation/` to `content/docs/`.
4.  Write copy for Home and Features pages.
5.  Generate assets (screenshots, diagrams).
6.  Setup CI/CD for auto-deployment.
