package task

import (
	"context"
	"fmt"

	"dev.helix.code/internal/database"
	dag "dev.helix.dag"
	"github.com/google/uuid"
)

// noopNodeFn is the Execute function for graph-validation-only nodes. The
// DependencyManager uses dag.Build purely for its VALIDATION (cycle + unknown
// dependency rejection); it never runs the DAG, so the node body is never
// invoked. It returns nil so the contract is satisfied if ever called.
func noopNodeFn(context.Context, dag.Inputs) (dag.Output, error) { return nil, nil }

// buildGraphNodes walks the dependency graph reachable from rootID (whose
// immediate dependencies are rootDeps) by querying the DB for each node's
// dependency edges, and returns the full node set as []dag.Node suitable for
// dag.Build. Every referenced node id is materialised as a node (leaves
// included) so dag.Build never reports a dangling-dependency error during pure
// cycle detection — only genuine cycles surface as Build errors.
func (dm *DependencyManager) buildGraphNodes(rootID uuid.UUID, rootDeps []uuid.UUID) ([]dag.Node, error) {
	edges := map[uuid.UUID][]string{
		rootID: uuidsToStrings(rootDeps),
	}

	// Frontier of node ids whose edges still need to be fetched from the DB.
	queue := append([]uuid.UUID{}, rootDeps...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if _, done := edges[id]; done {
			continue
		}

		deps, err := dm.getTaskDependencies(id)
		if err != nil {
			return nil, err
		}
		edges[id] = uuidsToStrings(deps)
		queue = append(queue, deps...)
	}

	nodes := make([]dag.Node, 0, len(edges))
	for id, deps := range edges {
		nodes = append(nodes, &dag.FuncNode{
			NodeID: id.String(),
			Deps:   deps,
			Fn:     noopNodeFn,
		})
	}
	return nodes, nil
}

// uuidsToStrings converts a UUID slice to a string slice for dag node IDs.
func uuidsToStrings(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// NewDependencyManager creates a new dependency manager
func NewDependencyManager(db database.DatabaseInterface) *DependencyManager {
	return &DependencyManager{
		db: db,
	}
}

// ValidateDependencies validates that all dependencies exist and are valid
func (dm *DependencyManager) ValidateDependencies(dependencies []uuid.UUID) error {
	if len(dependencies) == 0 {
		return nil
	}

	ctx := context.Background()

	// Check if all dependencies exist
	for _, depID := range dependencies {
		var exists bool
		err := dm.db.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM distributed_tasks WHERE id = $1)
		`, depID).Scan(&exists)

		if err != nil {
			return fmt.Errorf("%s", tr(ctx, "internal_task_dep_check_existence_failed", map[string]any{"Err": err.Error()}))
		}

		if !exists {
			return fmt.Errorf("%s", tr(ctx, "internal_task_dep_not_found", map[string]any{"ID": depID.String()}))
		}
	}

	return nil
}

// CheckDependenciesCompleted checks if all dependencies are completed
func (dm *DependencyManager) CheckDependenciesCompleted(dependencies []uuid.UUID) (bool, error) {
	if len(dependencies) == 0 {
		return true, nil
	}

	ctx := context.Background()

	// Count completed dependencies
	var completedCount int
	err := dm.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM distributed_tasks
		WHERE id = ANY($1) AND status = 'completed'
	`, dependencies).Scan(&completedCount)

	if err != nil {
		return false, fmt.Errorf("%s", tr(ctx, "internal_task_dep_check_status_failed", map[string]any{"Err": err.Error()}))
	}

	return completedCount == len(dependencies), nil
}

