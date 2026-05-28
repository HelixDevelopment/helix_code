package repomap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 stress coverage for the REAL repomap.RepoMap.
//
// The unit under stress is the actual *RepoMap built over a real on-disk repo
// tree (t.TempDir() populated with real Go/Python/JS/etc. source files). No
// fakes: every iteration runs the real discover -> tree-sitter parse -> rank ->
// token-budget pipeline (GetOptimalContext / GetStatistics / RefreshCache) and
// the real disk-backed RepoCache with its background-writer goroutine.
//
// Coverage:
//   - sustained query load (N>=100, p50/p95/p99 captured) on GetOptimalContext,
//   - sustained statistics load on GetStatistics,
//   - N>=10 concurrent goroutines interleaving GetOptimalContext / GetStatistics
//     / RefreshCache / InvalidateFile against the SAME RepoMap (RWMutex + the
//     parser pool + the cache's RWMutex all under genuine contention; run under
//     -race to catch data races in the parse/cache/rank paths),
//   - boundary conditions: empty repo, deeply nested tree, many files.

// realSourceFiles returns a representative spread of REAL, parseable source
// files (one per supported language) keyed by relative path. The content is
// genuine code so tree-sitter actually extracts symbols — proving real work.
func realSourceFiles() map[string]string {
	return map[string]string{
		"go/server.go": `package server

import "fmt"

// Server is a real Go type the parser will extract.
type Server struct {
	Addr string
	Port int
}

func NewServer(addr string, port int) *Server {
	return &Server{Addr: addr, Port: port}
}

func (s *Server) Start() error {
	fmt.Printf("listening on %s:%d\n", s.Addr, s.Port)
	return nil
}

func (s *Server) Stop() error { return nil }
`,
		"go/auth.go": `package server

type Authenticator interface {
	Authenticate(token string) (bool, error)
}

type jwtAuth struct{ secret string }

func (j *jwtAuth) Authenticate(token string) (bool, error) {
	return token == j.secret, nil
}
`,
		"py/handler.py": `class RequestHandler:
    def __init__(self, name):
        self.name = name

    def handle(self, request):
        return {"handled_by": self.name, "request": request}

def make_handler(name):
    return RequestHandler(name)
`,
		"js/client.js": `class ApiClient {
  constructor(baseUrl) {
    this.baseUrl = baseUrl;
  }
  get(path) {
    return fetch(this.baseUrl + path);
  }
}

function createClient(url) {
  return new ApiClient(url);
}
`,
		"ts/model.ts": `export class UserModel {
  constructor(public id: number, public name: string) {}
  greet(): string {
    return "hello " + this.name;
  }
}

export function userFactory(id: number): UserModel {
  return new UserModel(id, "anon");
}
`,
		"java/Worker.java": `public class Worker {
    private final int id;
    public Worker(int id) { this.id = id; }
    public int getId() { return id; }
    public void run() { System.out.println("worker " + id); }
}
`,
		"rust/lib.rs": `pub struct Point {
    pub x: i32,
    pub y: i32,
}

pub fn distance(a: &Point, b: &Point) -> i32 {
    (a.x - b.x).abs() + (a.y - b.y).abs()
}
`,
		"c/util.c": `#include <stdio.h>

int add(int a, int b) {
    return a + b;
}

void greet(const char *name) {
    printf("hi %s\n", name);
}
`,
		"rb/service.rb": `class Service
  def initialize(name)
    @name = name
  end

  def call
    "service #{@name}"
  end
end
`,
	}
}

// buildRealRepo materialises a real source tree under t.TempDir() and returns
// the root path. The number of duplicated copies multiplies the base set so a
// caller can scale the file count for sustained/concurrent load.
func buildRealRepo(t testing.TB, copies int) string {
	t.Helper()
	root := t.TempDir()
	base := realSourceFiles()
	for c := 0; c < copies; c++ {
		for rel, content := range base {
			// Each copy lands in its own subtree so paths stay unique.
			dst := filepath.Join(root, fmt.Sprintf("copy%02d", c), filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
			}
			if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
				t.Fatalf("write %s: %v", dst, err)
			}
		}
	}
	return root
}

// newStressRepoMap builds a RepoMap over a real repo and registers cache cleanup.
func newStressRepoMap(t testing.TB, copies int, cacheEnabled bool) *RepoMap {
	t.Helper()
	root := buildRealRepo(t, copies)
	cfg := DefaultConfig()
	cfg.CacheEnabled = cacheEnabled
	cfg.MaxConcurrency = 4
	rm, err := NewRepoMap(root, cfg)
	if err != nil {
		t.Fatalf("NewRepoMap: %v", err)
	}
	if cacheEnabled && rm.cache != nil {
		t.Cleanup(func() { _ = rm.cache.Close() })
	}
	return rm
}

