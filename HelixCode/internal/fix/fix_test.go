package fix

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/security"
)

// TestFixResult tests the FixResult data structure
func TestFixResult(t *testing.T) {
	t.Run("InitialFixResult_ZeroValues", func(t *testing.T) {
		result := &FixResult{}

		assert.Equal(t, 0, result.TotalIssues)
		assert.Equal(t, 0, result.FixedIssues)
		assert.Equal(t, 0, result.FailedFixes)
		assert.Equal(t, 0, result.ManualFixes)
		assert.Equal(t, 0, result.SkippedIssues)
		assert.False(t, result.Success)
		assert.Nil(t, result.Validation)
	})

	t.Run("FixResult_WithFullData", func(t *testing.T) {
		startTime := time.Now()
		endTime := startTime.Add(5 * time.Minute)

		result := &FixResult{
			TotalIssues:   10,
			FixedIssues:   7,
			FailedFixes:   2,
			ManualFixes:   1,
			SkippedIssues: 0,
			StartTime:     startTime,
			EndTime:       endTime,
			Success:       true,
			Validation: &FixValidationResult{
				RemainingCriticalIssues: 0,
				RemainingHighIssues:     1,
				FixesValidated:          true,
			},
		}

		assert.Equal(t, 10, result.TotalIssues)
		assert.Equal(t, 7, result.FixedIssues)
		assert.Equal(t, 2, result.FailedFixes)
		assert.Equal(t, 1, result.ManualFixes)
		assert.True(t, result.Success)
		assert.NotNil(t, result.Validation)
		assert.Equal(t, 0, result.Validation.RemainingCriticalIssues)
	})

	t.Run("FixResult_CalculateDuration", func(t *testing.T) {
		startTime := time.Now()
		endTime := startTime.Add(10 * time.Second)

		result := &FixResult{
			StartTime: startTime,
			EndTime:   endTime,
		}

		duration := result.EndTime.Sub(result.StartTime)
		assert.Equal(t, 10*time.Second, duration)
	})
}

// TestFixValidationResult tests the FixValidationResult data structure
func TestFixValidationResult(t *testing.T) {
	t.Run("InitialValidationResult_ZeroValues", func(t *testing.T) {
		result := &FixValidationResult{}

		assert.Equal(t, 0, result.RemainingCriticalIssues)
		assert.Equal(t, 0, result.RemainingHighIssues)
		assert.False(t, result.FixesValidated)
		assert.Nil(t, result.ScanResult)
	})

	t.Run("ValidationResult_WithScanResult", func(t *testing.T) {
		scanResult := &security.FeatureScanResult{
			FeatureName:   "test-feature",
			Success:       true,
			SecurityScore: 95,
		}

		result := &FixValidationResult{
			ScanResult:              scanResult,
			RemainingCriticalIssues: 0,
			RemainingHighIssues:     2,
			FixesValidated:          true,
		}

		assert.NotNil(t, result.ScanResult)
		assert.Equal(t, "test-feature", result.ScanResult.FeatureName)
		assert.Equal(t, 95, result.ScanResult.SecurityScore)
		assert.True(t, result.FixesValidated)
	})
}

