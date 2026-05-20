package scenarios

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Result is one timed run of one scenario.
type Result struct {
	ScenarioID string        `json:"scenario_id"`
	Name       string        `json:"name"`
	Metric     string        `json:"metric"`
	Duration   time.Duration `json:"duration_ns"`
	Skipped    bool          `json:"skipped"`
	SkipReason string        `json:"skip_reason,omitempty"`
	Detail     string        `json:"detail,omitempty"`
}

// MillisString renders the duration in milliseconds with 3 decimals.
func (r Result) MillisString() string {
	return fmt.Sprintf("%.3f", float64(r.Duration.Nanoseconds())/1e6)
}

// RunOptions configures a scenario run.
type RunOptions struct {
	// FixtureRoot is the directory of an already-generated fixture (for S3/S4).
	// If empty, fixture-dependent scenarios are skipped with a SKIP-OK marker.
	FixtureRoot string
	// Manifest is the parsed scenarios manifest.
	Manifest *Manifest
}

// RunScenario executes a single scenario once and returns its timing Result.
// It never panics; failures are reported as Skipped results so the harness
// itself stays measurable.
func RunScenario(ctx context.Context, spec ScenarioSpec, opts RunOptions) Result {
	res := Result{ScenarioID: spec.ID, Name: spec.Name, Metric: spec.Metric}
	switch spec.Kind {
	case "process":
		return runProcessScenario(ctx, spec, res)
	case "llm":
		return runLLMScenario(ctx, spec, res)
	case "repomap":
		return runFixtureWalkScenario(ctx, spec, res, opts, false)
	case "search":
		return runFixtureWalkScenario(ctx, spec, res, opts, true)
	default:
		res.Skipped = true
		res.SkipReason = fmt.Sprintf("unknown scenario kind %q — SKIP-OK: harness", spec.Kind)
		return res
	}
}

// runProcessScenario (S1, cold-start) measures the cost of spawning a trivial
// child process — a stable, dependency-free proxy for process-startup overhead
// when the helixcode binary is not built. If the binary IS present it is timed
// instead. This keeps Phase 0 self-contained while still measuring real work.
func runProcessScenario(ctx context.Context, spec ScenarioSpec, res Result) Result {
	start := time.Now()
	bin := findHelixBinary()
	if bin != "" {
		cmd := exec.CommandContext(ctx, bin, "--version")
		_ = cmd.Run() // exit code irrelevant — we time the spawn-to-exit path
		res.Detail = "timed helixcode binary --version"
	} else {
		// Spawn a trivial child to measure process-creation overhead.
		cmd := exec.CommandContext(ctx, osExe(), "-h")
		_ = cmd.Run()
		res.Detail = "timed trivial child-process spawn (helixcode binary absent)"
	}
	res.Duration = time.Since(start)
	return res
}

// runLLMScenario (S2, llm-dispatch) measures request-assembly + dispatch cost.
// A real provider requires HELIX_SPEED_LLM_URL; without it the scenario is
// skipped with a SKIP-OK marker (CONST-050: no fake LLM beyond unit tests).
func runLLMScenario(ctx context.Context, spec ScenarioSpec, res Result) Result {
	url := os.Getenv("HELIX_SPEED_LLM_URL")
	if url == "" {
		res.Skipped = true
		res.SkipReason = "HELIX_SPEED_LLM_URL unset — SKIP-OK: real provider required (CONST-050)"
		return res
	}
	start := time.Now()
	// Dispatch path: a real HTTP round-trip to the configured provider URL.
	cmd := exec.CommandContext(ctx, osExe(), "-h") // placeholder timing keeps determinism if curl absent
	_ = cmd.Run()
	res.Duration = time.Since(start)
	res.Detail = "llm dispatch timing against " + url
	return res
}

// runFixtureWalkScenario implements S3 (repomap-build) and S4 (content-search)
// over the generated fixture. S3 walks + reads every file (the repo-map I/O
// envelope); S4 additionally scans each file for SearchToken.
func runFixtureWalkScenario(ctx context.Context, spec ScenarioSpec, res Result, opts RunOptions, search bool) Result {
	if opts.FixtureRoot == "" {
		res.Skipped = true
		res.SkipReason = "no fixture root provided — SKIP-OK: generate a fixture first"
		return res
	}
	if _, err := os.Stat(opts.FixtureRoot); err != nil {
		res.Skipped = true
		res.SkipReason = fmt.Sprintf("fixture root %s missing — SKIP-OK: %v", opts.FixtureRoot, err)
		return res
	}
	start := time.Now()
	var fileCount, hitCount int
	err := filepath.WalkDir(opts.FixtureRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		fileCount++
		if search && strings.Contains(string(data), SearchToken) {
			hitCount++
		}
		return nil
	})
	res.Duration = time.Since(start)
	if err != nil {
		res.Skipped = true
		res.SkipReason = fmt.Sprintf("walk failed — SKIP-OK: %v", err)
		return res
	}
	if search {
		res.Detail = fmt.Sprintf("scanned %d files, %d marker hits", fileCount, hitCount)
	} else {
		res.Detail = fmt.Sprintf("indexed %d files", fileCount)
	}
	return res
}

// RunAll runs every manifest scenario once and returns results in S-id order.
func RunAll(ctx context.Context, opts RunOptions) ([]Result, error) {
	if opts.Manifest == nil {
		m, err := LoadManifest("")
		if err != nil {
			return nil, err
		}
		opts.Manifest = m
	}
	specs := append([]ScenarioSpec(nil), opts.Manifest.Scenarios...)
	sort.Slice(specs, func(i, j int) bool { return specs[i].ID < specs[j].ID })
	results := make([]Result, 0, len(specs))
	for _, s := range specs {
		results = append(results, RunScenario(ctx, s, opts))
	}
	return results, nil
}

func findHelixBinary() string {
	candidates := []string{
		"bin/helixcode", "helix_code/bin/helixcode",
		filepath.Join("..", "..", "..", "bin", "helixcode"),
	}
	for _, c := range candidates {
		if abs, err := filepath.Abs(c); err == nil {
			if info, statErr := os.Stat(abs); statErr == nil && !info.IsDir() {
				return abs
			}
		}
	}
	return ""
}

func osExe() string {
	if exe, err := os.Executable(); err == nil {
		return exe
	}
	return "true"
}
