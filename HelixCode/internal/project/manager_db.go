package project

import (
	"context"
	"fmt"
	"time"

	"dev.helix.code/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// DatabaseManager handles project lifecycle and operations with database persistence
type DatabaseManager struct {
	db database.DatabaseInterface
}

// NewDatabaseManager creates a new project manager with database persistence
func NewDatabaseManager(db database.DatabaseInterface) *DatabaseManager {
	return &DatabaseManager{
		db: db,
	}
}

// CreateProjectWithUser creates a new project with database persistence
func (m *DatabaseManager) CreateProjectWithUser(ctx context.Context, name, description, path, projectType, ownerID string) (*Project, error) {
	ownerUUID, err := uuid.Parse(ownerID)
	if err != nil {
		return nil, fmt.Errorf("invalid owner ID: %v", err)
	}

	// Detect project type and metadata
	metadata := Metadata{
		Environment: make(map[string]string),
	}
	m.detectProjectType(path, projectType, &metadata)

	project := &Project{
		ID:          uuid.New().String(),
		Name:        name,
		Description: description,
		Path:        path,
		Type:        projectType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Metadata:    metadata,
		Active:      false,
	}

	// Insert into database
	query := `
		INSERT INTO projects (id, name, description, owner_id, workspace_path, config, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'active')
		RETURNING created_at, updated_at
	`

	config := map[string]interface{}{
		"type":     projectType,
		"metadata": metadata,
	}

	var createdAt, updatedAt time.Time
	err = m.db.QueryRow(ctx, query,
		project.ID, name, description, ownerUUID, path, config,
	).Scan(&createdAt, &updatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create project in database: %v", err)
	}

	project.CreatedAt = createdAt
	project.UpdatedAt = updatedAt

	return project, nil
}

// GetProject retrieves a project by ID from database
func (m *DatabaseManager) GetProject(ctx context.Context, id string) (*Project, error) {
	projectID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid project ID: %v", err)
	}

	query := `
		SELECT id, name, description, owner_id, workspace_path, config, status, created_at, updated_at
		FROM projects
		WHERE id = $1 AND status = 'active'
	`

	var (
		dbID          uuid.UUID
		name          string
		description   string
		ownerID       uuid.UUID
		workspacePath string
		config        map[string]interface{}
		status        string
		createdAt     time.Time
		updatedAt     time.Time
	)

	err = m.db.QueryRow(ctx, query, projectID).Scan(
		&dbID, &name, &description, &ownerID, &workspacePath, &config, &status, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("project not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get project from database: %v", err)
	}

	// Extract type and metadata from config
	projectType, _ := config["type"].(string)
	metadataMap, _ := config["metadata"].(map[string]interface{})
	metadata := m.convertToMetadata(metadataMap)

	project := &Project{
		ID:          dbID.String(),
		Name:        name,
		Description: description,
		Path:        workspacePath,
		Type:        projectType,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		Metadata:    metadata,
		Active:      false, // This would need to be tracked separately
	}

	return project, nil
}

// CreateProject creates a new project with database persistence (for compatibility with Manager interface)
func (m *DatabaseManager) CreateProject(ctx context.Context, name, description, path, projectType string) (*Project, error) {
	return m.CreateProjectWithUser(ctx, name, description, path, projectType, "default-user")
}

// detectProjectType detects project type and sets appropriate metadata
func (m *DatabaseManager) detectProjectType(path, projectType string, metadata *Metadata) {
	// This would implement actual project type detection
	// For now, use the provided type and set default commands

	switch projectType {
	case "go":
		metadata.BuildCommand = "go build"
		metadata.TestCommand = "go test ./..."
		metadata.LintCommand = "gofmt -l ."
	case "node":
		metadata.BuildCommand = "npm run build"
		metadata.TestCommand = "npm test"
		metadata.LintCommand = "npm run lint"
	case "python":
		metadata.BuildCommand = "python setup.py build"
		metadata.TestCommand = "python -m pytest"
		metadata.LintCommand = "flake8 ."
	case "rust":
		metadata.BuildCommand = "cargo build"
		metadata.TestCommand = "cargo test"
		metadata.LintCommand = "cargo clippy"
	default:
		metadata.BuildCommand = "echo 'No build command configured'"
		metadata.TestCommand = "echo 'No test command configured'"
		metadata.LintCommand = "echo 'No lint command configured'"
	}
}

// convertToMetadata converts map to Metadata struct
func (m *DatabaseManager) convertToMetadata(data map[string]interface{}) Metadata {
	metadata := Metadata{
		Environment: make(map[string]string),
	}

	if buildCmd, ok := data["build_command"].(string); ok {
		metadata.BuildCommand = buildCmd
	}
	if testCmd, ok := data["test_command"].(string); ok {
		metadata.TestCommand = testCmd
	}
	if lintCmd, ok := data["lint_command"].(string); ok {
		metadata.LintCommand = lintCmd
	}
	if deps, ok := data["dependencies"].([]string); ok {
		metadata.Dependencies = deps
	}
	if env, ok := data["environment"].(map[string]string); ok {
		metadata.Environment = env
	}
	if framework, ok := data["framework"].(string); ok {
		metadata.Framework = framework
	}
	if langVersion, ok := data["language_version"].(string); ok {
		metadata.LanguageVersion = langVersion
	}

	return metadata
}

// ListProjects returns all projects for a user from database
func (m *DatabaseManager) ListProjects(ctx context.Context, ownerID string) ([]*Project, error) {
	ownerUUID, err := uuid.Parse(ownerID)
	if err != nil {
		return nil, fmt.Errorf("invalid owner ID: %v", err)
	}

	query := `
		SELECT id, name, description, owner_id, workspace_path, config, status, created_at, updated_at
		FROM projects
		WHERE owner_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := m.db.Query(ctx, query, ownerUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %v", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		var (
			dbID          uuid.UUID
			name          string
			description   string
			ownerID       uuid.UUID
			workspacePath string
			config        map[string]interface{}
			status        string
			createdAt     time.Time
			updatedAt     time.Time
		)

		if err := rows.Scan(
			&dbID, &name, &description, &ownerID, &workspacePath, &config, &status, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project row: %v", err)
		}

		// Extract type and metadata from config
		projectType, _ := config["type"].(string)
		metadataMap, _ := config["metadata"].(map[string]interface{})
		metadata := m.convertToMetadata(metadataMap)

		project := &Project{
			ID:          dbID.String(),
			Name:        name,
			Description: description,
			Path:        workspacePath,
			Type:        projectType,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
			Metadata:    metadata,
			Active:      false,
		}

		projects = append(projects, project)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project rows: %v", err)
	}

	return projects, nil
}

// UpdateProjectMetadata updates project metadata in the database
func (m *DatabaseManager) UpdateProjectMetadata(ctx context.Context, projectID string, metadata Metadata) error {
	query := `
		UPDATE projects
		SET config = $1, updated_at = $2
		WHERE id = $3
	`

	_, err := m.db.Exec(ctx, query, metadata, time.Now(), projectID)
	if err != nil {
		return fmt.Errorf("failed to update project metadata: %v", err)
	}

	return nil
}

// DeleteProject marks a project as deleted in the database
func (m *DatabaseManager) DeleteProject(ctx context.Context, projectID string) error {
	query := `
		UPDATE projects
		SET status = 'deleted', updated_at = $1
		WHERE id = $2
	`

	_, err := m.db.Exec(ctx, query, time.Now(), projectID)
	if err != nil {
		return fmt.Errorf("failed to delete project: %v", err)
	}

	return nil
}
