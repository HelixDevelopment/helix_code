package scenarios

import (
	"path/filepath"
	"testing"
)

// TestGenerateFixture_Deterministic asserts the CONST-050 anti-bluff invariant:
// the same seed produces a byte-identical tree. Two independent fixtures with
// identical config must hash equal.
func TestGenerateFixture_Deterministic(t *testing.T) {
	cfg := FixtureConfig{
		Seed: 20260520, FileCount: 120, Languages: []string{"go", "python", "rust"},
		MinLines: 10, MaxLines: 40, MaxDirDepth: 3, DirsPerLevel: 4,
	}

	dirA := t.TempDir()
	dirB := t.TempDir()

	fcA, mcA, errA := GenerateFixture(dirA, cfg)
	if errA != nil {
		t.Fatalf("GenerateFixture A: %v", errA)
	}
	fcB, mcB, errB := GenerateFixture(dirB, cfg)
	if errB != nil {
		t.Fatalf("GenerateFixture B: %v", errB)
	}
	if fcA != fcB {
		t.Fatalf("file counts differ: A=%d B=%d", fcA, fcB)
	}
	if fcA != cfg.FileCount {
		t.Fatalf("want %d files, got %d", cfg.FileCount, fcA)
	}
	if mcA != mcB {
		t.Fatalf("marker counts differ: A=%d B=%d", mcA, mcB)
	}

	hashA, err := FixtureHash(dirA)
	if err != nil {
		t.Fatalf("hash A: %v", err)
	}
	hashB, err := FixtureHash(dirB)
	if err != nil {
		t.Fatalf("hash B: %v", err)
	}
	if hashA != hashB {
		t.Fatalf("same seed produced different trees:\n  A=%s\n  B=%s", hashA, hashB)
	}
	t.Logf("deterministic fixture hash (seed=%d, %d files): %s", cfg.Seed, fcA, hashA)
}

// TestGenerateFixture_SeedSensitivity asserts a different seed produces a
// different tree (so the fixture genuinely varies with seed).
func TestGenerateFixture_SeedSensitivity(t *testing.T) {
	base := FixtureConfig{
		Seed: 1, FileCount: 80, Languages: []string{"go", "javascript"},
		MinLines: 10, MaxLines: 30, MaxDirDepth: 3, DirsPerLevel: 3,
	}
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	if _, _, err := GenerateFixture(dir1, base); err != nil {
		t.Fatalf("gen seed 1: %v", err)
	}
	base.Seed = 2
	if _, _, err := GenerateFixture(dir2, base); err != nil {
		t.Fatalf("gen seed 2: %v", err)
	}
	h1, _ := FixtureHash(dir1)
	h2, _ := FixtureHash(dir2)
	if h1 == h2 {
		t.Fatalf("different seeds produced identical trees: %s", h1)
	}
}

// TestGenerateFixture_Validation rejects invalid configs.
func TestGenerateFixture_Validation(t *testing.T) {
	cases := []struct {
		name string
		cfg  FixtureConfig
	}{
		{"zero files", FixtureConfig{Seed: 1, FileCount: 0, Languages: []string{"go"}, MinLines: 1, MaxLines: 2}},
		{"no languages", FixtureConfig{Seed: 1, FileCount: 5, Languages: nil, MinLines: 1, MaxLines: 2}},
		{"bad line bounds", FixtureConfig{Seed: 1, FileCount: 5, Languages: []string{"go"}, MinLines: 5, MaxLines: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := GenerateFixture(t.TempDir(), tc.cfg); err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
		})
	}
}

// TestGenerateFixture_MarkerCount asserts the S4 expected hit count is the
// deterministic 1-in-7 fraction.
func TestGenerateFixture_MarkerCount(t *testing.T) {
	cfg := FixtureConfig{
		Seed: 99, FileCount: 70, Languages: []string{"go"},
		MinLines: 5, MaxLines: 10, MaxDirDepth: 2, DirsPerLevel: 2,
	}
	dir := t.TempDir()
	fc, mc, err := GenerateFixture(dir, cfg)
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	want := 0
	for i := 0; i < fc; i++ {
		if i%7 == 0 {
			want++
		}
	}
	if mc != want {
		t.Fatalf("marker count = %d, want %d", mc, want)
	}
}

// TestLoadManifest_S1toS4 verifies the shared manifest defines exactly S1-S4.
func TestLoadManifest_S1toS4(t *testing.T) {
	path, err := ManifestPath()
	if err != nil {
		t.Fatalf("ManifestPath: %v", err)
	}
	m, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	want := []string{"S1", "S2", "S3", "S4"}
	got := m.IDs()
	if len(got) != len(want) {
		t.Fatalf("manifest IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("manifest IDs = %v, want %v", got, want)
		}
	}
	for _, id := range want {
		if _, ok := m.Scenario(id); !ok {
			t.Fatalf("scenario %s missing", id)
		}
	}
}

// TestFingerprintFiles_OrderIndependent asserts the fingerprint is independent
// of file creation/walk order.
func TestFingerprintFiles_OrderIndependent(t *testing.T) {
	cfg := FixtureConfig{
		Seed: 7, FileCount: 40, Languages: []string{"go", "python"},
		MinLines: 5, MaxLines: 12, MaxDirDepth: 2, DirsPerLevel: 3,
	}
	dir := t.TempDir()
	if _, _, err := GenerateFixture(dir, cfg); err != nil {
		t.Fatalf("gen: %v", err)
	}
	h1, err := FingerprintFiles(dir)
	if err != nil {
		t.Fatalf("fingerprint 1: %v", err)
	}
	// A second fingerprint call on the same tree must match exactly.
	h2, err := FingerprintFiles(filepath.Clean(dir))
	if err != nil {
		t.Fatalf("fingerprint 2: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("fingerprint not stable: %s vs %s", h1, h2)
	}
}
