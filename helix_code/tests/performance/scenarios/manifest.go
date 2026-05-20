// Package scenarios defines the canonical speed-programme scenarios S1-S4 and a
// deterministic large-repo fixture generator used to make every later phase's
// speedup claim falsifiable.
//
// It is the shared definition point for:
//   - scripts/testing/run_speed_scenarios.sh (the canonical-scenario runner)
//   - the Go scenario runner cmd (cmd/runner)
//   - later P0/P2/P4 pprof + benchmark tasks
//
// Constitutional anchors: built for R4 phased plan P0-T04 (docs/research/speed/
// 04-phased-implementation-plan.md §3). No production code is changed by this
// package — it is measurement infrastructure (Phase 0 is the baseline phase).
// CONST-046: no user-facing text is hardcoded here; the strings below are
// developer-facing scenario identifiers / diagnostics, not localized end-user
// content.
package scenarios

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// Manifest is the parsed form of scenarios.json. The JSON file is the single
// source of truth; this struct mirrors it.
type Manifest struct {
	SchemaVersion int               `json:"schema_version"`
	Description   string            `json:"description"`
	Fixture       FixtureDefaults   `json:"fixture"`
	Scenarios     []ScenarioSpec    `json:"scenarios"`
}

// FixtureDefaults carries the default fixture-generation parameters.
type FixtureDefaults struct {
	DefaultSeed      int64    `json:"default_seed"`
	DefaultFileCount int      `json:"default_file_count"`
	DefaultLanguages []string `json:"default_languages"`
}

// ScenarioSpec is one canonical scenario (S1-S4).
type ScenarioSpec struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	Kind         string `json:"kind"`
	NeedsFixture bool   `json:"needs_fixture"`
	Metric       string `json:"metric"`
}

// ManifestPath returns the absolute path to scenarios.json relative to this
// source file's directory. It walks up from the working directory until it finds
// the manifest, so callers from any working directory resolve it.
func ManifestPath() (string, error) {
	// Try common relative locations first (fast path for in-tree callers).
	candidates := []string{
		"scenarios.json",
		filepath.Join("tests", "performance", "scenarios", "scenarios.json"),
		filepath.Join("helix_code", "tests", "performance", "scenarios", "scenarios.json"),
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if _, statErr := os.Stat(abs); statErr == nil {
				return abs, nil
			}
		}
	}
	// Walk up from cwd looking for the inner module's manifest.
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(dir, "helix_code", "tests", "performance", "scenarios", "scenarios.json")
		if _, statErr := os.Stat(p); statErr == nil {
			return p, nil
		}
		p = filepath.Join(dir, "tests", "performance", "scenarios", "scenarios.json")
		if _, statErr := os.Stat(p); statErr == nil {
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("scenarios.json not found from working directory")
}

// LoadManifest reads and parses scenarios.json from the given path. If path is
// empty, ManifestPath() is used to locate it.
func LoadManifest(path string) (*Manifest, error) {
	if path == "" {
		var err error
		path, err = ManifestPath()
		if err != nil {
			return nil, err
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if m.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported manifest schema_version %d", m.SchemaVersion)
	}
	if len(m.Scenarios) == 0 {
		return nil, fmt.Errorf("manifest %s defines no scenarios", path)
	}
	return &m, nil
}

// Scenario returns the spec for the given ID (e.g. "S2"), or false if absent.
func (m *Manifest) Scenario(id string) (ScenarioSpec, bool) {
	for _, s := range m.Scenarios {
		if s.ID == id {
			return s, true
		}
	}
	return ScenarioSpec{}, false
}

// IDs returns the scenario IDs sorted (S1..S4).
func (m *Manifest) IDs() []string {
	ids := make([]string, 0, len(m.Scenarios))
	for _, s := range m.Scenarios {
		ids = append(ids, s.ID)
	}
	sort.Strings(ids)
	return ids
}

// FingerprintFiles returns a deterministic content hash over the supplied set of
// relative paths and their byte contents within root. The same tree always
// yields the same fingerprint regardless of directory-walk order.
func FingerprintFiles(root string) (string, error) {
	type entry struct {
		rel  string
		hash [32]byte
	}
	var entries []entry
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		entries = append(entries, entry{rel: filepath.ToSlash(rel), hash: sha256.Sum256(data)})
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].rel < entries[j].rel })
	h := sha256.New()
	for _, e := range entries {
		h.Write([]byte(e.rel))
		h.Write([]byte{0})
		h.Write(e.hash[:])
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
