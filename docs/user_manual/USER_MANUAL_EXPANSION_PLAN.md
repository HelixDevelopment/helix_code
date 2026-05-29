# User Manual Expansion Plan
**Goal:** Update documentation to cover new features and providers.

## 1. New Provider Guides
*   **Anthropic/Claude**:
    *   Configuration (`config.yaml` settings).
    *   Enabling "Extended Thinking".
    *   Using Prompt Caching.
*   **Google Gemini**:
    *   Setup and API Keys.
    *   Using Multimodal features (Images/Video).
*   **Enterprise Providers (AWS/Azure)**:
    *   IAM/Entra ID Authentication setup.
    *   Region selection and deployment.

## 2. Advanced Feature Guides
*   **Memory System**:
    *   "Choosing the Right Vector DB" (Comparison of Zep, Weaviate, etc.).
    *   Configuring persistence.
*   **Tool Usage**:
    *   "Safe Shell Execution": How to configure the sandbox.
    *   "Web Tools": How to enable and use search.

## 3. Developer Guides
*   **"Creating a Custom Provider"**: Step-by-step tutorial.
*   **"Adding a New Tool"**: Using the MCP SDK.

## 4. Troubleshooting
*   Common errors with SSH workers.
*   Debugging LLM connection issues.
*   Rate limit handling.

---

## Sources verified 2026-05-29: project go.mod + CLAUDE.md §3.1 (planning doc; no third-party operator instructions)

Reviewed against Constitution §11.4.99. This is a forward-looking **expansion
plan** — a checklist of guides still to be written (Anthropic/Gemini/AWS/Azure
setup, Memory System / vector-DB comparison, sandbox, custom-provider). It
contains **no executable third-party setup instructions and no version pins** to
cross-reference against a vendor doc yet — those obligations attach to the future
guides when they are authored, at which point each MUST carry its own §11.4.99
footer (fetched Anthropic / Google / AWS / Azure / Zep / Weaviate official docs).
Negative finding: nothing in this plan required (or could be) verified against a
vendor source this run. Version authority for any guide produced from this plan
is `go.mod` + CLAUDE.md §3.1 (Go 1.26 / go1.26.3 latest, PostgreSQL 15+, Redis
7+); provider/model IDs remain LLMsVerifier-runtime-sourced per CONST-036.
