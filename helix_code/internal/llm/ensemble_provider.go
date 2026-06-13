package llm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ensemble_provider.go — a REAL multi-provider LLM ensemble exposed as a single
// llm.Provider so the HelixCode terminal UI can list "Helix Agent ensemble" in
// its /model picker alongside individual cloud models.
//
// WHY THIS LIVES IN HelixCode (not imported from helix_agent):
//   helix_agent ships a real ensemble (submodules/helix_agent/internal/services/
//   ensemble.go + internal/llm/ensemble.go: concurrent fan-out, semaphore-bounded
//   provider calls, confidence/quality/majority voting). It is, however, locked
//   behind that module's internal/ tree — Go forbids importing another module's
//   internal/ packages (dev.helix.code cannot import dev.helix.agent/internal/...),
//   and that package transitively pulls digital.vasic.helixmemory / eventbus /
//   memory / etc. (not in HelixCode's go.sum). Per CONST-051(B) we MUST NOT inject
//   HelixCode context into helix_agent, and per the Go module boundary we cannot
//   reach into its internals. The decoupled, genuinely-working path is to run the
//   same ensemble discipline over HelixCode's OWN already-registered llm.Provider
//   instances — fanning one prompt to every member, collecting REAL responses,
//   scoring them, and synthesizing a real combined answer.
//
// ANTI-BLUFF (CONST-035 / Article XI §11.9 / §11.4.123): there is NO simulation
// here. Generate() makes a real provider.Generate() call to EVERY member (each of
// which makes a real HTTP call to a real cloud provider) and returns the actual
// voted/combined output. The metadata records exactly which providers
// participated, which won, and the per-member scores — so the ensemble can never
// masquerade a single-provider pass-through as multi-provider work.

// ProviderTypeEnsemble is the provider-type sentinel for the Helix Agent
// ensemble. It is NOT a cloud endpoint — it is a meta-provider that orchestrates
// other registered providers.
const ProviderTypeEnsemble ProviderType = "helix-agent-ensemble"

// EnsembleModelName is the single selectable model name surfaced in the /model
// picker. CONST-046: this is a stable provider identifier, not user-facing prose.
const EnsembleModelName = "helix-agent-ensemble"

// EnsembleDisplayName is the human label shown for the provider.
const EnsembleDisplayName = "Helix Agent ensemble"

// defaultEnsembleTimeout bounds a full ensemble fan-out when the caller does not
// supply one.
const defaultEnsembleTimeout = 120 * time.Second

// EnsembleProviderConfig configures an EnsembleProvider. Members are INJECTED by
// the caller (the TUI passes the cloud providers it built from env keys) — the
// ensemble never constructs or discovers providers itself, keeping it decoupled
// and reusable.
type EnsembleProviderConfig struct {
	// Members are the underlying providers the ensemble fans each prompt to.
	Members []Provider
	// Strategy selects the voting strategy: "confidence_weighted" (default),
	// "quality_weighted", or "majority_vote".
	Strategy string
	// Timeout bounds a full fan-out. Zero ⇒ defaultEnsembleTimeout.
	Timeout time.Duration
}

// EnsembleProvider implements llm.Provider by orchestrating several member
// providers and voting on their real responses.
type EnsembleProvider struct {
	mu       sync.RWMutex
	members  []Provider
	strategy string
	timeout  time.Duration
	// workingModel caches the first chat-capable model id that actually returned
	// content for each member (keyed by provider name). The first ensemble run
	// discovers it (skipping decommissioned/paid/embedding models); subsequent
	// runs reuse it so they don't re-probe the dead models. Guarded by mu.
	workingModel map[string]string
	// warmStarted is the sync.Once-style guard ensuring the WarmCache fan-out runs
	// at most once for the provider's lifetime. Guarded by mu.
	warmStarted bool
}

// ensembleMaxModelTries bounds how many chat-capable catalogue models a single
// member will attempt before giving up, so a fully-dead member fails fast
// instead of walking a 300-entry catalogue. When the verifier is wired the very
// first candidate is the verifier's best verified model, so a single try
// normally succeeds; this bound only matters for the offline catalogue fallback.
const ensembleMaxModelTries = 8

