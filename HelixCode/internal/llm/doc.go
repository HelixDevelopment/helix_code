// Package llm provides multi-provider Large Language Model (LLM) integration for the HelixCode platform.
//
// The llm package offers a unified interface for interacting with multiple LLM providers,
// both local inference servers and cloud APIs. It handles provider management, model selection,
// streaming responses, token counting, load balancing, and health monitoring.
//
// # Supported Providers
//
// Local Providers:
//   - Ollama: Local LLM server with model management
//   - Llama.cpp: Direct llama.cpp inference
//   - vLLM: High-throughput inference server
//   - LocalAI: OpenAI-compatible local server
//   - LM Studio: Desktop LLM application
//   - Jan: Open-source AI assistant
//   - GPT4All: Privacy-focused local models
//   - KoboldAI: Text generation suite
//   - TabbyAPI: Tabby inference server
//   - MLX: Apple Silicon optimized inference
//   - MistralRS: Rust-based Mistral inference
//
// Cloud Providers:
//   - OpenAI: GPT-4, GPT-3.5, and other models
//   - Anthropic: Claude models
//   - Google Gemini: Gemini Pro and Ultra
//   - Vertex AI: Google Cloud AI platform
//   - Qwen: Alibaba's Qwen models
//   - xAI: Grok models
//   - Groq: High-speed inference
//   - OpenRouter: Multi-provider routing
//   - GitHub Copilot: Code-focused assistance
//   - AWS Bedrock: Amazon's model marketplace
//   - Azure OpenAI: Microsoft-hosted OpenAI
//
// # Provider Interface
//
// All providers implement the unified Provider interface:
//
//	type Provider interface {
//	    GetType() ProviderType
//	    GetName() string
//	    GetModels() []ModelInfo
//	    GetCapabilities() []ModelCapability
//	    Generate(ctx context.Context, request *LLMRequest) (*LLMResponse, error)
//	    GenerateStream(ctx context.Context, request *LLMRequest, ch chan<- LLMResponse) error
//	    IsAvailable(ctx context.Context) bool
//	    GetHealth(ctx context.Context) (*ProviderHealth, error)
//	    Close() error
//	}
//
// # Basic Usage
//
// Creating a provider and generating text:
//
//	// Create provider from configuration
//	config := llm.ProviderConfigEntry{
//	    Type:    llm.ProviderTypeOpenAI,
//	    APIKey:  os.Getenv("OPENAI_API_KEY"),
//	    Enabled: true,
//	}
//	provider, err := llm.NewProvider(config)
//
//	// Generate text
//	req := &llm.LLMRequest{
//	    Model:       "gpt-4",
//	    Messages:    []llm.Message{{Role: "user", Content: "Write a function"}},
//	    MaxTokens:   500,
//	    Temperature: 0.7,
//	}
//	resp, err := provider.Generate(ctx, req)
//	fmt.Println(resp.Content)
//
// # Streaming Responses
//
// For long-running generations, use streaming:
//
//	req := &llm.LLMRequest{
//	    Model:    "gpt-4",
//	    Messages: []llm.Message{{Role: "user", Content: "Explain ML"}},
//	    Stream:   true,
//	}
//
//	responseChan := make(chan llm.LLMResponse, 100)
//	go func() {
//	    err := provider.GenerateStream(ctx, req, responseChan)
//	    if err != nil {
//	        log.Printf("Stream error: %v", err)
//	    }
//	}()
//
//	for chunk := range responseChan {
//	    fmt.Print(chunk.Content)
//	}
//
// # Model Manager
//
// The ModelManager handles multi-provider orchestration and optimal model selection:
//
//	// Initialize with multiple providers
//	configs := []llm.ProviderConfigEntry{
//	    {Type: llm.ProviderTypeOpenAI, APIKey: openaiKey, Enabled: true},
//	    {Type: llm.ProviderTypeOllama, Endpoint: "http://localhost:11434", Enabled: true},
//	}
//	manager, err := llm.InitializeModelManager(configs)
//
//	// Select optimal model based on criteria
//	criteria := llm.ModelSelectionCriteria{
//	    TaskType:             "code_generation",
//	    RequiredCapabilities: []llm.ModelCapability{llm.CapabilityCodeGeneration},
//	    MaxTokens:            8192,
//	    QualityPreference:    "balanced",
//	}
//	model, err := manager.SelectOptimalModel(criteria)
//
//	// Get provider for the selected model
//	provider, err := manager.GetProviderForModel(model.Name, model.Provider)
//
// # Model Capabilities
//
// Models declare their capabilities for intelligent selection:
//
//   - CapabilityCodeGeneration: Can generate source code
//   - CapabilityPlanning: Good at task planning and reasoning
//   - CapabilityDebugging: Skilled at debugging and error analysis
//   - CapabilityTesting: Can generate test cases
//   - CapabilityRefactoring: Can improve code structure
//   - CapabilityChat: General conversation ability
//   - CapabilityVision: Can process images
//   - CapabilityFunctionCalling: Supports tool/function calls
//
// # Provider Selection Strategies
//
// Configure provider selection via the selection strategy:
//
//   - "performance": Select provider with lowest latency
//   - "cost": Select cheapest provider for the request
//   - "availability": Select first available provider
//   - "round-robin": Distribute requests evenly across providers
//
// # Load Balancing
//
// The package includes built-in load balancing:
//
//	balancer := llm.NewLoadBalancer(providers)
//	balancer.SetStrategy(llm.StrategyLeastLoaded)
//	provider := balancer.SelectProvider(ctx, request)
//
// # Health Monitoring
//
// Monitor provider health across the system:
//
//	healthMap := manager.HealthCheck(ctx)
//	for provider, health := range healthMap {
//	    fmt.Printf("%s: %s (latency: %v)\n",
//	        provider, health.Status, health.Latency)
//	}
//
// # Token Usage Tracking
//
// Every response includes token usage information:
//
//	resp, _ := provider.Generate(ctx, req)
//	fmt.Printf("Prompt: %d, Completion: %d, Total: %d tokens\n",
//	    resp.Usage.PromptTokens,
//	    resp.Usage.CompletionTokens,
//	    resp.Usage.TotalTokens)
//
// # Tool/Function Calling
//
// Providers that support function calling can use tools:
//
//	req := &llm.LLMRequest{
//	    Model:    "gpt-4",
//	    Messages: messages,
//	    Tools: []llm.Tool{{
//	        Type: "function",
//	        Function: llm.FunctionDefinition{
//	            Name:        "get_weather",
//	            Description: "Get weather for a location",
//	            Parameters:  weatherSchema,
//	        },
//	    }},
//	}
//
// # Caching and Prompt Optimization
//
// The package supports prompt caching for supported providers:
//
//	req := &llm.LLMRequest{
//	    Model:       "claude-3-sonnet",
//	    Messages:    messages,
//	    CacheConfig: &llm.CacheConfig{
//	        Enabled:    true,
//	        TTL:        300,
//	        CacheBreakpoints: []int{0, 5}, // Cache system and first messages
//	    },
//	}
//
// # Subpackages
//
// The llm package includes specialized subpackages:
//
//   - compression: Context compression strategies for long conversations
//   - vision: Vision capability detection and model switching
//
// # Thread Safety
//
// All providers and the ModelManager are thread-safe for concurrent use.
// However, response channels for streaming should only be read by a single goroutine.
package llm
