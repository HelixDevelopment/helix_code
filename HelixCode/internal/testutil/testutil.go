// Package testutil provides testing utilities for HelixCode
package testutil

import (
	"os"
	"strconv"
	"testing"

	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/redis"
)

// TestInfrastructureAvailable returns true if full test infrastructure is available
func TestInfrastructureAvailable() bool {
	return os.Getenv("HELIX_TEST_INFRA") == "true"
}

// DatabaseAvailable returns true if test database is available
func DatabaseAvailable() bool {
	return os.Getenv("HELIX_TEST_DATABASE_HOST") != "" || os.Getenv("HELIX_DATABASE_HOST") != ""
}

// RedisAvailable returns true if test Redis is available
func RedisAvailable() bool {
	return os.Getenv("HELIX_TEST_REDIS_HOST") != "" || os.Getenv("HELIX_REDIS_HOST") != ""
}

// OllamaAvailable returns true if Ollama is available for testing
func OllamaAvailable() bool {
	return os.Getenv("HELIX_TEST_OLLAMA_URL") != "" || os.Getenv("OLLAMA_HOST") != ""
}

// MockLLMAvailable returns true if mock LLM server is available
func MockLLMAvailable() bool {
	return os.Getenv("HELIX_TEST_MOCK_LLM_URL") != ""
}

// SSHServerAvailable returns true if SSH test server is available
func SSHServerAvailable() bool {
	return os.Getenv("HELIX_TEST_SSH_HOST") != ""
}

// BrowserAvailable returns true if browser automation (Selenium/ChromeDP) is available
func BrowserAvailable() bool {
	return os.Getenv("HELIX_TEST_SELENIUM_URL") != "" || os.Getenv("HELIX_TEST_CHROMEDP_URL") != ""
}

// CogneeAvailable returns true if Cognee service is available
func CogneeAvailable() bool {
	return os.Getenv("HELIX_TEST_COGNEE_URL") != ""
}

// VectorDBAvailable returns true if vector database is available
func VectorDBAvailable() bool {
	return os.Getenv("HELIX_TEST_WEAVIATE_URL") != "" ||
		os.Getenv("HELIX_TEST_CHROMADB_URL") != "" ||
		os.Getenv("HELIX_TEST_QDRANT_URL") != ""
}

// SkipIfNoInfrastructure skips the test if full test infrastructure is not available
func SkipIfNoInfrastructure(t *testing.T) {
	t.Helper()
	if !TestInfrastructureAvailable() {
		t.Skip("Test infrastructure not available (set HELIX_TEST_INFRA=true and run docker-compose.full-test.yml)")
	}
}

// SkipIfNoDatabase skips the test if database is not available
func SkipIfNoDatabase(t *testing.T) {
	t.Helper()
	if !DatabaseAvailable() {
		t.Skip("Database not available for testing (set HELIX_DATABASE_HOST or run test infrastructure)")
	}
}

// SkipIfNoRedis skips the test if Redis is not available
func SkipIfNoRedis(t *testing.T) {
	t.Helper()
	if !RedisAvailable() {
		t.Skip("Redis not available for testing (set HELIX_REDIS_HOST or run test infrastructure)")
	}
}

// SkipIfNoOllama skips the test if Ollama is not available
func SkipIfNoOllama(t *testing.T) {
	t.Helper()
	if !OllamaAvailable() {
		t.Skip("Ollama not available for testing (set OLLAMA_HOST or run test infrastructure)")
	}
}

// SkipIfNoMockLLM skips the test if mock LLM server is not available
func SkipIfNoMockLLM(t *testing.T) {
	t.Helper()
	if !MockLLMAvailable() {
		t.Skip("Mock LLM server not available (set HELIX_TEST_MOCK_LLM_URL or run test infrastructure)")
	}
}

// SkipIfNoSSH skips the test if SSH server is not available
func SkipIfNoSSH(t *testing.T) {
	t.Helper()
	if !SSHServerAvailable() {
		t.Skip("SSH server not available for testing (set HELIX_TEST_SSH_HOST or run test infrastructure)")
	}
}

// SkipIfNoBrowser skips the test if browser automation is not available
func SkipIfNoBrowser(t *testing.T) {
	t.Helper()
	if !BrowserAvailable() {
		t.Skip("Browser automation not available (set HELIX_TEST_SELENIUM_URL or run test infrastructure)")
	}
}

// SkipIfNoCognee skips the test if Cognee is not available
func SkipIfNoCognee(t *testing.T) {
	t.Helper()
	if !CogneeAvailable() {
		t.Skip("Cognee not available for testing (set HELIX_TEST_COGNEE_URL or run test infrastructure)")
	}
}

// SkipIfNoVectorDB skips the test if vector database is not available
func SkipIfNoVectorDB(t *testing.T) {
	t.Helper()
	if !VectorDBAvailable() {
		t.Skip("Vector database not available (set HELIX_TEST_*_URL or run test infrastructure)")
	}
}

