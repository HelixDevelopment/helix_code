package agent

import (
	"strings"
	"testing"
)

// stringerSample implements fmt.Stringer with a recognizable text. Used to
// prove stringifyResult prefers String() over %v.
type stringerSample struct{ text string }

func (s stringerSample) String() string { return s.text }

// TestStringifyResult_Variants is the RED→GREEN guard for the agent tool-loop
// bug where a tool returning a non-string value (canonical case: fs_read's
// *FileContent, whose Content is a []byte) reached the model as a decimal byte
// array via fmt.Sprintf("%v", v). The fix renders common types readably:
// nil→"", string→as-is, error→.Error(), []byte→string(b), fmt.Stringer→
// .String(), else JSON, falling back to %v on marshal error.
func TestStringifyResult_Variants(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := stringifyResult(nil); got != "" {
			t.Fatalf("nil ⇒ \"\"; got %q", got)
		}
	})

	t.Run("string passes through", func(t *testing.T) {
		if got := stringifyResult("hello"); got != "hello" {
			t.Fatalf("string ⇒ as-is; got %q", got)
		}
	})

	t.Run("error renders Error()", func(t *testing.T) {
		err := errSample("boom")
		if got := stringifyResult(err); got != "boom" {
			t.Fatalf("error ⇒ .Error(); got %q", got)
		}
	})

	t.Run("[]byte renders as text not byte array", func(t *testing.T) {
		got := stringifyResult([]byte("hi"))
		if got != "hi" {
			t.Fatalf("[]byte(\"hi\") ⇒ \"hi\"; got %q", got)
		}
		if strings.Contains(got, "[104 105") { // "hi" as bytes
			t.Fatalf("[]byte must NOT render as a decimal byte array; got %q", got)
		}
	})

	t.Run("fmt.Stringer renders String()", func(t *testing.T) {
		got := stringifyResult(stringerSample{text: "stringer-text"})
		if got != "stringer-text" {
			t.Fatalf("Stringer ⇒ .String(); got %q", got)
		}
	})

	t.Run("struct renders JSON not %v map form", func(t *testing.T) {
		got := stringifyResult(struct {
			A string `json:"a"`
			B int    `json:"b"`
		}{A: "x", B: 7})
		if !strings.Contains(got, `"a"`) || !strings.Contains(got, "x") || !strings.Contains(got, "7") {
			t.Fatalf("struct ⇒ JSON with fields; got %q", got)
		}
		// %v form of a struct is "{x 7}" with no quoted keys.
		if strings.HasPrefix(strings.TrimSpace(got), "{x") {
			t.Fatalf("struct must render as JSON, not the %%v form; got %q", got)
		}
	})

	t.Run("map renders JSON not map[...] %v form", func(t *testing.T) {
		got := stringifyResult(map[string]int{"k": 3})
		if !strings.Contains(got, `"k"`) || !strings.Contains(got, "3") {
			t.Fatalf("map ⇒ JSON; got %q", got)
		}
		if strings.Contains(got, "map[") {
			t.Fatalf("map must render as JSON, not the %%v \"map[...]\" form; got %q", got)
		}
	})
}

// errSample is a minimal error value for the error-branch assertion.
type errSample string

func (e errSample) Error() string { return string(e) }