// TestFindGoFiles tests the findGoFiles helper function
func TestFindGoFiles(t *testing.T) {
	t.Run("FindGoFiles_InValidDirectory", func(t *testing.T) {
		// Create temporary directory structure
		tmpDir := t.TempDir()

		// Create some .go files
		file1 := filepath.Join(tmpDir, "main.go")
		file2 := filepath.Join(tmpDir, "utils.go")
		subDir := filepath.Join(tmpDir, "pkg")
		require.NoError(t, os.Mkdir(subDir, 0755))
		file3 := filepath.Join(subDir, "helper.go")

		require.NoError(t, os.WriteFile(file1, []byte("package main"), 0644))
		require.NoError(t, os.WriteFile(file2, []byte("package main"), 0644))
		require.NoError(t, os.WriteFile(file3, []byte("package pkg"), 0644))

		// Create non-.go file
		txtFile := filepath.Join(tmpDir, "readme.txt")
		require.NoError(t, os.WriteFile(txtFile, []byte("readme"), 0644))

		files, err := findGoFiles(tmpDir)

		assert.NoError(t, err)
		assert.Len(t, files, 3)
		assert.Contains(t, files, file1)
		assert.Contains(t, files, file2)
		assert.Contains(t, files, file3)
		assert.NotContains(t, files, txtFile)
	})

	t.Run("FindGoFiles_EmptyDirectory", func(t *testing.T) {
		tmpDir := t.TempDir()

		files, err := findGoFiles(tmpDir)

		assert.NoError(t, err)
		assert.Empty(t, files)
	})

	t.Run("FindGoFiles_NonExistentDirectory", func(t *testing.T) {
		files, err := findGoFiles("/nonexistent/path/that/does/not/exist")

		assert.Error(t, err)
		assert.Nil(t, files)
	})

	t.Run("FindGoFiles_OnlyGoFiles", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create only .go files
		file1 := filepath.Join(tmpDir, "file1.go")
		file2 := filepath.Join(tmpDir, "file2.go")

		require.NoError(t, os.WriteFile(file1, []byte("package test"), 0644))
		require.NoError(t, os.WriteFile(file2, []byte("package test"), 0644))

		files, err := findGoFiles(tmpDir)

		assert.NoError(t, err)
		assert.Len(t, files, 2)
	})

	t.Run("FindGoFiles_NestedDirectories", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create nested structure
		subDir1 := filepath.Join(tmpDir, "level1")
		subDir2 := filepath.Join(subDir1, "level2")
		require.NoError(t, os.MkdirAll(subDir2, 0755))

		file1 := filepath.Join(tmpDir, "root.go")
		file2 := filepath.Join(subDir1, "level1.go")
		file3 := filepath.Join(subDir2, "level2.go")

		require.NoError(t, os.WriteFile(file1, []byte("package root"), 0644))
		require.NoError(t, os.WriteFile(file2, []byte("package level1"), 0644))
		require.NoError(t, os.WriteFile(file3, []byte("package level2"), 0644))

		files, err := findGoFiles(tmpDir)

		assert.NoError(t, err)
		assert.Len(t, files, 3)
	})
}

