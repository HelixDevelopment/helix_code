package cognee

import (
	"time"
)

// CogneeMemory represents a memory entry stored in Cognee
type CogneeMemory struct {
	ID          string                 `json:"id" db:"id"`
	VectorID    string                 `json:"vector_id" db:"vector_id"`
	Content     string                 `json:"content" db:"content"`
	ContentType string                 `json:"content_type" db:"content_type"`
	DatasetName string                 `json:"dataset_name" db:"dataset_name"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
	GraphNodes  map[string]interface{} `json:"graph_nodes,omitempty" db:"graph_nodes"`
	Embedding   []float32              `json:"embedding,omitempty" db:"embedding"`
	Score       float64                `json:"score,omitempty" db:"score"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
	UserID      string                 `json:"user_id,omitempty" db:"user_id"`
	ProjectID   string                 `json:"project_id,omitempty" db:"project_id"`
}

// MemorySource represents a source of memory data
type MemorySource struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	Score       float64                `json:"score"`
	Source      string                 `json:"source"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	DatasetName string                 `json:"dataset_name,omitempty"`
}

// AddMemoryRequest represents a request to add memory to Cognee
type AddMemoryRequest struct {
	Content     string                 `json:"content" binding:"required"`
	DatasetName string                 `json:"dataset_name"`
	ContentType string                 `json:"content_type"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	ProjectID   string                 `json:"project_id,omitempty"`
}

// AddMemoryResponse represents a response from adding memory
type AddMemoryResponse struct {
	ID         string                 `json:"id"`
	VectorID   string                 `json:"vector_id"`
	GraphNodes map[string]interface{} `json:"graph_nodes,omitempty"`
	Message    string                 `json:"message"`
}

// SearchMemoryRequest represents a memory search request
type SearchMemoryRequest struct {
	Query       string   `json:"query" binding:"required"`
	DatasetName string   `json:"dataset_name,omitempty"`
	Datasets    []string `json:"datasets,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	Threshold   float64  `json:"threshold,omitempty"`
	SearchType  string   `json:"search_type,omitempty"` // "CHUNKS", "INSIGHTS", "GRAPH_COMPLETION"
	UserID      string   `json:"user_id,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
}

// SearchMemoryResponse represents a search response
type SearchMemoryResponse struct {
	Results    []MemorySource `json:"results"`
	TotalCount int            `json:"total_count"`
	Query      string         `json:"query"`
	Duration   time.Duration  `json:"duration"`
}

// CognifyRequest represents a request to cognify data
type CognifyRequest struct {
	Datasets []string `json:"datasets,omitempty"`
	UserID   string   `json:"user_id,omitempty"`
}

// CognifyResponse represents a cognify response
type CognifyResponse struct {
	Status    string    `json:"status"`
	Message   string    `json:"message,omitempty"`
	StartedAt time.Time `json:"started_at,omitempty"`
}

// InsightsRequest represents a request for insights
type InsightsRequest struct {
	Query    string   `json:"query" binding:"required"`
	Datasets []string `json:"datasets,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	UserID   string   `json:"user_id,omitempty"`
}

// InsightsResponse represents an insights response
type InsightsResponse struct {
	Insights []Insight     `json:"insights"`
	Query    string        `json:"query"`
	Duration time.Duration `json:"duration"`
}

// Insight represents a single insight from Cognee
type Insight struct {
	ID           string                 `json:"id"`
	Content      string                 `json:"content"`
	Type         string                 `json:"type"`
	Confidence   float64                `json:"confidence"`
	Sources      []string               `json:"sources,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	RelatedNodes []string               `json:"related_nodes,omitempty"`
}

// CodePipelineRequest represents a request to process code
type CodePipelineRequest struct {
	Code        string `json:"code" binding:"required"`
	DatasetName string `json:"dataset_name"`
	Language    string `json:"language,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	UserID      string `json:"user_id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
}

// CodePipelineResponse represents a code processing response
type CodePipelineResponse struct {
	Processed bool                   `json:"processed"`
	Results   map[string]interface{} `json:"results,omitempty"`
	Message   string                 `json:"message,omitempty"`
}

// Dataset represents a Cognee dataset
type Dataset struct {
	ID          string                 `json:"id" db:"id"`
	Name        string                 `json:"name" db:"name"`
	Description string                 `json:"description,omitempty" db:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
	UserID      string                 `json:"user_id,omitempty" db:"user_id"`
	MemoryCount int                    `json:"memory_count,omitempty"`
}

// CreateDatasetRequest represents a request to create a dataset
type CreateDatasetRequest struct {
	Name        string                 `json:"name" binding:"required"`
	Description string                 `json:"description,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
}

