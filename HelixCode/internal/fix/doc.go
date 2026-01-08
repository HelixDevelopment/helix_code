// Copyright 2024 HelixCode. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

/*
Package fix provides comprehensive security issue resolution and automated
code fixing capabilities for the HelixCode platform.

# Overview

The fix package handles automated detection and resolution of security vulnerabilities
in project code. It integrates with the security package to scan for issues and
applies fixes automatically where possible, while tracking which issues require
manual intervention.

# Key Types

FixResult represents the outcome of a fix operation:

	type FixResult struct {
	    TotalIssues   int                  // Total issues found
	    FixedIssues   int                  // Successfully fixed
	    FailedFixes   int                  // Failed to fix automatically
	    ManualFixes   int                  // Requires manual intervention
	    SkippedIssues int                  // Skipped (e.g., non-critical when criticalOnly=true)
	    StartTime     time.Time            // Operation start time
	    EndTime       time.Time            // Operation end time
	    Success       bool                 // Overall success status
	    Validation    *FixValidationResult // Post-fix validation
	}

FixValidationResult contains validation results after applying fixes:

	type FixValidationResult struct {
	    ScanResult              *security.FeatureScanResult
	    RemainingCriticalIssues int
	    RemainingHighIssues     int
	    FixesValidated          bool
	}

# Fixing Security Issues

The primary function for fixing security issues:

	result, err := fix.FixAllCriticalSecurityIssues(projectPath, criticalOnly)
	if err != nil {
	    log.Fatalf("Fix operation failed: %v", err)
	}

	log.Printf("Total Issues: %d", result.TotalIssues)
	log.Printf("Fixed: %d", result.FixedIssues)
	log.Printf("Failed: %d", result.FailedFixes)
	log.Printf("Manual Required: %d", result.ManualFixes)

The criticalOnly parameter controls whether to fix all issues or only critical ones.

# Fix Process

The fix process follows these steps:

 1. Initialize security manager
 2. Scan project for security issues
 3. Process each issue (attempt automatic fix)
 4. Validate fixes with post-fix scan
 5. Return comprehensive results

# Issue Categories

The package handles various types of security issues:

  - Critical vulnerabilities (always fixed when detected)
  - High-severity issues (fixed unless criticalOnly=true)
  - Medium and low severity issues (skipped when criticalOnly=true)

# Validation

After applying fixes, the package performs validation to ensure:

  - Fixes were applied correctly
  - No new issues were introduced
  - Remaining critical issues are counted
  - Security score is recalculated

# Success Criteria

A fix operation is considered successful when:

  - At least one issue was fixed, AND
  - No critical issues remain after validation

	result.Success = result.FixedIssues > 0 &&
	    (result.Validation == nil || result.Validation.RemainingCriticalIssues == 0)

# Integration with Security Package

The fix package integrates with the security package:

	// Initialize security manager (required)
	security.InitGlobalSecurityManager()

	// Scan for issues
	scanResult, err := security.GetGlobalSecurityManager().ScanFeature("comprehensive_security_scan")

	// Post-fix validation
	validationScan, err := security.GetGlobalSecurityManager().ScanFeature("post_fix_validation")

# File Operations

The package includes utilities for working with project files:

	// Find all Go files in a project
	goFiles, err := findGoFiles(projectPath)

This is used internally to locate files that need to be scanned and fixed.

# Logging

The package uses the logging package to report progress:

	logger := logging.DefaultLogger()
	logger.Info("Starting comprehensive security issue resolution")
	logger.Info("Fixed Issues: %d", result.FixedIssues)
	logger.Warn("Fix validation failed: %v", err)

# Configuration

Fix settings are typically configured via config.yaml:

	fix:
	  auto_fix: false
	  min_confidence: 0.8
	  backup: true

# Usage Example

Complete usage example:

	package main

	import (
	    "log"
	    "dev.helix.code/internal/fix"
	)

	func main() {
	    projectPath := "/path/to/project"

	    // Fix only critical issues
	    result, err := fix.FixAllCriticalSecurityIssues(projectPath, true)
	    if err != nil {
	        log.Fatal(err)
	    }

	    if result.Success {
	        log.Println("All critical issues resolved")
	    } else {
	        log.Printf("Manual fixes required: %d", result.ManualFixes)
	        if result.Validation != nil {
	            log.Printf("Remaining critical: %d", result.Validation.RemainingCriticalIssues)
	        }
	    }
	}

# Error Handling

The package returns errors for:

  - Security manager initialization failures
  - Security scan failures
  - Validation scan failures

Individual fix failures are tracked in FixResult.FailedFixes rather than
causing the overall operation to fail.
*/
package fix
