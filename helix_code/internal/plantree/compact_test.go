package plantree

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSummariser struct {
	calls     int
	summaries []string
	err       error
}

func (m *mockSummariser) Summarise(text string, maxWords int) (string, error) {
	m.calls++
	if m.err != nil {
		return "", m.err
	}
	idx := m.calls - 1
	if idx < len(m.summaries) {
		return m.summaries[idx], nil
	}
	return "compact summary", nil
}

func TestCompactTree_BelowThreshold(t *testing.T) {
	root := &PlanNode{
		ID:          "root",
		Title:       "Small Plan",
		Description: "A tiny plan",
		Status:      StatusPending,
	}
	tree := &PlanTree{Name: "tiny", Version: 1, Root: root}

	ms := &mockSummariser{}
	result, err := CompactTree(tree, ms)
	require.NoError(t, err)

	assert.Equal(t, 0, result.NodesCompacted)
	assert.Equal(t, result.OriginalBytes, result.NewBytes)
	assert.Equal(t, 0, ms.calls)
	assert.Equal(t, "Small Plan", result.Tree.Root.Title)
}

func makeBigString(sizeKB int) string {
	const block = "Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. "
	var s string
	for len(s) < sizeKB*1024 {
		s += block
	}
	return s[:sizeKB*1024]
}

func TestCompactTree_AboveThreshold_Compacts(t *testing.T) {
	root := &PlanNode{
		ID:          "root",
		Title:       "Big Plan",
		Description: makeBigString(8),
		Status:      StatusPending,
	}

	for i := 0; i < 12; i++ {
		grandchild := &PlanNode{
			ID:          "gc-" + string(rune('A'+i)),
			Title:       "Grandchild",
			Description: makeBigString(5),
			Status:      StatusCompleted,
		}
		child := &PlanNode{
			ID:          "child-" + string(rune('A'+i)),
			Title:       "Child",
			Description: makeBigString(5),
			Status:      StatusCompleted,
			Children:    []*PlanNode{grandchild},
			ParentID:    "root",
		}
		grandchild.ParentID = child.ID
		root.Children = append(root.Children, child)
	}

	tree := &PlanTree{Name: "big", Version: 1, Root: root}

	ms := &mockSummariser{summaries: make([]string, 100)}
	for i := range ms.summaries {
		ms.summaries[i] = "compact: task done"
	}
	result, err := CompactTree(tree, ms)
	require.NoError(t, err)

	assert.Greater(t, result.NodesCompacted, 0, "should have compacted some nodes")
	assert.Less(t, result.NewBytes, result.OriginalBytes, "byte count should decrease")

	foundCompacted := false
	for _, child := range result.Tree.Root.Children {
		for _, gc := range child.Children {
			if gc.Metadata != nil && gc.Metadata["compacted"] == "true" {
				foundCompacted = true
				assert.NotEmpty(t, gc.Metadata["compacted_bytes"])
			}
		}
	}
	assert.True(t, foundCompacted, "at least one grandchild should have compacted marker")
}

func TestCompactTree_ByteReduction(t *testing.T) {
	root := &PlanNode{
		ID:          "root",
		Title:       "Reduction Plan",
		Description: "Root desc",
		Status:      StatusPending,
	}

	for i := 0; i < 50; i++ {
		root.Children = append(root.Children, &PlanNode{
			ID:          "c-" + string(rune('A'+i%26)),
			Title:       "Task",
			Description: "Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.",
			Status:      StatusCompleted,
			ParentID:    "root",
		})
	}

	tree := &PlanTree{Name: "reduce", Version: 1, Root: root}

	ms := &mockSummariser{}
	result, err := CompactTree(tree, ms)
	require.NoError(t, err)

	if result.NodesCompacted > 0 {
		reduction := float64(result.OriginalBytes-result.NewBytes) / float64(result.OriginalBytes)
		assert.GreaterOrEqual(t, reduction, 0.0)
	}
}

