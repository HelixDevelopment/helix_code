package cognee

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/hardware"
	"dev.helix.code/internal/logging"
)

// ServiceStatus represents the current status of the Cognee service
type ServiceStatus string

const (
	ServiceStatusStopped  ServiceStatus = "stopped"
	ServiceStatusStarting ServiceStatus = "starting"
	ServiceStatusRunning  ServiceStatus = "running"
	ServiceStatusError    ServiceStatus = "error"
)

// CogneeService is the main service for Cognee integration
type CogneeService struct {
	config    *config.CogneeConfig
	hwProfile *hardware.HardwareProfile
	client    *Client
	optimizer *PerformanceOptimizer
	logger    *logging.Logger
	cache     *ServiceCache
	stats     *ServiceStatistics

	// State management
	mu        sync.RWMutex
	status    ServiceStatus
	startTime time.Time
	lastError error

	// Background processing
	stopChan          chan struct{}
	stopOnce          sync.Once
	bgTasks           sync.WaitGroup
	healthCheckTicker *time.Ticker

	// Event handling
	eventChan     chan *CogneeEvent
	eventHandlers []func(*CogneeEvent)
}

// ServiceCache provides caching for Cognee operations
type ServiceCache struct {
	mu          sync.RWMutex
	memories    map[string]*CogneeMemory
	searches    map[string]*SearchMemoryResponse
	datasets    map[string]*Dataset
	maxItems    int
	ttl         time.Duration
	lastCleanup time.Time
}

// ServiceStatistics tracks service metrics
type ServiceStatistics struct {
	mu              sync.RWMutex
	MemoriesAdded   int64
	MemoriesDeleted int64
	SearchesCount   int64
	CognifiesCount  int64
	ErrorsCount     int64
	CacheHits       int64
	CacheMisses     int64
	StartTime       time.Time
	LastActivity    time.Time
}

// NewCogneeService creates a new Cognee service instance
func NewCogneeService(cfg *config.CogneeConfig, hwProfile *hardware.HardwareProfile) (*CogneeService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}

	logger := logging.NewLoggerWithName("cognee_service")

	client := NewClient(cfg)

	var optimizer *PerformanceOptimizer
	var err error
	if cfg.Optimization != nil && hwProfile != nil {
		optimizer, err = NewPerformanceOptimizer(cfg, hwProfile)
		if err != nil {
			logger.Warn("Failed to create performance optimizer: %v", err)
		}
	}

	cacheMaxItems := 1000
	cacheTTL := time.Hour
	if cfg.Cache != nil {
		if cfg.Cache.MaxSize > 0 {
			cacheMaxItems = int(cfg.Cache.MaxSize / 1024)
		}
		if cfg.Cache.TTL > 0 {
			cacheTTL = cfg.Cache.TTL
		}
	}

	service := &CogneeService{
		config:    cfg,
		hwProfile: hwProfile,
		client:    client,
		optimizer: optimizer,
		logger:    logger,
		status:    ServiceStatusStopped,
		stopChan:  make(chan struct{}),
		eventChan: make(chan *CogneeEvent, 100),
		cache: &ServiceCache{
			memories:    make(map[string]*CogneeMemory),
			searches:    make(map[string]*SearchMemoryResponse),
			datasets:    make(map[string]*Dataset),
			maxItems:    cacheMaxItems,
			ttl:         cacheTTL,
			lastCleanup: time.Now(),
		},
		stats: &ServiceStatistics{
			StartTime: time.Now(),
		},
	}

	return service, nil
}

// Start starts the Cognee service
func (s *CogneeService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.status == ServiceStatusRunning {
		s.mu.Unlock()
		return nil
	}
	s.status = ServiceStatusStarting
	s.mu.Unlock()

	s.logger.Info("Starting Cognee service...")

	if s.config.AutoStart {
		if err := s.client.AutoContainerize(ctx); err != nil {
			s.logger.Warn("Failed to auto-start Cognee container: %v", err)
		}
	}

	if !s.client.TestConnection(ctx) && s.config.Enabled {
		s.logger.Warn("Cognee service not reachable at %s", s.client.GetBaseURL())
	}

	if s.optimizer != nil {
		if err := s.optimizer.Initialize(ctx); err != nil {
			s.logger.Warn("Failed to initialize performance optimizer: %v", err)
		}
		if err := s.optimizer.Start(ctx); err != nil {
			s.logger.Warn("Failed to start performance optimizer: %v", err)
		}
	}

	s.bgTasks.Add(1)
	go s.healthCheckLoop(ctx)

	s.bgTasks.Add(1)
	go s.eventProcessingLoop(ctx)

	s.bgTasks.Add(1)
	go s.cacheMaintenanceLoop(ctx)

	s.mu.Lock()
	s.status = ServiceStatusRunning
	s.startTime = time.Now()
	s.mu.Unlock()

	s.logger.Info("Cognee service started successfully")

	return nil
}

