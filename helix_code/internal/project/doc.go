// Package project provides project lifecycle management for HelixCode.
//
// This package implements project management functionality that handles
// creation, configuration, and tracking of development projects. It supports
// both in-memory and database-backed storage, automatic project type detection,
// and configurable build/test/lint commands.
//
// # Architecture
//
// The project system provides two manager implementations:
//
//   - Manager: In-memory project storage for testing and simple deployments
//   - DatabaseManager: PostgreSQL-backed storage for production use
//
// Both implement the same core operations for creating, retrieving,
// and managing projects.
//
// # Project Structure
//
// Projects contain comprehensive metadata:
//
//	type Project struct {
//	    ID          string    // Unique identifier
//	    Name        string    // Human-readable name
//	    Description string    // Project description
//	    Path        string    // Filesystem path
//	    Type        string    // Project type (go, node, python, rust, generic)
//	    CreatedAt   time.Time // Creation timestamp
//	    UpdatedAt   time.Time // Last update timestamp
//	    Metadata    Metadata  // Build configuration and settings
//	    Active      bool      // Currently active project flag
//	}
//
// # Project Metadata
//
// Metadata contains project-specific configuration:
//
//	type Metadata struct {
//	    BuildCommand    string            // Command to build the project
//	    TestCommand     string            // Command to run tests
//	    LintCommand     string            // Command to run linting
//	    Dependencies    []string          // List of dependencies
//	    Environment     map[string]string // Environment variables
//	    Framework       string            // Framework name (if applicable)
//	    LanguageVersion string            // Language version
//	}
//
// # In-Memory Manager Usage
//
// For simple deployments or testing:
//
//	manager := project.NewManager()
//
//	// Create a project
//	proj, err := manager.CreateProject(ctx, "myapp", "My Application", "/path/to/project", "go")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Set as active project
//	err = manager.SetActiveProject(ctx, proj.ID)
//
//	// Get active project
//	active, err := manager.GetActiveProject(ctx)
//
//	// List all projects
//	projects, err := manager.ListProjects(ctx, "")
//
// # Database Manager Usage
//
// For production with PostgreSQL persistence:
//
//	dbManager := project.NewDatabaseManager(db)
//
//	// Create a project with user ownership
//	proj, err := dbManager.CreateProjectWithUser(ctx, "myapp", "My Application", "/path/to/project", "go", userID)
//
//	// Get project by ID
//	proj, err = dbManager.GetProject(ctx, projectID)
//
//	// List user's projects
//	projects, err := dbManager.ListProjects(ctx, userID)
//
// # Automatic Project Type Detection
//
// Projects are automatically categorized based on configuration files:
//
//	go.mod           -> type: "go"
//	package.json     -> type: "node"
//	requirements.txt -> type: "python"
//	Cargo.toml       -> type: "rust"
//	(default)        -> type: "generic"
//
// Default commands are set based on detected type:
//
//	Go:
//	    Build: go build
//	    Test:  go test ./...
//	    Lint:  gofmt -l .
//
//	Node.js:
//	    Build: npm run build
//	    Test:  npm test
//	    Lint:  npm run lint
//
//	Python:
//	    Build: python setup.py build
//	    Test:  python -m pytest
//	    Lint:  flake8 .
//
//	Rust:
//	    Build: cargo build
//	    Test:  cargo test
//	    Lint:  cargo clippy
//
// # Active Project
//
// Only one project can be active at a time:
//
//	// Set active project
//	err := manager.SetActiveProject(ctx, projectID)
//
//	// Get current active project
//	active, err := manager.GetActiveProject(ctx)
//	if err != nil {
//	    // No active project
//	}
//
// When setting a new active project, the previous active project
// is automatically deactivated.
//
// # Updating Metadata
//
// Project configuration can be updated:
//
//	metadata := project.Metadata{
//	    BuildCommand: "make build",
//	    TestCommand:  "make test",
//	    LintCommand:  "make lint",
//	    Environment: map[string]string{
//	        "CGO_ENABLED": "0",
//	    },
//	}
//	err := manager.UpdateProjectMetadata(ctx, projectID, metadata)
//
// # Deleting Projects
//
// Remove projects from management:
//
//	err := manager.DeleteProject(ctx, projectID)
//
// For the database manager, deletion is a soft delete (status = 'deleted').
// For the in-memory manager, the project is removed completely.
//
// # Database Schema
//
// The DatabaseManager expects the following schema:
//
//	CREATE TABLE projects (
//	    id UUID PRIMARY KEY,
//	    name VARCHAR(255) NOT NULL,
//	    description TEXT,
//	    owner_id UUID REFERENCES users(id),
//	    workspace_path TEXT NOT NULL,
//	    config JSONB,
//	    status VARCHAR(50) DEFAULT 'active',
//	    created_at TIMESTAMP DEFAULT NOW(),
//	    updated_at TIMESTAMP DEFAULT NOW()
//	);
//
// # Thread Safety
//
// The in-memory Manager uses read-write mutex for thread safety:
//   - Multiple concurrent reads are allowed
//   - Writes are exclusive
//   - Active project state is synchronized
//
// The DatabaseManager delegates thread safety to the database layer.
//
// # ID Generation
//
// Project IDs are generated differently by each manager:
//   - Manager: "proj_{name}_{timestamp_nano}"
//   - DatabaseManager: UUID v4
package project
