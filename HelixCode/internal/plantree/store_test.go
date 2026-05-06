package plantree

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestTree(name string) PlanTree {
	now := time.Now().UTC()
	return PlanTree{
		Name:    name,
		Version: 1,
		Root: &PlanNode{
			ID:          "root-" + name,
			Title:       "Plan " + name,
			Description: "Test plan " + name,
			Status:      StatusPending,
			CreatedAt:   now,
			UpdatedAt:   now,
			Children: []*PlanNode{
				{
					ID:          "task-1-" + name,
					Title:       "Task 1",
					Description: "First task",
					Status:      StatusPending,
					CreatedAt:   now,
					UpdatedAt:   now,
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestFileStore_SaveLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	tree := makeTestTree("my-plan")
	err := store.Save(tree)
	require.NoError(t, err)

	loaded, err := store.Load("my-plan")
	require.NoError(t, err)

	assert.Equal(t, tree.Name, loaded.Name)
	assert.Equal(t, tree.Version, loaded.Version)
	assert.Equal(t, tree.Root.ID, loaded.Root.ID)
	assert.Equal(t, tree.Root.Title, loaded.Root.Title)
	assert.Equal(t, tree.Root.Description, loaded.Root.Description)
	assert.Equal(t, tree.Root.Status, loaded.Root.Status)
	require.Len(t, loaded.Root.Children, 1)
	assert.Equal(t, "task-1-my-plan", loaded.Root.Children[0].ID)
}

func TestFileStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	tree := makeTestTree("atomic-test")
	err := store.Save(tree)
	require.NoError(t, err)

	planPath := filepath.Join(dir, StorageDir, "atomic-test.json")
	tmpPath := planPath + ".tmp"

	_, err = os.Stat(planPath)
	assert.NoError(t, err, "plan file must exist")

	_, err = os.Stat(tmpPath)
	assert.True(t, os.IsNotExist(err), "tmp file must not remain")
}

func TestFileStore_LoadNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	_, err := store.Load("nonexistent")
	assert.ErrorIs(t, err, ErrPlanNotFound)
}

func TestFileStore_LoadCorrupted(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	planDir := filepath.Join(dir, StorageDir)
	require.NoError(t, os.MkdirAll(planDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(planDir, "corrupt.json"), []byte("not-valid-json"), 0600))

	_, err := store.Load("corrupt")
	assert.ErrorIs(t, err, ErrTreeCorrupt)
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	err := store.Save(makeTestTree("plan-a"))
	require.NoError(t, err)
	err = store.Save(makeTestTree("plan-b"))
	require.NoError(t, err)
	err = store.Save(makeTestTree("plan-c"))
	require.NoError(t, err)

	summaries, err := store.List()
	require.NoError(t, err)

	assert.Len(t, summaries, 3)

	names := make(map[string]bool)
	for _, s := range summaries {
		names[s.Name] = true
		assert.NotEmpty(t, s.RootID)
		assert.NotEmpty(t, s.RootTitle)
		assert.Equal(t, 2, s.NodeCount)
	}
	assert.True(t, names["plan-a"])
	assert.True(t, names["plan-b"])
	assert.True(t, names["plan-c"])
}

func TestFileStore_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	summaries, err := store.List()
	require.NoError(t, err)
	assert.Nil(t, summaries)
}

func TestFileStore_Delete(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	err := store.Save(makeTestTree("delete-me"))
	require.NoError(t, err)

	err = store.Delete("delete-me")
	require.NoError(t, err)

	_, err = store.Load("delete-me")
	assert.ErrorIs(t, err, ErrPlanNotFound)
}

func TestFileStore_DeleteIdempotent(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	err := store.Delete("nonexistent")
	assert.NoError(t, err)
}

func TestFileStore_FileMode(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	err := store.Save(makeTestTree("mode-test"))
	require.NoError(t, err)

	planPath := filepath.Join(dir, StorageDir, "mode-test.json")
	info, err := os.Stat(planPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestFileStore_DirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	baseDir := filepath.Join(dir, "deeply", "nested", "project")
	store := NewFileStore(baseDir)

	err := store.Save(makeTestTree("deep-plan"))
	require.NoError(t, err)

	planPath := filepath.Join(baseDir, StorageDir, "deep-plan.json")
	_, err = os.Stat(planPath)
	assert.NoError(t, err)
}

func TestFileStore_ListSkipsNonJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	planDir := filepath.Join(dir, StorageDir)
	require.NoError(t, os.MkdirAll(planDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(planDir, "readme.txt"), []byte("hello"), 0600))

	err := store.Save(makeTestTree("real-plan"))
	require.NoError(t, err)

	summaries, err := store.List()
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
	assert.Equal(t, "real-plan", summaries[0].Name)
}

func TestFileStore_ListSkipsCorrupted(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	planDir := filepath.Join(dir, StorageDir)
	require.NoError(t, os.MkdirAll(planDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(planDir, "bad.json"), []byte("{corrupt}"), 0600))

	err := store.Save(makeTestTree("good-plan"))
	require.NoError(t, err)

	summaries, err := store.List()
	require.NoError(t, err)
	assert.Len(t, summaries, 1)
	assert.Equal(t, "good-plan", summaries[0].Name)
}

func TestFileStore_SaveOverwrite(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	tree := makeTestTree("overwrite-test")
	err := store.Save(tree)
	require.NoError(t, err)

	tree.Version = 2
	tree.Root.Title = "Updated Title"
	err = store.Save(tree)
	require.NoError(t, err)

	loaded, err := store.Load("overwrite-test")
	require.NoError(t, err)

	assert.Equal(t, 2, loaded.Version)
	assert.Equal(t, "Updated Title", loaded.Root.Title)
}

func TestFileStore_ConcurrentSave(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	done := make(chan bool, 10)
	for i := 1; i <= 10; i++ {
		go func(idx int) {
			tree := makeTestTree("concurrent")
			tree.Version = idx
			_ = store.Save(tree)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	loaded, err := store.Load("concurrent")
	require.NoError(t, err)
	assert.NotZero(t, loaded.Version)
	assert.Equal(t, "concurrent", loaded.Name)
}

func TestFileStore_StoreInterface(t *testing.T) {
	var store Store = NewFileStore(t.TempDir())
	assert.NotNil(t, store)
}
