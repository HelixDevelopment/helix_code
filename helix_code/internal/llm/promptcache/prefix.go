package promptcache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// PrefixComponents holds the two pieces of an LLM request that, together,
// constitute the cacheable PREFIX: the system prompt text and the ordered
// tool-definition set. Provider-side prompt caching keys on the byte content
// of this prefix, so it MUST be assembled deterministically and frozen for the
// lifetime of a session.
//
// SystemPrompt is the resolved system instruction string (already composed —
// no further mutation expected).
//
// Tools is the ordered list of tool definitions. Order MATTERS to the cache
// hash: a different tool order is a different prefix even if the set is equal.
// Callers MUST present tools in a stable order (e.g. sorted by name) before
// constructing PrefixComponents — the request-builder integration does exactly
// this.
type PrefixComponents struct {
	// SystemPrompt is the resolved system instruction text.
	SystemPrompt string
	// Tools is the ordered tool-definition set. Each element may be any
	// JSON-serializable value (a provider-specific tool struct or a generic
	// map); CanonicalJSONSorted normalizes its internal ordering.
	Tools []interface{}
}

// CanonicalBytes returns the deterministic, byte-stable serialization of the
// prefix. The same PrefixComponents value (and any value equal to it) produces
// byte-identical output on every call, in every process. This is the exact
// byte sequence whose stability determines whether the provider cache hits.
//
// The output is a two-field JSON object {"system":..., "tools":[...]} so the
// system prompt and tool set are hashed together as one prefix unit.
func (p PrefixComponents) CanonicalBytes() ([]byte, error) {
	// Use a struct (fixed field order) wrapping CanonicalJSON-normalized parts.
	// We normalize tools individually then re-wrap so array order of tools is
	// preserved (caller-controlled) while each tool's internal map keys and
	// set-like arrays are sorted.
	normalizedTools := make([]interface{}, len(p.Tools))
	for i, t := range p.Tools {
		canon, err := CanonicalJSONSorted(t)
		if err != nil {
			return nil, fmt.Errorf("promptcache: tool[%d] canonicalize failed: %w", i, err)
		}
		// Re-decode so the final wrapping marshal nests it as structured JSON
		// rather than an escaped string.
		var decoded interface{}
		if err := decodeCanon(canon, &decoded); err != nil {
			return nil, fmt.Errorf("promptcache: tool[%d] re-decode failed: %w", i, err)
		}
		normalizedTools[i] = decoded
	}

	wrapper := map[string]interface{}{
		"system": p.SystemPrompt,
		"tools":  normalizedTools,
	}
	return CanonicalJSON(wrapper)
}

// Hash returns the lowercase-hex SHA-256 of the canonical prefix bytes. Two
// prefixes hash-equal if and only if they serialize byte-identically. This is
// the value the cache-break detector compares across a session.
func (p PrefixComponents) Hash() (string, error) {
	b, err := p.CanonicalBytes()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// CacheBreakDetector watches the request prefix across a session and reports
// when it changes — a "cache break". Mirrors Claude Code's
// `promptCacheBreakDetection` concept: the prefix is expected to be frozen at
// session start; any mid-session change silently destroys every subsequent
// cache hit, so the detector surfaces it loudly.
//
// Typical use:
//
//	det := promptcache.NewCacheBreakDetector()
//	frozen, _ := det.Freeze(prefix)        // session start — establishes baseline
//	...
//	res, _ := det.Check(nextPrefix)        // each turn — was the prefix mutated?
//	if res.Broken { log.Warn("cache break", res.Reason) }
//
// CacheBreakDetector is safe for concurrent use.
type CacheBreakDetector struct {
	mu       sync.RWMutex
	frozen   bool
	baseline string // hex hash of the frozen prefix
}

// NewCacheBreakDetector returns a detector with no prefix frozen yet.
func NewCacheBreakDetector() *CacheBreakDetector {
	return &CacheBreakDetector{}
}

// Freeze records p as the session's stable prefix baseline. Call this exactly
// once, at session start, after the prefix has been fully assembled. Returns
// the baseline hash. Subsequent Freeze calls overwrite the baseline (and reset
// the broken state) — useful only when a session legitimately restarts.
func (d *CacheBreakDetector) Freeze(p PrefixComponents) (string, error) {
	h, err := p.Hash()
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	d.frozen = true
	d.baseline = h
	d.mu.Unlock()
	return h, nil
}

// IsFrozen reports whether a baseline prefix has been established.
func (d *CacheBreakDetector) IsFrozen() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.frozen
}

// Baseline returns the frozen baseline hash, or "" if no prefix is frozen.
func (d *CacheBreakDetector) Baseline() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.baseline
}

// CacheBreakResult is the outcome of a CacheBreakDetector.Check call.
type CacheBreakResult struct {
	// Broken is true when the checked prefix differs from the frozen baseline.
	Broken bool
	// Reason is a human-readable explanation. Empty when not broken.
	Reason string
	// BaselineHash is the hash the prefix was expected to match.
	BaselineHash string
	// ObservedHash is the hash of the prefix that was checked.
	ObservedHash string
}

// Check compares p against the frozen baseline. If no prefix is frozen yet it
// returns a non-broken result (nothing to break against). When the prefix
// matches the baseline, Broken is false and the provider cache will hit. When
// it differs, Broken is true and the caller SHOULD log it — every cache hit for
// the rest of the session is forfeit until the prefix is re-frozen.
//
// Check NEVER mutates the baseline; the baseline only changes via Freeze. This
// is intentional: the detector's job is to catch unexpected drift, not to
// silently accept it.
func (d *CacheBreakDetector) Check(p PrefixComponents) (CacheBreakResult, error) {
	observed, err := p.Hash()
	if err != nil {
		return CacheBreakResult{}, err
	}
	d.mu.RLock()
	frozen := d.frozen
	baseline := d.baseline
	d.mu.RUnlock()

	res := CacheBreakResult{
		BaselineHash: baseline,
		ObservedHash: observed,
	}
	if !frozen {
		// Nothing frozen — caller should Freeze first. Not a break.
		return res, nil
	}
	if observed != baseline {
		res.Broken = true
		res.Reason = fmt.Sprintf(
			"prompt-cache prefix changed mid-session: baseline=%s observed=%s "+
				"(system prompt or tool definitions were mutated after session "+
				"start — provider cache will miss until prefix is re-frozen)",
			short(baseline), short(observed))
	}
	return res, nil
}

// short truncates a hex hash for compact log lines.
func short(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}

// decodeCanon decodes canonical JSON bytes into dst. Kept private — callers
// outside this package should not need to re-decode canonical output.
func decodeCanon(b []byte, dst interface{}) error {
	return jsonUnmarshalNumberAware(b, dst)
}