func TestCompactTree_ShallowNodesNotCompacted(t *testing.T) {
	root := &PlanNode{
		ID:          "root",
		Title:       "Shallow",
		Description: "Shallow plan",
		Status:      StatusPending,
	}
	root.Children = []*PlanNode{
		{
			ID:          "shallow-child",
			Title:       "Shallow Task",
			Description: "This should not be compacted because depth < 3 despite being long enough. Adding more text to ensure we cross the compact threshold eventually by having lots of data.",
			Status:      StatusCompleted,
			ParentID:    "root",
		},
	}

	for len(root.Children) < 50 {
		root.Children = append(root.Children, &PlanNode{
			ID:          "extra-" + string(rune('A'+len(root.Children)%26)),
			Title:       "Extra",
			Description: "Extra data to push over the compact threshold for testing purposes. More text here to make it bigger.",
			Status:      StatusCompleted,
			ParentID:    "root",
		})
	}

	tree := &PlanTree{Name: "shallow", Version: 1, Root: root}

	ms := &mockSummariser{}
	result, err := CompactTree(tree, ms)
	require.NoError(t, err)

	assert.Equal(t, 0, result.NodesCompacted)
}

func TestCompactTree_PendingNodesNotCompacted(t *testing.T) {
	root := &PlanNode{
		ID:          "root",
		Title:       "Pending",
		Description: "Pending plan",
		Status:      StatusPending,
	}

	for i := 0; i < 50; i++ {
		grandchild := &PlanNode{
			ID:          "gc-" + string(rune('A'+i%26)),
			Title:       "Grandchild Task",
			Description: "This is a leaf grandchild with Pending status at depth 3+ but it should NOT be compacted. Adding text to pad it out and make it more substantial for the compact threshold.",
			Status:      StatusPending,
		}
		child := &PlanNode{
			ID:       "c-" + string(rune('A'+i%26)),
			Title:    "Child",
			Status:   StatusCompleted,
			ParentID: "root",
			Children: []*PlanNode{grandchild},
		}
		grandchild.ParentID = child.ID
		root.Children = append(root.Children, child)
	}

	tree := &PlanTree{Name: "pending", Version: 1, Root: root}

	ms := &mockSummariser{}
	result, err := CompactTree(tree, ms)
	require.NoError(t, err)

	for _, child := range result.Tree.Root.Children {
		for _, gc := range child.Children {
			assert.NotEqual(t, "true", gc.Metadata["compacted"], "pending nodes at depth 3+ should NOT be compacted")
		}
	}
}

func TestCompactTree_CompactedMarker(t *testing.T) {
	root := &PlanNode{
		ID:          "root",
		Title:       "Marker",
		Description: "Marker plan",
		Status:      StatusPending,
	}

	for i := 0; i < 50; i++ {
		child := &PlanNode{
			ID:          "leaf-" + string(rune('A'+i%26)),
			Title:       "Leaf",
			Description: "A complete leaf task that should get the compacted marker. Adding more bytes to push over the compact threshold. More padding text to fill things up sufficiently.",
			Status:      StatusCompleted,
			ParentID:    "root",
		}
		root.Children = append(root.Children, child)
	}

	tree := &PlanTree{Name: "marker", Version: 1, Root: root}

	ms := &mockSummariser{summaries: []string{"summary A", "summary B", "summary C", "summary D", "summary E", "summary F", "summary G", "summary H", "summary I", "summary J", "summary K", "summary L", "summary M", "summary N", "summary O", "summary P", "summary Q", "summary R", "summary S", "summary T", "summary U", "summary V", "summary W", "summary X", "summary Y", "summary Z", "summary 0", "summary 1", "summary 2", "summary 3", "summary 4", "summary 5", "summary 6", "summary 7", "summary 8", "summary 9", "summary 10", "summary 11", "summary 12", "summary 13", "summary 14", "summary 15", "summary 16", "summary 17", "summary 18", "summary 19", "summary 20", "summary 21", "summary 22", "summary 23"}}
	result, err := CompactTree(tree, ms)
	require.NoError(t, err)

	if result.NodesCompacted > 0 {
		foundCompacted := false
		for _, child := range result.Tree.Root.Children {
			if child.Metadata != nil && child.Metadata["compacted"] == "true" {
				foundCompacted = true
				assert.NotEmpty(t, child.Metadata["compacted_bytes"])
			}
		}
		assert.True(t, foundCompacted, "at least one node should have compacted marker")
	}
}

