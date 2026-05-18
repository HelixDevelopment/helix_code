package llm

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"dev.helix.code/internal/hardware"
)

// IntegratedModelManager combines download, conversion, and provider management
type IntegratedModelManager struct {
	baseDir          string
	downloadManager  *ModelDownloadManager
	converter        *ModelConverter
	registry         *CrossProviderRegistry
	localLLMManager  *LocalLLMManager
	hardwareDetector *hardware.Detector
	mu               sync.RWMutex

	// Event channels
	downloadEvents   chan ModelDownloadProgress
	conversionEvents chan ConversionJob

	// Active operations
	activeDownloads   map[string]context.CancelFunc
	activeConversions map[string]context.CancelFunc
}

// IntegratedModelRequest represents a complete model management request
type IntegratedModelRequest struct {
	ModelID         string                 `json:"model_id"`
	TargetProvider  string                 `json:"target_provider"`
	TargetFormat    ModelFormat            `json:"target_format,omitempty"`
	SourceProvider  string                 `json:"source_provider,omitempty"`
	ForceDownload   bool                   `json:"force_download"`
	ConvertIfNeeded bool                   `json:"convert_if_needed"`
	OptimizeFor     string                 `json:"optimize_for,omitempty"` // "performance", "memory", "compatibility"
	Constraints     map[string]interface{} `json:"constraints,omitempty"`
	AutoStart       bool                   `json:"auto_start"`
}

// IntegratedModelResult represents the result of a model management operation
type IntegratedModelResult struct {
	Success             bool          `json:"success"`
	ModelID             string        `json:"model_id"`
	Provider            string        `json:"provider"`
	Format              ModelFormat   `json:"format"`
	Path                string        `json:"path"`
	Converted           bool          `json:"converted"`
	DownloadTime        time.Duration `json:"download_time"`
	ConversionTime      time.Duration `json:"conversion_time"`
	TotalTime           time.Duration `json:"total_time"`
	Size                int64         `json:"size"`
	CompatibleProviders []string      `json:"compatible_providers"`
	Warnings            []string      `json:"warnings"`
	Recommendations     []string      `json:"recommendations"`
	Error               string        `json:"error,omitempty"`
}

// ModelOperationStatus represents the status of an ongoing operation
type ModelOperationStatus struct {
	OperationID  string    `json:"operation_id"`
	Type         string    `json:"type"` // "download", "conversion", "integrated"`
	ModelID      string    `json:"model_id"`
	Progress     float64   `json:"progress"`
	Status       string    `json:"status"`
	StartTime    time.Time `json:"start_time"`
	EstimatedETA int64     `json:"estimated_eta"`
	CurrentStep  string    `json:"current_step"`
	Error        string    `json:"error,omitempty"`
}

// NewIntegratedModelManager creates a new integrated model manager
func NewIntegratedModelManager(baseDir string) *IntegratedModelManager {
	return &IntegratedModelManager{
		baseDir:           baseDir,
		downloadManager:   NewModelDownloadManager(baseDir),
		converter:         NewModelConverter(baseDir),
		registry:          NewCrossProviderRegistry(baseDir),
		localLLMManager:   NewLocalLLMManager(baseDir),
		hardwareDetector:  hardware.NewDetector(),
		downloadEvents:    make(chan ModelDownloadProgress, 100),
		conversionEvents:  make(chan ConversionJob, 100),
		activeDownloads:   make(map[string]context.CancelFunc),
		activeConversions: make(map[string]context.CancelFunc),
	}
}

// AcquireModel acquires a model for a specific provider, handling download and conversion
func (m *IntegratedModelManager) AcquireModel(ctx context.Context, req IntegratedModelRequest) (<-chan ModelOperationStatus, error) {
	// Validate request
	if err := m.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Create operation status channel
	statusChan := make(chan ModelOperationStatus, 100)

	// Start operation in background
	go m.acquireModelOperation(ctx, req, statusChan)

	return statusChan, nil
}

