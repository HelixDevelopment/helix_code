// Package providers implements vector database and memory storage backends.
//
// Long-term memory enables agents to remember context across sessions.
// This package provides a unified VectorProvider interface and implementations
// for various vector databases and memory services.
//
// # Supported Providers
//
// Vector Databases:
//   - Pinecone: Cloud-native vector database
//   - Weaviate: Open-source vector search engine
//   - Qdrant: High-performance vector similarity search
//   - ChromaDB: AI-native embedding database
//   - FAISS: Facebook AI similarity search
//   - Milvus: Cloud-native vector database
//
// Memory Services:
//   - Mem0: AI memory layer
//   - Zep: Long-term memory for AI assistants
//   - Memonto: Semantic memory service
//   - BaseAI: Base AI memory provider
//
// AI Integration:
//   - Anima: Conversational AI memory
//   - CharacterAI: Character-based memory
//
// # VectorProvider Interface
//
// All providers implement the VectorProvider interface:
//
//	provider.Store(ctx, vectors)        // Store embeddings
//	provider.Search(ctx, query)         // Similarity search
//	provider.CreateCollection(ctx, name, cfg)  // Manage collections
//	provider.Health(ctx)                // Check health status
//
// # Factory Pattern
//
// Create providers through the factory:
//
//	provider, err := providers.NewProvider(providers.ProviderTypePinecone, config)
//	if err != nil {
//	    return err
//	}
//	provider.Initialize(ctx, providerConfig)
//	provider.Start(ctx)
//
// # Provider Registry
//
// Register and manage multiple providers:
//
//	registry := providers.NewRegistry()
//	registry.Register("primary", pineconeProvider)
//	registry.Register("backup", weaviateProvider)
//	provider := registry.Get("primary")
//
// # Load Balancing
//
// The package supports load balancing across providers:
//   - LoadBalanceRoundRobin: Distribute requests evenly
//   - LoadBalanceWeighted: Distribute by configured weights
//   - LoadBalanceRandom: Random selection
package providers
