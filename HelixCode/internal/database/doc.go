// Package database provides PostgreSQL database connectivity and schema management
// for HelixCode.
//
// # Overview
//
// The database package implements connection pooling using pgx/v5, automatic schema
// initialization, and a clean interface for database operations. It serves as the
// persistence layer for all HelixCode data.
//
// # Architecture
//
// The package is organized around these core components:
//
//   - Database: Main database struct wrapping pgxpool.Pool
//   - DatabaseInterface: Interface for database operations (enables mocking)
//   - Config: Database configuration struct
//   - Schema: Complete DDL for all tables, indexes, and extensions
//
// # Connection Pooling
//
// The package uses pgx connection pooling with sensible defaults:
//
//   - MaxConns: 20 connections
//   - MinConns: 5 connections
//   - MaxConnLifetime: 1 hour
//   - MaxConnIdleTime: 30 minutes
//
// # Basic Usage
//
// Creating a database connection:
//
//	config := database.Config{
//	    Host:     "localhost",
//	    Port:     5432,
//	    User:     "helixcode",
//	    Password: "secret",
//	    DBName:   "helixcode",
//	    SSLMode:  "disable",
//	}
//
//	db, err := database.New(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer db.Close()
//
// # Schema Initialization
//
// Initialize the database schema automatically:
//
//	err := db.InitializeSchema()
//	if err != nil {
//	    log.Fatal(err)
//	}
//
// The schema initialization is idempotent - it checks for existing tables
// and only creates them if they do not exist. It also handles schema migrations
// for adding new columns to existing tables.
//
// # Executing Queries
//
// Execute queries using pgx semantics:
//
//	// Execute without returning rows (INSERT, UPDATE, DELETE)
//	tag, err := db.Exec(ctx, "UPDATE users SET active = $1 WHERE id = $2", true, userID)
//	rowsAffected := tag.RowsAffected()
//
//	// Query returning multiple rows
//	rows, err := db.Query(ctx, "SELECT id, username FROM users WHERE active = $1", true)
//	defer rows.Close()
//	for rows.Next() {
//	    var id uuid.UUID
//	    var username string
//	    err := rows.Scan(&id, &username)
//	    // handle row
//	}
//
//	// Query returning single row
//	var username string
//	err := db.QueryRow(ctx, "SELECT username FROM users WHERE id = $1", userID).Scan(&username)
//
// # Health Checks
//
// Check database connectivity:
//
//	// Simple ping
//	err := db.Ping(ctx)
//
//	// Health check with timeout
//	err := db.HealthCheck()
//
// # Standard Library Compatibility
//
// Get a standard *sql.DB for compatibility with other libraries:
//
//	sqlDB, err := db.GetDB()
//	// Use with libraries that require *sql.DB
//
// # Database Schema
//
// The package manages these database tables:
//
// Users & Authentication:
//   - users: User accounts with credentials
//   - user_sessions: Active user sessions
//
// Workers & Distributed Computing:
//   - workers: Registered workers with capabilities
//   - worker_metrics: Resource usage metrics
//   - worker_connectivity_events: Connection state changes
//
// Tasks:
//   - distributed_tasks: Task queue with priorities and dependencies
//   - task_checkpoints: Work preservation checkpoints
//
// Projects & Sessions:
//   - projects: User projects
//   - sessions: Development sessions (planning, building, testing, etc.)
//
// LLM Providers:
//   - llm_providers: Configured LLM providers
//   - llm_models: Available models per provider
//
// MCP Integration:
//   - mcp_servers: Model Context Protocol servers
//   - tools: Available tools from MCP servers
//
// Notifications & Audit:
//   - notifications: User notifications
//   - audit_logs: System audit trail
//
// # Required PostgreSQL Extensions
//
// The schema requires these PostgreSQL extensions:
//
//   - uuid-ossp: UUID generation with uuid_generate_v4()
//   - pgcrypto: Cryptographic functions
//
// # Database Interface
//
// The DatabaseInterface enables testing with mocks:
//
//	type DatabaseInterface interface {
//	    Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
//	    Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
//	    QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
//	    Ping(ctx context.Context) error
//	    Close()
//	}
//
// Create mock implementations for testing:
//
//	type MockDatabase struct {
//	    ExecFunc     func(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
//	    QueryFunc    func(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
//	    QueryRowFunc func(ctx context.Context, sql string, args ...interface{}) pgx.Row
//	    PingFunc     func(ctx context.Context) error
//	}
//
// # Configuration
//
// Database configuration via Config struct:
//
//	type Config struct {
//	    Host     string // Database host (default: localhost)
//	    Port     int    // Database port (default: 5432)
//	    User     string // Database user
//	    Password string // Database password
//	    DBName   string // Database name
//	    SSLMode  string // SSL mode (disable, require, verify-ca, verify-full)
//	}
//
// Configuration can also be provided via environment variables:
//
//	HELIX_DATABASE_HOST     // Override Host
//	HELIX_DATABASE_PORT     // Override Port
//	HELIX_DATABASE_USER     // Override User
//	HELIX_DATABASE_PASSWORD // Override Password
//	HELIX_DATABASE_NAME     // Override DBName
//
// # Connection String
//
// The package builds a connection string from Config:
//
//	host=localhost port=5432 user=helixcode password=secret dbname=helixcode sslmode=disable
//
// # Error Handling
//
// The package returns standard pgx errors. Common patterns:
//
//	import "github.com/jackc/pgx/v5"
//
//	row := db.QueryRow(ctx, "SELECT ...")
//	err := row.Scan(&val)
//	if errors.Is(err, pgx.ErrNoRows) {
//	    // Handle not found
//	}
//
// # Thread Safety
//
// The Database type is safe for concurrent use. The underlying pgxpool.Pool
// handles connection multiplexing automatically.
//
// # Best Practices
//
//  1. Always use context with appropriate timeouts
//  2. Close Rows iterators after use
//  3. Use transactions for multi-statement operations
//  4. Call Close() on shutdown to release connections
//  5. Use HealthCheck() for load balancer probes
//  6. Initialize schema on application startup
//
// # Related Packages
//
//   - internal/config: Database configuration loading
//   - internal/auth: User authentication using database
//   - internal/worker: Worker registration and metrics
//   - internal/task: Task queue persistence
package database
