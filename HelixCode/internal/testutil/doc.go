// Package testutil provides testing utilities for the HelixCode project.
//
// This package offers helper functions and utilities for testing various
// components of HelixCode, including database connections, Redis clients,
// and infrastructure availability detection.
//
// # Infrastructure Detection
//
// The package provides functions to detect available test infrastructure:
//
//	testutil.DatabaseAvailable()       // Check if PostgreSQL is available
//	testutil.RedisAvailable()          // Check if Redis is available
//	testutil.OllamaAvailable()         // Check if Ollama is available
//	testutil.MockLLMAvailable()        // Check if mock LLM server is available
//	testutil.SSHServerAvailable()      // Check if SSH test server is available
//	testutil.BrowserAvailable()        // Check if Selenium/ChromeDP is available
//	testutil.CogneeAvailable()         // Check if Cognee is available
//	testutil.VectorDBAvailable()       // Check if any vector DB is available
//
// # Skip Helpers
//
// Helper functions to skip tests when infrastructure is unavailable:
//
//	func TestDatabaseFeature(t *testing.T) {
//	    testutil.SkipIfNoDatabase(t)
//	    // Test proceeds only if database is available
//	}
//
//	func TestRedisFeature(t *testing.T) {
//	    testutil.SkipIfNoRedis(t)
//	    // Test proceeds only if Redis is available
//	}
//
// # Configuration Helpers
//
// Get test-specific configurations:
//
//	cfg := testutil.GetTestDatabaseConfig()
//	redisCfg := testutil.GetTestRedisConfig()
//	ollamaURL := testutil.GetOllamaURL()
//	sshHost, port, user, pass, key := testutil.GetSSHTestConfig()
//
// # Database and Redis Clients
//
// Create test database and Redis connections with automatic cleanup:
//
//	func TestWithDatabase(t *testing.T) {
//	    db := testutil.GetTestDatabase(t)
//	    // db is automatically closed when test completes
//	    // use db for testing...
//	}
//
//	func TestWithRedis(t *testing.T) {
//	    client := testutil.GetTestRedis(t)
//	    // client is automatically closed when test completes
//	    // use client for testing...
//	}
//
// # Environment Variables
//
// The package reads the following environment variables:
//
//	HELIX_TEST_INFRA        - Set to "true" to enable full test infrastructure
//	HELIX_DATABASE_HOST     - PostgreSQL host
//	HELIX_DATABASE_PORT     - PostgreSQL port (default: 5432)
//	HELIX_DATABASE_USER     - Database user (default: helixcode)
//	HELIX_DATABASE_PASSWORD - Database password (default: helixcode_test)
//	HELIX_DATABASE_NAME     - Database name (default: helixcode_test)
//	HELIX_REDIS_HOST        - Redis host
//	HELIX_REDIS_PORT        - Redis port (default: 6379)
//	HELIX_REDIS_PASSWORD    - Redis password
//	HELIX_TEST_OLLAMA_URL   - Ollama URL for testing
//	HELIX_TEST_MOCK_LLM_URL - Mock LLM server URL
//	HELIX_TEST_SSH_HOST     - SSH test server host
//	HELIX_TEST_SSH_PORT     - SSH test server port (default: 2222)
//	HELIX_TEST_SSH_USER     - SSH test user (default: helixcode)
//	HELIX_TEST_SSH_PASSWORD - SSH test password
//	HELIX_TEST_SELENIUM_URL - Selenium WebDriver URL
//	HELIX_TEST_CHROMEDP_URL - ChromeDP URL
//	HELIX_TEST_COGNEE_URL   - Cognee service URL
//	HELIX_TEST_WEAVIATE_URL - Weaviate vector DB URL
//	HELIX_TEST_CHROMADB_URL - ChromaDB URL
//	HELIX_TEST_QDRANT_URL   - Qdrant URL
//
// # Full Test Infrastructure
//
// To run tests with full infrastructure, start the test containers:
//
//	make test-infra-up    # Start all test containers
//	make test-full        # Run tests with full infrastructure
//	make test-infra-down  # Stop test containers
//
// The docker-compose.full-test.yml provides:
//   - PostgreSQL database
//   - Redis cache
//   - Ollama LLM server
//   - Mock LLM server (multiple providers)
//   - Selenium Chrome
//   - ChromeDP headless browser
//   - SSH test servers
//   - Cognee memory service
//   - Vector databases (Weaviate, ChromaDB, Qdrant)
//
// # Example Test
//
//	func TestProjectCreation(t *testing.T) {
//	    // Skip if database is not available
//	    testutil.SkipIfNoDatabase(t)
//
//	    // Get test database connection (auto-cleanup)
//	    db := testutil.GetTestDatabase(t)
//
//	    // Create manager and test
//	    manager := project.NewDatabaseManager(db)
//	    proj, err := manager.CreateProject(ctx, "test", "desc", "/path", "go")
//	    assert.NoError(t, err)
//	    assert.NotEmpty(t, proj.ID)
//	}
package testutil