// TestAttemptFix tests the attemptFix function.
// Anti-bluff (CONST-035 / Article XI §11.9): each fixer must be exercised
// against real Go-source files on disk so the test fails when the underlying
// pattern-detection regresses, not merely when error-returning code paths
// are wired together. "No patterns found" in an empty tmpdir is a free PASS
// and is therefore explicitly NOT exercised here.
func TestAttemptFix(t *testing.T) {
	t.Run("AttemptFix_HardcodedSecret_CleanDirectory_ReturnsTrue", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Place a clearly-clean source file so the walker has real work to do.
		cleanGo := filepath.Join(tmpDir, "clean.go")
		require.NoError(t, os.WriteFile(cleanGo, []byte("package p\nfunc f() {}\n"), 0644))

		result := attemptFix(tmpDir, "hardcoded secret detected in config")

		assert.True(t, result, "clean source must yield true (no patterns found)")
	})

	t.Run("AttemptFix_HardcodedSecret_DirtyDirectory_ReturnsFalse", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirty := filepath.Join(tmpDir, "secret.go")
		require.NoError(t, os.WriteFile(dirty, []byte(
			"package p\nvar password = \"hunter2\"\n"), 0644))

		result := attemptFix(tmpDir, "hardcoded secret detected in config")

		assert.False(t, result,
			"file containing hardcoded password literal must be detected and flagged for manual review")
	})

	t.Run("AttemptFix_SQLInjection_CleanDirectory_ReturnsTrue", func(t *testing.T) {
		tmpDir := t.TempDir()
		clean := filepath.Join(tmpDir, "clean.go")
		require.NoError(t, os.WriteFile(clean, []byte(
			"package p\nimport \"database/sql\"\nfunc f(db *sql.DB){db.QueryRow(\"SELECT 1\")}\n"), 0644))

		result := attemptFix(tmpDir, "sql injection vulnerability found")

		assert.True(t, result, "no fmt.Sprintf or string-concat SELECT => safe")
	})

	t.Run("AttemptFix_SQLInjection_DirtyDirectory_ReturnsFalse", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirty := filepath.Join(tmpDir, "bad.go")
		require.NoError(t, os.WriteFile(dirty, []byte(
			"package p\nimport \"fmt\"\nfunc f(id string) string { return fmt.Sprintf(\"SELECT * FROM x WHERE id=%s\", id) }\n"), 0644))

		result := attemptFix(tmpDir, "sql injection vulnerability found")

		assert.False(t, result,
			"fmt.Sprintf around SELECT must be detected as potential SQL injection")
	})

	t.Run("AttemptFix_WeakCrypto_DirtyDirectory_ReturnsFalse", func(t *testing.T) {
		tmpDir := t.TempDir()
		dirty := filepath.Join(tmpDir, "weak.go")
		require.NoError(t, os.WriteFile(dirty, []byte(
			"package p\nimport \"crypto/md5\"\nfunc h(password string) []byte { x := md5.Sum([]byte(password)); return x[:] }\n"), 0644))

		result := attemptFix(tmpDir, "weak crypto detected for password")

		assert.False(t, result,
			"md5 of password literal must be detected as weak-crypto-for-secret")
	})

	t.Run("AttemptFix_UnknownIssue_ReturnsFalse", func(t *testing.T) {
		tmpDir := t.TempDir()

		result := attemptFix(tmpDir, "test issue")

		assert.False(t, result, "unrecognised issue strings must not silently report 'fixed'")
	})

	t.Run("AttemptFix_WithNilIssue_ReturnsFalse", func(t *testing.T) {
		tmpDir := t.TempDir()

		result := attemptFix(tmpDir, nil)

		assert.False(t, result, "nil must not silently report 'fixed'")
	})

	t.Run("AttemptFix_ComplexIssue_UnknownType", func(t *testing.T) {
		tmpDir := t.TempDir()
		issue := map[string]interface{}{
			"type":     "security",
			"severity": "critical",
			"message":  "unknown vulnerability type",
		}

		result := attemptFix(tmpDir, issue)

		assert.False(t, result, "structured-but-unrecognised issue must not report 'fixed'")
	})

	t.Run("AttemptFix_XSS_AlwaysRequiresManualReview", func(t *testing.T) {
		tmpDir := t.TempDir()

		result := attemptFix(tmpDir, "xss vulnerability in template")

		assert.False(t, result,
			"XSS auto-fix is documented as manual-only; must return false")
	})

	t.Run("AttemptFix_CSRF_AlwaysRequiresManualReview", func(t *testing.T) {
		tmpDir := t.TempDir()

		result := attemptFix(tmpDir, "csrf token missing")

		assert.False(t, result,
			"CSRF auto-fix is documented as manual-only; must return false")
	})

	t.Run("AttemptFix_MissingAuth_AlwaysRequiresManualReview", func(t *testing.T) {
		tmpDir := t.TempDir()

		result := attemptFix(tmpDir, "missing auth on endpoint")

		assert.False(t, result,
			"missing-auth auto-fix is documented as manual-only; must return false")
	})

	t.Run("AttemptFix_InsecureDependency_AlwaysRequiresManualReview", func(t *testing.T) {
		tmpDir := t.TempDir()

		result := attemptFix(tmpDir, "insecure dependency in go.mod")

		assert.False(t, result,
			"insecure-dependency auto-fix is documented as manual-only; must return false")
	})
}

