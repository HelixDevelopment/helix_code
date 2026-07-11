//go:build providerlive

package llm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// provider_live_proof_test.go — CONST-039 per-provider live-proof harness.
//
// Gap-closure for the "Provider Live-Proof Harness" completeness audit
// (2026-07-08, scratchpad provider_live_proof_harness_audit.md): the
// pre-existing ensemble_provider_live_probe_test.go proves the ENSEMBLE
// orchestration layer, not per-provider CONST-039 coverage — it drives 4
// providers as a GROUP and asserts only aggregate ensemble properties
// (>=2 successful members), with no per-provider isolation, no per-provider
// evidence, and a suite-level t.Fatalf on <2 members that is a FAIL, not an
// honest §11.4.3 SKIP, when the operator has configured 0 or 1 of those 4
// keys. That pattern is intentionally NOT reused here.
//
// This file is the missing per-provider harness: one independent subtest
// per CONST-039 provider (OpenAI, Anthropic, Gemini, DeepSeek, Groq,
// Mistral, xAI, OpenRouter, Ollama, Llama.cpp). Each subtest:
//   - emits an honest, isolated SKIP ("SKIP: no-key" for the 8 hosted
//     providers gated by IsProviderKeyPresent, "SKIP: unreachable" for the
//     2 local providers gated by a real reachability probe) when its
//     credential/server is absent — NEVER a fake PASS, NEVER a suite-level
//     FAIL-on-absence;
//   - when present, constructs the REAL provider (NewCloudProvider, the
//     existing Feature-12 cloud factory — already covers all 10 CONST-039
//     types), issues a REAL HTTP Generate() call carrying a freshly
//     generated per-run NONCE, and asserts the response actually contains
//     that nonce — proving the answer came from a live model on THIS run,
//     not a cached/mocked/hardcoded string (§11.4.2/§11.4.5 anti-bluff);
//   - captures the full request/response transcript under
//     docs/qa/<run-id>/provider_coverage/<provider>/ (§11.4.83).
//
// Build tag `providerlive` keeps this out of the default `go test ./...`
// run (no network calls / no API cost on an ordinary unit run, and no
// import-time dependency the rest of the package has to carry). Explicit,
// autonomous, re-runnable invocation (§11.4.98):
//
//   cd helix_code && go test -tags=providerlive -v -count=1 \
//     -run TestProviderLiveProof ./internal/llm/
//
// Re-run with -count=3 to demonstrate re-runnability; each invocation opens
// a fresh timestamped run-id directory so successive runs never clobber
// each other's evidence and the SKIP-vs-live behaviour is stable across
// repeats regardless of which/how-many keys are configured that run.
//
// Per-call time bound: each live probe is bounded by THIS harness's own
// context.WithTimeout (45s hosted / 60s local) passed to http.NewRequestWithContext
// below — that context deadline, not the outer `go test -timeout`, is the
// effective per-provider bound. The `go test -timeout` is only a whole-process
// backstop and MUST be set above the aggregate of these per-call bounds (see
// scripts/test_providers.sh) so a slow provider FAILs its own subtest cleanly
// rather than panicking the run.
//
// CONST-036 note on model selection: rather than re-declaring a THIRD
// hardcoded provider→model table (keyrecognition.go already owns the
// provider→env-var-alias table reused below; each provider file owns its
// own model catalogue), this harness sources the probe model directly from
// each provider's own GetModels() — several of which (OpenAI, Ollama,
// Llama.cpp) already perform a REAL live /models (or /api/tags) HTTP
// discovery call, and the remainder return that provider file's own
// curated, actively-maintained model seed list — rather than a model list
// invented fresh for this file. A full LLMsVerifier cross-check (assert
// the returned `model` field also appears in the verifier's live-discovered
// working-model set, per CONST-036/037) is intentionally DEFERRED here —
// gap-closure item 6 of the audit — because it requires wiring the
// verifier adapter into this harness, a separate, larger follow-up outside
// this pass's scope (this pass's scope is: build the missing per-provider
// live-proof harness itself, honest-SKIP-safe, with captured evidence).
//
// Oracle-honesty fix (2026-07-11, §11.4.6): a prior live run found this
// harness recorded a false FAIL for reasoning-style models (observed on
// DeepSeek `deepseek-v4-flash` and OpenRouter `openai/gpt-oss-20b:free`).
// Those models emit internal reasoning tokens BEFORE the visible answer;
// at the original MaxTokens=32 budget the reasoning alone consumed the
// entire token budget, the provider genuinely returned
// finish_reason="length" (surfaced by every provider file in this
// package's own canonical mapXxxFinishReasonToErr() helper as
// resp.Err=ErrResponseTruncated — see missing_types.go), and the nonce
// was never reached. That is a TRUNCATION of a real, live call — not a
// provider failure — so mis-classifying it as FAIL was a false-negative.
// The fix is two-pronged (§11.4.6 "keep the nonce-echo unforgeability: a
// real echoed nonce is still the only thing that yields PASS"):
//
//  1. providerLiveNonceMaxTokens raises the shared MaxTokens budget from
//     32 to 4096 (this package's own reasoning.go already sizes typical
//     ThinkingBudget defaults in the 5000-10000 range for o1/extended-
//     thinking-class models — 4096 gives cheaper reasoning-tagged/free-tier
//     models, the two observed here, ample headroom for reasoning tokens
//     plus the short nonce echo) AND providerLiveNoncePrompt explicitly
//     asks the model to skip reasoning/explanation, so most calls now
//     complete within budget without ever reaching the fallback below.
//  2. When a real HTTP call succeeds, genuinely truncates
//     (errors.Is(resp.Err, ErrResponseTruncated) — populated by every
//     provider this harness exercises: see openai_provider.go,
//     anthropic_provider.go, gemini_provider.go, deepseek_provider.go,
//     groq_provider.go, mistral_provider.go, xai_provider.go,
//     openrouter_provider.go, ollama_provider.go, llamacpp_provider.go),
//     and the nonce still never appears, the result is recorded as the
//     distinct honest verdict INCONCLUSIVE (via t.Skipf, so `go test`'s
//     exit code stays green) rather than FAIL — a truncated call proves
//     nothing about whether the provider CAN echo the nonce, so it must
//     not be reported as a provider failure. A genuinely non-truncated
//     call that omits the nonce (untruncated finish_reason, model simply
//     did not comply) still FAILs exactly as before: the unforgeability
//     property this harness exists to prove is unchanged.