// OptimizeModel optimizes a model for specific hardware or constraints
func (m *IntegratedModelManager) OptimizeModel(ctx context.Context, modelPath string, optimizeFor string, constraints map[string]interface{}) (<-chan ModelOperationStatus, error) {
	statusChan := make(chan ModelOperationStatus, 100)

	go func() {
		defer close(statusChan)

		opID := fmt.Sprintf("opt_%d", time.Now().UnixNano())
		status := ModelOperationStatus{
			OperationID: opID,
			Type:        "optimization",
			Progress:    0.0,
			Status:      "starting",
			StartTime:   time.Now(),
		}

		statusChan <- status

		// Detect current format
		sourceFormat, err := m.detectModelFormat(modelPath)
		if err != nil {
			status.Status = "failed"
			status.Error = fmt.Sprintf("failed to detect format: %v", err)
			statusChan <- status
			return
		}

		// Determine optimal format based on constraints
		targetFormat, err := m.determineOptimalFormat(sourceFormat, optimizeFor, constraints)
		if err != nil {
			status.Status = "failed"
			status.Error = fmt.Sprintf("failed to determine optimal format: %v", err)
			statusChan <- status
			return
		}

		// Convert if needed
		if targetFormat != sourceFormat {
			status.CurrentStep = "converting model"
			status.Progress = 0.3
			statusChan <- status

			config := ConversionConfig{
				SourcePath:   modelPath,
				SourceFormat: sourceFormat,
				TargetFormat: targetFormat,
				Optimization: &OptimizationConfig{
					OptimizeFor:        optimizeFor,
					RemoveUnusedLayers: true,
					FuseOperations:     true,
				},
				Timeout: 30,
			}

			job, err := m.converter.ConvertModel(ctx, config)
			if err != nil {
				status.Status = "failed"
				status.Error = fmt.Sprintf("conversion failed: %v", err)
				statusChan <- status
				return
			}

			// Monitor conversion
			for {
				currentJob, err := m.converter.GetConversionStatus(job.ID)
				if err != nil {
					status.Status = "failed"
					status.Error = fmt.Sprintf("failed to get conversion status: %v", err)
					statusChan <- status
					return
				}

				status.Progress = 0.3 + (currentJob.Progress * 0.7)
				status.CurrentStep = currentJob.CurrentStep
				statusChan <- status

				if currentJob.Status == StatusCompleted {
					break
				}

				if currentJob.Status == StatusFailed {
					status.Status = "failed"
					status.Error = fmt.Sprintf("conversion failed: %s", currentJob.Error)
					statusChan <- status
					return
				}

				select {
				case <-ctx.Done():
					m.converter.CancelConversion(job.ID)
					status.Status = "cancelled"
					statusChan <- status
					return
				case <-time.After(time.Second):
				}
			}
		}

		// Complete
		status.Status = "completed"
		status.Progress = 1.0
		status.CurrentStep = "optimization completed"
		statusChan <- status
	}()

	return statusChan, nil
}

