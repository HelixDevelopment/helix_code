package cognee

import (
	"context"
	"fmt"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/hardware"
	"dev.helix.code/internal/logging"
)

// CacheManager manages caching for Cognee operations
type CacheManager struct {
	service *CogneeService
	logger  *logging.Logger
}

// NewCacheManager creates a new cache manager
func NewCacheManager(config interface{}) (*CacheManager, error) {
	return &CacheManager{
		logger: logging.NewLoggerWithName("cognee_cache"),
	}, nil
}

// SetService sets the underlying Cognee service
func (cm *CacheManager) SetService(service *CogneeService) {
	cm.service = service
}

// Clear clears all cached data
func (cm *CacheManager) Clear() {
	if cm.service != nil && cm.service.cache != nil {
		cm.service.cache.mu.Lock()
		cm.service.cache.memories = make(map[string]*CogneeMemory)
		cm.service.cache.searches = make(map[string]*SearchMemoryResponse)
		cm.service.cache.datasets = make(map[string]*Dataset)
		cm.service.cache.mu.Unlock()
	}
}

// CogneeManager is the main manager for Cognee integration
// This wraps the CogneeService and provides backward compatibility
type CogneeManager struct {
	config    *config.HelixConfig
	hwProfile *hardware.HardwareProfile
	logger    *logging.Logger
	service   *CogneeService
}

// NewCogneeManager creates a new Cognee manager
func NewCogneeManager(cfg *config.HelixConfig, hwProfile *hardware.HardwareProfile) (*CogneeManager, error) {
	logger := logging.NewLoggerWithName("cognee_manager")

	var cogneeConfig *config.CogneeConfig
	if cfg != nil && cfg.Cognee != nil {
		cogneeConfig = cfg.Cognee
	} else {
		cogneeConfig = config.DefaultCogneeConfig()
	}

	service, err := NewCogneeService(cogneeConfig, hwProfile)
	if err != nil {
		logger.Warn("Failed to create Cognee service: %v", err)
		service = nil
	}

	return &CogneeManager{
		config:    cfg,
		hwProfile: hwProfile,
		logger:    logger,
		service:   service,
	}, nil
}

// Start starts the Cognee manager
func (cm *CogneeManager) Start(ctx context.Context) error {
	if cm.service == nil {
		return fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}
	return cm.service.Start(ctx)
}

// Stop stops the Cognee manager
func (cm *CogneeManager) Stop(ctx context.Context) error {
	if cm.service == nil {
		return nil
	}
	return cm.service.Stop(ctx)
}