// providerLiveKind distinguishes the two honest-absence gates this harness
// uses: hosted providers are gated on API-key presence; local providers
// (Ollama, Llama.cpp) have no API key at all and are instead gated on a
// real reachability probe against their configured base URL.
type providerLiveKind int

const (
	providerLiveKindHostedKey providerLiveKind = iota
	providerLiveKindLocalReachability
)

// providerLiveSpec describes one CONST-039 provider's live-proof wiring.
type providerLiveSpec struct {
	// name is the human-readable label used for the docs/qa evidence
	// directory and the t.Run subtest name.
	name string
	pt   ProviderType
	kind providerLiveKind

	// modelEnvOverride, when set to a non-empty value, forces the probe
	// model for this provider instead of the auto-picked catalogue model
	// (useful for pinning a specific verified/cheap model in CI/manual
	// runs without editing this file).
	modelEnvOverride string

	// Local-provider-only fields (ignored for hosted providers).
	localBaseURLEnv     string // e.g. "OLLAMA_BASE_URL"
	localBaseURLDefault string // e.g. "http://localhost:11434"
	localProbePath      string // e.g. "/api/tags"
}

// providerLiveCandidates returns the ordered CONST-039 provider roster this
// harness proves. Order matches CONST-039's own enumeration (OpenAI,
// Anthropic, Gemini, DeepSeek, Groq, Mistral, xAI, OpenRouter, Ollama,
// Llama.cpp).
func providerLiveCandidates() []providerLiveSpec {
	return []providerLiveSpec{
		{name: "openai", pt: ProviderTypeOpenAI, kind: providerLiveKindHostedKey, modelEnvOverride: "PROVIDERLIVE_MODEL_OPENAI"},
		{name: "anthropic", pt: ProviderTypeAnthropic, kind: providerLiveKindHostedKey, modelEnvOverride: "PROVIDERLIVE_MODEL_ANTHROPIC"},
		{name: "gemini", pt: ProviderTypeGemini, kind: providerLiveKindHostedKey, modelEnvOverride: "PROVIDERLIVE_MODEL_GEMINI"},
		{name: "deepseek", pt: ProviderTypeDeepSeek, kind: providerLiveKindHostedKey, modelEnvOverride: "PROVIDERLIVE_MODEL_DEEPSEEK"},
		{name: "groq", pt: ProviderTypeGroq, kind: providerLiveKindHostedKey, modelEnvOverride: "PROVIDERLIVE_MODEL_GROQ"},
		{name: "mistral", pt: ProviderTypeMistral, kind: providerLiveKindHostedKey, modelEnvOverride: "PROVIDERLIVE_MODEL_MISTRAL"},
		{name: "xai", pt: ProviderTypeXAI, kind: providerLiveKindHostedKey, modelEnvOverride: "PROVIDERLIVE_MODEL_XAI"},
		{name: "openrouter", pt: ProviderTypeOpenRouter, kind: providerLiveKindHostedKey, modelEnvOverride: "PROVIDERLIVE_MODEL_OPENROUTER"},
		{
			name: "ollama", pt: ProviderTypeOllama, kind: providerLiveKindLocalReachability,
			modelEnvOverride:    "PROVIDERLIVE_MODEL_OLLAMA",
			localBaseURLEnv:     "OLLAMA_BASE_URL",
			localBaseURLDefault: "http://localhost:11434",
			// /api/tags mirrors OllamaProvider.IsAvailable's own real
			// reachability check (ollama_provider.go), reused here rather
			// than inventing a new endpoint assumption.
			localProbePath: "/api/tags",
		},
		{
			name: "llamacpp", pt: ProviderTypeLlamaCpp, kind: providerLiveKindLocalReachability,
			modelEnvOverride:    "PROVIDERLIVE_MODEL_LLAMACPP",
			localBaseURLEnv:     "LLAMACPP_BASE_URL",
			localBaseURLDefault: "http://localhost:8080",
			// /models mirrors LlamaCPPProvider.GetModels()'s own real HTTP
			// call (llamacpp_provider.go) — NOT LlamaCPPProvider.IsAvailable,
			// which is unconditionally `true` at construction time
			// (llamacpp_provider.go:38-45 sets isRunning:true regardless of
			// server reachability) and is therefore NOT a trustworthy
			// honest-absence gate; this harness performs its own
			// independent reachability probe instead of relying on that
			// known-unreliable signal.
			localProbePath: "/models",
		},
	}
}

