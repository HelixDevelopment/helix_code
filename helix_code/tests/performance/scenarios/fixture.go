package scenarios

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FixtureConfig parameterizes the deterministic large-repo fixture generator.
// Same Seed + same other fields => byte-identical tree (verified by the unit
// tests via FingerprintFiles). This determinism is the anti-bluff guarantee:
// a benchmark run on the fixture is reproducible.
type FixtureConfig struct {
	// Seed drives every pseudo-random choice. Identical Seed => identical tree.
	Seed int64
	// FileCount is the total number of source files to synthesize.
	FileCount int
	// Languages is the set of language extensions to spread files across.
	Languages []string
	// MinLines / MaxLines bound per-file size.
	MinLines int
	MaxLines int
	// MaxDirDepth bounds the directory nesting depth.
	MaxDirDepth int
	// DirsPerLevel bounds the directory fan-out at each level.
	DirsPerLevel int
}

// DefaultFixtureConfig builds a FixtureConfig from the manifest defaults.
func DefaultFixtureConfig(m *Manifest) FixtureConfig {
	langs := m.Fixture.DefaultLanguages
	if len(langs) == 0 {
		langs = []string{"go", "python", "javascript"}
	}
	return FixtureConfig{
		Seed:         m.Fixture.DefaultSeed,
		FileCount:    m.Fixture.DefaultFileCount,
		Languages:    langs,
		MinLines:     20,
		MaxLines:     220,
		MaxDirDepth:  5,
		DirsPerLevel: 6,
	}
}

// extForLang maps a language token to a file extension.
var extForLang = map[string]string{
	"go":         "go",
	"python":     "py",
	"javascript": "js",
	"typescript": "ts",
	"java":       "java",
	"rust":       "rs",
	"c":          "c",
	"cpp":        "cpp",
	"ruby":       "rb",
}

// SearchToken is the deterministic token embedded in a known fraction of files.
// Scenario S4 (content-search) searches for it; the count is reproducible.
const SearchToken = "HELIX_SPEED_FIXTURE_MARKER"

// GenerateFixture deterministically synthesizes a representative source tree
// under root. root must be an existing empty (or non-existent) directory; it is
// created if absent. The same FixtureConfig always produces the identical tree.
//
// Returns the number of files written and the number of files that contain
// SearchToken (the expected S4 hit count).
func GenerateFixture(root string, cfg FixtureConfig) (fileCount int, markerCount int, err error) {
	if cfg.FileCount <= 0 {
		return 0, 0, fmt.Errorf("FixtureConfig.FileCount must be > 0, got %d", cfg.FileCount)
	}
	if len(cfg.Languages) == 0 {
		return 0, 0, fmt.Errorf("FixtureConfig.Languages must be non-empty")
	}
	if cfg.MinLines <= 0 || cfg.MaxLines < cfg.MinLines {
		return 0, 0, fmt.Errorf("invalid line bounds min=%d max=%d", cfg.MinLines, cfg.MaxLines)
	}
	if cfg.MaxDirDepth < 1 {
		cfg.MaxDirDepth = 1
	}
	if cfg.DirsPerLevel < 1 {
		cfg.DirsPerLevel = 1
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return 0, 0, fmt.Errorf("create fixture root: %w", err)
	}

	// Sort languages for deterministic ordering regardless of caller input order.
	langs := append([]string(nil), cfg.Languages...)
	sort.Strings(langs)

	rng := rand.New(rand.NewSource(cfg.Seed))

	// Pre-generate a deterministic set of directory paths.
	dirs := buildDirTree(rng, cfg.MaxDirDepth, cfg.DirsPerLevel)

	for i := 0; i < cfg.FileCount; i++ {
		lang := langs[rng.Intn(len(langs))]
		ext := extForLang[lang]
		if ext == "" {
			ext = lang
		}
		dir := dirs[rng.Intn(len(dirs))]
		name := fmt.Sprintf("file_%05d.%s", i, ext)
		relPath := filepath.Join(dir, name)
		absPath := filepath.Join(root, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return fileCount, markerCount, fmt.Errorf("mkdir for %s: %w", relPath, err)
		}
		// Every 7th file embeds the search marker — deterministic S4 hit count.
		withMarker := i%7 == 0
		content := synthFile(rng, lang, i, cfg.MinLines, cfg.MaxLines, withMarker)
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			return fileCount, markerCount, fmt.Errorf("write %s: %w", relPath, err)
		}
		fileCount++
		if withMarker {
			markerCount++
		}
	}
	return fileCount, markerCount, nil
}