// FindBestModel finds the best model for specific requirements
func (m *IntegratedModelManager) FindBestModel(criteria ModelSelectionCriteria) (*IntegratedModelResult, error) {
	// Get available models from download manager
	availableModels := m.downloadManager.GetAvailableModels()

	// Get hardware information
	hwInfo, err := m.hardwareDetector.Detect()
	if err != nil {
		log.Printf("Warning: hardware detection failed: %v", err)
		hwInfo = &hardware.HardwareInfo{} // Use empty info as fallback
	}

	// Score models based on criteria and hardware
	var bestModel *DownloadableModelInfo
	var bestScore float64 = 0
	var bestProvider string
	var bestFormat ModelFormat

	for _, model := range availableModels {
		// Check if model matches criteria
		if !m.modelMatchesCriteria(model, criteria) {
			continue
		}

		// Find best provider and format for this model
		providerScore, provider, format := m.scoreModelForHardware(model, hwInfo)

		// Calculate total score
		totalScore := providerScore * m.calculateModelScore(model, criteria)

		if totalScore > bestScore {
			bestScore = totalScore
			bestModel = model
			bestProvider = provider
			bestFormat = format
		}
	}

	if bestModel == nil {
		return nil, fmt.Errorf("no suitable model found for criteria")
	}

	// Check if model is already downloaded
	downloadedModels := m.registry.GetDownloadedModels()
	var downloadedPath string
	for _, downloaded := range downloadedModels {
		if downloaded.ModelID == bestModel.ID && downloaded.Provider == bestProvider {
			downloadedPath = downloaded.Path
			break
		}
	}

	result := &IntegratedModelResult{
		Success:             true,
		ModelID:             bestModel.ID,
		Provider:            bestProvider,
		Format:              bestFormat,
		Path:                downloadedPath,
		CompatibleProviders: m.findCompatibleProviders(bestModel.ID, bestFormat),
	}

	if downloadedPath == "" {
		result.Warnings = append(result.Warnings, "Model needs to be downloaded")
	}

	return result, nil
}

// GetModelStatus returns status information for a specific model
func (m *IntegratedModelManager) GetModelStatus(modelID string) (*ModelStatus, error) {
	// Check if model is downloaded
	downloadedModels := m.registry.GetDownloadedModels()
	var downloaded *DownloadedModel
	for _, model := range downloadedModels {
		if model.ModelID == modelID {
			downloaded = model
			break
		}
	}

	status := &ModelStatus{
		ModelID:   modelID,
		Available: downloaded != nil,
	}

	if downloaded != nil {
		status.Provider = downloaded.Provider
		status.Format = downloaded.Format
		status.Path = downloaded.Path
		status.Size = downloaded.Size
		status.DownloadTime = downloaded.DownloadTime
		status.LastUsed = downloaded.LastUsed
		status.UseCount = downloaded.UseCount
		status.CompatibleProviders = downloaded.CompatibleProviders
	}

	// Check model info
	modelInfo, err := m.downloadManager.GetModelByID(modelID)
	if err == nil {
		status.Name = modelInfo.Name
		status.Description = modelInfo.Description
		status.AvailableFormats = modelInfo.AvailableFormats
		status.ModelSize = modelInfo.ModelSize
		status.ContextSize = modelInfo.ContextSize
		status.Requirements = modelInfo.Requirements
	}

	return status, nil
}

// ListAvailableModels lists all models that can be acquired
func (m *IntegratedModelManager) ListAvailableModels() ([]*IntegratedModelInfo, error) {
	availableModels := m.downloadManager.GetAvailableModels()
	downloadedModels := m.registry.GetDownloadedModels()

	// Create map of downloaded models for quick lookup
	downloadedMap := make(map[string]*DownloadedModel)
	for _, model := range downloadedModels {
		key := fmt.Sprintf("%s:%s", model.ModelID, model.Provider)
		downloadedMap[key] = model
	}

	var integratedModels []*IntegratedModelInfo

	for _, model := range availableModels {
		integrated := &IntegratedModelInfo{
			DownloadableModelInfo: *model,
			Downloaded:            false,
			Providers:             m.findCompatibleProviders(model.ID, model.DefaultFormat),
		}

		// Check if downloaded
		for _, provider := range integrated.Providers {
			key := fmt.Sprintf("%s:%s", model.ID, provider)
			if downloaded, exists := downloadedMap[key]; exists {
				integrated.Downloaded = true
				integrated.DownloadedPath = downloaded.Path
				integrated.DownloadedFormat = downloaded.Format
				break
			}
		}

		integratedModels = append(integratedModels, integrated)
	}

	return integratedModels, nil
}

// Private methods

