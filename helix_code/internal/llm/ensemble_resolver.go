package llm

import (
	"context"
	"sort"
	"strings"
)

// ensemble_resolver.go — DYNAMIC, verification-driven per-member model resolution
// for the Helix Agent ensemble (CONST-036 / CONST-040 / operator hard directive
// "nothing can be hardcoded, everything MUST BE dynamic").
//
// WHY THIS EXISTS:
//   The ensemble sentinel model ("helix-agent-ensemble") is not a real cloud
//   model. When the ensemble fans a prompt to a member, each member must run on a
//   model IT actually serves. The model id MUST NOT be a hardcoded name, a
//   hardcoded family marker, or GetModels()[0] (catalogues routinely lead with
//   decommissioned / paid-402 / embedding models). The single source of truth for
//   "which model of provider X is verified + chat-capable + best" is LLMsVerifier
//   (CONST-036): its VerifiedModel catalogue carries Provider, Verified,
//   Deprecated, SupportsEmbeddings, Capabilities, and OverallScore.
//
// REUSE (§11.4.74): the llm package already owns a decoupled bridge to the
// verifier subsystem — the package-level verifierAdapter set via
// SetVerifierAdapter (verifier_bridge.go). This resolver consumes that SAME
// adapter; it adds no new injection surface and no new verifier import path.
//
// ANTI-BLUFF (CONST-035 / §11.4.123): this file contains ZERO hardcoded model
// id / family strings. Every selectable model id is read at runtime from the
// verifier catalogue (primary) or the provider's OWN live catalogue capability
// flags (fallback) — never from a literal name list.

// embeddingCapabilityHints are NOT model names — they are ModelCapability /
// verifier-capability *capability tokens* used to recognise an embedding-only
// catalogue entry from its declared CAPABILITIES (not its name). The verifier
// supplies a first-class SupportsEmbeddings bool which is preferred; this set is
// the catalogue-capability analogue for the offline fallback, matched against a
// model's declared capability tokens, never against its id/family. Keeping the
// match on declared *capabilities* (not id substrings) is what makes it dynamic:
// a model is excluded because it DECLARES itself an embedding model, not because
// its name happens to contain a banned substring.
var embeddingCapabilityHints = map[string]bool{
	"embedding":  true,
	"embeddings": true,
	"embed":      true,
}

// ensembleVerifiedModelFor returns the best verified, non-deprecated,
// chat/text-generation-capable model id that LLMsVerifier reports for the given
// provider type, or "" when the verifier is disabled/unreachable or reports no
// eligible model for that provider. The selection is purely
// verification+capability+score driven (CONST-036/040): highest OverallScore
// among verified, non-deprecated, non-embedding models of that provider, with a
// declared text-generation capability (or no declared capabilities at all, in
// which case the verifier's Verified flag already certifies usability).
//
// It reads the package-level verifierAdapter (verifier_bridge.go) — the SAME
// adapter the rest of the llm package uses — so when an application wires the
// verifier (server / CLI / a future TUI), the ensemble automatically becomes
// fully verifier-driven with no extra plumbing.
func ensembleVerifiedModelFor(ctx context.Context, providerType ProviderType) string {
	if verifierAdapter == nil || !verifierAdapter.IsEnabled() {
		return ""
	}
	models, err := verifierAdapter.GetVerifiedModels(ctx)
	if err != nil || len(models) == 0 {
		// Verifier reachable-but-erroring or empty: degrade to the catalogue
		// fallback (handled by the caller). Never guess a name here (§11.4.6).
		return ""
	}

	want := strings.ToLower(strings.TrimSpace(string(providerType)))
	type cand struct {
		id    string
		score float64
	}
	cands := make([]cand, 0, len(models))
	for _, m := range models {
		if m == nil || m.ID == "" {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(m.Provider), want) &&
			!strings.EqualFold(strings.TrimSpace(m.ProviderType), want) {
			continue
		}
		if !m.Verified || m.Deprecated {
			continue // CONST-037: only verifier-verified, non-retired models.
		}
		if m.SupportsEmbeddings {
			continue // embedding endpoints cannot serve a chat prompt.
		}
		if verifierCapabilitiesAreEmbeddingOnly(m.Capabilities) {
			continue
		}
		score := m.OverallScore
		if score == 0 {
			score = m.Score
		}
		cands = append(cands, cand{id: strings.TrimSpace(m.ID), score: score})
	}
	if len(cands) == 0 {
		return ""
	}
	// Highest score wins; ties broken by id for deterministic, reproducible
	// resolution (§11.4.50).
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].id < cands[j].id
	})
	return cands[0].id
}

