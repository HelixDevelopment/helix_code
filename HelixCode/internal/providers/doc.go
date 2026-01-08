// Package providers implements AI and vector database integrations for HelixCode.
//
// This package provides a unified integration layer for AI providers, vector
// databases, memory systems, and conversation management. It orchestrates
// multiple AI backends and provides high-level APIs for text generation,
// embeddings, chat, classification, and entity extraction.
//
// # Architecture
//
// The package is built around several integration components:
//
//   - AIIntegration: Central hub for AI provider management
//   - VectorIntegration: Vector database operations and similarity search
//   - MemoryIntegration: Conversation and generation history storage
//   - ConversationManager: Multi-turn conversation handling with compression
//   - PersonalityManager: AI personality profiles and customization
//
// # AI Integration
//
// The AIIntegration provides unified access to multiple AI providers:
//
//	config := &providers.AIConfig{
//	    DefaultLLM:    "openai",
//	    DefaultMemory: "memgpt",
//	    Providers: map[string]*providers.AIProviderConfig{
//	        "openai": {
//	            Type:        providers.ProviderTypeOpenAI,
//	            Enabled:     true,
//	            Model:       "gpt-4",
//	            MaxTokens:   4096,
//	            Temperature: 0.7,
//	        },
//	    },
//	    CacheEnabled: true,
//	    CacheSize:    1000,
//	}
//
//	ai := providers.NewAIIntegration(config)
//	err := ai.Initialize(ctx)
//
// # Text Generation
//
// Generate text using the configured providers:
//
//	result, err := ai.GenerateText(ctx, "Write a haiku about coding", &providers.GenerationOptions{
//	    MaxTokens:   100,
//	    Temperature: 0.8,
//	})
//	fmt.Println(result.Text)
//
// Or use a specific provider:
//
//	result, err := ai.GenerateTextWithProvider(ctx, "anthropic", prompt, options)
//
// # Chat Generation
//
// Multi-turn conversations:
//
//	messages := []*providers.ChatMessage{
//	    {Role: "system", Content: "You are a helpful assistant."},
//	    {Role: "user", Content: "What is Go?"},
//	}
//
//	result, err := ai.GenerateChat(ctx, messages, &providers.ChatOptions{
//	    MaxTokens:   1000,
//	    Temperature: 0.7,
//	})
//	fmt.Println(result.Message.Content)
//
// # Embeddings
//
// Generate vector embeddings for semantic search:
//
//	embedding, err := ai.GenerateEmbedding(ctx, "Hello, world!")
//	// Returns []float64 vector
//
// # Text Classification
//
// Classify text into categories:
//
//	result, err := ai.ClassifyText(ctx, "This is great!", []string{"positive", "negative", "neutral"})
//	fmt.Printf("Category: %s (%.2f confidence)", result.Category, result.Confidence)
//
// # Entity Extraction
//
// Extract named entities from text:
//
//	entities, err := ai.ExtractEntities(ctx, "John works at Google in Mountain View")
//	for _, entity := range entities {
//	    fmt.Printf("%s: %s\n", entity.Type, entity.Text)
//	}
//
// # Vector Integration
//
// Store and search vector embeddings:
//
//	vectorIntegration := ai.GetVector()
//
//	// Store a vector
//	err := vectorIntegration.StoreVector(ctx, &providers.VectorData{
//	    ID:        "doc1",
//	    Embedding: embedding,
//	    Metadata:  map[string]interface{}{"source": "document.txt"},
//	    IndexName: "documents",
//	})
//
//	// Search for similar vectors
//	results, err := vectorIntegration.FindSimilarVectors(ctx, queryEmbedding, 10, nil)
//
//	// Create an index
//	err = vectorIntegration.CreateVectorIndex(ctx, "my_index", &providers.VectorIndexConfig{
//	    Dimension: 1536,
//	    Metric:    "cosine",
//	})
//
// # Memory Integration
//
// Track generations and conversations:
//
//	memory := ai.GetMemory()
//	stats, err := memory.GetMemoryStats(ctx)
//	fmt.Printf("Total generations: %d\n", stats.TotalGenerations)
//
// # Conversation Management
//
// Manage multi-turn conversations with automatic compression:
//
//	convMgr := ai.GetConversation()
//
//	conv, err := convMgr.CreateConversation(ctx, "conv-123")
//
//	err = convMgr.AddMessage(ctx, "conv-123", &providers.ChatMessage{
//	    Role:    "user",
//	    Content: "Hello!",
//	    Tokens:  5,
//	})
//
//	conv, err = convMgr.GetConversation(ctx, "conv-123")
//
// # Personality Management
//
// Configure AI personalities for different use cases:
//
//	personalityMgr := ai.GetPersonality()
//
//	// Get available personalities
//	personalities := personalityMgr.ListPersonalities()
//
//	// Switch personality
//	err := personalityMgr.SetActivePersonality("technical")
//
//	// Get current personality
//	active := personalityMgr.GetActivePersonality()
//
//	// Add custom personality
//	err = personalityMgr.AddPersonality(&providers.Personality{
//	    ID:           "custom",
//	    Name:         "Custom Assistant",
//	    SystemPrompt: "You are a specialized assistant.",
//	    Temperature:  0.5,
//	    Enabled:      true,
//	})
//
// Default personalities include:
//   - default: General-purpose helpful assistant
//   - technical: Technical expert for development tasks
//   - creative: Creative and imaginative responses
//
// # Health Checks
//
// Monitor AI provider health:
//
//	health, err := ai.HealthCheck(ctx)
//	fmt.Printf("Status: %s (%d/%d healthy)\n",
//	    health.Status, health.HealthyProviders, health.TotalProviders)
//
// # Statistics
//
// Get usage statistics:
//
//	stats, err := ai.GetStats(ctx)
//	for name, providerStats := range stats.Providers {
//	    fmt.Printf("%s: %d requests, %.4f cost\n",
//	        name, providerStats.Requests, providerStats.TotalCost)
//	}
//
// # Supported AI Providers
//
// The package supports multiple AI provider types:
//
//	ProviderTypeOpenAI      - OpenAI API
//	ProviderTypeAnthropic   - Anthropic Claude
//	ProviderTypeCohere      - Cohere API
//	ProviderTypeHuggingFace - Hugging Face models
//	ProviderTypeMistral     - Mistral AI
//	ProviderTypeGemini      - Google Gemini
//	ProviderTypeGemma       - Google Gemma
//	ProviderTypeLlamaIndex  - LlamaIndex integration
//	ProviderTypeMemGPT      - MemGPT for long-term memory
//	ProviderTypeCrewAI      - CrewAI multi-agent
//	ProviderTypeCharacterAI - Character.AI
//	ProviderTypeReplika     - Replika AI
//	ProviderTypeAnima       - Anima AI
//
// # Cleanup
//
// Properly shut down the integration:
//
//	err := ai.Stop(ctx)
//
// This stops all sub-components (vector, memory, conversation, personality).
//
// # Thread Safety
//
// All components use read-write mutexes for thread-safe operation:
//   - Multiple concurrent reads are allowed
//   - Writes are exclusive
//   - Provider access is synchronized
package providers
