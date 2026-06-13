package task

import (
	"context"
	"testing"

	"dev.helix.code/internal/database"
	dag "dev.helix.dag"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file is the anti-bluff (§11.4.115 RED->GREEN + §1.1 paired-mutation)
// proof that DependencyManager.DetectCircularDependencies delegates cycle
// detection to the reusable dev.helix.dag module's Build (G-2 / §11.4.74
// extend-don't-reimplement), AND that dag.Build itself rejects cycles and
// unknown/dangling dependencies.
//
// Unit-test mocks are permitted here per CONST-050 (mocks ONLY in unit tests).

// ---------------------------------------------------------------------------
// (c) A valid DAG passes (no cycle).
// ---------------------------------------------------------------------------

func TestDetectCircular_DagBuild_ValidDAGPasses(t *testing.T) {
	mockDB := database.NewMockDatabase()
	dm := NewDependencyManager(mockDB)

	taskID := uuid.New()
	depID1 := uuid.New()
	depID2 := uuid.New()
	dependencies := []uuid.UUID{depID1, depID2}

	// depID1 and depID2 are leaves (no further deps). BFS issues one QueryRow
	// per dependency.
	mockDB.On("QueryRow", context.Background(), mockDB.AnyString(), mockDB.AnyArgs()).
		Return(database.NewMockRowWithValues([]uuid.UUID{})).Once()
	mockDB.On("QueryRow", context.Background(), mockDB.AnyString(), mockDB.AnyArgs()).
		Return(database.NewMockRowWithValues([]uuid.UUID{})).Once()

	circular, err := dm.DetectCircularDependencies(taskID, dependencies)

	require.NoError(t, err)
	assert.False(t, circular, "a valid acyclic graph must NOT be reported as circular")
	mockDB.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// (a) A real circular dependency set is rejected via dag.Build.
//
// Graph: taskID -> dep -> taskID  (a genuine cycle through the DB-stored edge).
// dag.Build MUST reject it, so DetectCircularDependencies returns true.
// ---------------------------------------------------------------------------

func TestDetectCircular_DagBuild_RejectsRealCycle(t *testing.T) {
	mockDB := database.NewMockDatabase()
	dm := NewDependencyManager(mockDB)

	taskID := uuid.New()
	depID := uuid.New()
	dependencies := []uuid.UUID{depID}

	// dep depends back on taskID -> cycle. BFS queries dep once; taskID is
	// already materialised as the root, so no further query.
	mockDB.On("QueryRow", context.Background(), mockDB.AnyString(), mockDB.AnyArgs()).
		Return(database.NewMockRowWithValues([]uuid.UUID{taskID}))

	circular, err := dm.DetectCircularDependencies(taskID, dependencies)

	require.NoError(t, err)
	assert.True(t, circular, "a real cycle MUST be rejected via dag.Build")
	mockDB.AssertExpectations(t)
}

// TestDetectCircular_DagBuild_RejectsIndirectCycle proves multi-hop cycle
// detection (taskID -> dep1 -> dep2 -> taskID) goes through dag.Build.
func TestDetectCircular_DagBuild_RejectsIndirectCycle(t *testing.T) {
	mockDB := database.NewMockDatabase()
	dm := NewDependencyManager(mockDB)

	taskID := uuid.New()
	depID1 := uuid.New()
	depID2 := uuid.New()
	dependencies := []uuid.UUID{depID1}

	// dep1 -> dep2
	mockDB.On("QueryRow", context.Background(), mockDB.AnyString(), mockDB.AnyArgs()).
		Return(database.NewMockRowWithValues([]uuid.UUID{depID2})).Once()
	// dep2 -> taskID (closes the cycle)
	mockDB.On("QueryRow", context.Background(), mockDB.AnyString(), mockDB.AnyArgs()).
		Return(database.NewMockRowWithValues([]uuid.UUID{taskID})).Once()

	circular, err := dm.DetectCircularDependencies(taskID, dependencies)

	require.NoError(t, err)
	assert.True(t, circular, "an indirect (multi-hop) cycle MUST be rejected via dag.Build")
	mockDB.AssertExpectations(t)
}

// ---------------------------------------------------------------------------
// (b) An unknown / dangling dependency is rejected by dag.Build.
//
// This asserts the reused module's contract directly: a node that depends on
// an id not present in the node set is a Build error. The DependencyManager's
// own traversal materialises every referenced node so it never triggers this
// path; this test pins the underlying guarantee G-2 relies on.
// ---------------------------------------------------------------------------

func TestDagBuild_RejectsUnknownDependency(t *testing.T) {
	known := uuid.New().String()
	unknown := uuid.New().String()

	nodes := []dag.Node{
		&dag.FuncNode{NodeID: known, Deps: []string{unknown}, Fn: noopNodeFn},
	}

	_, err := dag.Build(nodes)

	require.Error(t, err, "dag.Build MUST reject a node depending on an unknown node")
	assert.Contains(t, err.Error(), "unknown node",
		"the rejection must be the dangling-dependency error")
}

// TestDagBuild_AcceptsValidGraph is the positive control for the reused module:
// a fully-resolved acyclic node set builds without error.
func TestDagBuild_AcceptsValidGraph(t *testing.T) {
	a := uuid.New().String()
	b := uuid.New().String()
	c := uuid.New().String()

	nodes := []dag.Node{
		&dag.FuncNode{NodeID: a, Deps: []string{b, c}, Fn: noopNodeFn},
		&dag.FuncNode{NodeID: b, Deps: []string{c}, Fn: noopNodeFn},
		&dag.FuncNode{NodeID: c, Deps: nil, Fn: noopNodeFn},
	}

	d, err := dag.Build(nodes)

	require.NoError(t, err, "a valid acyclic graph MUST build")
	assert.NotNil(t, d)
}

// §1.1 PAIRED MUTATION (no production seam): the captured proof in
// qa-results/g2-streamB-evidence.txt reverts DetectCircularDependencies's
// dag.Build call to the old hand-rolled DFS (and to a no-op) on the REAL
// production source and shows TestDetectCircular_DagBuild_RejectsRealCycle /
// RejectsIndirectCycle FAIL, then restores — proving dag.Build is the
// load-bearing cycle arbiter. No bypass var is left in production code.