func (m *IntegratedModelManager) validateRequest(req IntegratedModelRequest) error {
	if req.ModelID == "" {
		return fmt.Errorf("model_id is required")
	}
	if req.TargetProvider == "" {
		return fmt.Errorf("target_provider is required")
	}

	// Check if provider exists in registry
	if _, err := m.registry.GetProviderInfo(req.TargetProvider); err != nil {
		return fmt.Errorf("unknown provider: %s", req.TargetProvider)
	}

	return nil
}

func (m *IntegratedModelManager) acquireModelOperation(ctx context.Context, req IntegratedModelRequest, statusChan chan<- ModelOperationStatus) {
	defer close(statusChan)

	opID := fmt.Sprintf("acq_%d", time.Now().UnixNano())
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Register active operation
	m.mu.Lock()
	m.activeDownloads[opID] = cancel
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.activeDownloads, opID)
		m.mu.Unlock()
	}()

	status := ModelOperationStatus{
		OperationID: opID,
		Type:        "integrated",
		ModelID:     req.ModelID,
		Progress:    0.0,
		Status:      "starting",
		StartTime:   time.Now(),
	}

	statusChan <- status

	startTime := time.Now()

	// Check if model is already available
	downloadedModels := m.registry.GetDownloadedModels()

	var existingModel *DownloadedModel
	for _, model := range downloadedModels {
		if model.ModelID == req.ModelID && model.Provider == req.TargetProvider {
			existingModel = model
			break
		}
	}

	if existingModel != nil && !req.ForceDownload {
		status.Status = "completed"
		status.Progress = 1.0
		status.CurrentStep = "Model already available"
		statusChan <- status
		return
	}

	// Check compatibility
	compatResult, err := m.registry.CheckCompatibility(ModelCompatibilityQuery{
		ModelID:        req.ModelID,
		TargetProvider: req.TargetProvider,
		TargetFormat:   req.TargetFormat,
		Constraints:    req.Constraints,
	})

	if err != nil || !compatResult.IsCompatible {
		status.Status = "failed"
		status.Error = fmt.Sprintf("Model not compatible: %v", err)
		statusChan <- status
		return
	}

	// Determine if conversion is needed
	sourceFormat := req.TargetFormat
	if compatResult.ConversionRequired && req.ConvertIfNeeded {
		// Find available source format for download
		modelInfo, err := m.downloadManager.GetModelByID(req.ModelID)
		if err != nil {
			status.Status = "failed"
			status.Error = fmt.Sprintf("Failed to get model info: %v", err)
			statusChan <- status
			return
		}

		sourceFormat = modelInfo.DefaultFormat
	}

	// Download model
	status.CurrentStep = "downloading model"
	status.Progress = 0.1
	statusChan <- status

	downloadReq := ModelDownloadRequest{
		ModelID:        req.ModelID,
		Format:         sourceFormat,
		TargetProvider: req.TargetProvider,
		ForceDownload:  req.ForceDownload,
	}

	progressChan, err := m.downloadManager.DownloadModel(ctx, downloadReq)
	if err != nil {
		status.Status = "failed"
		status.Error = fmt.Sprintf("Download failed: %v", err)
		statusChan <- status
		return
	}

	// Monitor download
	downloadProgress := 0.0
	for progress := range progressChan {
		downloadProgress = progress.Progress
		status.Progress = 0.1 + (downloadProgress * 0.6)
		status.EstimatedETA = progress.ETA

		if progress.Error != "" {
			status.Status = "failed"
			status.Error = fmt.Sprintf("Download failed: %s", progress.Error)
			statusChan <- status
			return
		}

		statusChan <- status
	}

	// Convert if needed
	if compatResult.ConversionRequired && req.ConvertIfNeeded {
		status.CurrentStep = "converting model"
		status.Progress = 0.7
		statusChan <- status

		downloadedPath := m.getModelPath(req.TargetProvider, req.ModelID, sourceFormat)

		convertConfig := ConversionConfig{
			SourcePath:   downloadedPath,
			SourceFormat: sourceFormat,
			TargetFormat: req.TargetFormat,
			Optimization: &OptimizationConfig{
				OptimizeFor: req.OptimizeFor,
			},
		}

		job, err := m.converter.ConvertModel(ctx, convertConfig)
		if err != nil {
			status.Status = "failed"
			status.Error = fmt.Sprintf("Conversion failed: %v", err)
			statusChan <- status
			return
		}

		// Monitor conversion
		for {
			convStatus, err := m.converter.GetConversionStatus(job.ID)
			if err != nil {
				status.Status = "failed"
				status.Error = fmt.Sprintf("Failed to get conversion status: %v", err)
				statusChan <- status
				return
			}

			status.Progress = 0.7 + (convStatus.Progress * 0.3)
			status.CurrentStep = convStatus.CurrentStep
			statusChan <- status

			if convStatus.Status == StatusCompleted {
				break
			}

			if convStatus.Status == StatusFailed {
				status.Status = "failed"
				status.Error = fmt.Sprintf("Conversion failed: %s", convStatus.Error)
				statusChan <- status
				return
			}

			select {
			case <-ctx.Done():
				m.converter.CancelConversion(job.ID)
				status.Status = "cancelled"
				statusChan <- status
				return
			case <-time.After(time.Second):
			}
		}
	}

	// Register downloaded model
	finalPath := m.getModelPath(req.TargetProvider, req.ModelID, req.TargetFormat)
	downloadedModel := &DownloadedModel{
		ModelID:             req.ModelID,
		Provider:            req.TargetProvider,
		Format:              req.TargetFormat,
		Path:                finalPath,
		DownloadTime:        time.Now(),
		LastUsed:            time.Now(),
		UseCount:            0,
		CompatibleProviders: m.findCompatibleProviders(req.ModelID, req.TargetFormat),
	}

	if err := m.registry.RegisterDownloadedModel(downloadedModel); err != nil {
		log.Printf("Warning: failed to register downloaded model: %v", err)
	}

	// Auto-start provider if requested
	if req.AutoStart {
		status.CurrentStep = "starting provider"
		status.Progress = 0.95
		statusChan <- status

		if err := m.localLLMManager.StartProvider(ctx, req.TargetProvider); err != nil {
			log.Printf("Warning: failed to start provider %s: %v", req.TargetProvider, err)
		}
	}

	// Complete
	status.Status = "completed"
	status.Progress = 1.0
	status.CurrentStep = "acquisition completed"
	totalTime := time.Since(startTime)
	statusChan <- status

	log.Printf("✅ Model acquisition completed: %s for %s in %v", req.ModelID, req.TargetProvider, totalTime)
}