// Stop stops the Cognee service. It is safe under concurrent invocation: the
// transition out of ServiceStatusRunning is performed UNDER the status lock, so
// a concurrent second Stop observes the non-running status and returns early
// instead of both callers passing the guard and racing to close(s.stopChan).
// stopOnce additionally guarantees the channel is closed at most once as
// defense-in-depth (e.g. a Stop racing a Start that re-set Running).
func (s *CogneeService) Stop(ctx context.Context) error {
	s.mu.Lock()
	if s.status != ServiceStatusRunning {
		s.mu.Unlock()
		return nil
	}
	// Flip out of Running while still holding the lock so any concurrent Stop
	// fails the guard above and returns early — only the winner reaches the
	// channel close below.
	s.status = ServiceStatusStopped
	s.mu.Unlock()

	s.logger.Info("Stopping Cognee service...")

	s.stopOnce.Do(func() {
		close(s.stopChan)
	})

	done := make(chan struct{})
	go func() {
		s.bgTasks.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("All background tasks stopped")
	case <-ctx.Done():
		s.logger.Warn("Context cancelled while waiting for background tasks")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Timeout waiting for background tasks to stop")
	}

	if s.optimizer != nil {
		if err := s.optimizer.Stop(ctx); err != nil {
			s.logger.Warn("Failed to stop performance optimizer: %v", err)
		}
	}

	if err := s.client.Close(); err != nil {
		s.logger.Warn("Failed to close client: %v", err)
	}

	s.mu.Lock()
	s.status = ServiceStatusStopped
	s.mu.Unlock()

	s.logger.Info("Cognee service stopped")

	return nil
}

// EnsureRunning ensures the service is running
func (s *CogneeService) EnsureRunning(ctx context.Context) error {
	s.mu.RLock()
	status := s.status
	s.mu.RUnlock()

	if status == ServiceStatusRunning {
		return nil
	}

	return s.Start(ctx)
}

// AddMemory adds a memory entry to Cognee
func (s *CogneeService) AddMemory(ctx context.Context, req *AddMemoryRequest) (*AddMemoryResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if req.Content == "" {
		return nil, fmt.Errorf("content is required")
	}

	if req.DatasetName == "" {
		req.DatasetName = "default"
	}

	resp, err := s.client.AddMemory(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("failed to add memory: %w", err)
	}

	memory := &CogneeMemory{
		ID:          resp.ID,
		VectorID:    resp.VectorID,
		Content:     req.Content,
		ContentType: req.ContentType,
		DatasetName: req.DatasetName,
		Metadata:    req.Metadata,
		GraphNodes:  resp.GraphNodes,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		UserID:      req.UserID,
		ProjectID:   req.ProjectID,
	}

	s.cacheMemory(memory)
	s.stats.incrementMemoriesAdded()
	s.recordActivity()

	s.emitEvent(&CogneeEvent{
		ID:        uuid.New().String(),
		Type:      "memory",
		Action:    "added",
		Data:      map[string]interface{}{"memory_id": resp.ID, "dataset": req.DatasetName},
		UserID:    req.UserID,
		Timestamp: time.Now(),
	})

	return resp, nil
}

// SearchMemory searches for memories in Cognee
func (s *CogneeService) SearchMemory(ctx context.Context, req *SearchMemoryRequest) (*SearchMemoryResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	cacheKey := s.buildSearchCacheKey(req)
	if cached := s.getCachedSearch(cacheKey); cached != nil {
		s.stats.incrementCacheHits()
		return cached, nil
	}
	s.stats.incrementCacheMisses()

	resp, err := s.client.SearchMemory(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("search failed: %w", err)
	}

	s.cacheSearch(cacheKey, resp)
	s.stats.incrementSearches()
	s.recordActivity()

	return resp, nil
}

