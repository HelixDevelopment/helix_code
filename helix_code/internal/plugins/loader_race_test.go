package plugins

// HXC-RACE: data-race regression guard (DEFECT-2).
//
// Loader.Load reads l.plugins[dep] (dependency-existence check) without holding
// l.mu, while it writes l.plugins[name] under l.mu.Lock(). Concurrent Load of
// dependency-declaring manifests races the unlocked map read against the locked
// map write. Run under `go test -race` this test reproduces the race on the
// broken artifact (RED) and passes cleanly on the fixed artifact (GREEN).

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

// TestLoader_ConcurrentDependencyLoad_Race exercises the unlocked map read at
// loader.go:30. Many goroutines concurrently Load manifests that each declare a
// dependency on "base" while other goroutines concurrently Load "base" itself
// (which writes the map under lock). Under -race this triggers the detector on
// the pre-fix code; it is race-free after the read is guarded.
func TestLoader_ConcurrentDependencyLoad_Race(t *testing.T) {
	dir := t.TempDir()

	// Manifest for the dependency that the others declare a dep on.
	baseManifest := filepath.Join(dir, "base-manifest.yaml")
	if err := os.WriteFile(baseManifest, []byte("name: base\nversion: 1.0.0\nentrypoint: main"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-create N dependent manifests, each declaring `dependencies: [base]`.
	const n = 40
	depManifests := make([]string, n)
	for i := 0; i < n; i++ {
		p := filepath.Join(dir, "dep-"+strconv.Itoa(i)+".yaml")
		content := "name: dep" + strconv.Itoa(i) + "\nversion: 1.0.0\nentrypoint: main\ndependencies:\n  - base\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		depManifests[i] = p
	}

	loader := NewLoader(dir)
	// Seed "base" so the dependency check has something to read.
	if _, err := loader.Load(context.Background(), baseManifest); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	start := make(chan struct{})

	// Writers: concurrently re-Load "base" (writes the map under lock).
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for j := 0; j < 25; j++ {
				_, _ = loader.Load(context.Background(), baseManifest)
			}
		}()
	}

	// Readers: concurrently Load dependent manifests (reads l.plugins[dep]).
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(mp string) {
			defer wg.Done()
			<-start
			_, _ = loader.Load(context.Background(), mp)
		}(depManifests[i])
	}

	close(start)
	wg.Wait()
}