func (m *IntegratedModelManager) detectModelFormat(path string) (ModelFormat, error) {
	// Use existing detection logic from converter
	// This is a simplified version
	ext := filepath.Ext(path)
	switch ext {
	case ".gguf":
		return FormatGGUF, nil
	case ".safetensors", ".bin":
		return FormatHF, nil
	default:
		return "", fmt.Errorf("unknown model format: %s", ext)
	}
}

// determineOptimalFormat picks a target ModelFormat for conversion based on
// the caller's optimisation hint and constraint map.
//
// Round-36 §11.4 anti-bluff fix (CONST-035 / Article XI §11.9, Pattern A1
// parameter-discard): the previous implementation discarded the
// `constraints` map entirely. Callers passing `{"cpu_only": true}` while
// asking to "optimize for performance" got FormatGPTQ — a format that
// typically requires GPU, contradicting the CPU-only constraint. The fix
// reads cpu_only / gpu_required from the constraints map and overrides
// the optimisation hint when the two conflict (constraints win — they
// represent hard hardware limits, not preferences).
func (m *IntegratedModelManager) determineOptimalFormat(sourceFormat ModelFormat, optimizeFor string, constraints map[string]interface{}) (ModelFormat, error) {
	// Constraint-driven hard overrides
	if constraints != nil {
		if cpuOnly, ok := constraints["cpu_only"].(bool); ok && cpuOnly {
			// GPTQ/AWQ generally assume GPU; GGUF is CPU-friendly.
			return FormatGGUF, nil
		}
		if gpuRequired, ok := constraints["gpu_required"].(bool); ok && gpuRequired && optimizeFor == "memory" {
			// "memory" usually returns GGUF (CPU-friendly), but caller
			// explicitly wants GPU — prefer GPTQ which is GPU-native.
			return FormatGPTQ, nil
		}
		if preferred, ok := constraints["preferred_format"].(string); ok && preferred != "" {
			return ModelFormat(preferred), nil
		}
	}

	// Optimisation-hint fallback (preserved behaviour)
	switch optimizeFor {
	case "memory":
		return FormatGGUF, nil
	case "performance":
		return FormatGPTQ, nil
	case "compatibility":
		return FormatGGUF, nil
	default:
		return sourceFormat, nil
	}
}

