package main

import (
	"context"
	"log"
	"os"

	"dev.helix.code/cmd/security_fix/i18n"
	"dev.helix.code/internal/fix"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this CLI. Defaults to i18n.NoopTranslator{} (loud
// message-ID echo) so unit tests + ad-hoc invocations remain obvious.
// helix_code wires a real *i18nadapter.Translator at boot via
// SetTranslator (round-143 §11.4 anti-bluff sweep, 2026-05-18).
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — main()'s linear call graph does
// not warrant a constructor-injected struct.
var translator i18n.Translator = i18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr i18n.Translator) {
	if tr == nil {
		translator = i18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this file. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
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

func main() {
	ctx := context.Background()

	log.Println(tr(ctx, "security_fix_banner_start", nil))
	log.Println(tr(ctx, "security_fix_banner_policy", nil))

	// Get current project path
	projectPath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	log.Println(tr(ctx, "security_fix_path_echo", map[string]any{"Path": projectPath}))
	log.Println(tr(ctx, "security_fix_critical_only_echo", nil))
	log.Println("")

	// Execute zero-tolerance security issue resolution
	log.Println(tr(ctx, "security_fix_executing", nil))

	fixResult, err := fix.FixAllCriticalSecurityIssues(projectPath, true)
	if err != nil {
		log.Fatalf("Security issue resolution failed: %v", err)
	}

	// Display results. Residual log.Printf format strings (table
	// rows + validation block) deferred to a future round to stay
	// under the round-143 size cap; headline rows already migrated.
	log.Printf("")
	log.Printf("========================================")
	log.Printf("ZERO-TOLERANCE SECURITY FIX COMPLETE")
	log.Printf("========================================")
	log.Println(tr(ctx, "security_fix_summary_total", map[string]any{"Count": fixResult.TotalIssues}))
	log.Println(tr(ctx, "security_fix_summary_fixed", map[string]any{"Count": fixResult.FixedIssues}))
	log.Printf("Failed Fixes: %d", fixResult.FailedFixes)
	log.Printf("Manual Fixes Required: %d", fixResult.ManualFixes)
	log.Printf("Issues Skipped: %d", fixResult.SkippedIssues)
	log.Printf("Fix Duration: %v", fixResult.EndTime.Sub(fixResult.StartTime))
	log.Printf("Overall Success: %t", fixResult.Success)

	if fixResult.Validation != nil {
		log.Printf("")
		log.Printf("VALIDATION RESULTS:")
		log.Printf("Post-Fix Security Score: %d", fixResult.Validation.ScanResult.SecurityScore)
		log.Printf("Remaining Critical Issues: %d", fixResult.Validation.RemainingCriticalIssues)
		log.Printf("Remaining High Issues: %d", fixResult.Validation.RemainingHighIssues)
		log.Printf("All Fixes Validated: %t", fixResult.Validation.FixesValidated)
	}

	if fixResult.Success {
		log.Printf("")
		log.Println(tr(ctx, "security_fix_result_success", nil))
	} else {
		log.Printf("")
		log.Println(tr(ctx, "security_fix_result_failure", nil))
	}

	log.Printf("========================================")
	log.Println(tr(ctx, "security_fix_report_pointer", nil))
	log.Printf("========================================")
}