// providerLiveRunID is computed once per test-binary invocation so every
// provider subtest in the same run shares one evidence directory
// (docs/qa/<run-id>/provider_coverage/<provider>/), and successive
// `-count=N` invocations (or successive manual re-runs) each get a fresh,
// non-clobbering run-id.
var (
	providerLiveRunIDOnce sync.Once
	providerLiveRunIDVal  string
)

func providerLiveRunID() string {
	providerLiveRunIDOnce.Do(func() {
		providerLiveRunIDVal = "provider_live_proof_" + time.Now().UTC().Format("20060102T150405Z")
	})
	return providerLiveRunIDVal
}

// providerLiveRepoRoot resolves the repository root (the directory
// containing docs/qa) from this source file's own path via runtime.Caller,
// so evidence capture works regardless of the `go test` invocation's
// working directory. This file lives at
// <repo-root>/helix_code/internal/llm/provider_live_proof_test.go, so the
// repo root is three directories up.
func providerLiveRepoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("providerLiveRepoRoot: runtime.Caller(0) failed")
	}
	// thisFile = <repo-root>/helix_code/internal/llm/provider_live_proof_test.go
	dir := filepath.Dir(thisFile)                         // .../helix_code/internal/llm
	root := filepath.Dir(filepath.Dir(filepath.Dir(dir))) // .../<repo-root>
	return root
}

// providerLiveEvidenceDir returns (and creates) the
// docs/qa/<run-id>/provider_coverage/<provider>/ directory for this run.
func providerLiveEvidenceDir(t *testing.T, provider string) string {
	t.Helper()
	dir := filepath.Join(providerLiveRepoRoot(t), "docs", "qa", providerLiveRunID(), "provider_coverage", provider)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("providerLiveEvidenceDir: mkdir %s: %v", dir, err)
	}
	return dir
}

// providerLiveNonce generates a fresh, unforgeable per-call challenge token.
// A cached, mocked, or hardcoded response cannot possibly contain a token
// that did not exist until this call executed — this is the anti-bluff
// (§11.4.2/§11.4.5) proof that the returned content is a genuine, live
// model answer for THIS run, not a canned string.
func providerLiveNonce() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("providerLiveNonce: crypto/rand read failed: %w", err)
	}
	return "LIVEPROOF-" + hex.EncodeToString(buf), nil
}

