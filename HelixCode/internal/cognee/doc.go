// Package cognee provides integration with Cognee, a knowledge graph and memory
// management system for AI applications.
//
// # Overview
//
// The cognee package enables HelixCode to leverage Cognee's capabilities for
// knowledge management, semantic search, and code understanding. It provides
// a comprehensive API client, service layer with caching, and manager for
// high-level operations.
//
// # Architecture
//
// The package is organized around several core components:
//
//   - Client: HTTP client for Cognee API communication
//   - CogneeService: Service layer with caching, statistics, and lifecycle management
//   - CogneeManager: High-level manager for application integration
//   - PerformanceOptimizer: Hardware-aware performance optimization
//   - Models: Request/response types for all Cognee operations
//
// # Key Features
//
// The package supports the following Cognee capabilities:
//
//   - Memory management: Store and retrieve memories with vector embeddings
//   - Semantic search: Search memories using natural language queries
//   - Knowledge graphs: Process data into connected knowledge graphs (cognify)
//   - Insights: Extract insights using graph reasoning
//   - Code understanding: Process and analyze code through specialized pipelines
//   - Dataset management: Organize memories into datasets
//   - Graph visualization: Visualize knowledge graph structures
//
// # Basic Usage
//
// Creating and starting the Cognee service:
//
//	// Create configuration
//	cfg := config.DefaultCogneeConfig()
//	cfg.Enabled = true
//	cfg.Host = "localhost"
//	cfg.Port = 8000
//
//	// Create service
//	service, err := cognee.NewCogneeService(cfg, hwProfile)
//	if err != nil {
//	    return err
//	}
//
//	// Start service
//	err = service.Start(ctx)
//
// # Adding Memories
//
// Storing knowledge in Cognee:
//
//	req := &cognee.AddMemoryRequest{
//	    Content:     "Go is a statically typed, compiled programming language.",
//	    ContentType: "text",
//	    DatasetName: "programming",
//	    Metadata:    map[string]interface{}{"language": "go"},
//	}
//
//	resp, err := service.AddMemory(ctx, req)
//	// resp.ID contains the memory identifier
//	// resp.VectorID contains the vector embedding ID
//
// # Searching Memories
//
// Semantic search across stored memories:
//
//	req := &cognee.SearchMemoryRequest{
//	    Query:       "What are Go's key features?",
//	    DatasetName: "programming",
//	    Limit:       10,
//	}
//
//	resp, err := service.SearchMemory(ctx, req)
//	for _, result := range resp.Results {
//	    fmt.Printf("Score: %.2f, Content: %s\n", result.Score, result.Content)
//	}
//
// # Knowledge Graph Processing
//
// Processing data into knowledge graphs:
//
//	req := &cognee.CognifyRequest{
//	    Datasets: []string{"programming", "architecture"},
//	}
//
//	resp, err := service.Cognify(ctx, req)
//	// Knowledge graph is built asynchronously
//
// # Insights and Graph Completion
//
// Extracting insights using graph reasoning:
//
//	req := &cognee.InsightsRequest{
//	    Query:    "How do microservices communicate?",
//	    Datasets: []string{"architecture"},
//	    Limit:    5,
//	}
//
//	resp, err := service.GetInsights(ctx, req)
//	for _, insight := range resp.Insights {
//	    fmt.Printf("Insight: %s (confidence: %.2f)\n", insight.Content, insight.Confidence)
//	}
//
// # Code Understanding
//
// Processing code through the code pipeline:
//
//	req := &cognee.CodePipelineRequest{
//	    Code:        "func Hello() string { return \"Hello, World!\" }",
//	    Language:    "go",
//	    DatasetName: "code",
//	    FilePath:    "hello.go",
//	}
//
//	resp, err := service.ProcessCode(ctx, req)
//
// # Dataset Management
//
// Managing datasets:
//
//	// Create dataset
//	req := &cognee.CreateDatasetRequest{
//	    Name:        "my-project",
//	    Description: "Knowledge base for my project",
//	}
//	resp, err := service.CreateDataset(ctx, req)
//
//	// List datasets
//	datasets, err := service.ListDatasets(ctx)
//
//	// Delete dataset
//	err = service.DeleteDataset(ctx, "my-project")
//
// # Graph Visualization
//
// Retrieving graph visualization data:
//
//	req := &cognee.GraphVisualizationRequest{
//	    DatasetName: "programming",
//	    Format:      "json",
//	    Depth:       2,
//	}
//
//	resp, err := service.VisualizeGraph(ctx, req)
//	// resp.Graph.Nodes and resp.Graph.Edges contain the graph structure
//
// # Using CogneeManager
//
// High-level manager for simpler integration:
//
//	manager, err := cognee.NewCogneeManager(helixConfig, hwProfile)
//	if err != nil {
//	    return err
//	}
//
//	// Start manager
//	err = manager.Start(ctx)
//
//	// Process knowledge
//	err = manager.ProcessKnowledge(ctx, "Some content to remember")
//
//	// Search knowledge
//	results, err := manager.SearchKnowledge(ctx, "query")
//
//	// Stop manager
//	err = manager.Stop(ctx)
//
// # Caching
//
// The service includes built-in caching for improved performance:
//
//   - Memory caching: Recently accessed memories
//   - Search caching: Recent search results
//   - Dataset caching: Dataset metadata
//
// Cache statistics are available through GetStatistics:
//
//	stats, err := service.GetStatistics(ctx)
//	fmt.Printf("Cache hit rate: %.2f%%\n", stats.CacheHitRate * 100)
//
// # Event Handling
//
// Register handlers for Cognee events:
//
//	service.RegisterEventHandler(func(event *cognee.CogneeEvent) {
//	    fmt.Printf("Event: %s, Action: %s\n", event.Type, event.Action)
//	})
//
// # Health Monitoring
//
// Check service health:
//
//	health, err := service.GetHealth(ctx)
//	// health.Status: "healthy", "degraded", or "unhealthy"
//	// health.ResponseTime: API response latency
//
// # Service Status
//
// The service tracks its status through the lifecycle:
//
//	const (
//	    ServiceStatusStopped  // Service is not running
//	    ServiceStatusStarting // Service is starting up
//	    ServiceStatusRunning  // Service is operational
//	    ServiceStatusError    // Service encountered an error
//	)
//
//	status := service.GetStatus()
//
// # Auto-Containerization
//
// The client can automatically start Cognee in a container:
//
//	cfg.AutoStart = true // Enable auto-start
//	// Client will attempt to start Cognee via Docker/Podman if not running
//
// # Batch Operations
//
// Add multiple memories efficiently:
//
//	req := &cognee.BatchMemoryRequest{
//	    Memories: []cognee.AddMemoryRequest{
//	        {Content: "Memory 1", DatasetName: "batch"},
//	        {Content: "Memory 2", DatasetName: "batch"},
//	    },
//	    Options: &cognee.BatchOptions{
//	        CognifyAfter:   true,
//	        SkipDuplicates: true,
//	    },
//	}
//
//	resp, err := service.AddBatchMemory(ctx, req)
//
// # Thread Safety
//
// All public types in this package are safe for concurrent use.
// The service uses appropriate synchronization for state management and caching.
package cognee
