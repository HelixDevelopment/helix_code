package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ErrProjectNotFound is returned by manager / DB-manager lookups when
// the requested project doesn't exist. Handlers MUST errors.Is-check
// this sentinel and return 404 Not Found — never 500 (CONST-035: 500
// lies about a missing-resource client error being a server fault).
var ErrProjectNotFound = errors.New("project not found")

// Project represents a development project
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Path        string    `json:"path"`
	Type        string    `json:"type"` // "go", "node", "python", "rust", etc.
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Metadata    Metadata  `json:"metadata"`
	Active      bool      `json:"active"`
}

// Metadata contains project-specific configuration
type Metadata struct {
	BuildCommand    string            `json:"build_command"`
	TestCommand     string            `json:"test_command"`
	LintCommand     string            `json:"lint_command"`
	Dependencies    []string          `json:"dependencies"`
	Environment     map[string]string `json:"environment"`
	Framework       string            `json:"framework"`
	LanguageVersion string            `json:"language_version"`
}

// Manager handles project lifecycle and operations
type Manager struct {
	mu            sync.RWMutex
	projects      map[string]*Project
	activeProject *Project
}

// NewManager creates a new project manager
func NewManager() *Manager {
	return &Manager{
		projects: make(map[string]*Project),
	}
}

// CreateProject creates a new project
func (m *Manager) CreateProject(ctx context.Context, name, description, path, projectType string) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate project path
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("project path does not exist: %s", path)
	}

	// Generate unique ID
	id := generateProjectID(name)

	project := &Project{
		ID:          id,
		Name:        name,
		Description: description,
		Path:        path,
		Type:        projectType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata: Metadata{
			Environment: make(map[string]string),
		},
		Active: false,
	}

	// Detect project type and set appropriate metadata
	if err := m.detectProjectType(project); err != nil {
		return nil, fmt.Errorf("failed to detect project type: %v", err)
	}

	m.projects[id] = project
	return project, nil
}

// GetProject retrieves a project by ID
func (m *Manager) GetProject(ctx context.Context, id string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check cache first
	if project, exists := m.projects[id]; exists {
		return project, nil
	}

	// Check if project exists in memory
	project, exists := m.projects[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
	}

	return project, nil
}

// ListProjects returns all projects for a user (ownerID ignored for in-memory manager)
func (m *Manager) ListProjects(ctx context.Context, ownerID string) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return all projects from memory
	var projects []*Project
	for _, project := range m.projects {
		projects = append(projects, project)
	}

	return projects, nil
}

// SetActiveProject sets the currently active project
func (m *Manager) SetActiveProject(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Look up project directly to avoid deadlock
	project, exists := m.projects[id]
	if !exists {
		return fmt.Errorf("%w: %s", ErrProjectNotFound, id)
	}

	// Deactivate previous active project
	if m.activeProject != nil {
		m.activeProject.Active = false
	}

	// Activate new project
	project.Active = true
	project.UpdatedAt = time.Now()
	m.activeProject = project

	return nil
}

// GetActiveProject returns the currently active project
func (m *Manager) GetActiveProject(ctx context.Context) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.activeProject != nil {
		return m.activeProject, nil
	}

	// Try to find active project in memory
	for _, project := range m.projects {
		if project.Active {
			m.activeProject = project
			return project, nil
		}
	}

	return nil, fmt.Errorf("no active project found")
}

// CreateProjectWithUser creates a new project with user ID (for compatibility with DatabaseManager)
func (m *Manager) CreateProjectWithUser(ctx context.Context, name, description, path, projectType, userID string) (*Project, error) {
	return m.CreateProject(ctx, name, description, path, projectType)
}

// detectProjectType automatically detects project type and sets appropriate metadata
func (m *Manager) detectProjectType(project *Project) error {
	path := project.Path

	// Check for Go project
	if _, err := os.Stat(filepath.Join(path, "go.mod")); err == nil {
		project.Type = "go"
		project.Metadata.BuildCommand = "go build"
		project.Metadata.TestCommand = "go test ./..."
		project.Metadata.LintCommand = "gofmt -l ."
		return nil
	}

	// Check for Node.js project
	if _, err := os.Stat(filepath.Join(path, "package.json")); err == nil {
		project.Type = "node"
		project.Metadata.BuildCommand = "npm run build"
		project.Metadata.TestCommand = "npm test"
		project.Metadata.LintCommand = "npm run lint"
		return nil
	}

	// Check for Python project
	if _, err := os.Stat(filepath.Join(path, "requirements.txt")); err == nil {
		project.Type = "python"
		project.Metadata.BuildCommand = "python setup.py build"
		project.Metadata.TestCommand = "python -m pytest"
		project.Metadata.LintCommand = "flake8 ."
		return nil
	}

	// Check for Rust project
	if _, err := os.Stat(filepath.Join(path, "Cargo.toml")); err == nil {
		project.Type = "rust"
		project.Metadata.BuildCommand = "cargo build"
		project.Metadata.TestCommand = "cargo test"
		project.Metadata.LintCommand = "cargo clippy"
		return nil
	}

	// Default to generic project
	project.Type = "generic"
	return nil
}

// UpdateProject updates project name and description in memory
func (m *Manager) UpdateProject(ctx context.Context, projectID, name, description string) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	if name != "" {
		project.Name = name
	}
	if description != "" {
		project.Description = description
	}
	project.UpdatedAt = time.Now()

	return project, nil
}

// UpdateProjectMetadata updates project metadata in memory
func (m *Manager) UpdateProjectMetadata(ctx context.Context, projectID string, metadata Metadata) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	project.Metadata = metadata
	project.UpdatedAt = time.Now()

	return nil
}

// DeleteProject removes a project from memory
func (m *Manager) DeleteProject(ctx context.Context, projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[projectID]; !exists {
		return fmt.Errorf("%w: %s", ErrProjectNotFound, projectID)
	}

	// Clear activeProject if we're deleting the active project
	if m.activeProject != nil && m.activeProject.ID == projectID {
		m.activeProject = nil
	}

	delete(m.projects, projectID)
	return nil
}

// generateProjectID creates a unique project ID
func generateProjectID(name string) string {
	return fmt.Sprintf("proj_%s_%d", name, time.Now().UnixNano())
}