// verifierCapabilitiesAreEmbeddingOnly reports whether a verifier model's
// DECLARED capability tokens mark it as embedding-only (no chat surface). It
// matches capability *tokens*, never the model id/family — so the exclusion is
// driven by what the model declares it can do, not by a hardcoded name list.
// An entry with no declared capabilities is NOT embedding-only (the verifier's
// Verified flag already certifies it; we never reject on absence of data).
func verifierCapabilitiesAreEmbeddingOnly(caps []string) bool {
	if len(caps) == 0 {
		return false
	}
	hasChat := false
	hasEmbedding := false
	for _, c := range caps {
		token := strings.ToLower(strings.TrimSpace(c))
		if token == "" {
			continue
		}
		if embeddingCapabilityHints[token] {
			hasEmbedding = true
			continue
		}
		// Any non-embedding declared capability counts as a usable chat surface
		// signal (text generation / code / reasoning / chat / completion / etc.).
		hasChat = true
	}
	return hasEmbedding && !hasChat
}

// catalogueChatCandidatesFor returns a member provider's chat-capable model ids
// in deterministic order, derived ENTIRELY from the provider's OWN live
// catalogue capability flags — never a hardcoded name/family list. This is the
// honest offline fallback (§11.4.6) used when the verifier is not wired or
// reports nothing for the provider: the model itself, via ModelInfo.Capabilities
// / SupportsVision, tells us whether it can serve a chat prompt.
//
// Inclusion rule (capability-driven, not name-driven):
//   - a model is a chat candidate when it declares CapabilityTextGeneration,
//     OR declares NO capabilities at all (unknown ⇒ eligible: providers that do
//     not enrich per-model capabilities still surface their chat models here,
//     and the resilient try-loop verifies usability at call time);
//   - a model is EXCLUDED only when its declared capabilities are an
//     embedding/vision-only set with no text-generation capability — i.e. the
//     catalogue itself says it cannot do chat.
func catalogueChatCandidatesFor(p Provider) []string {
	models := p.GetModels()
	out := make([]string, 0, len(models))
	seen := map[string]bool{}
	for _, m := range models {
		id := strings.TrimSpace(m.ID)
		if id == "" {
			id = strings.TrimSpace(m.Name)
		}
		if id == "" || seen[id] {
			continue
		}
		if !modelInfoCanChat(m) {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// modelInfoCanChat reports whether a catalogue ModelInfo can serve a chat /
// text-generation prompt, judged from its DECLARED capabilities only. Unknown
// (no declared capabilities) ⇒ eligible (we never reject on missing data; the
// resilient call-time loop is the final arbiter). A model whose declared
// capabilities are vision-only / embedding-only with NO text-generation
// capability is excluded — the catalogue itself says it is not a chat model.
func modelInfoCanChat(m ModelInfo) bool {
	if len(m.Capabilities) == 0 {
		return true // unknown ⇒ eligible (no hardcoded name guard).
	}
	hasText := false
	hasNonChat := false
	for _, c := range m.Capabilities {
		switch c {
		case CapabilityTextGeneration, CapabilityCodeGeneration, CapabilityCodeAnalysis,
			CapabilityReasoning, CapabilityWriting, CapabilityAnalysis,
			CapabilityPlanning, CapabilityDebugging, CapabilityRefactoring,
			CapabilityTesting, CapabilityDocumentation:
			hasText = true
		case CapabilityVision:
			hasNonChat = true
		}
	}
	if hasText {
		return true
	}
	// No text capability declared. If the model declares ONLY a non-chat
	// capability (e.g. vision-only), it is not a chat candidate.
	return !hasNonChat
}