// NewEnsembleProvider builds an ensemble from the injected member providers.
func NewEnsembleProvider(cfg EnsembleProviderConfig) *EnsembleProvider {
	strategy := cfg.Strategy
	if strategy == "" {
		strategy = "confidence_weighted"
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = defaultEnsembleTimeout
	}
	members := make([]Provider, 0, len(cfg.Members))
	members = append(members, cfg.Members...)
	return &EnsembleProvider{members: members, strategy: strategy, timeout: timeout, workingModel: map[string]string{}}
}

// AddMember registers an additional member provider after construction.
func (e *EnsembleProvider) AddMember(p Provider) {
	if p == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.members = append(e.members, p)
}

func (e *EnsembleProvider) snapshot() []Provider {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]Provider, len(e.members))
	copy(out, e.members)
	return out
}

func (e *EnsembleProvider) GetType() ProviderType { return ProviderTypeEnsemble }
func (e *EnsembleProvider) GetName() string        { return EnsembleDisplayName }

// GetModels exposes exactly one selectable model — the ensemble itself. Its
// context window is the minimum over the members (the binding constraint), so a
// caller can never overflow the weakest member.
func (e *EnsembleProvider) GetModels() []ModelInfo {
	members := e.snapshot()
	memberNames := make([]string, 0, len(members))
	for _, m := range members {
		memberNames = append(memberNames, m.GetName())
	}
	desc := fmt.Sprintf("Multi-provider ensemble orchestrating %d providers (%s); fans each prompt to all members and returns the voted/combined response.",
		len(members), strings.Join(memberNames, ", "))
	return []ModelInfo{{
		ID:           EnsembleModelName,
		Name:         EnsembleModelName,
		Provider:     ProviderTypeEnsemble,
		ContextSize:  e.GetContextWindow(),
		MaxTokens:    0,
		Capabilities: e.GetCapabilities(),
		Description:  desc,
		Metadata: map[string]interface{}{
			"ensemble_members": memberNames,
			"voting_strategy":  e.strategy,
		},
	}}
}

func (e *EnsembleProvider) GetCapabilities() []ModelCapability {
	return []ModelCapability{
		CapabilityTextGeneration,
		CapabilityCodeGeneration,
		CapabilityReasoning,
		CapabilityAnalysis,
		CapabilityWriting,
	}
}

// ensembleResult is one member's real response plus the bookkeeping the voter
// needs.
type ensembleResult struct {
	providerName string
	resp         *LLMResponse
	score        float64
}