// modelMatchesCriteria filters DownloadableModelInfo entries against the
// caller-supplied criteria.
//
// Round-36 §11.4 anti-bluff fix (CONST-035 / Article XI §11.9, Pattern A1
// parameter-discard): the previous implementation contained two empty
// if-branches with comments "would need to be implemented" — meaning
// RequiredCapabilities and TaskType were silently dropped. Every model
// "matched" regardless. Callers using capability-filtered FindBestModel
// were receiving the same model set as callers with no filter at all —
// a §11.4 PASS-bluff at the model-selection layer.
//
// DownloadableModelInfo does not (yet) carry a typed Capabilities slice;
// the corresponding signal lives in the loosely-typed Tags slice (e.g.
// "code", "instruct", "chat", "general", "multilingual"). The filter
// implemented here is a Tags-based projection of ModelCapability →
// substring match: e.g. CapabilityCodeGeneration → any Tag containing
// "code", CapabilityReasoning → "reason"/"think". This is the honest
// best-effort mapping the data model currently supports; promoting Tags
// to a typed Capabilities slice is tracked for a separate refactor.
//
// Returns true iff: (a) every required capability has at least one matching
// tag, AND (b) context size meets MaxTokens demand, AND (c) task type has a
// matching tag (when specified).
func (m *IntegratedModelManager) modelMatchesCriteria(model *DownloadableModelInfo, criteria ModelSelectionCriteria) bool {
	// Context size — preserved from previous implementation
	if criteria.MaxTokens > 0 && model.ContextSize < criteria.MaxTokens {
		return false
	}

	// Required capabilities — Tags-based projection
	if len(criteria.RequiredCapabilities) > 0 {
		for _, required := range criteria.RequiredCapabilities {
			if !modelTagsSatisfyCapability(model.Tags, required) {
				return false
			}
		}
	}

	// Task type — Tags-based projection
	if criteria.TaskType != "" {
		if !modelTagsSatisfyTaskType(model.Tags, criteria.TaskType) {
			return false
		}
	}

	return true
}

// modelTagsSatisfyCapability returns true iff the provided tag slice
// contains at least one tag that maps to the requested capability under
// the substring-match projection described on modelMatchesCriteria.
func modelTagsSatisfyCapability(tags []string, required ModelCapability) bool {
	keywords := capabilityKeywords(required)
	if len(keywords) == 0 {
		// Unknown capability — accept (avoid over-restrictive filter for
		// caps the projection does not recognise; the alternative would
		// silently reject every model, which is worse).
		return true
	}
	for _, tag := range tags {
		lowered := strings.ToLower(tag)
		for _, kw := range keywords {
			if strings.Contains(lowered, kw) {
				return true
			}
		}
	}
	return false
}

