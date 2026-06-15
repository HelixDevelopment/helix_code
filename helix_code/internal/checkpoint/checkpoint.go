// Package checkpoint implements F12 — workspace checkpoints: a real
// "snapshot the working-tree files now, restore/undo later" safety net.
//
// HelixCode already has "checkpoints" in the task domain (task-DB rows that
// record where a long-running task was), but that is NOT a snapshot of the
// files on disk. This package fills the missing capability: it captures the
// actual byte contents of the working tree at a moment in time and can later
// restore those exact bytes — the file-level undo a CLI-agent user expects
// after the agent has rewritten code.
//
// Storage mechanism (two real backends, chosen automatically):
//
//   - Git work tree (preferred, robust): a snapshot is materialised with git
//     plumbing — `git stash create` produces a dangling commit whose tree is
//     the current working tree (tracked + modified files) WITHOUT touching the
//     working tree or the stash stack; when there is nothing to stash we fall
//     back to `git write-tree` of the index so a checkpoint always exists. The
//     resulting commit is pinned under a project ref
//     `refs/helix/checkpoints/<id>` so it survives process restart and is not
//     garbage-collected. Restore uses `git read-tree` + `git checkout-index`
//     to write the snapshot's real bytes back over the working tree.
//
//   - Plain directory (fallback, no git): a real recursive file copy under
//     `.helix/checkpoints/<id>/files/`, using os.ReadFile / os.WriteFile.
//     `.git`, the `.helix` store itself, and common heavy/vendored dirs are
//     skipped so a checkpoint stays cheap. Restore copies the bytes back.
//
// Anti-bluff (§11.4 / CONST-035): there is no in-memory-only path. Every
// snapshot is persisted on disk (a git object+ref, or copied files), so a
// checkpoint taken in one process is restorable from a fresh process. The
// restore path writes real bytes that checkpoint_test.go reads back off disk.
package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// refPrefix is the git ref namespace under which git-backed snapshots are
// pinned. Using a dedicated refs/helix/* namespace (not refs/heads or
// refs/tags) keeps checkpoints invisible to normal branch/tag listings while
// still protecting the commit object from garbage collection.
const refPrefix = "refs/helix/checkpoints/"

// storeDir is the on-disk store name used by the non-git fallback backend and
// for the git-backend's sidecar metadata (label + timestamp, which a bare git
// ref cannot carry portably).
const storeDir = ".helix/checkpoints"

// skipDirs are directory names never copied by the file-copy fallback. They are
// either recreatable build/vendor output or the checkpoint store itself.
var skipDirs = map[string]bool{
	".git":         true,
	".helix":       true,
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".idea":        true,
	".vscode":      true,
}

// Checkpoint is one persisted workspace snapshot.
type Checkpoint struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	CreatedAt time.Time `json:"created_at"`
	// Backend is "git" or "files" — which mechanism captured this snapshot.
	Backend string `json:"backend"`
	// Commit is the git snapshot commit SHA (git backend only; empty otherwise).
	Commit string `json:"commit,omitempty"`
}

// Manager creates, lists and restores workspace checkpoints rooted at workDir.
type Manager struct {
	workDir string
	// isGit is resolved once at construction: true when workDir is inside a git
	// work tree, selecting the git plumbing backend.
	isGit bool
}

// ErrNotFound is returned by Restore when no checkpoint with the given id exists.
var ErrNotFound = errors.New("checkpoint not found")

// NewManager builds a Manager for the given working directory. workDir is
// resolved to an absolute path; the git/files backend is selected by probing
// for a git work tree. A non-absolute or missing workDir is an error so callers
// fail loudly rather than snapshotting the wrong tree.
func NewManager(workDir string) (*Manager, error) {
	if strings.TrimSpace(workDir) == "" {
		return nil, errors.New("checkpoint: empty working directory")
	}
	abs, err := filepath.Abs(workDir)
	if err != nil {
		return nil, fmt.Errorf("checkpoint: resolve working dir: %w", err)
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("checkpoint: working dir %q is not a directory", abs)
	}
	m := &Manager{workDir: abs}
	m.isGit = m.detectGit()
	return m, nil
}

// Backend reports which storage backend this Manager uses ("git" or "files").
// Exposed so callers (and tests) can assert the active mechanism.
func (m *Manager) Backend() string {
	if m.isGit {
		return "git"
	}
	return "files"
}