// buildDirTree deterministically produces a slice of relative directory paths.
func buildDirTree(rng *rand.Rand, maxDepth, fanout int) []string {
	dirs := []string{"."}
	frontier := []string{"."}
	segWords := []string{"core", "api", "internal", "service", "model", "util",
		"handler", "client", "store", "engine", "parser", "render"}
	for depth := 0; depth < maxDepth; depth++ {
		var next []string
		for _, parent := range frontier {
			n := 1 + rng.Intn(fanout)
			for k := 0; k < n; k++ {
				seg := fmt.Sprintf("%s_%d", segWords[rng.Intn(len(segWords))], k)
				var p string
				if parent == "." {
					p = seg
				} else {
					p = filepath.Join(parent, seg)
				}
				dirs = append(dirs, p)
				next = append(next, p)
			}
		}
		frontier = next
	}
	sort.Strings(dirs)
	return dirs
}

// synthFile produces deterministic source-like content for a file.
func synthFile(rng *rand.Rand, lang string, idx, minLines, maxLines int, withMarker bool) string {
	lines := minLines + rng.Intn(maxLines-minLines+1)
	var b strings.Builder
	comment := commentPrefix(lang)
	fmt.Fprintf(&b, "%s synthesized speed-fixture file #%d (%s)\n", comment, idx, lang)
	fmt.Fprintf(&b, "%s deterministic content — regenerate with the scenario fixture generator\n", comment)
	identifiers := []string{"compute", "process", "transform", "validate", "resolve",
		"dispatch", "collect", "merge", "filter", "aggregate"}
	for ln := 0; ln < lines; ln++ {
		id := identifiers[rng.Intn(len(identifiers))]
		switch lang {
		case "go":
			fmt.Fprintf(&b, "func %s_%d_%d(x int) int { return x*%d + %d }\n", id, idx, ln, rng.Intn(97)+1, rng.Intn(97))
		case "python":
			fmt.Fprintf(&b, "def %s_%d_%d(x):\n    return x * %d + %d\n", id, idx, ln, rng.Intn(97)+1, rng.Intn(97))
		case "java":
			fmt.Fprintf(&b, "int %s_%d_%d(int x) { return x*%d + %d; }\n", id, idx, ln, rng.Intn(97)+1, rng.Intn(97))
		case "rust":
			fmt.Fprintf(&b, "fn %s_%d_%d(x: i64) -> i64 { x*%d + %d }\n", id, idx, ln, rng.Intn(97)+1, rng.Intn(97))
		default: // javascript, typescript, fallthrough
			fmt.Fprintf(&b, "function %s_%d_%d(x) { return x*%d + %d; }\n", id, idx, ln, rng.Intn(97)+1, rng.Intn(97))
		}
	}
	if withMarker {
		fmt.Fprintf(&b, "%s %s line %d\n", comment, SearchToken, idx)
	}
	return b.String()
}

func commentPrefix(lang string) string {
	switch lang {
	case "python", "ruby":
		return "#"
	default:
		return "//"
	}
}

// FixtureHash returns a deterministic fingerprint of a generated fixture, used
// by tests to assert same-seed => same-tree.
func FixtureHash(root string) (string, error) {
	return FingerprintFiles(root)
}
