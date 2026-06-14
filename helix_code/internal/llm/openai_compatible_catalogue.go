package llm

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// openai_compatible_catalogue.go — data-driven catalogue of HOSTED
// OpenAI-Chat-Completions-compatible LLM providers, layered over the existing
// generic OpenAICompatibleProvider so the TUI (and any model-listing surface)
// can USE every such provider whose key the operator has configured.
//
// CONST-074 (reuse-don't-reimplement) / §11.4.99 (no-guessing on remote
// endpoints): every BaseURL + KeyEnvAlias below is lifted from the sibling
// submodule helix_agent's authoritative `providerMappings` table
// (submodules/helix_agent/internal/services/provider_discovery.go). Copying that
// DATA is permitted (it is config, not internal/ code — CONST-051(B)); the table
// rows are cited per entry. Providers HelixCode already wires natively
// (deepseek/groq/mistral/openrouter/openai/anthropic/gemini/xai/qwen/ollama) are
// deliberately EXCLUDED — only the not-yet-wired hosted OpenAI-compatible
// providers are catalogued here.
//
// CONST-042 / §12.1 (no-secret-leak): this file NEVER logs or persists a key
// VALUE; key resolution reuses keyrecognition.go's present/placeholder logic.
//
// URL-composition gotcha (matches openai_compatible_provider.go getAPIURL):
//   getAPIURL(endpoint) = TrimSuffix(BaseURL,"/") + endpoint
//   default endpoints (when blank) are "/v1/models" + "/v1/chat/completions".
// So for a BaseURL that ALREADY ends in /v1 (or any version segment), we set
// ModelEndpoint="/models" + ChatEndpoint="/chat/completions" to avoid a doubled
// "/v1/v1/models". For a BaseURL WITHOUT a trailing version segment we leave the
// endpoints blank so the defaults apply. Each entry is set explicitly below.

// HostedOpenAICompatible is one catalogue row: a hosted provider reachable
// through the OpenAI Chat-Completions wire contract.
type HostedOpenAICompatible struct {
	Name          string
	BaseURL       string
	KeyEnvAliases []string
	ModelEndpoint string // path appended to BaseURL for GET models (blank → default /v1/models)
	ChatEndpoint  string // path appended to BaseURL for POST chat (blank → default /v1/chat/completions)
}

// ComposedModelsURL returns the effective models URL exactly as the underlying
// OpenAICompatibleProvider.getAPIURL would compose it — used to assert the
// no-doubled-/v1 composition is correct per entry.
func (h HostedOpenAICompatible) ComposedModelsURL() string {
	endpoint := h.ModelEndpoint
	if endpoint == "" {
		endpoint = "/v1/models"
	}
	return strings.TrimSuffix(h.BaseURL, "/") + endpoint
}

