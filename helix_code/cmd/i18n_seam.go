package cmd

import (
	"context"

	"dev.helix.code/cmd/i18n"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by the helix_code/cmd cobra command tree. Defaults
// to i18n.NoopTranslator{} (loud message-ID echo) so unit tests +
// ad-hoc invocations remain obvious. helix_code wires a real
// *i18nadapter.Translator at boot via SetTranslator (round-315 §11.4
// anti-bluff sweep, 2026-05-20).
//
// A package-level variable is the chosen DI seam because cobra
// command handlers (RunE func(*cobra.Command, []string) error) do not
// support extra injected parameters without restructuring the command
// tree — global injection matches the package's existing use of
// package-level state (rootCmd, viper bindings).
var translator i18n.Translator = i18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(t i18n.Translator) {
	if t == nil {
		translator = i18n.NoopTranslator{}
		return
	}
	translator = t
}

// tr is the internal CONST-046 resolver used by every migrated
// user-facing string emission in this package. It NEVER returns an
// error to the caller — translation failures degrade to the message
// ID itself (matching NoopTranslator behaviour) so production output
// remains loud + obvious instead of silently empty.
func tr(ctx context.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = i18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}

// trc is the CONST-046 resolver for strings needed at cobra
// command-construction time (Short / Long descriptions, flag-help
// text) — points where no request-scoped context.Context is
// available. It resolves against context.Background() through the
// same package-level translator the runtime tr() helper uses.
func trc(msgID string, data map[string]any) string {
	return tr(context.Background(), msgID, data)
}