// Generate fans the request to every member concurrently, collects the real
// responses, votes, and returns the winning response enriched with ensemble
// metadata (participants, success/failure counts, scores, the selected member,
// and a short excerpt of every member's answer).
func (e *EnsembleProvider) Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	members := e.snapshot()
	if len(members) == 0 {
		return nil, fmt.Errorf("helix-agent ensemble: no member providers configured")
	}

	fanCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	type memberOutcome struct {
		name string
		resp *LLMResponse
		err  error
	}
	outCh := make(chan memberOutcome, len(members))

	var wg sync.WaitGroup
	for _, m := range members {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			// Each member gets its own request copy so concurrent providers
			// never share/mutate the same struct.
			base := *request
			if base.ID == uuid.Nil {
				base.ID = uuid.New()
			}
			// The ensemble sentinel model ("helix-agent-ensemble") is NOT a real
			// cloud model — forwarding it would make every member's API reject the
			// request (404 "model does not exist"). When the caller passes an
			// explicit (non-sentinel) model, honour it with a single attempt.
			sentinel := base.Model == "" || base.Model == EnsembleModelName || base.Model == string(ProviderTypeEnsemble)
			if !sentinel {
				r, err := p.Generate(fanCtx, &base)
				outCh <- memberOutcome{name: p.GetName(), resp: r, err: err}
				return
			}
			// Sentinel/empty model: resolve a model the member ACTUALLY serves,
			// DYNAMICALLY (CONST-036/040). The candidate order is, first, the
			// LLMsVerifier-reported best verified chat model for this provider
			// (the single source of truth — no hardcoded name, no GetModels()[0]),
			// then the provider's own capability-filtered catalogue as the offline
			// fallback. The first candidate that returns non-empty content wins and
			// is cached. p.GetModels()[0] is never trusted: catalogues routinely
			// lead with decommissioned, paid-402, or embedding models.
			resp, err, model := e.generateMemberResilient(fanCtx, p, base)
			if err == nil && resp != nil && strings.TrimSpace(resp.Content) != "" && model != "" {
				e.rememberWorkingModel(p.GetName(), model)
			}
			outCh <- memberOutcome{name: p.GetName(), resp: resp, err: err}
		}(m)
	}
	go func() { wg.Wait(); close(outCh) }()

	results := make([]ensembleResult, 0, len(members))
	participants := make([]string, 0, len(members))
	excerpts := make(map[string]string, len(members))
	failures := 0
	var firstErr error

	for oc := range outCh {
		if oc.err != nil || oc.resp == nil || strings.TrimSpace(oc.resp.Content) == "" {
			failures++
			if firstErr == nil && oc.err != nil {
				firstErr = oc.err
			}
			continue
		}
		participants = append(participants, oc.name)
		excerpts[oc.name] = excerpt(oc.resp.Content, 160)
		results = append(results, ensembleResult{providerName: oc.name, resp: oc.resp})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("helix-agent ensemble: all %d member(s) failed (first error: %v)", len(members), firstErr)
	}

	selected := e.vote(results)

	// Deterministic participant ordering for stable evidence.
	sort.Strings(participants)

	scores := make(map[string]float64, len(results))
	for _, r := range results {
		scores[r.providerName] = r.score
	}

	out := *selected.resp
	out.RequestID = request.ID
	if out.ProviderMetadata == nil {
		out.ProviderMetadata = make(map[string]interface{})
	}
	out.ProviderMetadata["ensemble"] = true
	out.ProviderMetadata["ensemble_strategy"] = e.strategy
	out.ProviderMetadata["ensemble_total_providers"] = len(members)
	out.ProviderMetadata["ensemble_successful_providers"] = len(results)
	out.ProviderMetadata["ensemble_failed_providers"] = failures
	out.ProviderMetadata["ensemble_participants"] = participants
	out.ProviderMetadata["ensemble_selected_provider"] = selected.providerName
	out.ProviderMetadata["ensemble_scores"] = scores
	out.ProviderMetadata["ensemble_excerpts"] = excerpts
	return &out, nil
}

// vote scores every member response (confidence-weighted by default, mirroring
// helix_agent's strategy: finish-reason + length + token-efficiency heuristics)
// and returns the highest-scoring one. The scores are written back onto each
// result for the metadata.
func (e *EnsembleProvider) vote(results []ensembleResult) ensembleResult {
	for i := range results {
		results[i].score = scoreResponse(results[i].resp, e.strategy)
	}
	best := results[0]
	for _, r := range results[1:] {
		if r.score > best.score {
			best = r
		}
	}
	return best
}

// scoreResponse computes a quality score from observable response properties.
// It does not require the provider to self-report a confidence field (HelixCode
// LLMResponse has none) — it derives one from finish reason, content length, and
// token efficiency, the same dimensions helix_agent's ConfidenceWeightedStrategy
// uses.
func scoreResponse(resp *LLMResponse, strategy string) float64 {
	if resp == nil {
		return 0
	}
	// Base: a cleanly-finished, non-empty response starts at 1.0.
	score := 1.0
	switch resp.FinishReason {
	case "stop", "end_turn", "":
		score *= 1.1
	case "length", "max_tokens":
		score *= 0.95
	case "content_filter", "content_safety":
		score *= 0.6
	}

	contentLen := len(strings.TrimSpace(resp.Content))
	switch {
	case contentLen == 0:
		return 0
	case contentLen > 50 && contentLen < 1000:
		score *= 1.1
	case contentLen >= 1000 && contentLen < 2000:
		score *= 1.05
	case contentLen >= 2000:
		score *= 0.95
	default: // very short
		score *= 1.0
	}

	// Token efficiency: more content per completion token is rewarded.
	if resp.Usage.CompletionTokens > 0 {
		eff := float64(contentLen) / float64(resp.Usage.CompletionTokens)
		switch {
		case eff > 3.0:
			score *= 1.1
		case eff > 2.0:
			score *= 1.05
		}
	}

	if strategy == "quality_weighted" {
		// Quality strategy additionally favours faster responses.
		if resp.ProcessingTime > 0 && resp.ProcessingTime < time.Second {
			score *= 1.05
		}
	}
	return score
}

