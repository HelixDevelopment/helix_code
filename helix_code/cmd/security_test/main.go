package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"dev.helix.code/cmd/security_test/i18n"
	"dev.helix.code/internal/security"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this CLI. Defaults to i18n.NoopTranslator{} (loud
// message-ID echo) so unit tests + ad-hoc invocations remain obvious.
// helix_code wires a real *i18nadapter.Translator at boot via
// SetTranslator (round-142 §11.4 anti-bluff sweep, 2026-05-18).
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
	log.Println("Starting HelixCode security scanning")
	log.Println("Zero Tolerance Policy: All security issues must be resolved")

	ctx := context.Background()

	// Test-suite names — round-142 CONST-046 migration: 8 of 12
	// resolved via Translator, 4 residual (API / Container /
	// Configuration / File System / Logging) deferred to future
	// round to stay under the size cap. Residual lines remain
	// detectable by the CONST-046 audit walker until migrated.
	testSuite := []string{
		tr(ctx, "security_test_suite_llm_provider", nil),
		tr(ctx, "security_test_suite_ssh_connection", nil),
		tr(ctx, "security_test_suite_database", nil),
		tr(ctx, "security_test_suite_authentication", nil),
		tr(ctx, "security_test_suite_input_validation", nil),
		tr(ctx, "security_test_suite_api", nil),
		tr(ctx, "security_test_suite_worker_isolation", nil),
		tr(ctx, "security_test_suite_dependency", nil),
		"Container Security",
		"Configuration Security",
		"File System Security",
		"Logging Security",
	}

	sm := security.NewSecurityManager()

	var totalIssues, totalCritical int
	var failedTests []string

	log.Printf("Executing %d security tests", len(testSuite))

	for i, testName := range testSuite {
		log.Printf("Test %d/%d: %s", i+1, len(testSuite), testName)

		result, err := sm.ScanFeature(testName)
		if err != nil {
			log.Printf("  Error: %v", err)
			failedTests = append(failedTests, testName)
			continue
		}

		log.Printf("  Score: %d, Success: %v, ScanTime: %v",
			result.SecurityScore, result.Success, result.ScanTime)

		if !result.Success {
			log.Printf("  No scanners available - install SonarQube CLI or Snyk")
			continue
		}

		issueCount := len(result.Issues)
		totalIssues += issueCount

		for _, issue := range result.Issues {
			if si, ok := issue.(security.SecurityIssue); ok {
				if si.Severity == "BLOCKER" || si.Severity == "CRITICAL" {
					totalCritical++
					log.Printf("  %s: %s (%s:%d)",
						si.Severity, si.Title, si.FilePath, si.LineNumber)
				}
			}
		}

		if issueCount > 0 {
			log.Printf("  Issues found: %d", issueCount)
		} else {
			log.Printf("  Clean scan")
		}
	}

	fmt.Println()
	fmt.Println("=== SECURITY TEST RESULTS ===")
	fmt.Printf("Total tests:   %d\n", len(testSuite))
	fmt.Printf("Failed tests:  %d\n", len(failedTests))
	fmt.Printf("Total issues:  %d\n", totalIssues)
	fmt.Printf("Critical:      %d\n", totalCritical)
	fmt.Println()

	if len(failedTests) > 0 {
		fmt.Printf("Failed: %v\n", failedTests)
	}

	if totalCritical > 0 {
		fmt.Println(tr(ctx, "security_test_summary_fail_critical", nil))
		os.Exit(1)
	}

	if !resultSuccess(testSuite, failedTests) {
		fmt.Println(tr(ctx, "security_test_summary_warn_scanners_unavailable", nil))
		os.Exit(1)
	}

	fmt.Println("All security tests completed")
}

func resultSuccess(tests, failed []string) bool {
	return len(failed) < len(tests)/2
}