// GetTestDatabaseConfig returns database configuration for testing
func GetTestDatabaseConfig() database.Config {
	host := os.Getenv("HELIX_DATABASE_HOST")
	if host == "" {
		host = os.Getenv("HELIX_TEST_DATABASE_HOST")
	}
	if host == "" {
		host = "localhost"
	}

	port := 5432
	if p := os.Getenv("HELIX_DATABASE_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	user := os.Getenv("HELIX_DATABASE_USER")
	if user == "" {
		user = "helixcode"
	}

	password := os.Getenv("HELIX_DATABASE_PASSWORD")
	if password == "" {
		password = "helixcode_test"
	}

	dbname := os.Getenv("HELIX_DATABASE_NAME")
	if dbname == "" {
		dbname = "helixcode_test"
	}

	return database.Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbname,
		SSLMode:  "disable",
	}
}

// GetTestRedisConfig returns Redis configuration for testing
func GetTestRedisConfig() *config.RedisConfig {
	host := os.Getenv("HELIX_REDIS_HOST")
	if host == "" {
		host = os.Getenv("HELIX_TEST_REDIS_HOST")
	}
	if host == "" {
		host = "localhost"
	}

	port := 6379
	if p := os.Getenv("HELIX_REDIS_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	password := os.Getenv("HELIX_REDIS_PASSWORD")

	return &config.RedisConfig{
		Enabled:  true,
		Host:     host,
		Port:     port,
		Password: password,
		Database: 0,
	}
}

// GetTestDatabase creates a test database connection
func GetTestDatabase(t *testing.T) *database.Database {
	t.Helper()
	SkipIfNoDatabase(t)

	cfg := GetTestDatabaseConfig()
	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Initialize schema
	if err := db.InitializeSchema(); err != nil {
		t.Fatalf("Failed to initialize database schema: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}

// GetTestRedis creates a test Redis connection
func GetTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	SkipIfNoRedis(t)

	cfg := GetTestRedisConfig()
	client, err := redis.NewClient(cfg)
	if err != nil {
		t.Fatalf("Failed to connect to test Redis: %v", err)
	}

	t.Cleanup(func() {
		client.Close()
	})

	return client
}

// GetOllamaURL returns the Ollama URL for testing
func GetOllamaURL() string {
	url := os.Getenv("HELIX_TEST_OLLAMA_URL")
	if url == "" {
		url = os.Getenv("OLLAMA_HOST")
	}
	if url == "" {
		url = "http://localhost:11434"
	}
	return url
}

// GetMockLLMURL returns the mock LLM server URL
func GetMockLLMURL() string {
	url := os.Getenv("HELIX_TEST_MOCK_LLM_URL")
	if url == "" {
		url = "http://localhost:8090"
	}
	return url
}

// GetSeleniumURL returns the Selenium URL for browser testing
func GetSeleniumURL() string {
	url := os.Getenv("HELIX_TEST_SELENIUM_URL")
	if url == "" {
		url = "http://localhost:4444"
	}
	return url
}

// GetChromeDPURL returns the ChromeDP URL for browser testing
func GetChromeDPURL() string {
	url := os.Getenv("HELIX_TEST_CHROMEDP_URL")
	if url == "" {
		url = "http://localhost:9222"
	}
	return url
}

// GetCogneeURL returns the Cognee URL for testing
func GetCogneeURL() string {
	url := os.Getenv("HELIX_TEST_COGNEE_URL")
	if url == "" {
		url = "http://localhost:8000"
	}
	return url
}

// GetSSHTestConfig returns SSH test server configuration
func GetSSHTestConfig() (host string, port int, user string, password string, keyPath string) {
	host = os.Getenv("HELIX_TEST_SSH_HOST")
	if host == "" {
		host = "localhost"
	}

	port = 2222
	if p := os.Getenv("HELIX_TEST_SSH_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	user = os.Getenv("HELIX_TEST_SSH_USER")
	if user == "" {
		user = "helixcode"
	}

	password = os.Getenv("HELIX_TEST_SSH_PASSWORD")
	if password == "" {
		password = "helixcode_test"
	}

	keyPath = os.Getenv("HELIX_TEST_SSH_KEY_PATH")

	return
}

// CleanupTestData cleans up test data from database
func CleanupTestData(t *testing.T, db *database.Database) {
	t.Helper()

	// Clean up test tables in reverse dependency order
	tables := []string{
		"checkpoints",
		"task_dependencies",
		"tasks",
		"sessions",
		"workers",
		"projects",
		"users",
	}

	for _, table := range tables {
		// Use TRUNCATE for faster cleanup, with CASCADE for foreign keys
		_, err := db.Pool.Exec(nil, "TRUNCATE TABLE "+table+" CASCADE")
		if err != nil {
			// Table might not exist, that's OK
			t.Logf("Note: Could not truncate %s: %v", table, err)
		}
	}
}