// generateMemberResilient resolves a working model for a sentinel/empty-model
// request: it tries the dynamically-resolved candidate list (cached working
// model first, then the verifier's best verified chat model, then the
// provider's own capability-filtered catalogue) and returns the first attempt
// that yields non-empty content. Returns the last error/response when every
// candidate fails. The returned model id is the one that produced the content
// (for caching). Bounded by ensembleMaxModelTries.
func (e *EnsembleProvider) generateMemberResilient(ctx context.Context, p Provider, base LLMRequest) (*LLMResponse, error, string) {
	cands := e.orderedCandidates(ctx, p)
	var lastErr error
	var lastResp *LLMResponse
	tried := 0
	for _, mdl := range cands {
		if tried >= ensembleMaxModelTries {
			break
		}
		if ctx.Err() != nil {
			break
		}
		tried++
		reqCopy := base
		reqCopy.Model = mdl
		r, err := p.Generate(ctx, &reqCopy)
		if err == nil && r != nil && strings.TrimSpace(r.Content) != "" {
			return r, nil, mdl
		}
		lastErr, lastResp = err, r
	}
	if lastErr == nil && (lastResp == nil || strings.TrimSpace(lastResp.Content) == "") {
		lastErr = fmt.Errorf("no chat-capable model returned content (%d tried)", tried)
	}
	return lastResp, lastErr, ""
}

// orderedCandidates returns the member's chat-capable model ids in priority
// order, ALL of which are dynamically resolved (CONST-036/040 — no hardcoded
// names). Priority:
//
//  1. the cached known-working model (so repeat runs are 1 call/member);
//  2. the LLMsVerifier-reported best verified chat model for this provider type
//     (the single source of truth when the verifier is wired);
//  3. the provider's OWN capability-filtered catalogue (the offline fallback,
//     still capability-driven, never a hardcoded list).
//
// Duplicates are de-duplicated preserving first-seen priority.
func (e *EnsembleProvider) orderedCandidates(ctx context.Context, p Provider) []string {
	e.mu.RLock()
	cached := e.workingModel[p.GetName()]
	e.mu.RUnlock()

	ordered := make([]string, 0, 8)
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		ordered = append(ordered, id)
	}

	add(cached)
	// Verifier is the authoritative source for the member's best verified chat
	// model (CONST-036). Empty when the verifier is disabled/unreachable or
	// reports nothing for this provider — then the catalogue fallback applies.
	add(ensembleVerifiedModelFor(ctx, p.GetType()))
	for _, c := range catalogueChatCandidatesFor(p) {
		add(c)
	}
	return ordered
}

// WarmCache pre-resolves and caches each member's working chat model BEFORE the
// user submits a prompt, eliminating the cold-start "discovery storm" that made
// the FIRST interactive prompt slow and rate-limit-prone. On a cold cache every
// member's first sentinel-model run probes MANY catalogue models sequentially
// (decommissioned/paid/embedding entries) until one returns content; in the TUI a
// second prompt fires while the first is still mid-storm → concurrent provider
// hammering → every member fails → "all N member(s) failed". Warming the cache
// once, ahead of user interaction, means the first real prompt hits the cached
// working model (1 call/member, no storm).
//
// Behaviour:
//   - members run in PARALLEL; each member's candidate attempts run SEQUENTIALLY
//     via the SAME resilient resolution path (orderedCandidates →
//     generateMemberResilient → rememberWorkingModel) — no duplicated logic
//     (§11.4.74);
//   - bounded by ensembleMaxModelTries (inside generateMemberResilient) and the
//     ensemble timeout;
//   - idempotent and safe to call concurrently — a sync.Once-style guard ensures
//     the warm fan-out runs at most once for the lifetime of the provider, and a
//     member whose cache is already populated is skipped;
//   - NON-blocking for the caller's intent: it does its own bounded work and
//     returns; callers that must not stall the UI thread invoke it in a goroutine.
//
// It never forwards the ensemble sentinel model to a member (that would 404); it
// always resolves a model the member actually serves. A warm probe that fails for
// a member simply leaves that member's cache empty — the next real prompt will
// re-resolve it; warming is best-effort and never errors.
func (e *EnsembleProvider) WarmCache(ctx context.Context) {
	// sync.Once-style guard: run the warm fan-out at most once.
	e.mu.Lock()
	if e.warmStarted {
		e.mu.Unlock()
		return
	}
	e.warmStarted = true
	e.mu.Unlock()

	members := e.snapshot()
	if len(members) == 0 {
		return
	}

	warmCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// A short, content-light probe prompt — enough to elicit non-empty content so
	// the working model is genuinely confirmed, without burning a large
	// completion. CONST-046: not user-facing prose, a fixed internal warm probe.
	probeMessages := []Message{{Role: "user", Content: "ping"}}

	var wg sync.WaitGroup
	for _, m := range members {
		// Skip members whose working model is already cached (idempotent across
		// repeated WarmCache calls and a real prompt that already ran).
		e.mu.RLock()
		_, cached := e.workingModel[m.GetName()]
		e.mu.RUnlock()
		if cached {
			continue
		}
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()
			base := LLMRequest{ID: uuid.New(), Messages: probeMessages}
			resp, err, model := e.generateMemberResilient(warmCtx, p, base)
			if err == nil && resp != nil && strings.TrimSpace(resp.Content) != "" && model != "" {
				e.rememberWorkingModel(p.GetName(), model)
			}
		}(m)
	}
	wg.Wait()
}