// TestRepoMap_Stress_SustainedGetOptimalContext drives the real
// discover->parse->rank->budget pipeline under sustained load (N>=100),
// recording per-call latency. Each iteration asks for context with a varying
// query and asserts the pipeline returns a non-empty, well-formed context set —
// so the run proves real parsing/ranking work, not a cached no-op shortcut.
func TestRepoMap_Stress_SustainedGetOptimalContext(t *testing.T) {
	rm := newStressRepoMap(t, 3, true) // 3 copies * 9 langs = 27 real source files

	queries := []string{"Server", "Authenticate", "handle", "ApiClient", "Worker", "distance", "", "UserModel"}

	var calls int64
	rm.cache.ResetStats()
	stresschaos.RunSustainedLoad(t, "repomap_sustained_get_optimal_context",
		stresschaos.SustainedConfig{N: 200, MaxErrorRate: 0.0},
		func(i int) error {
			q := queries[i%len(queries)]
			contexts, err := rm.GetOptimalContext(q, nil)
			if err != nil {
				return fmt.Errorf("GetOptimalContext(%q): %w", q, err)
			}
			if len(contexts) == 0 {
				return fmt.Errorf("GetOptimalContext(%q) returned 0 contexts over a 27-file repo", q)
			}
			// Each returned context must reference a real file with real content.
			for _, c := range contexts {
				if c.FilePath == "" {
					return fmt.Errorf("context with empty FilePath")
				}
				if c.TokenCount < 0 {
					return fmt.Errorf("negative token count %d for %s", c.TokenCount, c.FilePath)
				}
			}
			atomic.AddInt64(&calls, 1)
			return nil
		})

	if atomic.LoadInt64(&calls) == 0 {
		t.Fatal("repomap produced zero contexts under sustained load — not real work")
	}
	hits, misses := rm.cache.Stats()
	t.Logf("repomap sustained GetOptimalContext: %d calls, cache hits=%d misses=%d", atomic.LoadInt64(&calls), hits, misses)
	if hits == 0 {
		t.Fatal("cache reported zero hits across 200 context builds — caching path not exercised")
	}
}

// TestRepoMap_Stress_SustainedStatistics drives the real GetStatistics pipeline
// (discover + language tally + cache-probe + parse-per-file) under sustained
// load, asserting consistent file/symbol totals every call.
func TestRepoMap_Stress_SustainedStatistics(t *testing.T) {
	rm := newStressRepoMap(t, 2, true)

	first, err := rm.GetStatistics()
	if err != nil {
		t.Fatalf("initial GetStatistics: %v", err)
	}
	if first.TotalFiles == 0 || first.TotalSymbols == 0 {
		t.Fatalf("expected non-zero files/symbols, got files=%d symbols=%d", first.TotalFiles, first.TotalSymbols)
	}

	stresschaos.RunSustainedLoad(t, "repomap_sustained_statistics",
		stresschaos.SustainedConfig{N: 150, MaxErrorRate: 0.0},
		func(i int) error {
			stats, err := rm.GetStatistics()
			if err != nil {
				return fmt.Errorf("GetStatistics: %w", err)
			}
			// The repo is immutable during the run — totals must be stable.
			if stats.TotalFiles != first.TotalFiles {
				return fmt.Errorf("file count drift: got %d want %d", stats.TotalFiles, first.TotalFiles)
			}
			if stats.TotalSymbols != first.TotalSymbols {
				return fmt.Errorf("symbol count drift: got %d want %d", stats.TotalSymbols, first.TotalSymbols)
			}
			return nil
		})

	t.Logf("repomap sustained GetStatistics: files=%d symbols=%d langs=%d",
		first.TotalFiles, first.TotalSymbols, len(first.Languages))
}

// TestRepoMap_Stress_ConcurrentMixedOps hammers a SINGLE shared *RepoMap from
// N>=10 goroutines that interleave GetOptimalContext (RLock + parser pool +
// cache), GetStatistics (RLock + parser pool + cache probe), RefreshCache
// (RLock + cache writes) and InvalidateFile (write Lock + cache delete). This
// drives genuine RWMutex read/write contention, parser-pool sharing, and
// cache-map contention simultaneously. Run under -race to catch data races in
// any of those paths; the harness also fails on deadlock or goroutine leak.
func TestRepoMap_Stress_ConcurrentMixedOps(t *testing.T) {
	rm := newStressRepoMap(t, 3, true) // real cache so cache-map contention is real
	queries := []string{"Server", "handle", "ApiClient", "Worker", "distance", ""}

	// Snapshot a real file path from the repo so InvalidateFile targets a real key.
	var sampleFile string
	_ = filepath.Walk(rm.rootPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".go") && sampleFile == "" {
			sampleFile = path
		}
		return nil
	})
	if sampleFile == "" {
		t.Fatal("no .go file found in built repo")
	}

	var ops int64
	stresschaos.RunConcurrent(t, "repomap_concurrent_mixed_ops",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 40, Timeout: 60 * time.Second},
		func(g, it int) error {
			switch (g + it) % 4 {
			case 0:
				ctxs, err := rm.GetOptimalContext(queries[(g+it)%len(queries)], nil)
				if err != nil {
					return fmt.Errorf("GetOptimalContext: %w", err)
				}
				if len(ctxs) == 0 {
					return fmt.Errorf("empty context under concurrency")
				}
			case 1:
				stats, err := rm.GetStatistics()
				if err != nil {
					return fmt.Errorf("GetStatistics: %w", err)
				}
				if stats.TotalFiles == 0 {
					return fmt.Errorf("zero files under concurrency")
				}
			case 2:
				if err := rm.RefreshCache(); err != nil {
					return fmt.Errorf("RefreshCache: %w", err)
				}
			default:
				if err := rm.InvalidateFile(sampleFile); err != nil {
					return fmt.Errorf("InvalidateFile: %w", err)
				}
			}
			atomic.AddInt64(&ops, 1)
			return nil
		})

	if atomic.LoadInt64(&ops) == 0 {
		t.Fatal("zero mixed ops executed under concurrent load")
	}
	// The RepoMap must still serve correct results after the contention storm.
	post, err := rm.GetStatistics()
	if err != nil {
		t.Fatalf("post-contention GetStatistics: %v", err)
	}
	if post.TotalFiles == 0 || post.TotalSymbols == 0 {
		t.Fatalf("repomap corrupted after contention: files=%d symbols=%d", post.TotalFiles, post.TotalSymbols)
	}
	t.Logf("repomap concurrent mixed ops: %d ops, post-run files=%d symbols=%d",
		atomic.LoadInt64(&ops), post.TotalFiles, post.TotalSymbols)
}