// modelTagsSatisfyTaskType returns true iff the tag slice contains at
// least one tag that loosely associates with the requested task type.
func modelTagsSatisfyTaskType(tags []string, taskType string) bool {
	keywords := taskTypeKeywords(taskType)
	if len(keywords) == 0 {
		return true
	}
	for _, tag := range tags {
		lowered := strings.ToLower(tag)
		for _, kw := range keywords {
			if strings.Contains(lowered, kw) {
				return true
			}
		}
	}
	return false
}

// capabilityKeywords returns the lowercase substring set that signals
// presence of the given capability in a DownloadableModelInfo.Tags entry.
// "general"/"instruct"/"chat" tags are accepted as fallbacks for the
// generic text-generation capability since most models carry them.
func capabilityKeywords(cap ModelCapability) []string {
	switch cap {
	case CapabilityCodeGeneration, CapabilityCodeAnalysis, CapabilityRefactoring:
		return []string{"code", "programming", "develop"}
	case CapabilityDebugging:
		return []string{"code", "debug", "develop"}
	case CapabilityTesting:
		return []string{"code", "test"}
	case CapabilityPlanning:
		return []string{"reason", "think", "instruct"}
	case CapabilityReasoning:
		return []string{"reason", "think"}
	case CapabilityTextGeneration:
		return []string{"chat", "instruct", "general", "text"}
	}
	return nil
}

// taskTypeKeywords mirrors capabilityKeywords for the loosely-typed
// criteria.TaskType field used by SelectOptimalModel callers.
func taskTypeKeywords(taskType string) []string {
	switch strings.ToLower(strings.TrimSpace(taskType)) {
	case "code_generation", "code_edit", "refactoring":
		return []string{"code", "programming", "develop"}
	case "debugging":
		return []string{"code", "debug"}
	case "testing":
		return []string{"code", "test"}
	case "planning", "analysis":
		return []string{"reason", "think", "instruct"}
	case "documentation":
		return []string{"chat", "instruct", "general", "text"}
	}
	return nil
}

func (m *IntegratedModelManager) scoreModelForHardware(model *DownloadableModelInfo, hwInfo *hardware.HardwareInfo) (float64, string, ModelFormat) {
	// Simplified scoring - in real implementation would be more comprehensive
	bestScore := 0.0
	bestProvider := "llamacpp"
	bestFormat := FormatGGUF

	// Check GPU availability
	if hwInfo.GPU.Vendor != "" {
		bestProvider = "vllm"
		bestFormat = FormatGPTQ
		bestScore = 0.9
	} else {
		bestProvider = "llamacpp"
		bestFormat = FormatGGUF
		bestScore = 0.8
	}

	return bestScore, bestProvider, bestFormat
}