// DatasetResponse represents a dataset response
type DatasetResponse struct {
	Dataset *Dataset `json:"dataset"`
	Message string   `json:"message,omitempty"`
}

// DatasetsResponse represents a list of datasets
type DatasetsResponse struct {
	Datasets []Dataset `json:"datasets"`
	Total    int       `json:"total"`
}

// GraphVisualizationRequest represents a graph visualization request
type GraphVisualizationRequest struct {
	DatasetName string `json:"dataset_name,omitempty"`
	Format      string `json:"format,omitempty"` // "json", "graphml", "d3"
	Depth       int    `json:"depth,omitempty"`
	NodeID      string `json:"node_id,omitempty"`
}

// GraphVisualizationResponse represents a graph visualization response
type GraphVisualizationResponse struct {
	Graph     GraphData `json:"graph"`
	Format    string    `json:"format"`
	NodeCount int       `json:"node_count"`
	EdgeCount int       `json:"edge_count"`
}

// GraphData represents graph structure
type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode represents a node in the knowledge graph
type GraphNode struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Label      string                 `json:"label"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// GraphEdge represents an edge in the knowledge graph
type GraphEdge struct {
	ID         string                 `json:"id"`
	Source     string                 `json:"source"`
	Target     string                 `json:"target"`
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// FeedbackRequest represents feedback on search results
type FeedbackRequest struct {
	QueryID  string `json:"query_id" binding:"required"`
	ResultID string `json:"result_id" binding:"required"`
	Rating   int    `json:"rating" binding:"required,min=1,max=5"`
	Comment  string `json:"comment,omitempty"`
	Relevant bool   `json:"relevant"`
	UserID   string `json:"user_id,omitempty"`
}

// FeedbackResponse represents a feedback response
type FeedbackResponse struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// CogneeStatistics represents Cognee usage statistics
type CogneeStatistics struct {
	TotalMemories  int64         `json:"total_memories"`
	TotalDatasets  int64         `json:"total_datasets"`
	TotalSearches  int64         `json:"total_searches"`
	TotalCognifies int64         `json:"total_cognifies"`
	AverageScore   float64       `json:"average_score"`
	CacheHitRate   float64       `json:"cache_hit_rate"`
	LastUpdated    time.Time     `json:"last_updated"`
	ServiceUptime  time.Duration `json:"service_uptime"`
	GraphNodeCount int64         `json:"graph_node_count"`
	GraphEdgeCount int64         `json:"graph_edge_count"`
}

// HealthStatus represents Cognee service health
type HealthStatus struct {
	Status       string            `json:"status"` // "healthy", "degraded", "unhealthy"
	Components   map[string]string `json:"components"`
	Timestamp    time.Time         `json:"timestamp"`
	Version      string            `json:"version,omitempty"`
	Uptime       time.Duration     `json:"uptime"`
	LastCheck    time.Time         `json:"last_check"`
	ResponseTime time.Duration     `json:"response_time"`
}

// DeleteDataRequest represents a request to delete data
type DeleteDataRequest struct {
	DatasetName string   `json:"dataset_name" binding:"required"`
	DataIDs     []string `json:"data_ids" binding:"required"`
	UserID      string   `json:"user_id,omitempty"`
}

// DeleteDataResponse represents a delete response
type DeleteDataResponse struct {
	Deleted int    `json:"deleted"`
	Message string `json:"message"`
}

// CogneeEvent represents an event in the Cognee system
type CogneeEvent struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Action    string                 `json:"action"`
	Data      map[string]interface{} `json:"data,omitempty"`
	UserID    string                 `json:"user_id,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// BatchMemoryRequest represents a batch memory operation
type BatchMemoryRequest struct {
	Memories []AddMemoryRequest `json:"memories" binding:"required"`
	Options  *BatchOptions      `json:"options,omitempty"`
}

// BatchOptions represents options for batch operations
type BatchOptions struct {
	CognifyAfter    bool `json:"cognify_after"`
	SkipDuplicates  bool `json:"skip_duplicates"`
	ValidateContent bool `json:"validate_content"`
}

// BatchMemoryResponse represents a batch memory response
type BatchMemoryResponse struct {
	Processed int      `json:"processed"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors,omitempty"`
	IDs       []string `json:"ids"`
	Message   string   `json:"message"`
}
