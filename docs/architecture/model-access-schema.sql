-- =============================================================================
-- model-access-schema.sql — Model-Access data model (DESIGN DDL)
-- -----------------------------------------------------------------------------
-- Revision      : 1
-- Created       : 2026-06-10
-- Last modified : 2026-06-10
-- Status        : DESIGN — target relational model; NOT a deployed migration.
-- Maintainer    : DOCUMENTATION subagent (READ-ONLY on code)
-- Target engine : PostgreSQL 15+ (project stack: pgx/v5). Notes for SQLite given
--                 inline where the project's workable-items DB (§11.4.93) differs.
-- Authority     : CONST-036 (LLMsVerifier is the SINGLE SOURCE OF TRUTH for model
--                 + provider metadata, verification status, scoring); CONST-040
--                 (capability flags MUST come from VerificationResult); §11.4
--                 anti-bluff. Verifier-owned columns are marked SOT=verifier.
-- =============================================================================
--
-- HONESTY / SCOPE NOTE (§11.4 / §11.4.123):
--   This schema is a DESIGN artifact. It models the SP1/SP2/SP4 flows so they can
--   be persisted/cached/reported on. As of 2026-06-10 the live code consumes the
--   verifier over REST (helix_code/internal/verifier/client.go GET /api/models) and
--   holds results in memory/cache — there is NO HelixCode-side table named below in
--   production. SP2·T2.4.3 flags an OPTIONAL catalog cache/view; OP-1 must confirm
--   whether the catalog is persisted at all. Treat every CREATE TABLE here as
--   "persist THIS shape IF/when the design persists it", not as an existing object.
--
-- SOURCE-OF-TRUTH (SOT) tag per column group:
--   SOT=verifier  -> CONST-036/040: authoritative value is LLMsVerifier's; any local
--                    row is a CACHE of the verifier's truth, refreshed within the
--                    CONST-037 (<=24h verified) / CONST-038 (<=60s status) windows.
--   SOT=local     -> HelixCode-owned operational state (key-presence, enable flags,
--                    CLI-bridge wiring) that the verifier does not own.
--
-- Field grounding (real Go structs read 2026-06-10):
--   helix_code/internal/verifier/types.go  : VerifiedModel, ProviderStatus,
--                                            VerificationResult, RateLimitStatus,
--                                            CooldownInfo.
--   submodules/llms_verifier/llm-verifier/llmverifier/models.go : canonical
--                                            VerificationResult / ModelInfo / Capabilities.
--   submodules/helix_agent/internal/verifier/provider_types.go:348-385 :
--                                            SupportedProviders.EnvVars (alias table).
-- =============================================================================