// TestProcessSecurityIssues tests the processSecurityIssues function.
// Anti-bluff (CONST-035 / Article XI §11.9): exercise both the clean-source
// path (fixer returns true) and the dirty-source path (fixer detects pattern
// and returns false) so the test fails when pattern detection regresses.
func TestProcessSecurityIssues(t *testing.T) {
	t.Run("ProcessSecurityIssues_AllCritical_KnownTypes_CleanSource", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Provide a clean Go file so the walker runs but finds no patterns.
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "clean.go"),
			[]byte("package p\nfunc f(){}\n"), 0644))
		issues := []interface{}{
			"critical: hardcoded secret found",
			"critical: sql injection vulnerability",
			"critical: path traversal issue",
		}

		fixed, failed, manual, skipped := processSecurityIssues(tmpDir, issues, true)

		assert.Equal(t, 3, fixed,
			"all three known critical types must run pattern check and report 'fixed' for clean source")
		assert.Equal(t, 0, failed)
		assert.Equal(t, 0, manual)
		assert.Equal(t, 0, skipped)
		// Invariant: sum of buckets must equal input length.
		assert.Equal(t, len(issues), fixed+failed+manual+skipped,
			"every issue must be accounted for in exactly one bucket")
	})

	t.Run("ProcessSecurityIssues_DirtySource_DetectsAndReportsFailed", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Plant a real SQL-injection pattern so fixSQLInjection returns false.
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.go"), []byte(
			"package p\nimport \"fmt\"\nfunc q(id string) string { return fmt.Sprintf(\"SELECT * FROM x WHERE id=%s\", id) }\n"),
			0644))
		issues := []interface{}{"critical: sql injection vulnerability"}

		fixed, failed, manual, skipped := processSecurityIssues(tmpDir, issues, true)

		assert.Equal(t, 0, fixed,
			"fmt.Sprintf SQL pattern must NOT be reported as fixed")
		assert.Equal(t, 1, failed,
			"fmt.Sprintf SQL pattern must be reported as failed (manual review required)")
		assert.Equal(t, 0, manual)
		assert.Equal(t, 0, skipped)
		assert.Equal(t, len(issues), fixed+failed+manual+skipped)
	})

	t.Run("ProcessSecurityIssues_MixedWithCriticalOnly", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "clean.go"),
			[]byte("package p\nfunc f(){}\n"), 0644))
		issues := []interface{}{
			"critical: sql injection vulnerability",
			"high: xss vulnerability",
			"medium: insecure config",
		}

		fixed, failed, manual, skipped := processSecurityIssues(tmpDir, issues, true)

		assert.Equal(t, 1, fixed,
			"only the 'critical' SQL issue must be attempted; clean source => 'fixed'")
		assert.Equal(t, 0, failed)
		assert.Equal(t, 0, manual)
		assert.Equal(t, 2, skipped,
			"high+medium must be skipped when criticalOnly=true")
		assert.Equal(t, len(issues), fixed+failed+manual+skipped)
	})

	t.Run("ProcessSecurityIssues_AllIssues_KnownTypes", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "clean.go"),
			[]byte("package p\nfunc f(){}\n"), 0644))
		issues := []interface{}{
			"critical: hardcoded secret",
			"high: weak crypto detected",
			"medium: missing auth check",
		}

		fixed, failed, manual, skipped := processSecurityIssues(tmpDir, issues, false)

		// Clean source: hardcoded-secret + weak-crypto report 'fixed' (no patterns found).
		// missing-auth ALWAYS returns false → bucketed as failed.
		assert.Equal(t, 2, fixed, "hardcoded-secret and weak-crypto on clean source must be 'fixed'")
		assert.Equal(t, 1, failed, "missing-auth is documented manual-only and must be 'failed'")
		assert.Equal(t, 0, manual)
		assert.Equal(t, 0, skipped)
		assert.Equal(t, len(issues), fixed+failed+manual+skipped)
	})

	t.Run("ProcessSecurityIssues_EmptyIssues", func(t *testing.T) {
		tmpDir := t.TempDir()
		issues := []interface{}{}

		fixed, failed, manual, skipped := processSecurityIssues(tmpDir, issues, false)

		assert.Equal(t, 0, fixed)
		assert.Equal(t, 0, failed)
		assert.Equal(t, 0, manual)
		assert.Equal(t, 0, skipped)
	})

	t.Run("ProcessSecurityIssues_NoCriticalWithCriticalOnly", func(t *testing.T) {
		tmpDir := t.TempDir()
		issues := []interface{}{
			"high: XSS vulnerability",
			"medium: insecure config",
			"low: missing header",
		}

		fixed, failed, manual, skipped := processSecurityIssues(tmpDir, issues, true)

		assert.Equal(t, 0, fixed)
		assert.Equal(t, 0, failed)
		assert.Equal(t, 0, manual)
		assert.Equal(t, 3, skipped)
	})
}

