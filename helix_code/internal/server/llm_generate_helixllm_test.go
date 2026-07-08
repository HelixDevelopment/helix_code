package server

import (
	"testing"

	"dev.helix.code/internal/llm"
	"github.com/stretchr/testify/require"
)

// llm_generate_helixllm_test.go — RED-first coverage (§11.4.115 / §11.4.146)
// proving resolveLLMProvider gained a real route to the local HelixLLM coder
// (the in-repo llama.cpp OpenAI-compatible sidecar the dual-wire facade
// report — scratchpad/phase1_dual_wire_facade.md — flagged as unreachable:
// "resolveLLMProvider ... has NO path to the LOCAL HelixLLM coder").
//
// RED (pre-fix, captured verbatim before this file's fix landed — see
// scratchpad/phase1_route_to_coder.md for the full transcript):
//
//	--- FAIL: TestResolveLLMProvider_HelixLLMLocal_DefaultEndpoint
//	    llm_generate_helixllm_test.go:26:
//	        Error: "failed to construct provider \"\": unknown provider \"helixllm\" (...)"
//	        is not nil (resolveLLMProvider("helixllm", "") returned an error)
//
// Before the fix, naming "helixllm" (or "local") as the provider fell through
// to llm.Select -> parseCloudProviderType, which recognises neither name, so
// resolveLLMProvider returned errUnknownProvider — exactly the gap this test
// captures.
//
// GREEN (post-fix): resolveLLMProvider("helixllm"|"local", model) constructs
// a REAL *llm.OpenAICompatibleProvider — the SAME generic, already-tested
// OpenAI-compatible HTTP client HelixCode already uses for VLLM / LMStudio /
// LocalAI / etc. (internal/llm/openai_compatible_provider.go) — pointed at
// HELIX_LLM_LOCAL_OPENAI_ENDPOINT (default http://localhost:18434). No new
// HTTP client is written (CONST-036 / §11.4.74 reuse mandate).
func TestResolveLLMProvider_HelixLLMLocal_DefaultEndpoint(t *testing.T) {
	provider, err := resolveLLMProvider("helixllm", "")
	require.NoError(t, err, `resolveLLMProvider("helixllm", "") must construct the local HelixLLM coder route`)
	require.NotNil(t, provider)
	defer func() { _ = provider.Close() }()

	oc, ok := provider.(*llm.OpenAICompatibleProvider)
	require.True(t, ok, "expected the reused *llm.OpenAICompatibleProvider, got %T", provider)
	require.Equal(t, helixLLMLocalDefaultEndpoint, oc.BaseURL(),
		"with HELIX_LLM_LOCAL_OPENAI_ENDPOINT unset, the local coder route must default to %q", helixLLMLocalDefaultEndpoint)
	require.Equal(t, "helixllm", provider.GetName())
}

// TestResolveLLMProvider_HelixLLMLocal_AliasCaseInsensitive proves both
// documented selectors ("helixllm" and the "local" alias) resolve, in any
// case, per the task's convention: "via HELIX_LLM_LOCAL_OPENAI_ENDPOINT, or a
// helixllm/local provider selector".
func TestResolveLLMProvider_HelixLLMLocal_AliasCaseInsensitive(t *testing.T) {
	for _, name := range []string{"HelixLLM", "HELIXLLM", "local", "LOCAL", "Local"} {
		name := name
		t.Run(name, func(t *testing.T) {
			provider, err := resolveLLMProvider(name, "")
			require.NoError(t, err)
			require.NotNil(t, provider)
			defer func() { _ = provider.Close() }()
			_, ok := provider.(*llm.OpenAICompatibleProvider)
			require.True(t, ok, "expected *llm.OpenAICompatibleProvider for provider name %q, got %T", name, provider)
		})
	}
}

// TestResolveLLMProvider_HelixLLMLocal_EnvOverride proves the base URL is
// read from HELIX_LLM_LOCAL_OPENAI_ENDPOINT (§11.4.28 — no hardcoded host
// beyond the documented default) rather than a value baked into the code.
func TestResolveLLMProvider_HelixLLMLocal_EnvOverride(t *testing.T) {
	t.Setenv("HELIX_LLM_LOCAL_OPENAI_ENDPOINT", "http://127.0.0.1:19999")

	provider, err := resolveLLMProvider("helixllm", "")
	require.NoError(t, err)
	require.NotNil(t, provider)
	defer func() { _ = provider.Close() }()

	oc, ok := provider.(*llm.OpenAICompatibleProvider)
	require.True(t, ok)
	require.Equal(t, "http://127.0.0.1:19999", oc.BaseURL())
}

// TestResolveLLMProvider_HelixLLMLocal_UnrelatedUnknownProviderUnaffected is a
// non-regression guard for server defect #4 (llm_generate_regression_test.go):
// this change must not weaken the existing "explicitly-named-but-unknown
// provider surfaces errUnknownProvider" behaviour for any OTHER provider name.
func TestResolveLLMProvider_HelixLLMLocal_UnrelatedUnknownProviderUnaffected(t *testing.T) {
	_, err := resolveLLMProvider("totally-not-a-real-provider", "")
	require.Error(t, err)
	require.ErrorIs(t, err, errUnknownProvider)
}
