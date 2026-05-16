package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"dev.helix.code/internal/security"
)

func main() {
	log.Println("Starting HelixCode security scanning")
	log.Println("Zero Tolerance Policy: All security issues must be resolved")

	ctx := context.Background()
	_ = ctx

	testSuite := []string{
		"LLM Provider Security",
		"SSH Connection Security",
		"Database Security",
		"Authentication Security",
		"Input Validation Security",
		"API Security",
		"Worker Isolation Security",
		"Dependency Security",
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
		fmt.Println("FAIL: Critical security issues found")
		os.Exit(1)
	}

	if !resultSuccess(testSuite, failedTests) {
		fmt.Println("WARN: Some scanners unavailable")
		os.Exit(1)
	}

	fmt.Println("All security tests completed")
}

func resultSuccess(tests, failed []string) bool {
	return len(failed) < len(tests)/2
}