// TestRepoMap_Stress_BoundaryConditions exercises the §11.4.85(A)(3) boundary
// cases against the REAL RepoMap: (empty) a repo with no source files must
// return an empty-but-non-error context/stats; (max) many files must all be
// discovered; (off-by-one / deep nesting) a deeply nested tree must still be
// fully walked and parsed.
func TestRepoMap_Stress_BoundaryConditions(t *testing.T) {
	// Empty: a directory with no supported source files.
	t.Run("empty_repo", func(t *testing.T) {
		root := t.TempDir()
		// Drop one unsupported file so the dir isn't literally empty but still
		// yields zero supported files.
		if err := os.WriteFile(filepath.Join(root, "README.txt"), []byte("no code here"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := DefaultConfig()
		cfg.CacheEnabled = false
		rm, err := NewRepoMap(root, cfg)
		if err != nil {
			t.Fatalf("NewRepoMap: %v", err)
		}
		ctxs, err := rm.GetOptimalContext("anything", nil)
		if err != nil {
			t.Fatalf("GetOptimalContext on empty repo must not error: %v", err)
		}
		if len(ctxs) != 0 {
			t.Fatalf("empty repo should yield 0 contexts, got %d", len(ctxs))
		}
		stats, err := rm.GetStatistics()
		if err != nil {
			t.Fatalf("GetStatistics on empty repo must not error: %v", err)
		}
		if stats.TotalFiles != 0 || stats.TotalSymbols != 0 {
			t.Fatalf("empty repo stats should be zero, got files=%d symbols=%d", stats.TotalFiles, stats.TotalSymbols)
		}
	})

	// Max: many real files must all be discovered.
	t.Run("many_files", func(t *testing.T) {
		rm := newStressRepoMap(t, 10, false) // 10 * 9 = 90 files
		stats, err := rm.GetStatistics()
		if err != nil {
			t.Fatalf("GetStatistics: %v", err)
		}
		if stats.TotalFiles != 90 {
			t.Fatalf("expected 90 discovered files, got %d", stats.TotalFiles)
		}
		if stats.TotalSymbols == 0 {
			t.Fatal("expected non-zero symbols across 90 files")
		}
	})

	// Deep nesting: a file buried many directories deep must still be found.
	t.Run("deep_nesting", func(t *testing.T) {
		root := t.TempDir()
		deep := root
		for i := 0; i < 40; i++ {
			deep = filepath.Join(deep, fmt.Sprintf("lvl%02d", i))
		}
		if err := os.MkdirAll(deep, 0o755); err != nil {
			t.Fatal(err)
		}
		buried := filepath.Join(deep, "buried.go")
		if err := os.WriteFile(buried, []byte("package buried\nfunc Deep() int { return 42 }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg := DefaultConfig()
		cfg.CacheEnabled = false
		rm, err := NewRepoMap(root, cfg)
		if err != nil {
			t.Fatalf("NewRepoMap: %v", err)
		}
		stats, err := rm.GetStatistics()
		if err != nil {
			t.Fatalf("GetStatistics: %v", err)
		}
		if stats.TotalFiles != 1 {
			t.Fatalf("expected to discover the 1 deeply-nested file, got %d", stats.TotalFiles)
		}
		ctxs, err := rm.GetOptimalContext("Deep", nil)
		if err != nil {
			t.Fatalf("GetOptimalContext: %v", err)
		}
		if len(ctxs) != 1 {
			t.Fatalf("expected 1 context for deeply-nested file, got %d", len(ctxs))
		}
	})
}

// withWriteCtx is a tiny helper so the chaos suite can share the kill-during
// context shape without importing context everywhere; kept here to avoid an
// unused-import churn between the two files.
var _ = context.Background
