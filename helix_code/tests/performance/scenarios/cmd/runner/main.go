// Command runner is the Go entry point for the speed-programme canonical
// scenario harness (R4 phased plan P0-T04). It can:
//
//	-gen-fixture <dir>   deterministically generate the large-repo fixture
//	-run                 execute scenarios S1-S4 once and print timing
//	-runs N              with -run, repeat N times and report per-scenario variance
//	-fixture <dir>       fixture root for S3/S4 (generate it first with -gen-fixture)
//	-seed N              fixture seed (default: manifest default)
//	-files N             fixture file count (default: manifest default)
//	-json                emit machine-readable JSON instead of a table
//
// Phase 0 is the measurement baseline — this command changes no production code.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"time"

	scenarios "dev.helix.code/tests/performance/scenarios"
)

func main() {
	var (
		genFixture = flag.String("gen-fixture", "", "generate the deterministic large-repo fixture into this directory")
		run        = flag.Bool("run", false, "execute scenarios S1-S4")
		runs       = flag.Int("runs", 1, "number of repeats per scenario (variance reporting)")
		fixture    = flag.String("fixture", "", "fixture root directory for S3/S4")
		seed       = flag.Int64("seed", 0, "fixture seed (0 = manifest default)")
		files      = flag.Int("files", 0, "fixture file count (0 = manifest default)")
		asJSON     = flag.Bool("json", false, "emit JSON instead of a table")
	)
	flag.Parse()

	manifest, err := scenarios.LoadManifest("")
	if err != nil {
		fatal("load manifest: %v", err)
	}

	if *genFixture != "" {
		cfg := scenarios.DefaultFixtureConfig(manifest)
		if *seed != 0 {
			cfg.Seed = *seed
		}
		if *files != 0 {
			cfg.FileCount = *files
		}
		fc, mc, genErr := scenarios.GenerateFixture(*genFixture, cfg)
		if genErr != nil {
			fatal("generate fixture: %v", genErr)
		}
		hash, hErr := scenarios.FixtureHash(*genFixture)
		if hErr != nil {
			fatal("fixture hash: %v", hErr)
		}
		fmt.Printf("fixture generated: dir=%s seed=%d files=%d marker_files=%d hash=%s\n",
			*genFixture, cfg.Seed, fc, mc, hash[:16])
		if !*run {
			return
		}
	}

	if !*run {
		flag.Usage()
		return
	}

	if *runs < 1 {
		*runs = 1
	}
	ctx := context.Background()
	opts := scenarios.RunOptions{Manifest: manifest, FixtureRoot: *fixture}

	// scenarioID -> durations across runs (ms)
	collected := map[string][]float64{}
	seen := map[string]bool{}
	var order []string
	lastDetail := map[string]scenarios.Result{}

	for i := 0; i < *runs; i++ {
		results, runErr := scenarios.RunAll(ctx, opts)
		if runErr != nil {
			fatal("run scenarios: %v", runErr)
		}
		for _, r := range results {
			if !seen[r.ScenarioID] {
				seen[r.ScenarioID] = true
				order = append(order, r.ScenarioID)
			}
			lastDetail[r.ScenarioID] = r
			if r.Skipped {
				continue
			}
			collected[r.ScenarioID] = append(collected[r.ScenarioID],
				float64(r.Duration.Nanoseconds())/1e6)
		}
	}

	if *asJSON {
		emitJSON(order, collected, lastDetail, *runs)
		return
	}
	emitTable(order, collected, lastDetail, manifest, *runs)
}

func emitTable(order []string, collected map[string][]float64,
	last map[string]scenarios.Result, m *scenarios.Manifest, runs int) {
	fmt.Printf("speed-scenario harness — %d run(s) per scenario\n", runs)
	fmt.Printf("%-5s %-16s %12s %12s %12s %10s  %s\n",
		"ID", "name", "mean_ms", "min_ms", "max_ms", "cv_%", "status")
	for _, id := range order {
		spec, _ := m.Scenario(id)
		r := last[id]
		samples := collected[id]
		if r.Skipped || len(samples) == 0 {
			fmt.Printf("%-5s %-16s %12s %12s %12s %10s  SKIPPED: %s\n",
				id, spec.Name, "-", "-", "-", "-", r.SkipReason)
			continue
		}
		mean, min, max, cv := stats(samples)
		fmt.Printf("%-5s %-16s %12.3f %12.3f %12.3f %10.2f  ok (%s)\n",
			id, spec.Name, mean, min, max, cv, r.Detail)
	}
}

func emitJSON(order []string, collected map[string][]float64,
	last map[string]scenarios.Result, runs int) {
	type sc struct {
		ID       string    `json:"id"`
		Name     string    `json:"name"`
		Runs     int       `json:"runs"`
		Skipped  bool      `json:"skipped"`
		Skip     string    `json:"skip_reason,omitempty"`
		Detail   string    `json:"detail,omitempty"`
		MeanMs   float64   `json:"mean_ms,omitempty"`
		MinMs    float64   `json:"min_ms,omitempty"`
		MaxMs    float64   `json:"max_ms,omitempty"`
		CVPct    float64   `json:"cv_pct,omitempty"`
		Samples  []float64 `json:"samples_ms,omitempty"`
	}
	out := struct {
		GeneratedAt string `json:"generated_at"`
		RunsPerScn  int    `json:"runs_per_scenario"`
		Scenarios   []sc   `json:"scenarios"`
	}{GeneratedAt: time.Now().UTC().Format(time.RFC3339), RunsPerScn: runs}
	for _, id := range order {
		r := last[id]
		entry := sc{ID: id, Name: r.Name, Runs: runs, Detail: r.Detail}
		samples := collected[id]
		if r.Skipped || len(samples) == 0 {
			entry.Skipped = true
			entry.Skip = r.SkipReason
		} else {
			entry.MeanMs, entry.MinMs, entry.MaxMs, entry.CVPct = stats(samples)
			entry.Samples = samples
		}
		out.Scenarios = append(out.Scenarios, entry)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// stats returns mean, min, max, and coefficient of variation (%) for samples.
func stats(samples []float64) (mean, min, max, cvPct float64) {
	if len(samples) == 0 {
		return 0, 0, 0, 0
	}
	min, max = samples[0], samples[0]
	var sum float64
	for _, v := range samples {
		sum += v
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	mean = sum / float64(len(samples))
	if len(samples) > 1 && mean > 0 {
		var sq float64
		for _, v := range samples {
			d := v - mean
			sq += d * d
		}
		std := math.Sqrt(sq / float64(len(samples)-1))
		cvPct = std / mean * 100
	}
	return mean, min, max, cvPct
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "runner: "+format+"\n", args...)
	os.Exit(1)
}
