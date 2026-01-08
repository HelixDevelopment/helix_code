// Package provider defines the unified LLM provider interface for HelixCode.
//
// This package establishes the core interface contract that all LLM providers
// must implement, enabling HelixCode to work seamlessly with multiple AI
// backends including cloud services and local deployments.
//
// # Architecture
//
// The package defines:
//
//   - Provider: The core interface for LLM operations
//   - ProviderType: Enumeration of supported provider types
//
// This separation allows the llm package to manage provider selection,
// fallback, and load balancing while individual provider implementations
// focus on their specific API integrations.
//
// # Provider Interface
//
// All LLM providers must implement the Provider interface:
//
//	type Provider interface {
//	    GetType() ProviderType
//	    GetName() string
//	    GetModels() []llm.ModelInfo
//	    GetCapabilities() []llm.ModelCapability
//	    Generate(ctx, *llm.LLMRequest) (*llm.LLMResponse, error)
//	    GenerateStream(ctx, *llm.LLMRequest, chan<- llm.LLMResponse) error
//	    IsAvailable(ctx) bool
//	    GetHealth(ctx) (*llm.ProviderHealth, error)
//	    Close() error
//
//	    // Cognee integration
//	    SupportsCognee() bool
//	    InitializeCognee(config, options interface{}) error
//	    GetModelName() string
//	    GetModelInfo() *llm.ModelInfo
//	    GetHardwareProfile() *hardware.HardwareProfile
//	}
//
// # Supported Provider Types
//
// Cloud Providers:
//
//	ProviderTypeOpenAI     - OpenAI API (GPT-4, GPT-3.5)
//	ProviderTypeAnthropic  - Anthropic API (Claude models)
//	ProviderTypeGemini     - Google Gemini API
//	ProviderTypeVertexAI   - Google Vertex AI
//	ProviderTypeAzure      - Azure OpenAI Service
//	ProviderTypeBedrock    - AWS Bedrock
//	ProviderTypeGroq       - Groq API
//	ProviderTypeQwen       - Alibaba Qwen API
//	ProviderTypeCopilot    - GitHub Copilot
//	ProviderTypeOpenRouter - OpenRouter aggregator
//	ProviderTypeXAI        - xAI (Grok models)
//
// Local Providers:
//
//	ProviderTypeOllama   - Ollama local models
//	ProviderTypeLocal    - Generic local provider
//	ProviderTypeLlamaCpp - Llama.cpp integration
//	ProviderTypeVLLM     - vLLM server
//	ProviderTypeLocalAI  - LocalAI server
//
// # Usage Example
//
// Checking and using a provider:
//
//	var provider provider.Provider = someProviderInstance
//
//	// Check availability
//	if !provider.IsAvailable(ctx) {
//	    log.Println("Provider not available")
//	    return
//	}
//
//	// Check health
//	health, err := provider.GetHealth(ctx)
//	if err != nil || !health.Healthy {
//	    log.Println("Provider unhealthy")
//	    return
//	}
//
//	// Get provider info
//	log.Printf("Using %s provider (%s)", provider.GetName(), provider.GetType())
//
//	// List available models
//	for _, model := range provider.GetModels() {
//	    log.Printf("Model: %s", model.Name)
//	}
//
//	// Generate response
//	request := &llm.LLMRequest{
//	    Prompt:      "Explain Go interfaces",
//	    MaxTokens:   1000,
//	    Temperature: 0.7,
//	}
//	response, err := provider.Generate(ctx, request)
//
// # Streaming Generation
//
// For streaming responses:
//
//	ch := make(chan llm.LLMResponse)
//	go func() {
//	    for chunk := range ch {
//	        fmt.Print(chunk.Content)
//	    }
//	}()
//
//	err := provider.GenerateStream(ctx, request, ch)
//
// # Cognee Integration
//
// Providers can support Cognee for enhanced knowledge graph capabilities:
//
//	if provider.SupportsCognee() {
//	    err := provider.InitializeCognee(config, options)
//	    if err != nil {
//	        log.Printf("Cognee initialization failed: %v", err)
//	    }
//	}
//
// # Hardware Profile
//
// Local providers can report hardware capabilities:
//
//	profile := provider.GetHardwareProfile()
//	if profile != nil {
//	    log.Printf("GPU: %s", profile.GPU)
//	    log.Printf("VRAM: %d GB", profile.VRAM/(1024*1024*1024))
//	}
//
// # Provider Type String
//
// Convert provider type to string:
//
//	providerType := provider.ProviderTypeOpenAI
//	log.Printf("Type: %s", providerType.String()) // "openai"
//
// # Integration with LLM Package
//
// This interface is used by the llm package for:
//   - Provider selection strategies
//   - Automatic fallback when providers fail
//   - Health monitoring and availability checks
//   - Load balancing across multiple providers
//
// See internal/llm for provider management and selection logic.
package provider
