// Package approvalwire holds adapters that bridge the F21 approval gate
// to other subsystems (F19 askuser prompter, future stdin/RPC sources).
// Lives in its own package to avoid an import cycle:
//
//	internal/tools/askuser → internal/approval (RequiresApproval level type)
//	internal/approval (gate manager, no askuser import)
//	internal/approvalwire → both (adapter only — does NOT close the cycle)
//
// References:
//   - Spec 7128289 §4 (Data flow), §6 (User surface)
//   - Plan bbb61de T07
//   - F19 prompter: internal/tools/askuser/stdin_prompter.go
package approvalwire

import (
	"context"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/tools/askuser"
)

// AskUserYesNoPrompter adapts an askuser.Prompter (the F19 stdin prompter or
// any compatible implementation) into the approval.PromptResponder interface
// that ApprovalManager.PromptForApproval depends on.
//
// The adapter constructs a two-choice askuser.Question (Yes/No), forwards the
// question text through the F19 prompter (which honours TTY/non-TTY mode,
// retries, timeouts, render styling), and translates the chosen value back
// into the bool ApprovalManager expects.
//
// The default polarity is preserved: when defaultYes==true the "yes" choice
// becomes Question.Default; when defaultYes==false the "no" choice does. On
// non-TTY destinations the F19 prompter auto-picks the default and returns
// UsedDefault=true, which the adapter forwards as the corresponding bool.
//
// Anti-bluff anchor: there is no fake-yes / fake-no path here. Every call
// goes through the real prompter. If the prompter returns an error (EOF,
// timeout, ctx cancellation), the adapter surfaces it verbatim to the
// caller; ApprovalManager wraps the error and the gate denies the call.
type AskUserYesNoPrompter struct {
	// Inner is the F19 prompter (or any compatible Prompter). Required.
	Inner askuser.Prompter
}

// Compile-time assertion that the adapter satisfies the responder contract.
var _ approval.PromptResponder = (*AskUserYesNoPrompter)(nil)

// PromptYesNo renders a two-choice (Yes/No) question through the F19
// prompter and translates the chosen Choice.Value back into a bool.
//
// Default polarity:
//   - defaultYes==true  : Question.Default = "yes" (TTY ENTER → allow)
//   - defaultYes==false : Question.Default = "no"  (TTY ENTER → deny)
//
// On non-TTY destinations the F19 prompter auto-picks the default and
// returns UsedDefault=true; this maps to the corresponding bool here.
//
// Errors from the inner prompter (ErrUserCancelled, ErrPrompterTimeout,
// ErrInteractiveTerminalRequired without Default, etc.) are returned
// verbatim so ApprovalManager.PromptForApproval can wrap them.
func (p *AskUserYesNoPrompter) PromptYesNo(ctx context.Context, question string, defaultYes bool) (bool, error) {
	defaultVal := "no"
	if defaultYes {
		defaultVal = "yes"
	}
	q := askuser.Question{
		Question: question,
		Choices: []askuser.Choice{
			{Label: tr(ctx, "internal_approvalwire_yesno_label_yes", nil), Value: "yes"},
			{Label: tr(ctx, "internal_approvalwire_yesno_label_no", nil), Value: "no"},
		},
		Default: defaultVal,
	}
	result, err := p.Inner.Prompt(ctx, q)
	if err != nil {
		return false, err
	}
	if result == nil {
		// Defensive: the Prompter contract requires non-nil Result on nil
		// error, but a future implementation could regress. Treat nil as
		// "deny" rather than panicking.
		return false, nil
	}
	return result.Value == "yes", nil
}