// Cognify processes data into knowledge graphs
func (s *CogneeService) Cognify(ctx context.Context, req *CognifyRequest) (*CognifyResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	resp, err := s.client.Cognify(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("cognify failed: %w", err)
	}

	s.stats.incrementCognifies()
	s.recordActivity()

	s.emitEvent(&CogneeEvent{
		ID:        uuid.New().String(),
		Type:      "cognify",
		Action:    "started",
		Data:      map[string]interface{}{"datasets": req.Datasets},
		UserID:    req.UserID,
		Timestamp: time.Now(),
	})

	return resp, nil
}

// GetInsights retrieves insights from the knowledge graph
func (s *CogneeService) GetInsights(ctx context.Context, req *InsightsRequest) (*InsightsResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	resp, err := s.client.SearchInsights(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("get insights failed: %w", err)
	}

	s.recordActivity()

	return resp, nil
}

// GetGraphCompletion performs LLM-powered graph completion search
func (s *CogneeService) GetGraphCompletion(ctx context.Context, query string, datasets []string, limit int) (*SearchMemoryResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if limit <= 0 {
		limit = 10
	}

	resp, err := s.client.SearchGraphCompletion(ctx, query, datasets, limit)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("graph completion failed: %w", err)
	}

	s.recordActivity()

	return resp, nil
}

// ProcessCode processes code through Cognee's code understanding pipeline
func (s *CogneeService) ProcessCode(ctx context.Context, req *CodePipelineRequest) (*CodePipelineResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	if req.DatasetName == "" {
		req.DatasetName = "code"
	}

	resp, err := s.client.ProcessCodePipeline(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("code processing failed: %w", err)
	}

	s.recordActivity()

	s.emitEvent(&CogneeEvent{
		ID:        uuid.New().String(),
		Type:      "code",
		Action:    "processed",
		Data:      map[string]interface{}{"language": req.Language, "file_path": req.FilePath},
		UserID:    req.UserID,
		Timestamp: time.Now(),
	})

	return resp, nil
}

// CreateDataset creates a new dataset
func (s *CogneeService) CreateDataset(ctx context.Context, req *CreateDatasetRequest) (*DatasetResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if req.Name == "" {
		return nil, fmt.Errorf("dataset name is required")
	}

	resp, err := s.client.CreateDataset(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("create dataset failed: %w", err)
	}

	if resp.Dataset != nil {
		s.cacheDataset(resp.Dataset)
	}
	s.recordActivity()

	return resp, nil
}

// ListDatasets retrieves all datasets
func (s *CogneeService) ListDatasets(ctx context.Context) (*DatasetsResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	resp, err := s.client.ListDatasets(ctx)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("list datasets failed: %w", err)
	}

	for i := range resp.Datasets {
		s.cacheDataset(&resp.Datasets[i])
	}
	s.recordActivity()

	return resp, nil
}

// GetDataset retrieves a specific dataset
func (s *CogneeService) GetDataset(ctx context.Context, name string) (*Dataset, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if cached := s.getCachedDataset(name); cached != nil {
		s.stats.incrementCacheHits()
		return cached, nil
	}
	s.stats.incrementCacheMisses()

	dataset, err := s.client.GetDataset(ctx, name)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("get dataset failed: %w", err)
	}

	if dataset != nil {
		s.cacheDataset(dataset)
	}
	s.recordActivity()

	return dataset, nil
}

// DeleteDataset deletes a dataset
func (s *CogneeService) DeleteDataset(ctx context.Context, name string) error {
	if err := s.EnsureRunning(ctx); err != nil {
		return fmt.Errorf("service not running: %w", err)
	}

	if name == "" {
		return fmt.Errorf("dataset name is required")
	}

	if err := s.client.DeleteDataset(ctx, name); err != nil {
		s.recordError(err)
		return fmt.Errorf("delete dataset failed: %w", err)
	}

	s.removeCachedDataset(name)
	s.recordActivity()

	return nil
}

// VisualizeGraph retrieves graph visualization data
func (s *CogneeService) VisualizeGraph(ctx context.Context, req *GraphVisualizationRequest) (*GraphVisualizationResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	resp, err := s.client.VisualizeGraph(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("visualize graph failed: %w", err)
	}

	s.recordActivity()

	return resp, nil
}