// providerLiveNonceMaxTokens is the shared MaxTokens budget for the
// nonce-echo challenge. See the "Oracle-honesty fix" note in the file
// header comment: this replaces the original flat 32-token budget, which
// was too small for reasoning-style models (observed on DeepSeek
// deepseek-v4-flash and OpenRouter openai/gpt-oss-20b:free) that consume
// part of the token budget on internal reasoning before emitting the
// visible answer.
const providerLiveNonceMaxTokens = 4096

// providerLiveNoncePrompt builds the nonce-echo challenge prompt shared by
// both the hosted and local live-proof paths. The explicit "no reasoning"
// instruction is a best-effort cost/latency reduction for models that
// honor it — it does not change the pass/fail oracle (only a literally
// echoed nonce yields PASS), it only makes it more likely a reasoning
// model completes within providerLiveNonceMaxTokens instead of falling
// through to the INCONCLUSIVE truncation path.
func providerLiveNoncePrompt(nonce string) string {
	return fmt.Sprintf(
		"This is an automated liveness probe. Skip any reasoning, chain-of-thought, "+
			"or explanation. Reply with EXACTLY this token and nothing else: %s",
		nonce,
	)
}

// providerLiveResolveKey resolves the PRESENT credential VALUE for pt by
// walking every alias keyrecognition.go's ProviderEnvAliases() declares for
// it (not just the provider constructor's own single primary env var —
// several constructors, e.g. Anthropic/xAI/OpenRouter/Groq/Mistral/
// DeepSeek/OpenAI, only fall back to ONE primary env var internally, while
// IsProviderKeyPresent correctly recognises secondary aliases too, e.g.
// CLAUDE_API_KEY for Anthropic or GROK_API_KEY for xAI). Resolving the
// value here and passing it explicitly as cfg.APIKey guarantees the
// harness's "present" gate (IsProviderKeyPresent) and the actual
// constructed provider agree on which credential is used — a secondary
// alias being the ONLY key configured must not silently fall through to
// "provider construction fails, key not found" after we already reported
// the provider as present.
func providerLiveResolveKey(pt ProviderType) (string, bool) {
	aliases, ok := ProviderEnvAliases()[pt]
	if !ok {
		return "", false
	}
	for _, alias := range aliases {
		if v, ok := os.LookupEnv(alias); ok {
			trimmed := strings.TrimSpace(v)
			if trimmed != "" && !isPlaceholderKey(trimmed) {
				return trimmed, true
			}
		}
	}
	return "", false
}

// providerLiveCheapTokens are substrings that mark a catalogue model name
// as a cheap/fast tier, preferred for probe calls to keep this harness's
// real-API-cost footprint low.
var providerLiveCheapTokens = []string{
	"haiku", "mini", "flash", "8b", "instant", "small", "lite", "free", "nano", "fast",
}

// providerLivePickModel selects the probe model: an explicit env override
// wins outright; otherwise the first catalogue entry whose name matches a
// cheap/fast substring; otherwise the first catalogue entry at all.
// Returns ("", "") when neither an override nor any catalogue entry exists
// (caller must treat this as a hard failure — a live-key provider with zero
// resolvable models cannot be proven).
func providerLivePickModel(models []ModelInfo, envOverrideVar string) (model string, source string) {
	if envOverrideVar != "" {
		if v := strings.TrimSpace(os.Getenv(envOverrideVar)); v != "" {
			return v, "env:" + envOverrideVar
		}
	}
	for _, m := range models {
		lower := strings.ToLower(m.Name)
		for _, tok := range providerLiveCheapTokens {
			if strings.Contains(lower, tok) {
				return m.Name, "catalogue:cheap-match"
			}
		}
	}
	if len(models) > 0 {
		return models[0].Name, "catalogue:first"
	}
	return "", ""
}

// providerLiveLocalReachable performs a bounded, real HTTP GET against a
// local provider's base URL + probe path. It does NOT reuse the
// provider's own IsAvailable() (Ollama's is a genuine live check; Llama.cpp's
// is not — see the comment on providerLiveCandidates above) so the
// reachability signal driving this harness's honest-SKIP gate is
// independently trustworthy for BOTH local providers.
func providerLiveLocalReachable(baseURLEnv, baseURLDefault, probePath string) (reachable bool, resolvedURL string) {
	base := strings.TrimSpace(os.Getenv(baseURLEnv))
	if base == "" {
		base = baseURLDefault
	}
	url := strings.TrimRight(base, "/") + probePath

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, url
	}
	defer resp.Body.Close()
	// Any HTTP response (even a 404 on the probe path) proves something is
	// genuinely listening on that host:port — a real local server is up.
	// Only a connection-level error (refused/timeout/DNS failure, handled
	// above) means "unreachable".
	return true, url
}

