// Package autocommit — secret_filter.go (P2-F22-T04).
//
// SecretFilter strips four common credential patterns from the commit
// subject before the commit is exec'd. Best-effort — the filter is the
// final line of defence; the primary defence is that the LLM gets the
// staged diff, which may already contain secrets if the user accidentally
// staged them. The filter ensures the SUBJECT (which is exposed via
// `git log` and any subsequent `git push`) is at least scrubbed.
//
// Patterns covered (per CONST-042 + spec §3 #5):
//   - AKIA[0-9A-Z]{16}        — AWS access keys
//   - sk-[A-Za-z0-9]{20,}      — OpenAI / generic
//   - xox[baprs]-[A-Za-z0-9-]{10,}  — Slack tokens (b/a/p/r/s)
//   - gh[pousr]_[A-Za-z0-9]{36}     — GitHub PATs
//
// The filter does NOT touch the diff body — that lives only in `git
// show` and the user's local repo. CONST-043 forbids pushing
// auto-commits, so the diff body never leaves the host.
package autocommit

import "regexp"

// SecretFilter is a regex-replace pipeline. Construct via NewSecretFilter.
type SecretFilter struct {
	patterns []*regexp.Regexp
}

// NewSecretFilter constructs a default filter with the four canonical
// patterns. The order is irrelevant since each pattern is non-overlapping.
func NewSecretFilter() *SecretFilter {
	return &SecretFilter{
		patterns: []*regexp.Regexp{
			regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
			regexp.MustCompile(`sk-[A-Za-z0-9]{20,}`),
			regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
			regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36}`),
		},
	}
}

// Filter returns s with every match of every pattern replaced by the
// literal string "[REDACTED]". Multiple matches in one input are all
// replaced.
func (f *SecretFilter) Filter(s string) string {
	for _, p := range f.patterns {
		s = p.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}