-- -----------------------------------------------------------------------------
-- TABLE: providers
-- SOT: mixed. Identity/type/display = verifier-derived (CONST-036). enabled/
--      key-present/priority = local operational state (SP1 key-presence gate).
-- Mirrors: VerifiedModel.Provider/ProviderType (types.go), ProviderStatus
--          (types.go), ProviderRegistry.ListProviders (provider_registry.go:82).
-- -----------------------------------------------------------------------------
CREATE TABLE providers (
    provider            TEXT PRIMARY KEY,                       -- lowercase id, e.g. 'openai','anthropic','openrouter' [SOT=verifier]
    provider_type       TEXT NOT NULL,                          -- VerifiedModel.ProviderType [SOT=verifier]
    display_name        TEXT NOT NULL DEFAULT '',               -- ProviderStatus.DisplayName [SOT=verifier]
    -- Health/score snapshot (ProviderStatus, types.go) — CACHE of verifier truth:
    verified            BOOLEAN NOT NULL DEFAULT FALSE,         -- ProviderStatus.Verified [SOT=verifier]
    healthy             BOOLEAN NOT NULL DEFAULT FALSE,         -- ProviderStatus.Healthy [SOT=verifier]
    status              TEXT NOT NULL DEFAULT 'unknown',        -- unknown|healthy|degraded|unhealthy|offline [SOT=verifier]
    score               DOUBLE PRECISION NOT NULL DEFAULT 0,    -- ProviderStatus.Score [SOT=verifier]
    tier                INTEGER NOT NULL DEFAULT 0,             -- ProviderStatus.Tier (1..5) [SOT=verifier]
    model_count         INTEGER NOT NULL DEFAULT 0,             -- ProviderStatus.ModelCount [SOT=verifier]
    uptime_pct          DOUBLE PRECISION NOT NULL DEFAULT 0,    -- ProviderStatus.UptimePct [SOT=verifier]
    latency_ms          BIGINT NOT NULL DEFAULT 0,              -- ProviderStatus.Latency (ns->ms) [SOT=verifier]
    last_checked        TIMESTAMPTZ,                            -- ProviderStatus.LastChecked [SOT=verifier]
    -- Local operational state (SP1) — verifier does NOT own these:
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,          -- config.Providers[p].Enabled (adapter.go:277-289) [SOT=local]
    key_present          BOOLEAN NOT NULL DEFAULT FALSE,        -- PresentProviders() result (SP1·T1.1.2) [SOT=local]
    priority            INTEGER NOT NULL DEFAULT 0,             -- ProviderStatus.Priority [SOT=local]
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE providers IS
  'One row per provider HelixCode knows. Identity/health/score are a CACHE of the LLMsVerifier source of truth (CONST-036). enabled/key_present/priority are HelixCode-local operational state driving the SP1 key-presence availability gate.';


-- -----------------------------------------------------------------------------
-- TABLE: api_key_sources
-- SOT: local. Records WHERE a provider's key was recognized (NOT the secret).
-- CONST-042/§12.1: NO secret value is ever stored — only the env-var NAME, the
--   source kind, and a presence boolean. loader.go never logs values; this table
--   never holds them either.
-- Alias table mirrors helix_agent SupportedProviders.EnvVars (provider_types.go:357
--   anthropic {ANTHROPIC_API_KEY,CLAUDE_API_KEY}; :381 gemini {GEMINI_API_KEY,
--   GOOGLE_API_KEY,ApiKey_Gemini}; ...). SP1·T1.1.2 lifts a decoupled copy.
-- -----------------------------------------------------------------------------
CREATE TABLE api_key_sources (
    id                  BIGSERIAL PRIMARY KEY,
    provider            TEXT NOT NULL REFERENCES providers(provider) ON DELETE CASCADE,
    env_var_alias       TEXT NOT NULL,                          -- e.g. 'ANTHROPIC_API_KEY' or alias 'CLAUDE_API_KEY' [SOT=local]
    source_kind         TEXT NOT NULL                           -- where recognized:
                          CHECK (source_kind IN ('shell_export','api_keys_sh','dotenv')),
    is_present          BOOLEAN NOT NULL DEFAULT FALSE,         -- non-empty AND !isPlaceholder (SP1·T1.1.2) [SOT=local]
    is_placeholder      BOOLEAN NOT NULL DEFAULT FALSE,         -- value matched a placeholder pattern (rejected) [SOT=local]
    is_faulty           BOOLEAN NOT NULL DEFAULT FALSE,         -- faulty-key registry (llms_verifier/api_keys) [SOT=local]
    recognized_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- INVARIANT: no secret column exists by design (CONST-042). Do NOT add one.
    UNIQUE (provider, env_var_alias, source_kind)
);
COMMENT ON TABLE api_key_sources IS
  'Records that a provider key was RECOGNIZED and from which source/alias (shell export, api_keys.sh, or .env). CONST-042: never stores the secret value itself — only the env-var name + a presence flag. Drives providers.key_present.';


-- -----------------------------------------------------------------------------
-- TABLE: models
-- SOT: verifier (CONST-036). One row per model the verifier reports. Identity +
--      context/output sizes + open-source/deprecated/tier are verifier-owned.
-- Mirrors VerifiedModel (helix_code/internal/verifier/types.go).
-- -----------------------------------------------------------------------------
CREATE TABLE models (
    id                  TEXT NOT NULL,                          -- VerifiedModel.ID (model id) [SOT=verifier]
    provider            TEXT NOT NULL REFERENCES providers(provider) ON DELETE CASCADE,
    name                TEXT NOT NULL,                          -- VerifiedModel.Name [SOT=verifier]
    display_name        TEXT NOT NULL DEFAULT '',               -- VerifiedModel.DisplayName [SOT=verifier]
    context_window_tokens INTEGER NOT NULL DEFAULT 0,           -- VerifiedModel.ContextSize [SOT=verifier]
    max_output_tokens   INTEGER NOT NULL DEFAULT 0,             -- VerifiedModel.MaxOutputTokens [SOT=verifier]
    open_source         BOOLEAN NOT NULL DEFAULT FALSE,         -- VerifiedModel.OpenSource [SOT=verifier]
    deprecated          BOOLEAN NOT NULL DEFAULT FALSE,         -- VerifiedModel.Deprecated [SOT=verifier]
    tier                INTEGER NOT NULL DEFAULT 0,             -- VerifiedModel.Tier (1=Premium..5=Free) [SOT=verifier]
    input_token_cost    DOUBLE PRECISION NOT NULL DEFAULT 0,    -- VerifiedModel.CostPerInputToken [SOT=verifier]
    output_token_cost   DOUBLE PRECISION NOT NULL DEFAULT 0,    -- VerifiedModel.CostPerOutputToken [SOT=verifier]
    source              TEXT NOT NULL DEFAULT 'verifier'        -- VerifiedModel.Source: verifier|cache|fallback [SOT=verifier]
                          CHECK (source IN ('verifier','cache','fallback')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, id)
);
COMMENT ON TABLE models IS
  'One row per model from LLMsVerifier (CONST-036 single source of truth). A local row is a CACHE refreshed within CONST-037 (verified<=24h). source=fallback marks the constitutional hardcoded fallback list — it MUST NEVER be presented to a user as "working" (CONST-035/§11.4 PASS-bluff guard).';


-- -----------------------------------------------------------------------------
-- TABLE: verification_results
-- SOT: verifier (CONST-036/040). The authoritative verification + scoring record
--      per model. THIS is the table the "working model" predicate reads.
-- Mirrors VerifiedModel verification fields + VerificationResult (types.go) and
--      the canonical llms_verifier VerificationResult/ScoreDetails (models.go).
-- -----------------------------------------------------------------------------
CREATE TABLE verification_results (
    id                  BIGSERIAL PRIMARY KEY,
    provider            TEXT NOT NULL,
    model_id            TEXT NOT NULL,                          -- VerificationResult.ModelID [SOT=verifier]
    -- Status + verified flag (the working-model gate, SP1 §3):
    verified            BOOLEAN NOT NULL DEFAULT FALSE,         -- VerifiedModel.Verified [SOT=verifier]
    verification_status TEXT NOT NULL DEFAULT 'pending'         -- pending|verified|failed|rate_limited [SOT=verifier]
                          CHECK (verification_status IN ('pending','verified','failed','rate_limited')),
    run_status          TEXT NOT NULL DEFAULT 'completed'       -- VerificationResult.Status: started|completed|failed [SOT=verifier]
                          CHECK (run_status IN ('started','completed','failed')),
    model_exists        BOOLEAN,                                -- VerificationResult.ModelExists (nullable) [SOT=verifier]
    responsive          BOOLEAN,                                -- VerificationResult.Responsive (nullable) [SOT=verifier]
    overloaded          BOOLEAN,                                -- VerificationResult.Overloaded (nullable) [SOT=verifier]
    -- Scores (the min-score filter reads overall_score; SP1·T1.3.1 vs GetMinAcceptableScore):
    overall_score           DOUBLE PRECISION NOT NULL DEFAULT 0,  -- VerifiedModel.OverallScore [SOT=verifier]
    code_capability_score   DOUBLE PRECISION NOT NULL DEFAULT 0,  -- [SOT=verifier]
    responsiveness_score    DOUBLE PRECISION NOT NULL DEFAULT 0,  -- [SOT=verifier]
    reliability_score       DOUBLE PRECISION NOT NULL DEFAULT 0,  -- [SOT=verifier]
    feature_richness_score  DOUBLE PRECISION NOT NULL DEFAULT 0,  -- [SOT=verifier]
    value_proposition_score DOUBLE PRECISION NOT NULL DEFAULT 0,  -- [SOT=verifier]
    latency_ms          BIGINT NOT NULL DEFAULT 0,              -- VerifiedModel.Latency [SOT=verifier]
    error               TEXT,                                   -- VerificationResult.Error [SOT=verifier]
    last_verified       TIMESTAMPTZ,                            -- VerifiedModel.LastVerified / CompletedAt [SOT=verifier]
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (provider, model_id) REFERENCES models(provider, id) ON DELETE CASCADE,
    UNIQUE (provider, model_id, created_at)
);
COMMENT ON TABLE verification_results IS
  'Authoritative LLMsVerifier verification + scoring per model (CONST-036). The SP1 "working model" predicate = verified=TRUE AND verification_status=''verified'' AND overall_score >= min_acceptable_score (HELIX_VERIFIER_MIN_SCORE, default 6.0; adapter.go:175, currently loaded-but-unapplied = defect D-4). CONST-038: status freshness <=60s.';

CREATE INDEX idx_verif_working
    ON verification_results (provider, model_id)
    WHERE verified = TRUE AND verification_status = 'verified';


-- -----------------------------------------------------------------------------
-- TABLE: capabilities
-- SOT: verifier (CONST-040 — capability flags MUST come from VerificationResult,
--      NEVER hardcoded). Normalized one-row-per-(model,capability) so the closed
--      set can grow without ALTERs; mirrors VerifiedModel.Supports* booleans +
--      VerificationResult.Supports*/Code* and Capabilities (models.go).
-- -----------------------------------------------------------------------------
CREATE TABLE capabilities (
    id                  BIGSERIAL PRIMARY KEY,
    provider            TEXT NOT NULL,
    model_id            TEXT NOT NULL,
    capability          TEXT NOT NULL,                          -- closed set below [SOT=verifier]
    supported           BOOLEAN NOT NULL DEFAULT FALSE,         -- [SOT=verifier]
    -- Provenance: how this flag was determined (anti-bluff; CLI-bridge may runtime-probe):
    provenance          TEXT NOT NULL DEFAULT 'verifier'        -- verifier|runtime_probe|unknown
                          CHECK (provenance IN ('verifier','runtime_probe','unknown')),
    FOREIGN KEY (provider, model_id) REFERENCES models(provider, id) ON DELETE CASCADE,
    UNIQUE (provider, model_id, capability),
    -- Closed capability vocabulary (extend by adding, never silently contract):
    CHECK (capability IN (
        'streaming','tool_use','functions','code_generation','vision','audio',
        'video','reasoning','embeddings','json_mode',          -- VerifiedModel.Supports* (types.go)
        'code_debugging','code_optimization','test_generation',
        'documentation_generation','architecture_design',
        'security_assessment','pattern_recognition',           -- VerificationResult.* (types.go)
        'mcp','lsp','acp','rag','memory','generative'          -- power-features (CONST-040; SP4 CLI-bridge passthrough)
    ))
);
COMMENT ON TABLE capabilities IS
  'Per-model capability flags sourced from LLMsVerifier VerificationResult (CONST-040 — NEVER hardcoded). provenance distinguishes verifier-sourced flags from CLI-bridge runtime probes (SP4·T4.3.2 marks runtime_probe honestly when the verifier has no entry for a CLI agent).';


-- -----------------------------------------------------------------------------
-- TABLE: cli_agent_bridges
-- SOT: local. SP4 CLI-agent providers. One row per bridgeable CLI coding agent
--      (claude/qwen/opencode/gemini/crush/codex/goose/copilot tier-1; aider/
--      plandex/forge fallback). Identity is local wiring; its MODELS join into
--      `models` via catalog_entries (kind='cli'). [PLANNED — SP4·P4.1/P4.2.]
-- Mirrors AgentSpec / resolveBinary (SP4 plan §6) + analysis-D §2 versions.
-- -----------------------------------------------------------------------------
CREATE TABLE cli_agent_bridges (
    agent               TEXT PRIMARY KEY,                       -- e.g. 'claude','qwen','opencode' [SOT=local]
    display_name        TEXT NOT NULL DEFAULT '',               -- [SOT=local]
    binary_name         TEXT NOT NULL,                          -- AgentSpec.Binary for exec.LookPath [SOT=local]
    tier                INTEGER NOT NULL DEFAULT 1,             -- 1=system-installed-now, 2=fallback-build [SOT=local]
    resolved_path       TEXT,                                   -- chosen binary path (GetHealth provenance) [SOT=local]
    resolution          TEXT NOT NULL DEFAULT 'unresolved'      -- which path was chosen:
                          CHECK (resolution IN ('primary_system','fallback_submodule','unresolved')),
    detected_version    TEXT,                                   -- captured `--version`, e.g. '2.1.170' [SOT=local]
    is_available        BOOLEAN NOT NULL DEFAULT FALSE,         -- real exec.LookPath result, NOT APIKey!="" (de-bluff D-6) [SOT=local]
    non_interactive_args TEXT,                                  -- §11.4.99-researched invocation template [SOT=local]
    model_list_subcmd   TEXT,                                   -- dynamic GetModels subcommand (CONST-036) [SOT=local]
    config_install_path TEXT,                                   -- real config dir for INSTALL leg (e.g. ~/.claude.json) [SOT=local]
    last_validated_at   TIMESTAMPTZ,                            -- LIVE post-install validation (SP4·T4.4.3) [SOT=local]
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
COMMENT ON TABLE cli_agent_bridges IS
  'SP4 CLI-agent bridge providers (PLANNED). is_available MUST reflect a REAL exec.LookPath (base.go:120), never the qwencode APIKey!="" stub (D-6). resolution records PRIMARY system-installed vs FALLBACK submodule-built (analysis-D §5.3). Capabilities for these agents flow through `capabilities` with provenance=runtime_probe when the verifier has no entry (CONST-040 honest-provenance).';


-- -----------------------------------------------------------------------------
-- TABLE: catalog_entries
-- SOT: mixed (a JOIN/projection). The unified catalog (SP2·P2.1) under ONE root:
--      ensemble + helixllm + every provider + every VERIFIED model + every CLI
--      agent/model — each a uniformly-named selectable target. [PLANNED — SP2/SP4.]
-- Naming grammar (analysis-C §3.2; roadmap line 257): ensemble | ensemble/<preset>
--   | helixllm | helixllm/<model> | <provider> | <provider>/<model_id>
--   | cli/<agent> | cli/<agent>/<model>.
-- Mirrors catalog.Entry{Name,Kind,Provider,Verified,OverallScore,Enabled}
--   (SP2 plan §P2.1·T2.1.2).
-- -----------------------------------------------------------------------------
CREATE TABLE catalog_entries (
    name                TEXT PRIMARY KEY,                       -- the selector, e.g. 'anthropic/claude-3-sonnet-20240229' [SOT=local projection]
    kind                TEXT NOT NULL                           -- catalog.Entry.Kind:
                          CHECK (kind IN ('ensemble','provider','model','cli')),
    provider            TEXT,                                   -- nullable for kind='ensemble' [SOT=verifier where model]
    model_id            TEXT,                                   -- set for kind in ('model'); [SOT=verifier]
    agent               TEXT REFERENCES cli_agent_bridges(agent) ON DELETE CASCADE,  -- set for kind='cli' [SOT=local]
    -- Working/visibility flags (model entries emitted ONLY when verified, §11.4.122/CONST-036):
    verified            BOOLEAN NOT NULL DEFAULT FALSE,         -- DiscoveredModel.Verified [SOT=verifier]
    overall_score       DOUBLE PRECISION NOT NULL DEFAULT 0,    -- [SOT=verifier]
    enabled             BOOLEAN NOT NULL DEFAULT TRUE,          -- [SOT=local]
    built_at            TIMESTAMPTZ NOT NULL DEFAULT now(),     -- catalog assembly timestamp
    -- A 'model'/'cli-model' entry must reference a real model row:
    FOREIGN KEY (provider, model_id) REFERENCES models(provider, id) ON DELETE CASCADE
);
COMMENT ON TABLE catalog_entries IS
  'PLANNED unified catalog (SP2·T2.1.2 / SP4) projecting ensemble + HelixLLM + every provider + every VERIFIED model + every CLI-agent target into ONE selectable namespace. kind=''model'' rows are emitted ONLY for verifier-verified models (never static SupportedModels, never hardcoded — CONST-036/§11.4.122). The catalog is additive: existing /v1/* routes are untouched.';


-- =============================================================================
-- VIEW: working_models — the SP1 "working model" funnel as a query.
-- Encodes the exact predicate from SP1 §3 / analysis-B §4.1. This is what the
-- CLI/server SHOULD list once SP1·T1.3.2 routes through GetWorkingModels.
-- =============================================================================
CREATE VIEW working_models AS
SELECT  m.provider,
        m.id                     AS model_id,
        m.display_name,
        vr.overall_score,
        vr.verification_status,
        vr.last_verified
FROM    models m
JOIN    providers p             ON p.provider = m.provider
JOIN    verification_results vr ON vr.provider = m.provider AND vr.model_id = m.id
WHERE   p.key_present = TRUE                    -- SP1 key-presence gate (T1.2.1)
  AND   p.enabled     = TRUE                    -- adapter.go:277-289 Enabled flag
  AND   vr.verified   = TRUE                    -- VerifiedModel.Verified
  AND   vr.verification_status = 'verified'     -- not pending/failed/rate_limited (fixes D-2)
  AND   vr.overall_score >= 6.0                 -- GetMinAcceptableScore() default (D-4; parameterize via app config)
  AND   m.source <> 'fallback';                 -- NEVER present hardcoded fallback as working (CONST-035 guard)
COMMENT ON VIEW working_models IS
  'The SP1 working-model funnel expressed as SQL: key_present AND enabled AND verified AND status=verified AND overall_score>=min AND source<>fallback. The 6.0 literal is GetMinAcceptableScore() default (HELIX_VERIFIER_MIN_SCORE) — in the app it is config-driven, not a literal.';


-- =============================================================================
-- SQLite portability notes (project workable-items DB is SQLite, §11.4.93):
--   * BIGSERIAL        -> INTEGER PRIMARY KEY AUTOINCREMENT
--   * TIMESTAMPTZ      -> TEXT (ISO-8601) or INTEGER (unix epoch)
--   * DOUBLE PRECISION -> REAL ;  BOOLEAN -> INTEGER (0/1)
--   * now()            -> CURRENT_TIMESTAMP
--   * COMMENT ON ...   -> not supported; keep these as -- line comments
--   * partial indexes  -> supported by SQLite 3.8+ (idx_verif_working works)
-- =============================================================================