// DeleteData removes data from a dataset
func (s *CogneeService) DeleteData(ctx context.Context, req *DeleteDataRequest) (*DeleteDataResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if req.DatasetName == "" {
		return nil, fmt.Errorf("dataset name is required")
	}

	if len(req.DataIDs) == 0 {
		return nil, fmt.Errorf("data IDs are required")
	}

	resp, err := s.client.DeleteData(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("delete data failed: %w", err)
	}

	for _, id := range req.DataIDs {
		s.removeCachedMemory(id)
	}
	s.stats.incrementMemoriesDeleted(int64(resp.Deleted))
	s.recordActivity()

	return resp, nil
}

// AddBatchMemory adds multiple memories in batch
func (s *CogneeService) AddBatchMemory(ctx context.Context, req *BatchMemoryRequest) (*BatchMemoryResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	if len(req.Memories) == 0 {
		return nil, fmt.Errorf("memories are required")
	}

	resp, err := s.client.AddBatchMemory(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("batch add memory failed: %w", err)
	}

	s.stats.mu.Lock()
	s.stats.MemoriesAdded += int64(resp.Processed)
	s.stats.mu.Unlock()
	s.recordActivity()

	return resp, nil
}

// SubmitFeedback submits feedback on search results
func (s *CogneeService) SubmitFeedback(ctx context.Context, req *FeedbackRequest) (*FeedbackResponse, error) {
	if err := s.EnsureRunning(ctx); err != nil {
		return nil, fmt.Errorf("service not running: %w", err)
	}

	resp, err := s.client.SubmitFeedback(ctx, req)
	if err != nil {
		s.recordError(err)
		return nil, fmt.Errorf("submit feedback failed: %w", err)
	}

	s.recordActivity()

	return resp, nil
}

// GetHealth returns the health status of the Cognee service
func (s *CogneeService) GetHealth(ctx context.Context) (*HealthStatus, error) {
	health, err := s.client.GetHealth(ctx)
	if err != nil {
		health = &HealthStatus{
			Status:    "unhealthy",
			Timestamp: time.Now(),
			LastCheck: time.Now(),
		}
	}

	s.mu.RLock()
	if s.status == ServiceStatusRunning {
		health.Uptime = time.Since(s.startTime)
	}
	s.mu.RUnlock()

	return health, nil
}

// GetStatistics returns service statistics
func (s *CogneeService) GetStatistics(ctx context.Context) (*CogneeStatistics, error) {
	remoteStats, err := s.client.GetStatistics(ctx)
	if err != nil {
		s.logger.Debug("Failed to get remote statistics: %v", err)
	}

	s.stats.mu.RLock()
	localStats := &CogneeStatistics{
		TotalMemories:  s.stats.MemoriesAdded - s.stats.MemoriesDeleted,
		TotalSearches:  s.stats.SearchesCount,
		TotalCognifies: s.stats.CognifiesCount,
		LastUpdated:    time.Now(),
	}

	if remoteStats != nil {
		localStats.TotalDatasets = remoteStats.TotalDatasets
		localStats.GraphNodeCount = remoteStats.GraphNodeCount
		localStats.GraphEdgeCount = remoteStats.GraphEdgeCount
		localStats.AverageScore = remoteStats.AverageScore
	}

	totalCacheOps := s.stats.CacheHits + s.stats.CacheMisses
	if totalCacheOps > 0 {
		localStats.CacheHitRate = float64(s.stats.CacheHits) / float64(totalCacheOps)
	}
	s.stats.mu.RUnlock()

	s.mu.RLock()
	if s.status == ServiceStatusRunning {
		localStats.ServiceUptime = time.Since(s.startTime)
	}
	s.mu.RUnlock()

	return localStats, nil
}

// GetStatus returns the current service status
func (s *CogneeService) GetStatus() ServiceStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// GetLastError returns the last error encountered
func (s *CogneeService) GetLastError() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// RegisterEventHandler registers a handler for Cognee events
func (s *CogneeService) RegisterEventHandler(handler func(*CogneeEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventHandlers = append(s.eventHandlers, handler)
}

// Background processing loops

func (s *CogneeService) healthCheckLoop(ctx context.Context) {
	defer s.bgTasks.Done()

	interval := 30 * time.Second
	if s.config.Monitoring != nil && s.config.Monitoring.HealthCheck > 0 {
		interval = s.config.Monitoring.HealthCheck
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.performHealthCheck(ctx)
		}
	}
}

