// translator.go — CONST-046 message-ID resolver seam for
// internal/voice.
//
// Round-226 §11.4 anti-bluff sweep (2026-05-19). Mirrors the
// "consumer defines its own Translator + tr() helper" pattern used
// by every other CONST-046-migrated package in this codebase
// (rounds 93/94/95/96/108/131/134/136-225, most recently
// internal/secrets 225, internal/render 224, internal/quality 223,
// internal/logging 162).
//
// The seam routes the three tool-description literals emitted by
// voice_tools.go (voice_start / voice_stop / voice_transcribe
// Description() returns) through a locale-aware resolver. Without
// this migration, every non-English operator sees the English
// description text verbatim — silently breaking the CLI help
// surface for them per CONST-046.
package voice

import (
	"context"
	"sync"

	voicei18n "dev.helix.code/internal/voice/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this package. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator.
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — the Tool implementations
// (VoiceStartTool / VoiceStopTool / VoiceTranscribeTool) carry no
// translator field today and threading one through the
// NewVoice*Tool constructors would break every existing call site
// in cmd/cli/main.go (which already wires the tools at boot). The
// package-level seam matches the established pattern from
// auth/mcp/cognee/logging/secrets/etc.
//
// HXC-014b §11.4.85 fix: SetTranslator (a write) may run concurrently
// with tr() (a read), so both accesses MUST be guarded by translatorMu
// — otherwise the concurrent read/write is a data race (caught by
// `go test -race`), a §11.4.85(B) state-corruption defect.
var (
	translatorMu sync.RWMutex
	translator   voicei18n.Translator = voicei18n.NoopTranslator{}
)

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr voicei18n.Translator) {
	translatorMu.Lock()
	defer translatorMu.Unlock()
	if tr == nil {
		translator = voicei18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this package. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
//
// HXC-014b §11.4.85(B): a panicking Translator MUST NOT crash the
// emitting goroutine — the recover() below isolates such a panic and
// degrades to the message ID.
func tr(ctx context.Context, msgID string, data map[string]any) (result string) {
	translatorMu.RLock()
	active := translator
	translatorMu.RUnlock()
	if active == nil {
		active = voicei18n.NoopTranslator{}
	}

	defer func() {
		if r := recover(); r != nil {
			result = msgID
		}
	}()

	out, err := active.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}
