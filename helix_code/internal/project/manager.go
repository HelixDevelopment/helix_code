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

// ErrInvalidProjectID is returned by DB-manager paths when uuid.Parse
// fails on a caller-supplied project id (project_id column is UUID
// in the projects schema). Client input error → 400 Bad Request.
var ErrInvalidProjectID = errors.New("invalid project ID")

// ErrInvalidOwnerID is returned when uuid.Parse fails on the
// authenticated user's owner-id during project CRUD (only happens
// if context-user is corrupted; surface as 400 to the caller).
var ErrInvalidOwnerID = errors.New("invalid owner ID")

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
		return nil, errors.New(tr(ctx, "internal_project_path_does_not_exist", map[string]any{"Path": path}))
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
		return nil, fmt.Errorf("%s: %w", tr(ctx, "internal_project_detect_type_failed", nil), err)
	}

	m.projects[id] = project
	// Returns the live pointer by design: CreateProject hands the creating caller
	// a usable handle to the just-created project (existing callers observe later
	// state through it). The creating caller is the sole id-holder at this point,
	// so there is no concurrent writer to race. The read methods (GetProject/
	// ListProjects/GetActiveProject) and UpdateProject snapshot because they hand
	// the pointer to arbitrary later callers while concurrent writers exist.
	return project, nil
}

// GetProject retrieves a project by ID.
//
// HXC §11.4.85 race fix: returns a DEEP COPY snapshot, never the live
// map-stored pointer. The manager mutates the stored *Project's fields
// (Active / UpdatedAt / Name / …) under the write Lock in
// SetActiveProject / UpdateProject / SetActiveProject etc.; handing the
// live pointer back to a caller that reads those fields outside the lock
// is a data race (caught by `go test -race`). The snapshot decouples the
// caller's read from the manager's serialised writes. Callers consume the
// result as read-only data (HTTP serialisation, UI display) and never rely
// on pointer identity, so the copy is contract-preserving.
func (m *Manager) GetProject(ctx context.Context, id string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[id]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
	}

	return copyProject(project), nil
}

// ListProjects returns all projects for a user (ownerID ignored for in-memory manager)
func (m *Manager) ListProjects(ctx context.Context, ownerID string) ([]*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return DEEP-COPY snapshots from memory (see GetProject — handing live
	// map-stored pointers to callers that read fields outside the lock while
	// the manager mutates them under the write Lock is a data race).
	var projects []*Project
	for _, project := range m.projects {
		projects = append(projects, copyProject(project))
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

// GetActiveProject returns the currently active project.
//
// The fast path (cached m.activeProject set) reads under a shared RLock. The
// lazy-scan path MUTATES m.activeProject, so it MUST run under the exclusive
// write Lock — assigning a shared field under a read lock is a data race when
// two readers reach it concurrently (sync.RWMutex permits parallel RLock
// holders). We take RLock for the fast read, drop it, then re-acquire the write
// Lock and re-check before scanning so the lazy assignment is properly
// serialised against SetActiveProject / DeleteProject and against other
// GetActiveProject callers.
func (m *Manager) GetActiveProject(ctx context.Context) (*Project, error) {
	m.mu.RLock()
	if m.activeProject != nil {
		// Snapshot under the read lock — never hand the live map-stored
		// pointer back (its fields are mutated under the write Lock).
		ap := copyProject(m.activeProject)
		m.mu.RUnlock()
		return ap, nil
	}
	m.mu.RUnlock()

	// Lazy-scan path mutates shared state — take the exclusive write lock.
	m.mu.Lock()
	defer m.mu.Unlock()

	// Re-check under the write lock: another goroutine may have set the active
	// project (or it may have been set since we dropped the read lock).
	if m.activeProject != nil {
		return copyProject(m.activeProject), nil
	}

	// Try to find active project in memory
	for _, project := range m.projects {
		if project.Active {
			m.activeProject = project
			return copyProject(project), nil
		}
	}

	return nil, errors.New(tr(ctx, "internal_project_no_active_project", nil))
}

// copyProject returns a deep-copy snapshot of p so callers never share
// mutable state with the manager's internal store (HXC §11.4.85 race fix).
// Metadata carries a slice (Dependencies) and a map (Environment) which are
// deep-copied so a caller mutating either cannot corrupt the stored project,
// and a concurrent writer to the stored project cannot race a caller's read.
func copyProject(p *Project) *Project {
	if p == nil {
		return nil
	}
	cp := *p // shallow copy of value fields

	if p.Metadata.Dependencies != nil {
		deps := make([]string, len(p.Metadata.Dependencies))
		copy(deps, p.Metadata.Dependencies)
		cp.Metadata.Dependencies = deps
	}
	if p.Metadata.Environment != nil {
		env := make(map[string]string, len(p.Metadata.Environment))
		for k, v := range p.Metadata.Environment {
			env[k] = v
		}
		cp.Metadata.Environment = env
	}
	return &cp
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

	// Return a snapshot, never the live map-stored pointer: a concurrent
	// SetActiveProject/UpdateProject would otherwise mutate the same struct the
	// caller is reading (same data-race class fixed for the read methods).
	return copyProject(project), nil
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
