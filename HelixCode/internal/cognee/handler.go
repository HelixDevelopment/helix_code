package cognee

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"dev.helix.code/internal/logging"
)

// Handler provides HTTP handlers for Cognee API endpoints
type Handler struct {
	service *CogneeService
	manager *CogneeManager
	logger  *logging.Logger
}

// NewHandler creates a new Cognee HTTP handler
func NewHandler(service *CogneeService) *Handler {
	return &Handler{
		service: service,
		logger:  logging.NewLoggerWithName("cognee_handler"),
	}
}

// NewHandlerWithManager creates a new Cognee HTTP handler with manager
func NewHandlerWithManager(manager *CogneeManager) *Handler {
	var service *CogneeService
	if manager != nil {
		service = manager.GetService()
	}
	return &Handler{
		service: service,
		manager: manager,
		logger:  logging.NewLoggerWithName("cognee_handler"),
	}
}

// RegisterRoutes registers all Cognee routes
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	cognee := router.Group("/cognee")
	{
		cognee.GET("/health", h.GetHealth)
		cognee.GET("/stats", h.GetStatistics)

		cognee.POST("/memory", h.AddMemory)
		cognee.POST("/memory/batch", h.AddBatchMemory)
		cognee.POST("/search", h.SearchMemory)
		cognee.DELETE("/memory", h.DeleteData)

		cognee.POST("/cognify", h.Cognify)
		cognee.POST("/insights", h.GetInsights)
		cognee.POST("/graph/complete", h.GetGraphCompletion)

		cognee.POST("/code", h.ProcessCode)

		cognee.GET("/datasets", h.ListDatasets)
		cognee.POST("/datasets", h.CreateDataset)
		cognee.GET("/datasets/:name", h.GetDataset)
		cognee.DELETE("/datasets/:name", h.DeleteDataset)

		cognee.POST("/visualize", h.VisualizeGraph)

		cognee.POST("/feedback", h.SubmitFeedback)
	}
}

// GetHealth handles GET /cognee/health
func (h *Handler) GetHealth(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":  "unavailable",
			"message": "Cognee service not initialized",
		})
		return
	}

	health, err := h.service.GetHealth(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get health status",
			"details": err.Error(),
		})
		return
	}

	status := http.StatusOK
	if health.Status != "healthy" {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, health)
}

// GetStatistics handles GET /cognee/stats
func (h *Handler) GetStatistics(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	stats, err := h.service.GetStatistics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get statistics",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// AddMemory handles POST /cognee/memory
func (h *Handler) AddMemory(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req AddMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		req.UserID = uid
	}

	projectID := c.GetHeader("X-Project-ID")
	if projectID != "" {
		req.ProjectID = projectID
	}

	resp, err := h.service.AddMemory(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to add memory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to add memory",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// AddBatchMemory handles POST /cognee/memory/batch
func (h *Handler) AddBatchMemory(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req BatchMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		for i := range req.Memories {
			req.Memories[i].UserID = uid
		}
	}

	resp, err := h.service.AddBatchMemory(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to add batch memory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to add batch memory",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// SearchMemory handles POST /cognee/search
func (h *Handler) SearchMemory(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req SearchMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		req.UserID = uid
	}

	resp, err := h.service.SearchMemory(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to search memory: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to search memory",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteData handles DELETE /cognee/memory
func (h *Handler) DeleteData(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req DeleteDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		req.UserID = uid
	}

	resp, err := h.service.DeleteData(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to delete data: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to delete data",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Cognify handles POST /cognee/cognify
func (h *Handler) Cognify(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req CognifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		req.UserID = uid
	}

	resp, err := h.service.Cognify(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to cognify: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to cognify",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetInsights handles POST /cognee/insights
func (h *Handler) GetInsights(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req InsightsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		req.UserID = uid
	}

	resp, err := h.service.GetInsights(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to get insights: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get insights",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetGraphCompletion handles POST /cognee/graph/complete
func (h *Handler) GetGraphCompletion(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req struct {
		Query    string   `json:"query" binding:"required"`
		Datasets []string `json:"datasets,omitempty"`
		Limit    int      `json:"limit,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 10
	}

	resp, err := h.service.GetGraphCompletion(c.Request.Context(), req.Query, req.Datasets, req.Limit)
	if err != nil {
		h.logger.Error("Failed to get graph completion: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get graph completion",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ProcessCode handles POST /cognee/code
func (h *Handler) ProcessCode(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req CodePipelineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		req.UserID = uid
	}

	projectID := c.GetHeader("X-Project-ID")
	if projectID != "" {
		req.ProjectID = projectID
	}

	resp, err := h.service.ProcessCode(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to process code: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to process code",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListDatasets handles GET /cognee/datasets
func (h *Handler) ListDatasets(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	resp, err := h.service.ListDatasets(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list datasets: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to list datasets",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// CreateDataset handles POST /cognee/datasets
func (h *Handler) CreateDataset(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req CreateDatasetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		req.UserID = uid
	}

	resp, err := h.service.CreateDataset(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to create dataset: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create dataset",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetDataset handles GET /cognee/datasets/:name
func (h *Handler) GetDataset(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "dataset name is required",
		})
		return
	}

	dataset, err := h.service.GetDataset(c.Request.Context(), name)
	if err != nil {
		h.logger.Error("Failed to get dataset: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to get dataset",
			"details": err.Error(),
		})
		return
	}

	if dataset == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "dataset not found",
		})
		return
	}

	c.JSON(http.StatusOK, dataset)
}

// DeleteDataset handles DELETE /cognee/datasets/:name
func (h *Handler) DeleteDataset(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	name := c.Param("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "dataset name is required",
		})
		return
	}

	if err := h.service.DeleteDataset(c.Request.Context(), name); err != nil {
		h.logger.Error("Failed to delete dataset: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to delete dataset",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "dataset deleted successfully",
	})
}

// VisualizeGraph handles POST /cognee/visualize
func (h *Handler) VisualizeGraph(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req GraphVisualizationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	resp, err := h.service.VisualizeGraph(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to visualize graph: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to visualize graph",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// SubmitFeedback handles POST /cognee/feedback
func (h *Handler) SubmitFeedback(c *gin.Context) {
	if h.service == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Cognee service not initialized",
		})
		return
	}

	var req FeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	userID, _ := c.Get("user_id")
	if uid, ok := userID.(string); ok {
		req.UserID = uid
	}

	resp, err := h.service.SubmitFeedback(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to submit feedback: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to submit feedback",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, resp)
}