func (s *CogneeService) performHealthCheck(ctx context.Context) {
	health, err := s.client.GetHealth(ctx)
	if err != nil {
		s.logger.Warn("Health check failed: %v", err)
		s.mu.Lock()
		s.lastError = err
		s.mu.Unlock()
		return
	}

	if health.Status != "healthy" {
		s.logger.Warn("Cognee service unhealthy: %s", health.Status)
	}
}

func (s *CogneeService) eventProcessingLoop(ctx context.Context) {
	defer s.bgTasks.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case event := <-s.eventChan:
			s.processEvent(event)
		}
	}
}

func (s *CogneeService) processEvent(event *CogneeEvent) {
	s.mu.RLock()
	handlers := s.eventHandlers
	s.mu.RUnlock()

	for _, handler := range handlers {
		go func(h func(*CogneeEvent)) {
			defer func() {
				if r := recover(); r != nil {
					s.logger.Error("Event handler panic: %v", r)
				}
			}()
			h(event)
		}(handler)
	}
}

func (s *CogneeService) cacheMaintenanceLoop(ctx context.Context) {
	defer s.bgTasks.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.cleanupCache()
		}
	}
}

func (s *CogneeService) cleanupCache() {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	now := time.Now()
	s.cache.lastCleanup = now

	for key := range s.cache.searches {
		delete(s.cache.searches, key)
	}

	for key, dataset := range s.cache.datasets {
		if now.Sub(dataset.UpdatedAt) > s.cache.ttl {
			delete(s.cache.datasets, key)
		}
	}

	if len(s.cache.memories) > s.cache.maxItems {
		excess := len(s.cache.memories) - s.cache.maxItems
		count := 0
		for key := range s.cache.memories {
			if count >= excess {
				break
			}
			delete(s.cache.memories, key)
			count++
		}
	}
}

// Cache helper methods

func (s *CogneeService) cacheMemory(memory *CogneeMemory) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.memories[memory.ID] = memory
}

func (s *CogneeService) getCachedMemory(id string) *CogneeMemory {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	return s.cache.memories[id]
}

func (s *CogneeService) removeCachedMemory(id string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	delete(s.cache.memories, id)
}

func (s *CogneeService) buildSearchCacheKey(req *SearchMemoryRequest) string {
	return fmt.Sprintf("%s:%s:%d:%s", req.Query, req.DatasetName, req.Limit, req.SearchType)
}

func (s *CogneeService) cacheSearch(key string, resp *SearchMemoryResponse) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.searches[key] = resp
}

func (s *CogneeService) getCachedSearch(key string) *SearchMemoryResponse {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	return s.cache.searches[key]
}

func (s *CogneeService) cacheDataset(dataset *Dataset) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	s.cache.datasets[dataset.Name] = dataset
}

func (s *CogneeService) getCachedDataset(name string) *Dataset {
	s.cache.mu.RLock()
	defer s.cache.mu.RUnlock()
	return s.cache.datasets[name]
}

func (s *CogneeService) removeCachedDataset(name string) {
	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()
	delete(s.cache.datasets, name)
}

// Statistics helper methods

func (ss *ServiceStatistics) incrementMemoriesAdded() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.MemoriesAdded++
}

func (ss *ServiceStatistics) incrementMemoriesDeleted(count int64) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.MemoriesDeleted += count
}

func (ss *ServiceStatistics) incrementSearches() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.SearchesCount++
}

func (ss *ServiceStatistics) incrementCognifies() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.CognifiesCount++
}

func (ss *ServiceStatistics) incrementCacheHits() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.CacheHits++
}

func (ss *ServiceStatistics) incrementCacheMisses() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.CacheMisses++
}

func (ss *ServiceStatistics) incrementErrors() {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.ErrorsCount++
}

// Service helper methods

func (s *CogneeService) recordError(err error) {
	s.mu.Lock()
	s.lastError = err
	s.mu.Unlock()
	s.stats.incrementErrors()
}

func (s *CogneeService) recordActivity() {
	s.stats.mu.Lock()
	s.stats.LastActivity = time.Now()
	s.stats.mu.Unlock()
}

func (s *CogneeService) emitEvent(event *CogneeEvent) {
	select {
	case s.eventChan <- event:
	default:
		s.logger.Debug("Event channel full, dropping event: %s", event.Type)
	}
}

// GetClient returns the underlying client
func (s *CogneeService) GetClient() *Client {
	return s.client
}

// GetOptimizer returns the performance optimizer
func (s *CogneeService) GetOptimizer() *PerformanceOptimizer {
	return s.optimizer
}
