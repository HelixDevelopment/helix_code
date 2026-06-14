package llm

import (
	"net/url"
	"strings"
	"testing"
)

// openai_compatible_catalogue_test.go — RED-first coverage for the data-driven
// hosted OpenAI-Chat-Completions-compatible provider catalogue (CONST-074
// reuse-don't-reimplement: base URLs lifted from helix_agent's
// providerMappings table; CONST-042 anti-secret-leak: never prints key VALUES).

// alreadyWiredProviderNames is the set of providers HelixCode already wires
// natively (keyrecognition.go ProviderEnvAliases + dedicated providers). The
// hosted catalogue MUST NOT collide with any of these.
var alreadyWiredProviderNames = map[string]bool{
	"openai":     true,
	"anthropic":  true,
	"gemini":     true,
	"deepseek":   true,
	"mistral":    true,
	"groq":       true,
	"xai":        true,
	"openrouter": true,
	"qwen":       true,
	"copilot":    true,
	"ollama":     true,
}

func TestHostedOpenAICompatibleCatalogue_NonEmptyAndWellFormed(t *testing.T) {
	cat := HostedOpenAICompatibleCatalogue()
	if len(cat) == 0 {
		t.Fatalf("catalogue is empty; expected ≥1 hosted OpenAI-compatible provider")
	}
	seen := map[string]bool{}
	for _, h := range cat {
		if strings.TrimSpace(h.Name) == "" {
			t.Errorf("entry has empty Name: %+v", h)
		}
		if strings.TrimSpace(h.BaseURL) == "" {
			t.Errorf("%s: empty BaseURL", h.Name)
		}
		if !strings.HasPrefix(h.BaseURL, "https://") {
			t.Errorf("%s: BaseURL %q is not https", h.Name, h.BaseURL)
		}
		if len(h.KeyEnvAliases) == 0 {
			t.Errorf("%s: no KeyEnvAliases", h.Name)
		}
		for _, a := range h.KeyEnvAliases {
			if strings.TrimSpace(a) == "" {
				t.Errorf("%s: blank key env alias", h.Name)
			}
		}
		if seen[h.Name] {
			t.Errorf("%s: duplicate catalogue entry", h.Name)
		}
		seen[h.Name] = true
	}
}

func TestHostedOpenAICompatibleCatalogue_NoCollisionWithWiredProviders(t *testing.T) {
	for _, h := range HostedOpenAICompatibleCatalogue() {
		if alreadyWiredProviderNames[strings.ToLower(h.Name)] {
			t.Errorf("%s collides with an already-wired provider", h.Name)
		}
	}
}

// TestHostedOpenAICompatibleCatalogue_ModelsURLComposition asserts the
// URL-composition gotcha is handled: for every entry the effective models URL
// MUST contain exactly one "/v1/models" (or the documented variant) — never
// "/v1/v1/models".
func TestHostedOpenAICompatibleCatalogue_ModelsURLComposition(t *testing.T) {
	for _, h := range HostedOpenAICompatibleCatalogue() {
		modelsURL := h.ComposedModelsURL()
		if _, err := url.Parse(modelsURL); err != nil {
			t.Errorf("%s: composed models URL %q does not parse: %v", h.Name, modelsURL, err)
		}
		if strings.Contains(modelsURL, "/v1/v1/") {
			t.Errorf("%s: composed models URL has doubled /v1: %q", h.Name, modelsURL)
		}
		if strings.Count(modelsURL, "/models") != 1 {
			t.Errorf("%s: composed models URL must end in exactly one /models: %q", h.Name, modelsURL)
		}
		// When the base URL ends in /v1, the composed URL must be exactly
		// "<base>/models" → ".../v1/models".
		base := strings.TrimSuffix(h.BaseURL, "/")
		if strings.HasSuffix(base, "/v1") && !strings.HasSuffix(modelsURL, "/v1/models") {
			t.Errorf("%s: base %q ends /v1 but composed models URL %q is not <base>/models",
				h.Name, h.BaseURL, modelsURL)
		}
	}
}

// TestHostedOpenAICompatibleCatalogue_LiveVerifiedHosts regression-guards the
// §11.4.99 live-verified (2026-06-14) host corrections: siliconflow MUST use the
// international .com host (the .cn host rejected the key with HTTP 401 while .com
// returned HTTP 200 + 69 models for the SAME key) and kimi MUST use the
// api.moonshot.ai host (the official OpenAPI server URL; .cn is a separate
// China-mainland platform). A regression back to the .cn hosts would silently
// break model listing for these providers.
func TestHostedOpenAICompatibleCatalogue_LiveVerifiedHosts(t *testing.T) {
	want := map[string]string{
		"siliconflow": "https://api.siliconflow.com/v1",
		"kimi":        "https://api.moonshot.ai/v1",
	}
	got := map[string]string{}
	for _, h := range HostedOpenAICompatibleCatalogue() {
		got[h.Name] = h.BaseURL
	}
	for name, wantBase := range want {
		if got[name] != wantBase {
			t.Errorf("%s BaseURL = %q, want live-verified %q", name, got[name], wantBase)
		}
		if strings.Contains(got[name], ".cn/") {
			t.Errorf("%s BaseURL %q regressed to the .cn host that rejects the key", name, got[name])
		}
	}
}