// GetBlockingDependencies returns dependencies that are not completed
func (dm *DependencyManager) GetBlockingDependencies(dependencies []uuid.UUID) ([]uuid.UUID, error) {
	if len(dependencies) == 0 {
		return []uuid.UUID{}, nil
	}

	ctx := context.Background()

	rows, err := dm.db.Query(ctx, `
		SELECT id FROM distributed_tasks
		WHERE id = ANY($1) AND status != 'completed'
	`, dependencies)

	if err != nil {
		return nil, fmt.Errorf("failed to query blocking dependencies: %v", err)
	}
	defer rows.Close()

	var blockingDeps []uuid.UUID
	for rows.Next() {
		var depID uuid.UUID
		if err := rows.Scan(&depID); err != nil {
			return nil, fmt.Errorf("failed to scan dependency ID: %v", err)
		}
		blockingDeps = append(blockingDeps, depID)
	}

	return blockingDeps, nil
}

// DetectCircularDependencies detects circular dependencies in the task graph.
//
// It builds the dependency graph reachable from taskID (whose proposed
// immediate dependencies are `dependencies`) by traversing the DB-stored
// dependency edges, then delegates cycle detection to the reusable
// dev.helix.dag module's Build, which rejects any graph containing a cycle.
// A non-nil Build error means a cycle was found and we return (true, nil),
// preserving the existing public semantics: (circular bool, err error) where
// err is reserved for DB-access failures during traversal.
func (dm *DependencyManager) DetectCircularDependencies(taskID uuid.UUID, dependencies []uuid.UUID) (bool, error) {
	if len(dependencies) == 0 {
		return false, nil
	}

	nodes, err := dm.buildGraphNodes(taskID, dependencies)
	if err != nil {
		return false, err
	}

	// dag.Build returns an error only on a cycle (every referenced node is
	// materialised by buildGraphNodes, so dangling-dependency errors cannot
	// occur here). A Build error therefore means a circular dependency.
	if _, buildErr := dag.Build(nodes); buildErr != nil {
		return true, nil
	}

	return false, nil
}

// GetDependencyChain returns the complete dependency chain for a task
func (dm *DependencyManager) GetDependencyChain(taskID uuid.UUID) ([]uuid.UUID, error) {
	visited := make(map[uuid.UUID]bool)
	chain := make([]uuid.UUID, 0)

	if err := dm.traverseDependencies(taskID, visited, &chain); err != nil {
		return nil, err
	}

	return chain, nil
}

// GetDependentTasks returns all tasks that depend on the given task
func (dm *DependencyManager) GetDependentTasks(taskID uuid.UUID) ([]uuid.UUID, error) {
	ctx := context.Background()

	rows, err := dm.db.Query(ctx, `
		SELECT id FROM distributed_tasks
		WHERE $1 = ANY(dependencies)
	`, taskID)

	if err != nil {
		return nil, fmt.Errorf("failed to query dependent tasks: %v", err)
	}
	defer rows.Close()

	var dependentTasks []uuid.UUID
	for rows.Next() {
		var taskID uuid.UUID
		if err := rows.Scan(&taskID); err != nil {
			return nil, fmt.Errorf("failed to scan task ID: %v", err)
		}
		dependentTasks = append(dependentTasks, taskID)
	}

	return dependentTasks, nil
}

// Helper methods

func (dm *DependencyManager) getTaskDependencies(taskID uuid.UUID) ([]uuid.UUID, error) {
	ctx := context.Background()

	var dependencies []uuid.UUID
	err := dm.db.QueryRow(ctx, `
		SELECT dependencies FROM distributed_tasks WHERE id = $1
	`, taskID).Scan(&dependencies)

	if err != nil {
		return nil, fmt.Errorf("failed to get task dependencies: %v", err)
	}

	return dependencies, nil
}

func (dm *DependencyManager) traverseDependencies(taskID uuid.UUID, visited map[uuid.UUID]bool, chain *[]uuid.UUID) error {
	if visited[taskID] {
		return nil // Already visited
	}

	visited[taskID] = true
	*chain = append(*chain, taskID)

	// Get dependencies
	dependencies, err := dm.getTaskDependencies(taskID)
	if err != nil {
		return err
	}

	// Recursively traverse dependencies
	for _, depID := range dependencies {
		if err := dm.traverseDependencies(depID, visited, chain); err != nil {
			return err
		}
	}

	return nil
}
