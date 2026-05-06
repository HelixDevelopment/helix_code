package plantree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Store interface {
	Save(tree PlanTree) error
	Load(name string) (PlanTree, error)
	List() ([]PlanTreeSummary, error)
	Delete(name string) error
}

type FileStore struct {
	dir string
	mu  sync.Mutex
}

func NewFileStore(baseDir string) *FileStore {
	return &FileStore{dir: filepath.Join(baseDir, StorageDir)}
}

func (fs *FileStore) Save(tree PlanTree) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err := os.MkdirAll(fs.dir, 0700); err != nil {
		return fmt.Errorf("create storage dir: %w", err)
	}

	data, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal tree: %w", err)
	}

	path := filepath.Join(fs.dir, tree.Name+".json")
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
}

func (fs *FileStore) Load(name string) (PlanTree, error) {
	path := filepath.Join(fs.dir, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return PlanTree{}, ErrPlanNotFound
		}
		return PlanTree{}, fmt.Errorf("read plan file: %w", err)
	}

	var tree PlanTree
	if err := json.Unmarshal(data, &tree); err != nil {
		return PlanTree{}, fmt.Errorf("%w: %v", ErrTreeCorrupt, err)
	}

	return tree, nil
}

func (fs *FileStore) List() ([]PlanTreeSummary, error) {
	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read storage dir: %w", err)
	}

	var summaries []PlanTreeSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".json")
		tree, err := fs.Load(name)
		if err != nil {
			continue
		}
		summaries = append(summaries, tree.Summary())
	}

	return summaries, nil
}

func (fs *FileStore) Delete(name string) error {
	path := filepath.Join(fs.dir, name+".json")
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete plan file: %w", err)
	}
	return nil
}