// TestValidateFixes tests the validateFixes function
func TestValidateFixes(t *testing.T) {
	// Ensure security manager is initialized
	require.NoError(t, security.InitGlobalSecurityManager())

	t.Run("ValidateFixes_Success", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := validateFixes(tmpDir)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.ScanResult)
		assert.True(t, result.FixesValidated)
		assert.Equal(t, 0, result.RemainingCriticalIssues)
		assert.Equal(t, 0, result.RemainingHighIssues)
	})

	t.Run("ValidateFixes_ChecksScanResult", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := validateFixes(tmpDir)

		assert.NoError(t, err)
		assert.Equal(t, "post_fix_validation", result.ScanResult.FeatureName)
		assert.GreaterOrEqual(t, result.ScanResult.SecurityScore, 0)
	})
}

// TestFixAllCriticalSecurityIssues tests the main function
func TestFixAllCriticalSecurityIssues(t *testing.T) {
	// Ensure security manager is initialized
	require.NoError(t, security.InitGlobalSecurityManager())

	t.Run("FixAllCriticalSecurityIssues_Success", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := FixAllCriticalSecurityIssues(tmpDir, true)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, result.TotalIssues, 0)
		assert.False(t, result.StartTime.IsZero())
		assert.False(t, result.EndTime.IsZero())
		assert.True(t, result.EndTime.After(result.StartTime))
	})

	t.Run("FixAllCriticalSecurityIssues_CriticalOnly", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := FixAllCriticalSecurityIssues(tmpDir, true)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// With current stub implementation, should have no issues
		assert.Equal(t, 0, result.TotalIssues)
	})

	t.Run("FixAllCriticalSecurityIssues_AllIssues", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := FixAllCriticalSecurityIssues(tmpDir, false)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.GreaterOrEqual(t, result.TotalIssues, 0)
	})

	t.Run("FixAllCriticalSecurityIssues_ValidatesCounts", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := FixAllCriticalSecurityIssues(tmpDir, false)

		assert.NoError(t, err)
		// Verify all counters are non-negative
		assert.GreaterOrEqual(t, result.TotalIssues, 0)
		assert.GreaterOrEqual(t, result.FixedIssues, 0)
		assert.GreaterOrEqual(t, result.FailedFixes, 0)
		assert.GreaterOrEqual(t, result.ManualFixes, 0)
		assert.GreaterOrEqual(t, result.SkippedIssues, 0)
	})

	t.Run("FixAllCriticalSecurityIssues_HasValidation", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := FixAllCriticalSecurityIssues(tmpDir, true)

		assert.NoError(t, err)
		assert.NotNil(t, result.Validation)
		assert.NotNil(t, result.Validation.ScanResult)
		assert.True(t, result.Validation.FixesValidated)
	})

	t.Run("FixAllCriticalSecurityIssues_SuccessConditions", func(t *testing.T) {
		tmpDir := t.TempDir()

		result, err := FixAllCriticalSecurityIssues(tmpDir, true)

		// CONST-035: assert the documented success contract instead of using
		// assert.True(t, true). The contract (see fix.go) is:
		//   Success == (FixedIssues > 0) && (Validation == nil || Validation.RemainingCriticalIssues == 0)
		require.NoError(t, err)
		require.NotNil(t, result)

		var expectedSuccess bool
		if result.FixedIssues > 0 {
			if result.Validation == nil || result.Validation.RemainingCriticalIssues == 0 {
				expectedSuccess = true
			}
		}
		assert.Equal(t, expectedSuccess, result.Success,
			"Success flag must follow the documented contract: "+
				"FixedIssues>0 AND (Validation nil OR RemainingCriticalIssues==0)")

		// And invariant: when TotalIssues==0, Success MUST be false
		// (cannot have "fixed" anything we never saw).
		if result.TotalIssues == 0 {
			assert.False(t, result.Success,
				"Success must be false when TotalIssues==0 (nothing was actually fixed)")
		}
	})

	t.Run("FixAllCriticalSecurityIssues_TimingVerification", func(t *testing.T) {
		tmpDir := t.TempDir()
		before := time.Now()

		result, err := FixAllCriticalSecurityIssues(tmpDir, false)

		after := time.Now()

		assert.NoError(t, err)
		assert.True(t, result.StartTime.After(before) || result.StartTime.Equal(before))
		assert.True(t, result.EndTime.Before(after) || result.EndTime.Equal(after))
		assert.True(t, result.EndTime.After(result.StartTime) || result.EndTime.Equal(result.StartTime))
	})
}

