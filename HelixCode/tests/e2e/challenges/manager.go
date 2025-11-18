package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ChallengeManager manages challenge definitions and executions
type ChallengeManager struct {
	config     *ChallengeConfig
	executor   *ChallengeExecutor
	challenges map[string]*ChallengeSpec
	executions map[string]*ChallengeExecution
	batches    map[string]*ChallengeBatch
	mu         sync.RWMutex
}

// NewChallengeManager creates a new challenge manager
func NewChallengeManager(config *ChallengeConfig) *ChallengeManager {
	return &ChallengeManager{
		config:     config,
		executor:   NewChallengeExecutor(config),
		challenges: make(map[string]*ChallengeSpec),
		executions: make(map[string]*ChallengeExecution),
		batches:    make(map[string]*ChallengeBatch),
	}
}

// LoadChallenge loads a challenge from a JSON file
func (m *ChallengeManager) LoadChallenge(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read challenge file: %w", err)
	}

	var spec ChallengeSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("failed to parse challenge JSON: %w", err)
	}

	m.mu.Lock()
	m.challenges[spec.ID] = &spec
	m.mu.Unlock()

	return nil
}

// LoadChallengesFromDir loads all challenges from a directory
func (m *ChallengeManager) LoadChallengesFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		if err := m.LoadChallenge(path); err != nil {
			return fmt.Errorf("failed to load challenge %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// RegisterChallenge registers a challenge specification
func (m *ChallengeManager) RegisterChallenge(spec *ChallengeSpec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.challenges[spec.ID] = spec
}

// GetChallenge retrieves a challenge by ID
func (m *ChallengeManager) GetChallenge(id string) (*ChallengeSpec, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	spec, exists := m.challenges[id]
	if !exists {
		return nil, fmt.Errorf("challenge not found: %s", id)
	}

	return spec, nil
}

// ListChallenges returns all registered challenges
func (m *ChallengeManager) ListChallenges() []*ChallengeSpec {
	m.mu.RLock()
	defer m.mu.RUnlock()

	challenges := make([]*ChallengeSpec, 0, len(m.challenges))
	for _, spec := range m.challenges {
		challenges = append(challenges, spec)
	}

	return challenges
}

// ExecuteChallenge executes a single challenge
func (m *ChallengeManager) ExecuteChallenge(ctx context.Context, challengeID string, iface ChallengeInterface, dist ChallengeDistribution, provider LLMProviderType, model string) (*ChallengeExecution, error) {
	spec, err := m.GetChallenge(challengeID)
	if err != nil {
		return nil, err
	}

	execution, err := m.executor.Execute(ctx, spec, iface, dist, provider, model)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.executions[execution.ID] = execution
	m.mu.Unlock()

	return execution, nil
}

// CreateBatch creates a batch of challenge executions
func (m *ChallengeManager) CreateBatch(name, description string, challengeIDs []string, interfaces []ChallengeInterface, distributions []ChallengeDistribution, providers []LLMProviderType, models []string) (*ChallengeBatch, error) {
	batch := &ChallengeBatch{
		ID:            uuid.New().String(),
		Name:          name,
		Description:   description,
		Challenges:    challengeIDs,
		Interfaces:    interfaces,
		Distributions: distributions,
		Providers:     providers,
		Models:        models,
		StartTime:     time.Now(),
		Status:        StatusQueued,
		Executions:    []string{},
	}

	m.mu.Lock()
	m.batches[batch.ID] = batch
	m.mu.Unlock()

	return batch, nil
}

// ExecuteBatch executes a batch of challenges
func (m *ChallengeManager) ExecuteBatch(ctx context.Context, batchID string) error {
	m.mu.RLock()
	batch, exists := m.batches[batchID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("batch not found: %s", batchID)
	}

	batch.Status = StatusRunning
	batch.StartTime = time.Now()

	// Generate all combinations
	executions := []string{}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, m.config.MaxConcurrent)
	errChan := make(chan error, 100)

	for _, challengeID := range batch.Challenges {
		for _, iface := range batch.Interfaces {
			for _, dist := range batch.Distributions {
				for _, provider := range batch.Providers {
					for _, model := range batch.Models {
						wg.Add(1)
						go func(cid string, i ChallengeInterface, d ChallengeDistribution, p LLMProviderType, mod string) {
							defer wg.Done()

							// Acquire semaphore
							semaphore <- struct{}{}
							defer func() { <-semaphore }()

							execution, err := m.ExecuteChallenge(ctx, cid, i, d, p, mod)
							if err != nil {
								errChan <- err
								return
							}

							m.mu.Lock()
							executions = append(executions, execution.ID)
							m.mu.Unlock()
						}(challengeID, iface, dist, provider, model)
					}
				}
			}
		}
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	batch.Executions = executions
	batch.EndTime = time.Now()
	batch.Duration = batch.EndTime.Sub(batch.StartTime)

	// Calculate summary
	batch.Summary = m.calculateBatchSummary(executions)

	if len(errors) > 0 {
		batch.Status = StatusFailed
		return fmt.Errorf("batch had %d errors: %v", len(errors), errors[0])
	}

	batch.Status = StatusCompleted
	return nil
}

// calculateBatchSummary calculates summary statistics for a batch
func (m *ChallengeManager) calculateBatchSummary(executionIDs []string) BatchSummary {
	summary := BatchSummary{
		TotalExecutions: len(executionIDs),
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalDuration time.Duration
	var totalTokens, totalFiles, totalLOC int

	for _, id := range executionIDs {
		exec, exists := m.executions[id]
		if !exists {
			continue
		}

		switch exec.Status {
		case StatusCompleted:
			summary.Completed++
		case StatusFailed:
			summary.Failed++
		case StatusTimeout:
			summary.Timeout++
		case StatusValidationFailed:
			summary.ValidationFailed++
		}

		totalDuration += exec.Duration
		totalTokens += exec.Metrics.TokensUsed
		totalFiles += exec.Metrics.FilesGenerated
		totalLOC += exec.Metrics.LinesOfCode
	}

	if summary.TotalExecutions > 0 {
		summary.SuccessRate = float64(summary.Completed) / float64(summary.TotalExecutions) * 100
		summary.AvgDuration = totalDuration / time.Duration(summary.TotalExecutions)
	}

	summary.TotalTokens = totalTokens
	summary.TotalFilesGenerated = totalFiles
	summary.TotalLOC = totalLOC

	return summary
}

// GetExecution retrieves an execution by ID
func (m *ChallengeManager) GetExecution(id string) (*ChallengeExecution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exec, exists := m.executions[id]
	if !exists {
		return nil, fmt.Errorf("execution not found: %s", id)
	}

	return exec, nil
}

// GetBatch retrieves a batch by ID
func (m *ChallengeManager) GetBatch(id string) (*ChallengeBatch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	batch, exists := m.batches[id]
	if !exists {
		return nil, fmt.Errorf("batch not found: %s", id)
	}

	return batch, nil
}

// ExportBatchReport exports a batch report as JSON
func (m *ChallengeManager) ExportBatchReport(batchID, filename string) error {
	batch, err := m.GetBatch(batchID)
	if err != nil {
		return err
	}

	// Gather all executions
	report := struct {
		Batch      *ChallengeBatch         `json:"batch"`
		Executions []*ChallengeExecution `json:"executions"`
	}{
		Batch:      batch,
		Executions: make([]*ChallengeExecution, 0, len(batch.Executions)),
	}

	m.mu.RLock()
	for _, execID := range batch.Executions {
		if exec, exists := m.executions[execID]; exists {
			report.Executions = append(report.Executions, exec)
		}
	}
	m.mu.RUnlock()

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// SaveState saves the manager state to disk
func (m *ChallengeManager) SaveState(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// Save challenges
	challengesData, err := json.MarshalIndent(m.challenges, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "challenges.json"), challengesData, 0644); err != nil {
		return err
	}

	// Save executions
	executionsData, err := json.MarshalIndent(m.executions, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "executions.json"), executionsData, 0644); err != nil {
		return err
	}

	// Save batches
	batchesData, err := json.MarshalIndent(m.batches, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "batches.json"), batchesData, 0644); err != nil {
		return err
	}

	return nil
}

// LoadState loads the manager state from disk
func (m *ChallengeManager) LoadState(dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load challenges
	challengesData, err := os.ReadFile(filepath.Join(dir, "challenges.json"))
	if err == nil {
		json.Unmarshal(challengesData, &m.challenges)
	}

	// Load executions
	executionsData, err := os.ReadFile(filepath.Join(dir, "executions.json"))
	if err == nil {
		json.Unmarshal(executionsData, &m.executions)
	}

	// Load batches
	batchesData, err := os.ReadFile(filepath.Join(dir, "batches.json"))
	if err == nil {
		json.Unmarshal(batchesData, &m.batches)
	}

	return nil
}