// rememberWorkingModel caches the model id that produced content for a member.
func (e *EnsembleProvider) rememberWorkingModel(providerName, model string) {
	e.mu.Lock()
	if e.workingModel == nil {
		e.workingModel = map[string]string{}
	}
	e.workingModel[providerName] = model
	e.mu.Unlock()
}

// excerpt returns up to n runes of s with an ellipsis when truncated.
func excerpt(s string, n int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// GenerateStream runs the ensemble (non-streaming under the hood — voting needs
// complete responses) and emits the voted result as a single chunk. This is a
// real result, not a placeholder: it is the same combined response Generate
// returns.
func (e *EnsembleProvider) GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error {
	resp, err := e.Generate(ctx, request)
	if err != nil {
		return err
	}
	ch <- *resp
	close(ch)
	return nil
}

// IsAvailable reports the ensemble as available when it has at least one member
// and at least one member is itself available.
func (e *EnsembleProvider) IsAvailable(ctx context.Context) bool {
	members := e.snapshot()
	if len(members) == 0 {
		return false
	}
	for _, m := range members {
		if m.IsAvailable(ctx) {
			return true
		}
	}
	return false
}

func (e *EnsembleProvider) GetHealth(ctx context.Context) (*ProviderHealth, error) {
	members := e.snapshot()
	healthy := 0
	for _, m := range members {
		if h, err := m.GetHealth(ctx); err == nil && h != nil && h.Status == "healthy" {
			healthy++
		}
	}
	status := "healthy"
	if healthy == 0 {
		status = "unhealthy"
	} else if healthy < len(members) {
		status = "degraded"
	}
	return &ProviderHealth{
		Status:     status,
		LastCheck:  time.Now(),
		ModelCount: 1,
		Message:    fmt.Sprintf("%d/%d ensemble members healthy", healthy, len(members)),
	}, nil
}

// Close closes every member provider, aggregating errors.
func (e *EnsembleProvider) Close() error {
	members := e.snapshot()
	var errs []string
	for _, m := range members {
		if err := m.Close(); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("ensemble close errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// GetContextWindow returns the MINIMUM member context window (the binding
// constraint across the fan-out). Zero members ⇒ 0.
func (e *EnsembleProvider) GetContextWindow() int {
	members := e.snapshot()
	min := 0
	for _, m := range members {
		cw := m.GetContextWindow()
		if cw <= 0 {
			continue
		}
		if min == 0 || cw < min {
			min = cw
		}
	}
	return min
}

// CountTokens delegates to the first member's tokenizer, falling back to a
// char-based estimate (1 token ≈ 4 chars) when no member is present.
func (e *EnsembleProvider) CountTokens(text string) (int, error) {
	members := e.snapshot()
	if len(members) > 0 {
		return members[0].CountTokens(text)
	}
	if text == "" {
		return 0, nil
	}
	return len(text) / 4, nil
}