// TestFixAllCriticalSecurityIssues_Integration tests end-to-end scenarios
func TestFixAllCriticalSecurityIssues_Integration(t *testing.T) {
	// Ensure security manager is initialized
	require.NoError(t, security.InitGlobalSecurityManager())

	t.Run("Integration_WithRealProjectStructure", func(t *testing.T) {
		// Create a temporary project structure
		tmpDir := t.TempDir()

		// Create directories
		srcDir := filepath.Join(tmpDir, "src")
		testDir := filepath.Join(tmpDir, "test")
		require.NoError(t, os.MkdirAll(srcDir, 0755))
		require.NoError(t, os.MkdirAll(testDir, 0755))

		// Create some .go files
		mainFile := filepath.Join(srcDir, "main.go")
		testFile := filepath.Join(testDir, "test.go")
		require.NoError(t, os.WriteFile(mainFile, []byte("package main\n\nfunc main() {}"), 0644))
		require.NoError(t, os.WriteFile(testFile, []byte("package test"), 0644))

		result, err := FixAllCriticalSecurityIssues(tmpDir, true)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.NotNil(t, result.Validation)
		assert.False(t, result.StartTime.IsZero())
		assert.False(t, result.EndTime.IsZero())
	})

	t.Run("Integration_CriticalOnlyVsAll", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Run with criticalOnly=true
		result1, err1 := FixAllCriticalSecurityIssues(tmpDir, true)
		assert.NoError(t, err1)

		// Run with criticalOnly=false
		result2, err2 := FixAllCriticalSecurityIssues(tmpDir, false)
		assert.NoError(t, err2)

		// Both should succeed
		assert.NotNil(t, result1)
		assert.NotNil(t, result2)

		// With stub implementation, results should be similar
		assert.Equal(t, result1.TotalIssues, result2.TotalIssues)
	})
}

// TestConcurrentFixOperations tests concurrent fix operations
func TestConcurrentFixOperations(t *testing.T) {
	// Ensure security manager is initialized
	require.NoError(t, security.InitGlobalSecurityManager())

	t.Run("ConcurrentFixOperations_Multiple", func(t *testing.T) {
		tmpDir := t.TempDir()

		done := make(chan bool, 5)
		for i := 0; i < 5; i++ {
			go func() {
				result, err := FixAllCriticalSecurityIssues(tmpDir, true)
				assert.NoError(t, err)
				assert.NotNil(t, result)
				done <- true
			}()
		}

		for i := 0; i < 5; i++ {
			<-done
		}
	})
}

// TestEdgeCases tests edge cases and error conditions
func TestEdgeCases(t *testing.T) {
	// Ensure security manager is initialized
	require.NoError(t, security.InitGlobalSecurityManager())

	t.Run("EdgeCase_EmptyPath", func(t *testing.T) {
		result, err := FixAllCriticalSecurityIssues("", true)

		// Should handle empty path gracefully
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("EdgeCase_DotPath", func(t *testing.T) {
		result, err := FixAllCriticalSecurityIssues(".", true)

		// Should handle current directory
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
}
