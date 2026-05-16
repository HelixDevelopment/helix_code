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
		logger.Info("Global security manager initialized")
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
	sm.logger.Info("Starting security scan for feature: %s", featureName)

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
			sm.logger.Info("Scanner %s error: %v", scanner.Name(), err)
			continue
		}
		anySucceeded = true
		allIssues = append(allIssues, scanResult.Issues...)
	}

	if !anySucceeded {
		sm.logger.Info("No security scanners available for: %s", featureName)
		sm.logger.Info("Set SONARQUBE_URL/SONARQUBE_TOKEN or SNYK_TOKEN to enable security scanning")
		result := &FeatureScanResult{
			FeatureName:     featureName,
			Success:         false,
			CanProceed:      true,
			SecurityScore:   0,
			Issues:          []interface{}{},
			Recommendations: []string{"No security scanners configured"},
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
	recs = append(recs, "Review and address all identified security issues")

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

	sm.logger.Info("Security scan completed for feature: %s, score: %d", featureName, score)
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

	sm.logger.Info("Security metrics updated - Critical: %d, High: %d, Score: %d",
		critical, high, score)
}