// TestHostedOpenAICompatibleCatalogue_NoModelsEndpointProvidersExcluded guards
// the §11.4.99 live-verified (2026-06-14) exclusions: codestral.mistral.ai and
// api.upstage.ai expose NO GET /v1/models listing endpoint, so they cannot be
// catalogue-listed and MUST stay out of the catalogue. Re-adding either without
// a real models endpoint would make NewHostedOpenAICompatibleProvider fail at
// discoverModels for every operator who configures the key.
func TestHostedOpenAICompatibleCatalogue_NoModelsEndpointProvidersExcluded(t *testing.T) {
	excluded := map[string]bool{"codestral": true, "upstage": true}
	for _, h := range HostedOpenAICompatibleCatalogue() {
		if excluded[strings.ToLower(h.Name)] {
			t.Errorf("%s is in the catalogue but has no GET /models endpoint (live-verified 2026-06-14) — it must stay excluded", h.Name)
		}
	}
}

func TestNewHostedOpenAICompatibleProvider_ErrorWhenKeyAbsent(t *testing.T) {
	h := HostedOpenAICompatible{
		Name:          "fireworks",
		BaseURL:       "https://api.fireworks.ai/inference/v1",
		KeyEnvAliases: []string{"FIREWORKS_API_KEY", "ApiKey_Fireworks"},
	}
	// Ensure every alias is unset for this test.
	for _, a := range h.KeyEnvAliases {
		t.Setenv(a, "")
	}
	if _, err := NewHostedOpenAICompatibleProvider(h); err == nil {
		t.Fatalf("expected error when key env absent, got nil")
	}
}

func TestNewHostedOpenAICompatibleProvider_RejectsPlaceholder(t *testing.T) {
	h := HostedOpenAICompatible{
		Name:          "fireworks",
		BaseURL:       "https://api.fireworks.ai/inference/v1",
		KeyEnvAliases: []string{"FIREWORKS_API_KEY"},
	}
	t.Setenv("FIREWORKS_API_KEY", "your-api-key-here")
	if _, err := NewHostedOpenAICompatibleProvider(h); err == nil {
		t.Fatalf("expected error for placeholder key, got nil")
	}
}

func TestNewHostedOpenAICompatibleProvider_BuildsProviderWhenKeyPresent(t *testing.T) {
	h := HostedOpenAICompatible{
		Name:          "fireworks",
		BaseURL:       "https://api.fireworks.ai/inference/v1",
		KeyEnvAliases: []string{"FIREWORKS_API_KEY"},
	}
	// Dummy non-placeholder value; never a real key (CONST-042). Offline:
	// discoverModels will fail silently, provider still constructs.
	t.Setenv("FIREWORKS_API_KEY", "fw-dummy-real-looking-value-12345")
	p, err := NewHostedOpenAICompatibleProvider(h)
	if err != nil {
		t.Fatalf("unexpected error building provider with present key: %v", err)
	}
	if p.GetName() != "fireworks" {
		t.Errorf("GetName()=%q, want fireworks", p.GetName())
	}
	if got := string(p.GetType()); got != "fireworks" {
		t.Errorf("GetType()=%q, want distinct type \"fireworks\"", got)
	}
}

// TestHostedProvider_GetTypeDistinctAcrossCatalogue is the load-bearing
// GetType-collision guard: two different catalogue providers must NOT report the
// same ProviderType, and neither may report the generic "local" type.
func TestHostedProvider_GetTypeDistinctAcrossCatalogue(t *testing.T) {
	fw := HostedOpenAICompatible{Name: "fireworks", BaseURL: "https://api.fireworks.ai/inference/v1", KeyEnvAliases: []string{"FIREWORKS_API_KEY"}}
	ds := HostedOpenAICompatible{Name: "deepseek-fake", BaseURL: "https://api.deepseek.com/v1", KeyEnvAliases: []string{"FIREWORKS_API_KEY"}}
	t.Setenv("FIREWORKS_API_KEY", "fw-dummy-real-looking-value-12345")

	pfw, err := NewHostedOpenAICompatibleProvider(fw)
	if err != nil {
		t.Fatalf("build fireworks: %v", err)
	}
	pds, err := NewHostedOpenAICompatibleProvider(ds)
	if err != nil {
		t.Fatalf("build deepseek-fake: %v", err)
	}
	if pfw.GetType() == ProviderTypeLocal {
		t.Errorf("fireworks reports generic ProviderTypeLocal — names would collide")
	}
	if pfw.GetType() == pds.GetType() {
		t.Errorf("fireworks and deepseek-fake report the same ProviderType %q — collision", pfw.GetType())
	}
}

// TestOpenAICompatibleProvider_KnownLocalNamesUnchanged guards the GetType fix:
// the known local backends MUST keep their existing distinct types.
func TestOpenAICompatibleProvider_KnownLocalNamesUnchanged(t *testing.T) {
	cases := map[string]ProviderType{
		"vllm":     ProviderTypeVLLM,
		"localai":  ProviderTypeLocalAI,
		"lmstudio": ProviderTypeLMStudio,
		"jan":      ProviderTypeJan,
	}
	for name, want := range cases {
		p := &OpenAICompatibleProvider{name: name}
		if got := p.GetType(); got != want {
			t.Errorf("GetType() for %q = %q, want %q", name, got, want)
		}
	}
}