// ProcessKnowledge processes knowledge content
func (cm *CogneeManager) ProcessKnowledge(ctx context.Context, content string) error {
	if cm.service == nil {
		return fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	if content == "" {
		return fmt.Errorf("%s", tr(ctx, "internal_cognee_content_cannot_be_empty", nil))
	}

	req := &AddMemoryRequest{
		Content:     content,
		ContentType: "text",
		DatasetName: "knowledge",
	}

	_, err := cm.service.AddMemory(ctx, req)
	if err != nil {
		return fmt.Errorf("%s: %w", tr(ctx, "internal_cognee_failed_process_knowledge", map[string]any{"Err": err.Error()}), err)
	}

	return nil
}

// SearchKnowledge searches the knowledge base
func (cm *CogneeManager) SearchKnowledge(ctx context.Context, query string) (interface{}, error) {
	if cm.service == nil {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	if query == "" {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_cognee_query_cannot_be_empty", nil))
	}

	req := &SearchMemoryRequest{
		Query:       query,
		DatasetName: "knowledge",
		Limit:       10,
	}

	resp, err := cm.service.SearchMemory(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tr(ctx, "internal_cognee_failed_search_knowledge", map[string]any{"Err": err.Error()}), err)
	}

	return resp, nil
}

// Cognify processes data into knowledge graphs
func (cm *CogneeManager) Cognify(ctx context.Context, datasets []string) error {
	if cm.service == nil {
		return fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	req := &CognifyRequest{
		Datasets: datasets,
	}

	_, err := cm.service.Cognify(ctx, req)
	if err != nil {
		return fmt.Errorf("%s: %w", tr(ctx, "internal_cognee_failed_cognify", map[string]any{"Err": err.Error()}), err)
	}

	return nil
}

// GetInsights retrieves insights from the knowledge graph
func (cm *CogneeManager) GetInsights(ctx context.Context, query string) (interface{}, error) {
	if cm.service == nil {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	if query == "" {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_cognee_query_cannot_be_empty", nil))
	}

	req := &InsightsRequest{
		Query: query,
		Limit: 10,
	}

	resp, err := cm.service.GetInsights(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tr(ctx, "internal_cognee_failed_get_insights", map[string]any{"Err": err.Error()}), err)
	}

	return resp, nil
}

// ProcessCode processes code through the code understanding pipeline
func (cm *CogneeManager) ProcessCode(ctx context.Context, code string, language string) error {
	if cm.service == nil {
		return fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	if code == "" {
		return fmt.Errorf("%s", tr(ctx, "internal_cognee_code_cannot_be_empty", nil))
	}

	req := &CodePipelineRequest{
		Code:        code,
		Language:    language,
		DatasetName: "code",
	}

	_, err := cm.service.ProcessCode(ctx, req)
	if err != nil {
		return fmt.Errorf("%s: %w", tr(ctx, "internal_cognee_failed_process_code", map[string]any{"Err": err.Error()}), err)
	}

	return nil
}

// GetStatus returns the current status
func (cm *CogneeManager) GetStatus() string {
	if cm.service == nil {
		return "not_initialized"
	}

	status := cm.service.GetStatus()
	return string(status)
}

// GetHealth returns health information
func (cm *CogneeManager) GetHealth(ctx context.Context) (*HealthStatus, error) {
	if cm.service == nil {
		return &HealthStatus{
			Status: "not_initialized",
		}, nil
	}

	return cm.service.GetHealth(ctx)
}

// GetStatistics returns service statistics
func (cm *CogneeManager) GetStatistics(ctx context.Context) (*CogneeStatistics, error) {
	if cm.service == nil {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	return cm.service.GetStatistics(ctx)
}

// CreateDataset creates a new dataset
func (cm *CogneeManager) CreateDataset(ctx context.Context, name, description string) error {
	if cm.service == nil {
		return fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	req := &CreateDatasetRequest{
		Name:        name,
		Description: description,
	}

	_, err := cm.service.CreateDataset(ctx, req)
	if err != nil {
		return fmt.Errorf("%s: %w", tr(ctx, "internal_cognee_failed_create_dataset", map[string]any{"Err": err.Error()}), err)
	}

	return nil
}

// ListDatasets retrieves all datasets
func (cm *CogneeManager) ListDatasets(ctx context.Context) ([]Dataset, error) {
	if cm.service == nil {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	resp, err := cm.service.ListDatasets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list datasets: %w", err)
	}

	return resp.Datasets, nil
}

// DeleteDataset deletes a dataset
func (cm *CogneeManager) DeleteDataset(ctx context.Context, name string) error {
	if cm.service == nil {
		return fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	return cm.service.DeleteDataset(ctx, name)
}

// VisualizeGraph retrieves graph visualization data
func (cm *CogneeManager) VisualizeGraph(ctx context.Context, datasetName string) (*GraphVisualizationResponse, error) {
	if cm.service == nil {
		return nil, fmt.Errorf("%s", tr(ctx, "internal_cognee_service_not_initialized", nil))
	}

	req := &GraphVisualizationRequest{
		DatasetName: datasetName,
		Format:      "json",
	}

	return cm.service.VisualizeGraph(ctx, req)
}

// Close closes the Cognee manager
func (cm *CogneeManager) Close() error {
	if cm.service == nil {
		return nil
	}

	return cm.service.Stop(context.Background())
}

// GetService returns the underlying service
func (cm *CogneeManager) GetService() *CogneeService {
	return cm.service
}

// RegisterEventHandler registers a handler for Cognee events
func (cm *CogneeManager) RegisterEventHandler(handler func(*CogneeEvent)) {
	if cm.service != nil {
		cm.service.RegisterEventHandler(handler)
	}
}
