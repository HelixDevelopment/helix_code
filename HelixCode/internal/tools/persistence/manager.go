package persistence

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Manager owns a project's tool-result blob store.
type Manager struct {
	baseDir string       // <projectRoot>/<PersistDir>
	mu      sync.RWMutex // RLock for LoadPersisted; WLock for MaybePersist + CleanupOld
}

// NewManager returns a Manager rooted at projectRoot. The persistence dir
// is created lazily on the first persist; calling NewManager does NOT do I/O.
func NewManager(projectRoot string) *Manager {
	return &Manager{
		baseDir: filepath.Join(projectRoot, PersistDir),
	}
}

// MaybePersist returns an inline PersistedResult for output sized
// <= PersistThreshold; otherwise writes the content to disk and returns a
// path-reference. A nil *Manager passes through inline.
//
// Disk failures fall back to inline (logged at WARN); the tool call's
// downstream visibility is preserved at the cost of LLM token usage.
func (m *Manager) MaybePersist(toolName, toolCallID, output string) (*PersistedResult, error) {
	if m == nil {
		return &PersistedResult{
			Output:     output,
			ToolName:   toolName,
			ToolCallID: toolCallID,
		}, nil
	}

	if len(output) <= PersistThreshold {
		return &PersistedResult{
			Output:     output,
			ToolName:   toolName,
			ToolCallID: toolCallID,
		}, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := os.MkdirAll(m.baseDir, 0o755); err != nil {
		log.Printf("WARN persistence: mkdir %s: %v — falling back to inline", m.baseDir, err)
		return &PersistedResult{
			Output:     output,
			ToolName:   toolName,
			ToolCallID: toolCallID,
		}, nil
	}

	hash := sha256.Sum256([]byte(output))
	hashHex := hex.EncodeToString(hash[:8]) // 16 hex chars
	timestamp := time.Now().UTC().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s_%s.txt", sanitiseToolName(toolName), hashHex, timestamp)
	path := filepath.Join(m.baseDir, filename)

	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		log.Printf("WARN persistence: write %s: %v — falling back to inline", path, err)
		return &PersistedResult{
			Output:     output,
			ToolName:   toolName,
			ToolCallID: toolCallID,
		}, nil
	}

	return &PersistedResult{
		PersistedOutputPath: path,
		PersistedOutputSize: len(output),
		WasPersisted:        true,
		ToolName:            toolName,
		ToolCallID:          toolCallID,
	}, nil
}

// sanitiseToolName strips path separators, traversal, control characters,
// and clamps to 32 chars so the filename is filesystem-safe.
var safeNameRune = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func sanitiseToolName(name string) string {
	cleaned := safeNameRune.ReplaceAllString(name, "_")
	cleaned = strings.ReplaceAll(cleaned, "..", "__")
	if len(cleaned) > 32 {
		cleaned = cleaned[:32]
	}
	if cleaned == "" {
		cleaned = "tool"
	}
	return cleaned
}

// ErrPathTraversal is returned by LoadPersisted when the requested path
// resolves outside the manager's base directory.
var ErrPathTraversal = errors.New("path outside persistence directory")

// LoadPersisted reads a previously-persisted output by absolute path.
// Returns ErrPathTraversal if path resolves outside the manager's base
// directory; wraps os.ErrNotExist for missing files.
func (m *Manager) LoadPersisted(path string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", path, err)
	}
	absBase, err := filepath.Abs(m.baseDir)
	if err != nil {
		return "", fmt.Errorf("resolving base %s: %w", m.baseDir, err)
	}
	rel, err := filepath.Rel(absBase, absPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("%w: %s", ErrPathTraversal, path)
	}

	body, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", absPath, err)
	}
	return string(body), nil
}