// HostedOpenAICompatibleCatalogue returns the data-driven catalogue. Returned as
// a fresh slice each call so callers cannot mutate the canonical table.
//
// Each row cites the helix_agent providerMappings BaseURL it derives from. Where
// helix_agent stored the FULL chat-completions URL, the BaseURL here is
// normalised to the version-prefix base and the endpoints are set explicitly.
func HostedOpenAICompatibleCatalogue() []HostedOpenAICompatible {
	return []HostedOpenAICompatible{
		// cerebras — helix_agent: https://api.cerebras.ai/v1/chat/completions
		// → base .../v1 ; explicit endpoints so URL ends /v1/models.
		{
			Name:          "cerebras",
			BaseURL:       "https://api.cerebras.ai/v1",
			KeyEnvAliases: []string{"CEREBRAS_API_KEY", "ApiKey_Cerebras"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// fireworks — helix_agent: https://api.fireworks.ai/inference/v1/chat/completions
		// → base .../inference/v1.
		// §11.4.99 live-verified 2026-06-14: GET https://api.fireworks.ai/inference/v1/models
		// is the CORRECT endpoint (official docs https://docs.fireworks.ai/api-reference/list-models).
		// The 412 PRECONDITION_FAILED observed at startup is a key/account-side issue
		// ("Account ... is suspended, possibly due to reaching the monthly spending
		// limit or failure to pay past invoices"), NOT a wrong-URL bug — so the URL
		// stays as-is; the provider will skip until the billing block is lifted.
		{
			Name:          "fireworks",
			BaseURL:       "https://api.fireworks.ai/inference/v1",
			KeyEnvAliases: []string{"FIREWORKS_API_KEY", "ApiKey_Fireworks"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// hyperbolic — helix_agent: https://api.hyperbolic.xyz/v1 (already /v1 base).
		{
			Name:          "hyperbolic",
			BaseURL:       "https://api.hyperbolic.xyz/v1",
			KeyEnvAliases: []string{"HYPERBOLIC_API_KEY", "ApiKey_Hyperbolic"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// novita — helix_agent: https://api.novita.ai/v3/openai (OpenAI base path;
		// models + chat live UNDER it at /v1/... → leave defaults so the URL is
		// .../v3/openai/v1/models, NOT a doubled /v1).
		{
			Name:          "novita",
			BaseURL:       "https://api.novita.ai/v3/openai",
			KeyEnvAliases: []string{"NOVITA_API_KEY", "ApiKey_Novita"},
			// defaults: ModelEndpoint=/v1/models, ChatEndpoint=/v1/chat/completions
		},
		// sambanova — helix_agent: https://api.sambanova.ai/v1 (already /v1 base).
		{
			Name:          "sambanova",
			BaseURL:       "https://api.sambanova.ai/v1",
			KeyEnvAliases: []string{"SAMBANOVA_API_KEY", "ApiKey_SambaNova"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// siliconflow — helix_agent had https://api.siliconflow.cn/v1, but the .cn
		// host rejected our key with HTTP 401 "Api key is invalid". §11.4.99
		// live-verified 2026-06-14: GET https://api.siliconflow.com/v1/models returns
		// HTTP 200 with 69 models for the SAME key (the international .com host is the
		// correct one for this account; .cn is the separate China-mainland platform
		// with a distinct key namespace). BaseURL switched .cn → .com — FIXED.
		// Docs: https://docs.siliconflow.com/en/api-reference/models/get-model-list
		{
			Name:          "siliconflow",
			BaseURL:       "https://api.siliconflow.com/v1",
			KeyEnvAliases: []string{"SILICONFLOW_API_KEY", "ApiKey_SiliconFlow"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// nvidia — helix_agent: https://integrate.api.nvidia.com/v1 (already /v1).
		{
			Name:          "nvidia",
			BaseURL:       "https://integrate.api.nvidia.com/v1",
			KeyEnvAliases: []string{"NVIDIA_API_KEY", "NGC_API_KEY", "ApiKey_NVIDIA"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// zai (Zhipu / GLM) — helix_agent: https://api.z.ai/api/paas/v4 (v4 base;
		// models + chat sit directly under it → explicit /models + /chat/completions,
		// NOT the default /v1/... which z.ai's paas/v4 does not serve).
		{
			Name:          "zai",
			BaseURL:       "https://api.z.ai/api/paas/v4",
			KeyEnvAliases: []string{"ZAI_API_KEY", "ZHIPU_API_KEY", "GLM_API_KEY", "ApiKey_ZAI"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// kimi (Moonshot) — helix_agent had https://api.moonshot.cn/v1, but per the
		// official OpenAPI spec (platform.kimi.ai → server url https://api.moonshot.ai)
		// the INTERNATIONAL host is api.moonshot.ai; .cn is the separate
		// China-mainland platform with a distinct key namespace. §11.4.99
		// live-verified 2026-06-14: GET /v1/models AND POST /v1/chat/completions on
		// BOTH hosts returned HTTP 401 "Invalid Authentication" with OUR key — i.e.
		// the key itself is rejected (key-side issue, not our URL bug). BaseURL
		// corrected .cn → .ai (the right host for an international key); the provider
		// stays in the catalogue and will skip until a valid api.moonshot.ai key is
		// configured. Docs: https://platform.kimi.ai/docs/api/chat
		{
			Name:          "kimi",
			BaseURL:       "https://api.moonshot.ai/v1",
			KeyEnvAliases: []string{"KIMI_API_KEY", "MOONSHOT_API_KEY", "ApiKey_Kimi"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// upstage (Solar) — helix_agent: https://api.upstage.ai/v1/solar . §11.4.99
		// live-verified 2026-06-14: the official base is https://api.upstage.ai/v1 and
		// chat is OpenAI-compatible at /v1/chat/completions (corrected from the bogus
		// /v1/solar/chat/completions). HOWEVER two facts make upstage non-listable here:
		//   (1) Upstage's backend does NOT implement a GET /v1/models endpoint at all
		//       (confirmed by the maintainer in
		//       https://huggingface.co/upstage/solar-pro-preview-instruct/discussions/21
		//       — a user request to "add /models endpoint"), so the catalogue cannot
		//       enumerate its models.
		//   (2) From this network the AWS-ELB edge (server: awselb/2.0) returns a bare
		//       HTML "403 Forbidden" for EVERY request — with our key, with no key, and
		//       with a deliberately-bad key alike — i.e. an edge/WAF/geo block upstream
		//       of auth, not a URL or key fault we can fix in code.
		// Because there is no models-listing endpoint, upstage is EXCLUDED from the
		// catalogue (it could never be catalogue-listed). Docs:
		// https://console.upstage.ai/api/chat . If Upstage later ships GET /v1/models,
		// re-add: BaseURL "https://api.upstage.ai/v1", ModelEndpoint "/models",
		// ChatEndpoint "/chat/completions".
		// venice — helix_agent: https://api.venice.ai/api/v1/chat/completions
		// → base .../api/v1.
		{
			Name:          "venice",
			BaseURL:       "https://api.venice.ai/api/v1",
			KeyEnvAliases: []string{"VENICE_API_KEY", "ApiKey_Venice"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// chutes — helix_agent: https://llm.chutes.ai/v1/chat/completions
		// (NOTE: inference host is llm.chutes.ai, NOT api.chutes.ai) → base .../v1.
		{
			Name:          "chutes",
			BaseURL:       "https://llm.chutes.ai/v1",
			KeyEnvAliases: []string{"CHUTES_API_KEY", "ApiKey_Chutes"},
			ModelEndpoint: "/models",
			ChatEndpoint:  "/chat/completions",
		},
		// codestral (Mistral's code endpoint, distinct host) — helix_agent:
		// https://codestral.mistral.ai/v1 . §11.4.99 live-verified 2026-06-14:
		// the dedicated codestral.mistral.ai host serves ONLY /v1/chat/completions
		// (POST verified HTTP 200, real completion) and /v1/fim/completions — it has
		// NO /v1/models listing endpoint (GET returns HTTP 404 {"message":"no Route
		// matched with those values"}). The full model catalogue for the same key is
		// served instead by the main host https://api.mistral.ai/v1/models (verified
		// HTTP 200, 69 models), and "mistral" is ALREADY natively wired by HelixCode
		// — so codestral's models are already reachable through that provider. Because
		// codestral.mistral.ai exposes no models-listing endpoint, codestral is
		// EXCLUDED from this catalogue (it could never be catalogue-listed via its own
		// host, and listing it via api.mistral.ai would duplicate the wired "mistral"
		// provider). Docs: https://docs.mistral.ai/api/endpoint/fim ,
		// https://mistral.ai/news/codestral/ . Chat-only use of codestral-latest
		// remains available through the natively-wired mistral provider.
	}
}

// firstPresentHostedKey returns the first env alias whose value is present and
// non-placeholder, reusing keyrecognition.go's isPlaceholderKey. It returns the
// VALUE so the constructor can pass it to the underlying provider; the value is
// NEVER logged (CONST-042).
func firstPresentHostedKey(aliases []string) (string, bool) {
	for _, alias := range aliases {
		v, ok := os.LookupEnv(alias)
		if !ok {
			continue
		}
		if isPlaceholderKey(v) {
			continue
		}
		return v, true
	}
	return "", false
}

// NewHostedOpenAICompatibleProvider builds a real OpenAICompatibleProvider for a
// hosted catalogue entry, resolving the API key from the entry's env aliases via
// the same present/placeholder logic the rest of the package uses
// (keyrecognition.go). Returns an error when no alias holds a present,
// non-placeholder value — so a provider is only ever built for a configured key.
func NewHostedOpenAICompatibleProvider(h HostedOpenAICompatible) (Provider, error) {
	apiKey, ok := firstPresentHostedKey(h.KeyEnvAliases)
	if !ok {
		return nil, fmt.Errorf("hosted provider %q: no present non-placeholder key in env aliases %v",
			h.Name, h.KeyEnvAliases)
	}
	return NewOpenAICompatibleProvider(h.Name, OpenAICompatibleConfig{
		BaseURL:          h.BaseURL,
		APIKey:           apiKey,
		Timeout:          120 * time.Second,
		StreamingSupport: true,
		ModelEndpoint:    h.ModelEndpoint,
		ChatEndpoint:     h.ChatEndpoint,
	})
}
