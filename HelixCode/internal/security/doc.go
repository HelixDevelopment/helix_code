// Package security provides comprehensive security management and zero-tolerance policy enforcement.
//
// The security package implements centralized security monitoring, feature scanning,
// and policy enforcement for the HelixCode platform. It provides a global security
// manager that tracks security metrics, performs feature scans, and enforces
// zero-tolerance policies for critical security issues.
//
// # Key Components
//
// SecurityManager provides comprehensive security management:
//
//	// Initialize global security manager
//	err := security.InitGlobalSecurityManager()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Get the global instance
//	sm := security.GetGlobalSecurityManager()
//
//	// Or create a standalone instance
//	sm := security.NewSecurityManager()
//
// # Feature Scanning
//
// The package can scan features for security issues:
//
//	result, err := sm.ScanFeature("user-authentication")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Feature: %s\n", result.FeatureName)
//	fmt.Printf("Security Score: %d\n", result.SecurityScore)
//	fmt.Printf("Can Proceed: %v\n", result.CanProceed)
//	fmt.Printf("Issues: %v\n", result.Issues)
//	fmt.Printf("Recommendations: %v\n", result.Recommendations)
//
// # Security Metrics
//
// The manager tracks key security metrics:
//
//	// Get current security score
//	score := sm.GetSecurityScore()
//	fmt.Printf("Security Score: %d\n", score)
//
//	// Get issue counts
//	critical := sm.GetCriticalIssues()
//	high := sm.GetHighIssues()
//	fmt.Printf("Critical: %d, High: %d\n", critical, high)
//
//	// Update metrics
//	sm.UpdateSecurityMetrics(
//	    0,    // Critical issues
//	    2,    // High issues
//	    95,   // Security score
//	)
//
// # Zero-Tolerance Policy
//
// The package enforces zero-tolerance for critical security issues:
//
//	if !sm.ValidateZeroTolerance() {
//	    log.Fatal("Critical security issues detected - deployment blocked")
//	}
//
//	// Zero-tolerance is satisfied when critical issues = 0
//
// # Feature Scan Results
//
// FeatureScanResult provides detailed scan information:
//
//	type FeatureScanResult struct {
//	    FeatureName     string        // Name of scanned feature
//	    Success         bool          // Whether scan completed
//	    CanProceed      bool          // Whether feature can proceed
//	    SecurityScore   int           // 0-100 security score
//	    Issues          []interface{} // Detected issues
//	    Recommendations []string      // Security recommendations
//	    ScanTime        time.Duration // Scan duration
//	    Timestamp       time.Time     // When scan occurred
//	}
//
// # Global vs Local Instances
//
// The package supports both global and local security managers:
//
//	// Global instance (singleton, initialized once)
//	security.InitGlobalSecurityManager()
//	global := security.GetGlobalSecurityManager()
//
//	// Local instance (independent, for isolated testing)
//	local := security.NewSecurityManager()
//
// # Thread Safety
//
// All SecurityManager operations are thread-safe through internal mutex
// protection, allowing concurrent access from multiple goroutines.
//
// # Integration Example
//
// Integrating security checks into a deployment pipeline:
//
//	func DeployWithSecurityCheck(featureName string) error {
//	    sm := security.GetGlobalSecurityManager()
//
//	    // Scan the feature
//	    result, err := sm.ScanFeature(featureName)
//	    if err != nil {
//	        return fmt.Errorf("security scan failed: %w", err)
//	    }
//
//	    // Check scan results
//	    if !result.CanProceed {
//	        return fmt.Errorf("security check failed: score %d", result.SecurityScore)
//	    }
//
//	    // Validate zero-tolerance policy
//	    if !sm.ValidateZeroTolerance() {
//	        return fmt.Errorf("critical security issues present")
//	    }
//
//	    // Proceed with deployment
//	    return nil
//	}
//
// # Security Score Interpretation
//
// Security scores range from 0-100:
//   - 90-100: Excellent security posture
//   - 70-89: Good with minor issues
//   - 50-69: Moderate risk, attention needed
//   - Below 50: High risk, immediate action required
package security
