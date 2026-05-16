// Package autocommit — secret_filter_test.go (P2-F22-T04).
//
// Tests pin the four credential patterns (AWS access key, OpenAI sk-,
// Slack xox*, GitHub gh[pousr]_) byte-for-byte. CONST-042 hard rule.
package autocommit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSecretFilter_AKIA_Redacted(t *testing.T) {
	f := NewSecretFilter()
	out := f.Filter("key=AKIAABCDEFGHIJKLMNOP rest")
	require.Equal(t, "key=[REDACTED] rest", out)
}

func TestSecretFilter_OpenAI_Redacted(t *testing.T) {
	f := NewSecretFilter()
	out := f.Filter("key=sk-abcdefghij1234567890abcdef rest")
	require.Contains(t, out, "[REDACTED]")
	require.NotContains(t, out, "sk-abc")
}

func TestSecretFilter_Slack_Redacted_Bot(t *testing.T) {
	f := NewSecretFilter()
	out := f.Filter("xoxb-1234567890ab")
	require.Contains(t, out, "[REDACTED]")
	require.NotContains(t, out, "xoxb-1")
}

func TestSecretFilter_Slack_Redacted_AllVariants(t *testing.T) {
	f := NewSecretFilter()
	for _, prefix := range []string{"xoxb", "xoxa", "xoxp", "xoxr", "xoxs"} {
		raw := prefix + "-1234567890abcdef"
		out := f.Filter(raw)
		require.Contains(t, out, "[REDACTED]", "should redact %q", raw)
	}
}

func TestSecretFilter_GitHub_Redacted_All(t *testing.T) {
	f := NewSecretFilter()
	for _, prefix := range []string{"ghp", "gho", "ghu", "ghs", "ghr"} {
		raw := prefix + "_" + strings.Repeat("a", 36)
		out := f.Filter(raw)
		require.Contains(t, out, "[REDACTED]", "should redact %q", raw)
		require.NotContains(t, out, prefix+"_a")
	}
}

func TestSecretFilter_NoSecrets_Untouched(t *testing.T) {
	f := NewSecretFilter()
	in := "Auto-edit: fs_write on x.txt"
	out := f.Filter(in)
	require.Equal(t, in, out)
}

func TestSecretFilter_MultipleSecrets_AllRedacted(t *testing.T) {
	f := NewSecretFilter()
	in := "AKIAABCDEFGHIJKLMNOP and sk-abcdefghij1234567890abcdef"
	out := f.Filter(in)
	require.NotContains(t, out, "AKIA")
	require.NotContains(t, out, "sk-abc")
	// Two redactions expected.
	require.Equal(t, 2, strings.Count(out, "[REDACTED]"))
}