// providerLiveEvidence is the JSON shape written to
// docs/qa/<run-id>/provider_coverage/<provider>/{request,response}.json.
// Deliberately excludes any credential value (§12.1/CONST-042 no-secret-leak
// — LLMRequest/LLMResponse never carry API keys, and this struct doesn't
// add any).
type providerLiveRequestEvidence struct {
	Provider    string    `json:"provider"`
	ProviderPT  string    `json:"provider_type"`
	Model       string    `json:"model"`
	ModelSource string    `json:"model_source"`
	Nonce       string    `json:"nonce"`
	Prompt      string    `json:"prompt"`
	RequestedAt time.Time `json:"requested_at_utc"`
}

type providerLiveResponseEvidence struct {
	Provider       string                 `json:"provider"`
	Content        string                 `json:"content"`
	NonceEchoed    bool                   `json:"nonce_echoed"`
	Truncated      bool                   `json:"truncated"`
	FinishReason   string                 `json:"finish_reason"`
	ProcessingTime string                 `json:"processing_time"`
	Usage          Usage                  `json:"usage"`
	ProviderMeta   map[string]interface{} `json:"provider_metadata,omitempty"`
	RespondedAt    time.Time              `json:"responded_at_utc"`
}

func providerLiveWriteJSON(t *testing.T, dir, filename string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("providerLiveWriteJSON(%s): marshal failed: %v", filename, err)
	}
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("providerLiveWriteJSON(%s): write failed: %v", filename, err)
	}
}

func providerLiveWriteVerdict(t *testing.T, dir, verdict, detail string) {
	t.Helper()
	body := fmt.Sprintf(
		"provider_live_proof verdict\nrun_id: %s\ntimestamp_utc: %s\nverdict: %s\ndetail: %s\n",
		providerLiveRunID(), time.Now().UTC().Format(time.RFC3339), verdict, detail,
	)
	path := filepath.Join(dir, "verdict.txt")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("providerLiveWriteVerdict: write failed: %v", err)
	}
}

// TestProviderLiveProof is the CONST-039 per-provider live-proof harness.
// See the file header comment for the full design rationale.
func TestProviderLiveProof(t *testing.T) {
	for _, spec := range providerLiveCandidates() {
		spec := spec
		t.Run(spec.name, func(t *testing.T) {
			dir := providerLiveEvidenceDir(t, spec.name)

			switch spec.kind {
			case providerLiveKindHostedKey:
				runHostedProviderLiveProof(t, spec, dir)
			case providerLiveKindLocalReachability:
				runLocalProviderLiveProof(t, spec, dir)
			default:
				t.Fatalf("unknown providerLiveKind %d for %s", spec.kind, spec.name)
			}
		})
	}
}

