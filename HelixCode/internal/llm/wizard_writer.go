package llm

// wizard_writer.go (P1-F12-T08): secret-safe writer for the first-run
// cloud-provider WizardResult. Two paths:
//
//   WriteWizardConfig(path, result)
//     Uses O_WRONLY|O_CREATE|O_EXCL with mode 0600. FAILS if the file
//     already exists — this is the create-only path the interactive
//     wizard uses on first run so a saved config is never silently
//     clobbered. Parent directories are created with mode 0700 so the
//     file (which contains api keys / aws secrets / azure keys) is not
//     enumerable through a wider parent.
//
//   OverwriteWizardConfig(path, result)
//     Atomic replace via temp-file + rename(2). Same secret-safe modes
//     (0700 dir, 0600 file). Used by `helixcode wizard` invoked
//     explicitly with intent to replace a previously-saved config — the
//     T09 cobra command will wire this behind a `--force` / explicit
//     prompt. Atomic rename ensures readers never observe a partial
//     write or an empty file mid-flight.
//
// Anti-bluff anchor: this file ALWAYS writes to a real disk; tests use
// t.TempDir() and the os.Stat call to verify mode. There is no
// "in-memory writer" fallback or simulation path — file IO is the
// product, not a test-only seam.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrWizardConfigExists is returned by WriteWizardConfig when path
// already exists. It wraps os.ErrExist so callers can use
// errors.Is(err, os.ErrExist).
var ErrWizardConfigExists = fmt.Errorf("wizard config already exists: %w", os.ErrExist)

// WriteWizardConfig writes the wizard result to path using the
// secret-safe O_EXCL semantics described above. Returns
// ErrWizardConfigExists (wrapping os.ErrExist) if path is already
// present.
//
// Caller-side contract:
//   - path is a regular file path (not a directory).
//   - The wizard has already validated result; this function does not
//     re-run validation, it just persists.
//   - On any error, no partial file is left at path.
func WriteWizardConfig(path string, result *WizardResult) error {
	if result == nil {
		return errors.New("WriteWizardConfig: nil result")
	}
	if path == "" {
		return errors.New("WriteWizardConfig: empty path")
	}

	if err := ensureSecretSafeParent(path); err != nil {
		return err
	}

	data, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("WriteWizardConfig: marshal: %w", err)
	}

	// O_EXCL is the load-bearing flag. mode 0600 is the load-bearing
	// permission. Anything weaker leaks credentials.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrWizardConfigExists
		}
		return fmt.Errorf("WriteWizardConfig: open: %w", err)
	}

	// On any subsequent write/close failure, remove the half-written
	// file so we never leave a corrupt config on disk.
	cleanupOnError := true
	defer func() {
		_ = f.Close()
		if cleanupOnError {
			_ = os.Remove(path)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("WriteWizardConfig: write: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("WriteWizardConfig: fsync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("WriteWizardConfig: close: %w", err)
	}
	cleanupOnError = false
	return nil
}

// OverwriteWizardConfig atomically replaces the file at path with the
// marshalled result. Uses a temp file in the same directory + rename(2)
// so readers never observe a partially-written file. Final file mode is
// 0600; final parent dir mode is 0700.
func OverwriteWizardConfig(path string, result *WizardResult) error {
	if result == nil {
		return errors.New("OverwriteWizardConfig: nil result")
	}
	if path == "" {
		return errors.New("OverwriteWizardConfig: empty path")
	}

	if err := ensureSecretSafeParent(path); err != nil {
		return err
	}

	data, err := yaml.Marshal(result)
	if err != nil {
		return fmt.Errorf("OverwriteWizardConfig: marshal: %w", err)
	}

	dir := filepath.Dir(path)

	// Create temp file in the same directory so rename(2) is atomic
	// (cross-fs renames are NOT atomic on POSIX). os.CreateTemp gives
	// a unique name and mode 0600 by default.
	tmp, err := os.CreateTemp(dir, ".wizard-*.tmp")
	if err != nil {
		return fmt.Errorf("OverwriteWizardConfig: create temp: %w", err)
	}
	tmpPath := tmp.Name()

	cleanupTmp := true
	defer func() {
		// Best-effort: if we still own a temp file (rename never
		// succeeded), remove it so no junk leaks beside the real
		// config.
		_ = tmp.Close()
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	// CreateTemp on POSIX gives 0600 already; chmod is defensive for
	// platforms (or umasks) that bend the bits.
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("OverwriteWizardConfig: chmod tmp: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("OverwriteWizardConfig: write tmp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("OverwriteWizardConfig: fsync tmp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("OverwriteWizardConfig: close tmp: %w", err)
	}

	// Atomic rename. If this succeeds the temp file is gone (it took
	// over the destination path), so we do not need to clean it up.
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("OverwriteWizardConfig: rename: %w", err)
	}
	cleanupTmp = false
	return nil
}

// ensureSecretSafeParent guarantees that the parent directory of path
// exists with mode 0700. If the directory already exists we leave its
// permissions alone — overriding user-set perms on an existing dir
// would be presumptuous (and potentially worse than the current state).
func ensureSecretSafeParent(path string) error {
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	if _, err := os.Stat(dir); err == nil {
		return nil // already exists; respect user's chosen mode
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat parent dir %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("mkdir parent %s: %w", dir, err)
	}
	// MkdirAll honours umask — re-chmod the deepest dir we just made
	// so we get a deterministic 0700 even when umask is 022.
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("chmod parent %s: %w", dir, err)
	}
	return nil
}