func (m *Manager) detectGit() bool {
	out, err := m.git(context.Background(), "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// git runs a git command in workDir and returns trimmed stdout.
func (m *Manager) git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// newID returns a sortable, unique checkpoint id (UTC timestamp to nanosecond).
func newID() string {
	return time.Now().UTC().Format("20060102T150405.000000000")
}

// Create snapshots the current working tree and returns the new checkpoint id.
//
// Git backend: captures via `git stash create` (or `git write-tree` of the
// index when there is nothing modified to stash) and pins the snapshot commit
// under refs/helix/checkpoints/<id>. The working tree is NOT modified.
//
// Files backend: recursively copies the working tree's real bytes into the
// .helix/checkpoints/<id>/files store.
//
// Either way a meta.json sidecar records id/label/timestamp/backend so List can
// report them after a process restart.
func (m *Manager) Create(label string) (string, error) {
	ctx := context.Background()
	id := newID()
	cp := Checkpoint{ID: id, Label: strings.TrimSpace(label), CreatedAt: time.Now().UTC(), Backend: m.Backend()}

	if m.isGit {
		commit, err := m.gitSnapshot(ctx)
		if err != nil {
			return "", err
		}
		if _, err := m.git(ctx, "update-ref", refPrefix+id, commit); err != nil {
			return "", err
		}
		cp.Commit = commit
	} else {
		if err := m.fileSnapshot(id); err != nil {
			return "", err
		}
	}
	if err := m.writeMeta(id, cp); err != nil {
		return "", err
	}
	return id, nil
}

// gitSnapshot materialises a commit whose tree is the current working tree and
// returns its SHA. `git stash create` does exactly this without disturbing the
// working tree or the stash list; when there is nothing to stash it prints
// nothing, so we fall back to the index tree (write-tree + commit-tree) so a
// checkpoint is always created.
func (m *Manager) gitSnapshot(ctx context.Context) (string, error) {
	out, err := m.git(ctx, "stash", "create", "helix-checkpoint")
	if err != nil {
		return "", err
	}
	if sha := strings.TrimSpace(out); sha != "" {
		return sha, nil
	}
	// Clean working tree (relative to index): snapshot the index tree instead.
	treeOut, err := m.git(ctx, "write-tree")
	if err != nil {
		return "", err
	}
	tree := strings.TrimSpace(treeOut)
	// commit-tree needs a parent when HEAD exists; tolerate an unborn HEAD.
	args := []string{"commit-tree", tree, "-m", "helix-checkpoint"}
	if head, herr := m.git(ctx, "rev-parse", "--verify", "HEAD"); herr == nil {
		args = append(args, "-p", strings.TrimSpace(head))
	}
	commitOut, err := m.git(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(commitOut), nil
}

// List returns all checkpoints, newest first. It reads the meta.json sidecars
// from the .helix store, so it works across process restarts and reports the
// label and timestamp a bare git ref cannot carry.
func (m *Manager) List() []Checkpoint {
	root := filepath.Join(m.workDir, storeDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var cps []Checkpoint
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cp, err := m.readMeta(e.Name())
		if err != nil {
			continue
		}
		cps = append(cps, cp)
	}
	sort.Slice(cps, func(i, j int) bool { return cps[i].ID > cps[j].ID })
	return cps
}

// Restore writes the snapshot identified by id back over the working tree,
// restoring the real bytes captured at Create time.
//
// New-file semantics (documented + tested): Restore overwrites and (re)creates
// files that existed in the snapshot, but it does NOT delete files created in
// the working tree AFTER the checkpoint. A checkpoint is an additive/overwrite
// restore, not a destructive sync — this avoids silently destroying work the
// user did after the snapshot. Files that were tracked at snapshot time and
// later deleted ARE restored (their bytes come back).
func (m *Manager) Restore(id string) error {
	ctx := context.Background()
	cp, err := m.readMeta(id)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	if m.isGit && cp.Backend == "git" {
		return m.gitRestore(ctx, cp)
	}
	return m.fileRestore(id)
}

// gitRestore writes the snapshot tree's bytes onto the working tree using
// read-tree (to set the index to the snapshot) + checkout-index -af (to write
// every indexed file's real bytes to disk, overwriting). HEAD is left intact;
// only the index and working-tree files are updated. To avoid leaving the index
// pointing at the snapshot afterwards, the index is reset back to HEAD.
func (m *Manager) gitRestore(ctx context.Context, cp Checkpoint) error {
	ref := refPrefix + cp.ID
	// Verify the ref/commit still exists (survived restart, not GC'd).
	if _, err := m.git(ctx, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
		// Fall back to the recorded commit SHA if the ref was removed.
		if cp.Commit == "" {
			return fmt.Errorf("checkpoint %s: snapshot commit missing: %w", cp.ID, err)
		}
		ref = cp.Commit
	}
	if _, err := m.git(ctx, "read-tree", ref); err != nil {
		return err
	}
	if _, err := m.git(ctx, "checkout-index", "-a", "-f"); err != nil {
		return err
	}
	// Restore the index to HEAD so the user's staging area is not left mutated
	// by the restore (working-tree bytes are already written).
	if _, err := m.git(ctx, "rev-parse", "--verify", "HEAD"); err == nil {
		_, _ = m.git(ctx, "read-tree", "HEAD")
	}
	return nil
}

// fileSnapshot copies the working tree's real bytes into the store.
func (m *Manager) fileSnapshot(id string) error {
	dst := filepath.Join(m.workDir, storeDir, id, "files")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(m.workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(m.workDir, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // skip symlinks/devices — bytes restore covers regular files
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		info, _ := d.Info()
		mode := fs.FileMode(0o644)
		if info != nil {
			mode = info.Mode().Perm()
		}
		return os.WriteFile(target, data, mode)
	})
}

// fileRestore copies the snapshot's bytes back over the working tree (overwrite
// + create; does not delete newer files — see Restore docs).
func (m *Manager) fileRestore(id string) error {
	src := filepath.Join(m.workDir, storeDir, id, "files")
	if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
		return fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." || d.IsDir() {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(m.workDir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		info, _ := d.Info()
		mode := fs.FileMode(0o644)
		if info != nil {
			mode = info.Mode().Perm()
		}
		return os.WriteFile(target, data, mode)
	})
}

func (m *Manager) metaPath(id string) string {
	return filepath.Join(m.workDir, storeDir, id, "meta.json")
}

func (m *Manager) writeMeta(id string, cp Checkpoint) error {
	if err := os.MkdirAll(filepath.Join(m.workDir, storeDir, id), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.metaPath(id), data, 0o644)
}

func (m *Manager) readMeta(id string) (Checkpoint, error) {
	var cp Checkpoint
	data, err := os.ReadFile(m.metaPath(id))
	if err != nil {
		return cp, err
	}
	if err := json.Unmarshal(data, &cp); err != nil {
		return cp, err
	}
	return cp, nil
}