// calculateModelScore ranks a candidate DownloadableModelInfo against the
// caller-supplied criteria.
//
// Round-36 §11.4 anti-bluff fix (CONST-035 / Article XI §11.9, Pattern A1
// parameter-discard): the previous implementation read ONLY
// criteria.QualityPreference, silently discarding RequiredCapabilities,
// MaxTokens, TaskType, Budget, and LatencyRequirement. Callers setting
// MaxTokens=128k expected larger-context models to score higher than 4k
// ones; reality was a flat tie broken only by QualityPreference. This
// fix honours the available signals; Budget + LatencyRequirement remain
// unscored here because DownloadableModelInfo does not carry per-token
// price or latency metadata (those live in verifier scoring — promoted
// to ModelManager.calculateModelScore via the verifier adapter).
func (m *IntegratedModelManager) calculateModelScore(model *DownloadableModelInfo, criteria ModelSelectionCriteria) float64 {
	score := 1.0

	// Size preference based on quality preference (preserved behaviour)
	switch criteria.QualityPreference {
	case "quality":
		if model.ModelSize == "70B" {
			score *= 1.3
		} else if model.ModelSize == "34B" {
			score *= 1.2
		}
	case "fast":
		if model.ModelSize == "7B" {
			score *= 1.2
		}
	}

	// Context-size adequacy: reward headroom but cap at 2x to avoid
	// over-rewarding 128k models on a 4k request.
	if criteria.MaxTokens > 0 && model.ContextSize > 0 {
		ratio := float64(model.ContextSize) / float64(criteria.MaxTokens)
		if ratio > 2.0 {
			ratio = 2.0
		}
		score *= ratio
	}

	// Required-capabilities match — multiplicative bonus per satisfied cap
	// (modelMatchesCriteria already rejects models missing required caps;
	// this is for ranking among those that pass the filter).
	if len(criteria.RequiredCapabilities) > 0 {
		matched := 0
		for _, required := range criteria.RequiredCapabilities {
			if modelTagsSatisfyCapability(model.Tags, required) {
				matched++
			}
		}
		// Boost up to 1.5x for full capability coverage
		score *= 1.0 + (0.5 * float64(matched) / float64(len(criteria.RequiredCapabilities)))
	}

	// Task-type alignment — small bonus when tags align
	if criteria.TaskType != "" && modelTagsSatisfyTaskType(model.Tags, criteria.TaskType) {
		score *= 1.15
	}

	return score
}

// findCompatibleProviders returns the providers that can serve the given
// (modelID, format) pair.
//
// Round-36 §11.4 anti-bluff fix (CONST-035 / Article XI §11.9, Pattern A1
// parameter-discard): the previous body discarded BOTH modelID AND format,
// performed a side-effectful call whose result was thrown away (the
// `_, _ = m.registry.GetCompatibleFormats("vllm")` line), and returned a
// hardcoded four-provider list regardless of inputs. Callers asking "which
// providers support model X in format GGUF?" got the same list as callers
// asking about a non-existent model in a fictitious format — a §11.4
// PASS-bluff at the provider-compatibility surface.
//
// This implementation delegates to the registry's
// findCompatibleProvidersForModel helper, which respects both the format
// and any per-model CompatibleProviders allowlist that the downloader
// has registered for this modelID.
func (m *IntegratedModelManager) findCompatibleProviders(modelID string, format ModelFormat) []string {
	return m.registry.findCompatibleProvidersForModel(modelID, format)
}

func (m *IntegratedModelManager) getModelPath(provider, modelID string, format ModelFormat) string {
	return filepath.Join(m.baseDir, provider, modelID, fmt.Sprintf("model.%s", format))
}

// ModelStatus represents the status of a model
type ModelStatus struct {
	ModelID             string            `json:"model_id"`
	Name                string            `json:"name,omitempty"`
	Description         string            `json:"description,omitempty"`
	Available           bool              `json:"available"`
	Provider            string            `json:"provider,omitempty"`
	Format              ModelFormat       `json:"format,omitempty"`
	Path                string            `json:"path,omitempty"`
	Size                int64             `json:"size,omitempty"`
	DownloadTime        time.Time         `json:"download_time,omitempty"`
	LastUsed            time.Time         `json:"last_used,omitempty"`
	UseCount            int               `json:"use_count,omitempty"`
	AvailableFormats    []ModelFormat     `json:"available_formats,omitempty"`
	CompatibleProviders []string          `json:"compatible_providers,omitempty"`
	ModelSize           string            `json:"model_size,omitempty"`
	ContextSize         int               `json:"context_size,omitempty"`
	Requirements        ModelRequirements `json:"requirements,omitempty"`
}

// IntegratedModelInfo combines model info with download status
type IntegratedModelInfo struct {
	DownloadableModelInfo
	Downloaded       bool        `json:"downloaded"`
	DownloadedPath   string      `json:"downloaded_path,omitempty"`
	DownloadedFormat ModelFormat `json:"downloaded_format,omitempty"`
	Providers        []string    `json:"providers"`
}
