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
		return nil, fmt.Errorf("%w: %v", ErrInvalidOwnerID, err)
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
		return nil, fmt.Errorf("%w: %v", ErrInvalidProjectID, err)
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
			return nil, fmt.Errorf("%w: %s", ErrProjectNotFound, id)
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

// detectProjectType sets build/test/lint defaults for the supplied
// projectType. The current contract is provider-driven (the caller
// passes the type explicitly — typically from a UI choice or config);
// auto-detection from filesystem signals (go.mod, package.json,
// pyproject.toml, Cargo.toml, …) is a follow-up extension. The
// function never guesses or fabricates a type; it only maps a known
// type label to its default command set (round-33 §11.4 comment
// rewrite — previous "For now" lead-in implied a stub; the function
// is in fact the canonical type→commands mapper today;
// CONST-035 / Article XI §11.9).
func (m *DatabaseManager) detectProjectType(path, projectType string, metadata *Metadata) {

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
		return nil, fmt.Errorf("%w: %v", ErrInvalidOwnerID, err)
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

// UpdateProject updates project name and description in the database.
//
// Anti-bluff (CONST-035): the previous version's RETURNING clause
// referenced `path` and `type` columns that DON'T EXIST in the
// projects schema (`internal/database/database.go:333`). Real schema
// columns are `workspace_path` (mapped to Project.Path in Go) and
// `config` JSONB (which holds the project type under `config["type"]`,
// matching how GetProject loads it). Every successful UPDATE hit 500
// with "ERROR: column path does not exist (SQLSTATE 42703)" — the
// canonical "rename a project" call was unreachable.
//
// Fixed to mirror GetProject's pattern: RETURNING workspace_path +
// config, then extract type from config["type"] in Go.
func (m *DatabaseManager) UpdateProject(ctx context.Context, projectID, name, description string) (*Project, error) {
	// Pre-validate UUID format. Postgres would otherwise reject with
	// SQLSTATE 22P02 "invalid input syntax for type uuid: ..." which
	// leaks the SQLSTATE code in the API response (CONST-042) and
	// surfaces as HTTP 500 (CONST-035). Surface as ErrInvalidProjectID
	// → 400 instead.
	if _, err := uuid.Parse(projectID); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidProjectID, err)
	}
	query := `
		UPDATE projects
		SET name = COALESCE(NULLIF($1, ''), name),
		    description = COALESCE(NULLIF($2, ''), description),
		    updated_at = $3
		WHERE id = $4
		RETURNING id, name, description, workspace_path, owner_id, created_at, updated_at, status, config
	`

	var (
		dbID          uuid.UUID
		retName       string
		retDesc       string
		workspacePath string
		ownerID       uuid.UUID
		createdAt     time.Time
		updatedAt     time.Time
		status        string
		config        map[string]interface{}
	)
	err := m.db.QueryRow(ctx, query, name, description, time.Now(), projectID).Scan(
		&dbID, &retName, &retDesc, &workspacePath, &ownerID, &createdAt, &updatedAt, &status, &config,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update project: %v", err)
	}

	// Match GetProject's mapping: type lives inside config JSONB, not
	// as its own column.
	projectType, _ := config["type"].(string)
	metadataMap, _ := config["metadata"].(map[string]interface{})

	return &Project{
		ID:          dbID.String(),
		Name:        retName,
		Description: retDesc,
		Path:        workspacePath,
		Type:        projectType,
		Active:      status == "active",
		Metadata:    m.convertToMetadata(metadataMap),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
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
	// Pre-validate UUID format (same CONST-042/CONST-035 fix as
	// UpdateProject — postgres would otherwise leak SQLSTATE 22P02
	// in the API response).
	if _, err := uuid.Parse(projectID); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProjectID, err)
	}
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