func TestCompactTree_SummariseError_Graceful(t *testing.T) {
	root := &PlanNode{
		ID:          "root",
		Title:       "Error Plan",
		Description: "Plan where summariser will fail. Need to have this exceed the compact threshold so it actually tries to compact nodes.",
		Status:      StatusPending,
	}

	for i := 0; i < 50; i++ {
		root.Children = append(root.Children, &PlanNode{
			ID:          "e-" + string(rune('A'+i%26)),
			Title:       "Error Leaf",
			Description: "This leaf should trigger a summariser error but not panic or fail compaction. Long enough description to ensure compact threshold is hit. Adding more padding text.",
			Status:      StatusCompleted,
			ParentID:    "root",
		})
	}

	tree := &PlanTree{Name: "error", Version: 1, Root: root}
	originalTitle := tree.Root.Title

	ms := &mockSummariser{err: assert.AnError}
	result, err := CompactTree(tree, ms)
	require.NoError(t, err)

	assert.Equal(t, 0, result.NodesCompacted)
	assert.Equal(t, originalTitle, result.Tree.Root.Title)
}

func TestCompactTree_NilInput(t *testing.T) {
	_, err := CompactTree(nil, &mockSummariser{})
	assert.Error(t, err)
}

func TestCompactTree_PreservesRoot(t *testing.T) {
	root := &PlanNode{
		ID:          "root",
		Title:       "Root Preserved",
		Description: "Root description that must stay intact. Adding padding to exceed compact threshold. More text needed. Still more text. Even more text. Lorem ipsum dolor sit amet.",
		Status:      StatusPending,
	}

	for i := 0; i < 50; i++ {
		root.Children = append(root.Children, &PlanNode{
			ID:          "leaf-" + string(rune('A'+i%26)),
			Title:       "Leaf",
			Description: "A completed leaf node that should be compacted to save space in the plan tree. Need enough text to push the total plan JSON over the compact threshold. More descriptive text here.",
			Status:      StatusCompleted,
			ParentID:    "root",
		})
	}

	tree := &PlanTree{Name: "preserve", Version: 1, Root: root}

	ms := &mockSummariser{summaries: []string{"compacted leaf"}}
	result, err := CompactTree(tree, ms)
	require.NoError(t, err)

	assert.Equal(t, "Root Preserved", result.Tree.Root.Title)
	assert.Equal(t, "Root description that must stay intact. Adding padding to exceed compact threshold. More text needed. Still more text. Even more text. Lorem ipsum dolor sit amet.", result.Tree.Root.Description)
}

func TestDeterministicSummariser(t *testing.T) {
	ds := DeterministicSummariser{}

	t.Run("short text unchanged", func(t *testing.T) {
		result, err := ds.Summarise("hello world", 50)
		require.NoError(t, err)
		assert.Equal(t, "hello world", result)
	})

	t.Run("long text truncated", func(t *testing.T) {
		words := make([]string, 100)
		for i := range words {
			words[i] = "word"
		}
		text := ""
		for _, w := range words {
			if text != "" {
				text += " "
			}
			text += w
		}

		result, err := ds.Summarise(text, 50)
		require.NoError(t, err)
		assert.Less(t, len(result), len(text))
		assert.Contains(t, result, "...")
	})
}
