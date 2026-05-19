package security

import (
	"context"
	"os"
	"sync"
	"time"

	"dev.helix.code/internal/logging"
)

type SecurityManager struct {
	logger         *logging.Logger
	scanResults    map[string]*FeatureScanResult
	securityScore  int
	criticalIssues int
	highIssues     int
	scanners       []Scanner
	mutex          sync.RWMutex
}

// FeatureScanResult represents the result of a security feature scan
type FeatureScanResult struct {
	FeatureName     string        `json:"feature_name"`
	Success         bool          `json:"success"`
	CanProceed      bool          `json:"can_proceed"`
	SecurityScore   int           `json:"security_score"`
	Issues          []interface{} `json:"issues"`
	Recommendations []string      `json:"recommendations"`
	ScanTime        time.Duration `json:"scan_time"`
	Timestamp       time.Time     `json:"timestamp"`
}

var (
	globalSecurityManager *SecurityManager
	securityOnce          sync.Once
)

// InitGlobalSecurityManager initializes the global security manager
func InitGlobalSecurityManager() error {
	var err error
	securityOnce.Do(func() {
		logger := logging.DefaultLogger()
		globalSecurityManager = &SecurityManager{
			logger:      logger,
			scanResults: make(map[string]*FeatureScanResult),
			mutex:       sync.RWMutex{},
		}
		logger.Info("%s", tr(context.Background(), "internal_security_global_manager_initialized", nil))
	})
	return err
}

// GetGlobalSecurityManager returns the global security manager instance
func GetGlobalSecurityManager() *SecurityManager {
	return globalSecurityManager
}

func NewSecurityManager() *SecurityManager {
	cfg := ScannerConfig{
		SonarQubeURL:   os.Getenv("SONARQUBE_URL"),
		SonarQubeToken: os.Getenv("SONARQUBE_TOKEN"),
		SnykToken:      os.Getenv("SNYK_TOKEN"),
		Timeout:        30 * time.Second,
	}
	return &SecurityManager{
		logger:      logging.DefaultLogger(),
		scanResults: make(map[string]*FeatureScanResult),
		scanners: []Scanner{
			NewSonarQubeScanner(cfg),
			NewSnykScanner(cfg),
		},
		mutex: sync.RWMutex{},
	}
}

func NewSecurityManagerWithScanners(scanners ...Scanner) *SecurityManager {
	return &SecurityManager{
		logger:      logging.DefaultLogger(),
		scanResults: make(map[string]*FeatureScanResult),
		scanners:    scanners,
		mutex:       sync.RWMutex{},
	}
}

func (sm *SecurityManager) ScanFeature(featureName string) (*FeatureScanResult, error) {
	ctx := context.Background()
	return sm.ScanFeatureContext(ctx, featureName)
}

func (sm *SecurityManager) ScanFeatureContext(ctx context.Context, featureName string) (*FeatureScanResult, error) {
	sm.logger.Info("%s", tr(ctx, "internal_security_scan_starting",
		map[string]any{"Feature": featureName}))

	if featureName == "" {
		return &FeatureScanResult{
			FeatureName: featureName,
			Success:     false,
			CanProceed:  false,
			ScanTime:    0,
			Timestamp:   time.Now(),
		}, nil
	}

	select {
	case <-ctx.Done():
		return &FeatureScanResult{
			FeatureName: featureName,
			Success:     false,
			CanProceed:  false,
			ScanTime:    0,
			Timestamp:   time.Now(),
		}, nil
	default:
	}

	startTime := time.Now()
	var allIssues []SecurityIssue
	anySucceeded := false
	var recs []string

	for _, scanner := range sm.scanners {
		if !scanner.IsAvailable(ctx) {
			continue
		}
		scanResult, err := scanner.Scan(ctx, featureName)
		if err != nil {
			sm.logger.Info("%s", tr(ctx, "internal_security_scanner_error",
				map[string]any{"Scanner": scanner.Name(), "Error": err.Error()}))
			continue
		}
		anySucceeded = true
		allIssues = append(allIssues, scanResult.Issues...)
	}

	if !anySucceeded {
		sm.logger.Info("%s", tr(ctx, "internal_security_no_scanners_available",
			map[string]any{"Feature": featureName}))
		sm.logger.Info("%s", tr(ctx, "internal_security_no_scanners_hint", nil))
		result := &FeatureScanResult{
			FeatureName:     featureName,
			Success:         false,
			CanProceed:      true,
			SecurityScore:   0,
			Issues:          []interface{}{},
			Recommendations: []string{tr(ctx, "internal_security_recommendation_no_scanners", nil)},
			ScanTime:        time.Since(startTime),
			Timestamp:       time.Now(),
		}
		sm.mutex.Lock()
		sm.scanResults[featureName] = result
		sm.mutex.Unlock()
		return result, nil
	}

	ifaceIssues := make([]interface{}, len(allIssues))
	for i, issue := range allIssues {
		ifaceIssues[i] = issue
	}

	score := calculateScore(allIssues)
	recs = append(recs, tr(ctx, "internal_security_recommendation_review_issues", nil))

	result := &FeatureScanResult{
		FeatureName:     featureName,
		Success:         allIssues == nil || len(allIssues) == 0,
		CanProceed:      true,
		SecurityScore:   score,
		Issues:          ifaceIssues,
		Recommendations: recs,
		ScanTime:        time.Since(startTime),
		Timestamp:       time.Now(),
	}

	sm.mutex.Lock()
	sm.scanResults[featureName] = result
	sm.mutex.Unlock()

	sm.logger.Info("%s", tr(ctx, "internal_security_scan_completed",
		map[string]any{"Feature": featureName, "Score": score}))
	return result, nil
}

// GetSecurityScore returns the current security score
func (sm *SecurityManager) GetSecurityScore() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.securityScore
}

// GetCriticalIssues returns the number of critical security issues
func (sm *SecurityManager) GetCriticalIssues() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.criticalIssues
}

// GetHighIssues returns the number of high security issues
func (sm *SecurityManager) GetHighIssues() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.highIssues
}

// ValidateZeroTolerance checks if zero-tolerance security policy is satisfied
func (sm *SecurityManager) ValidateZeroTolerance() bool {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.criticalIssues == 0
}

// UpdateSecurityMetrics updates the security metrics
func (sm *SecurityManager) UpdateSecurityMetrics(critical, high int, score int) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	sm.criticalIssues = critical
	sm.highIssues = high
	sm.securityScore = score

	sm.logger.Info("%s", tr(context.Background(), "internal_security_metrics_updated",
		map[string]any{"Critical": critical, "High": high, "Score": score}))
}