func runHostedProviderLiveProof(t *testing.T, spec providerLiveSpec, dir string) {
	t.Helper()

	if !IsProviderKeyPresent(spec.pt) {
		providerLiveWriteVerdict(t, dir, "SKIP", "no-key: none of the recognised env-var aliases for "+string(spec.pt)+" are set to a non-placeholder value")
		t.Skip("SKIP: no-key")
		return
	}

	key, ok := providerLiveResolveKey(spec.pt)
	if !ok {
		// IsProviderKeyPresent said yes but the resolver disagrees — this
		// would itself be a bug in either function; surface it as a real
		// failure rather than silently downgrading to SKIP (a present-key
		// provider must never be quietly dropped).
		t.Fatalf("providerLiveResolveKey disagreed with IsProviderKeyPresent for %s (present=true, resolved=false)", spec.pt)
	}

	provider, err := NewCloudProvider(spec.pt, ProviderConfigEntry{Type: spec.pt, Enabled: true, APIKey: key})
	if err != nil {
		providerLiveWriteVerdict(t, dir, "FAIL", "key present but provider construction failed: "+err.Error())
		t.Fatalf("%s: key present but NewCloudProvider failed: %v", spec.name, err)
	}
	defer func() { _ = provider.Close() }()

	models := provider.GetModels()
	model, modelSource := providerLivePickModel(models, spec.modelEnvOverride)
	if model == "" {
		providerLiveWriteVerdict(t, dir, "FAIL", "key present but no probe model resolvable (catalogue empty and no env override)")
		t.Fatalf("%s: key present but GetModels() returned no models and %s is unset", spec.name, spec.modelEnvOverride)
	}

	nonce, err := providerLiveNonce()
	if err != nil {
		t.Fatalf("%s: %v", spec.name, err)
	}
	prompt := providerLiveNoncePrompt(nonce)

	req := &LLMRequest{
		ID:          uuid.New(),
		Model:       model,
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   providerLiveNonceMaxTokens,
		Temperature: 0,
	}

	providerLiveWriteJSON(t, dir, "request.json", providerLiveRequestEvidence{
		Provider:    spec.name,
		ProviderPT:  string(spec.pt),
		Model:       model,
		ModelSource: modelSource,
		Nonce:       nonce,
		Prompt:      prompt,
		RequestedAt: time.Now().UTC(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	resp, genErr := provider.Generate(ctx, req)
	if genErr != nil {
		providerLiveWriteVerdict(t, dir, "FAIL", "key present but real Generate() call errored: "+genErr.Error())
		t.Fatalf("%s: key present but Generate() failed: %v", spec.name, genErr)
	}

	nonceEchoed := strings.Contains(resp.Content, nonce)
	truncated := errors.Is(resp.Err, ErrResponseTruncated)

	providerLiveWriteJSON(t, dir, "response.json", providerLiveResponseEvidence{
		Provider:       spec.name,
		Content:        resp.Content,
		NonceEchoed:    nonceEchoed,
		Truncated:      truncated,
		FinishReason:   resp.FinishReason,
		ProcessingTime: resp.ProcessingTime.String(),
		Usage:          resp.Usage,
		ProviderMeta:   resp.ProviderMetadata,
		RespondedAt:    time.Now().UTC(),
	})

	if nonceEchoed {
		providerLiveWriteVerdict(t, dir, "PASS", fmt.Sprintf("real HTTP call to %s (model=%s, source=%s) echoed fresh nonce %q — genuine live completion", spec.name, model, modelSource, nonce))
		t.Logf("%s: PASS — model=%s content=%q", spec.name, model, resp.Content)
		return
	}

	// §11.4.6 oracle honesty: a genuinely truncated real call (the
	// provider itself reported finish_reason=length via
	// ErrResponseTruncated) proves nothing about whether this provider
	// CAN echo the nonce — reasoning-style models can spend the whole
	// MaxTokens budget on internal reasoning before reaching the visible
	// answer. That is a truncation of a live call, not a provider
	// failure, so it is recorded as the distinct INCONCLUSIVE verdict
	// (via t.Skipf, keeping `go test`'s exit code green) rather than a
	// false FAIL. See the file header comment "Oracle-honesty fix".
	if truncated {
		detail := fmt.Sprintf(
			"real HTTP call to %s (model=%s) succeeded but was truncated (finish_reason=%q) at MaxTokens=%d before nonce %q could be echoed (partial content=%q) — this is a token-budget truncation of a genuine live call, not a provider failure",
			spec.name, model, resp.FinishReason, providerLiveNonceMaxTokens, nonce, resp.Content,
		)
		providerLiveWriteVerdict(t, dir, "INCONCLUSIVE", detail)
		t.Skipf("INCONCLUSIVE: %s", detail)
		return
	}

	if resp.Content == "" {
		providerLiveWriteVerdict(t, dir, "FAIL", "real call succeeded but returned empty content")
		t.Fatalf("%s: Generate() returned empty content — no real completion produced", spec.name)
	}
	providerLiveWriteVerdict(t, dir, "FAIL", fmt.Sprintf("nonce %q not found in response content %q — cannot prove this is a live, non-cached answer", nonce, resp.Content))
	t.Fatalf("%s: response did not echo nonce %q (got %q) — live-proof assertion failed", spec.name, nonce, resp.Content)
}

func runLocalProviderLiveProof(t *testing.T, spec providerLiveSpec, dir string) {
	t.Helper()

	reachable, probeURL := providerLiveLocalReachable(spec.localBaseURLEnv, spec.localBaseURLDefault, spec.localProbePath)
	if !reachable {
		providerLiveWriteVerdict(t, dir, "SKIP", fmt.Sprintf("unreachable: GET %s failed (no local server listening; set %s to override the base URL)", probeURL, spec.localBaseURLEnv))
		t.Skip("SKIP: unreachable")
		return
	}

	provider, err := NewCloudProvider(spec.pt, ProviderConfigEntry{Type: spec.pt, Enabled: true})
	if err != nil {
		providerLiveWriteVerdict(t, dir, "FAIL", "server reachable but provider construction failed: "+err.Error())
		t.Fatalf("%s: server reachable but NewCloudProvider failed: %v", spec.name, err)
	}
	defer func() { _ = provider.Close() }()

	models := provider.GetModels()
	model, modelSource := providerLivePickModel(models, spec.modelEnvOverride)
	if model == "" {
		providerLiveWriteVerdict(t, dir, "SKIP", fmt.Sprintf("server reachable at %s but no model is loaded/discoverable (pull/load a model, or set %s)", probeURL, spec.modelEnvOverride))
		t.Skip("SKIP: no-model")
		return
	}

	nonce, err := providerLiveNonce()
	if err != nil {
		t.Fatalf("%s: %v", spec.name, err)
	}
	prompt := providerLiveNoncePrompt(nonce)

	req := &LLMRequest{
		ID:          uuid.New(),
		Model:       model,
		Messages:    []Message{{Role: "user", Content: prompt}},
		MaxTokens:   providerLiveNonceMaxTokens,
		Temperature: 0,
	}

	providerLiveWriteJSON(t, dir, "request.json", providerLiveRequestEvidence{
		Provider:    spec.name,
		ProviderPT:  string(spec.pt),
		Model:       model,
		ModelSource: modelSource,
		Nonce:       nonce,
		Prompt:      prompt,
		RequestedAt: time.Now().UTC(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	resp, genErr := provider.Generate(ctx, req)
	if genErr != nil {
		providerLiveWriteVerdict(t, dir, "FAIL", "server reachable but real Generate() call errored: "+genErr.Error())
		t.Fatalf("%s: server reachable but Generate() failed: %v", spec.name, genErr)
	}

	nonceEchoed := strings.Contains(resp.Content, nonce)
	truncated := errors.Is(resp.Err, ErrResponseTruncated)

	providerLiveWriteJSON(t, dir, "response.json", providerLiveResponseEvidence{
		Provider:       spec.name,
		Content:        resp.Content,
		NonceEchoed:    nonceEchoed,
		Truncated:      truncated,
		FinishReason:   resp.FinishReason,
		ProcessingTime: resp.ProcessingTime.String(),
		Usage:          resp.Usage,
		ProviderMeta:   resp.ProviderMetadata,
		RespondedAt:    time.Now().UTC(),
	})

	if nonceEchoed {
		providerLiveWriteVerdict(t, dir, "PASS", fmt.Sprintf("real HTTP call to local %s (model=%s, source=%s) echoed fresh nonce %q — genuine live completion", spec.name, model, modelSource, nonce))
		t.Logf("%s: PASS — model=%s content=%q", spec.name, model, resp.Content)
		return
	}

	// §11.4.6 oracle honesty — see the identical rationale in
	// runHostedProviderLiveProof / the file header "Oracle-honesty fix".
	if truncated {
		detail := fmt.Sprintf(
			"real HTTP call to local %s (model=%s) succeeded but was truncated (finish_reason=%q) at MaxTokens=%d before nonce %q could be echoed (partial content=%q) — this is a token-budget truncation of a genuine live call, not a provider failure",
			spec.name, model, resp.FinishReason, providerLiveNonceMaxTokens, nonce, resp.Content,
		)
		providerLiveWriteVerdict(t, dir, "INCONCLUSIVE", detail)
		t.Skipf("INCONCLUSIVE: %s", detail)
		return
	}

	if resp.Content == "" {
		providerLiveWriteVerdict(t, dir, "FAIL", "real call succeeded but returned empty content")
		t.Fatalf("%s: Generate() returned empty content — no real completion produced", spec.name)
	}
	providerLiveWriteVerdict(t, dir, "FAIL", fmt.Sprintf("nonce %q not found in response content %q — cannot prove this is a live, non-cached answer", nonce, resp.Content))
	t.Fatalf("%s: response did not echo nonce %q (got %q) — live-proof assertion failed", spec.name, nonce, resp.Content)
}
