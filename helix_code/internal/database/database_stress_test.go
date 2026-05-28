//go:build integration
// +build integration

package database

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
	"github.com/jackc/pgx/v5"
)

// §11.4.85 STRESS coverage for the REAL internal/database connection pool /
// query layer against a REAL PostgreSQL (no mocks — integration-gated per
// CONST-050: every non-unit test exercises real infrastructure).
//
// The unit under stress is the production *Database (pgxpool-backed Exec/
// Query/QueryRow/transaction paths) hammered with sustained INSERT/SELECT/
// UPDATE/tx load (N>=100) and concurrent contention (N>=10 goroutines), each
// against a real test-prefixed table created in setup and dropped in cleanup.
//
// SQL-injection safety: all *values* are bound parameters ($1,$2...). The only
// interpolated identifier is the table NAME, which is a controlled constant
// prefix + a hard-coded test suffix, validated against safeIdent (a strict
// lowercase-snake allowlist) before any query is built. No external/user data
// ever reaches a query string. The interpolation is centralised in tableQueries
// so every call site uses a pre-validated, pre-built constant query.

// stressTablePrefix namespaces every table this suite creates so cleanup never
// touches a non-test table.
const stressTablePrefix = "hxc014_stress_"

// safeIdent is the strict allowlist a table identifier must match before it can
// be interpolated into DDL/DML. It permits only lowercase letters, digits, and
// underscores — no quotes, whitespace, or punctuation — so a name that passes
// cannot carry an injection payload. External data never produces these names
// (they are constant-prefix + hard-coded test suffix), but validating defends
// the property mechanically.
var safeIdent = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// mustSafeIdent panics (test-time only) if the identifier is not allowlisted —
// it can never receive external input in this suite, but the guard makes the
// no-injection property explicit and self-checking.
func mustSafeIdent(t *testing.T, name string) string {
	t.Helper()
	if !safeIdent.MatchString(name) {
		t.Fatalf("unsafe table identifier %q (not [a-z][a-z0-9_]*)", name)
	}
	return name
}

// tableQueries holds every query string this suite issues, each built ONCE from
// a validated table identifier. Centralising construction here means call sites
// pass only bound parameters and never interpolate SQL themselves.
type tableQueries struct {
	table       string
	create      string
	truncate    string
	drop        string
	insert      string // $1=payload $2=n RETURNING id
	selectByID  string // $1=id -> payload,n
	selectPay   string // $1=id -> payload
	updateInc   string // $1=id  (n=n+1)
	updateSetN  string // $1=id  (n=1)
	updateSet99 string // $1=id  (n=99)
	countAll    string
	countN1     string
}

func newTableQueries(t *testing.T, suffix string) tableQueries {
	t.Helper()
	table := mustSafeIdent(t, stressTablePrefix+suffix)
	return tableQueries{
		table:    table,
		create:   "CREATE TABLE IF NOT EXISTS " + table + " (id BIGSERIAL PRIMARY KEY, payload TEXT NOT NULL, n INTEGER NOT NULL DEFAULT 0, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())",
		truncate: "TRUNCATE TABLE " + table + " RESTART IDENTITY",
		drop:     "DROP TABLE IF EXISTS " + table,
		// All value positions are bound parameters; only the validated table
		// identifier is concatenated.
		insert:      "INSERT INTO " + table + " (payload, n) VALUES ($1, $2) RETURNING id",
		selectByID:  "SELECT payload, n FROM " + table + " WHERE id = $1",
		selectPay:   "SELECT payload FROM " + table + " WHERE id = $1",
		updateInc:   "UPDATE " + table + " SET n = n + 1 WHERE id = $1",
		updateSetN:  "UPDATE " + table + " SET n = 1 WHERE id = $1",
		updateSet99: "UPDATE " + table + " SET n = 99 WHERE id = $1",
		countAll:    "SELECT COUNT(*) FROM " + table,
		countN1:     "SELECT COUNT(*) FROM " + table + " WHERE n = 1",
	}
}

// dbIntegrationConfig builds a real-PostgreSQL Config for the §11.4.85 stress +
// chaos suites. Connection parameters are read from the environment so the test
// follows §11.4.3 — it returns ok=false (caller skips with a SKIP-OK marker)
// when no real database is reachable instead of failing. Defaults target the
// live podman PostgreSQL described by the HXC-014 task: localhost:5432 helix/helix
// over the helix_test database.
func dbIntegrationConfig(t *testing.T) (Config, bool) {
	t.Helper()
	host := os.Getenv("HELIX_TEST_DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := 5432
	if p := os.Getenv("HELIX_TEST_DB_PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}
	user := os.Getenv("HELIX_TEST_DB_USER")
	if user == "" {
		user = "helix"
	}
	password := os.Getenv("HELIX_TEST_DB_PASSWORD")
	if password == "" {
		password = "helix"
	}
	dbname := os.Getenv("HELIX_TEST_DB_NAME")
	if dbname == "" {
		dbname = "helix_test"
	}
	cfg := Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbname,
		SSLMode:  "disable",
	}
	// Probe reachability. If New (which Pings) fails the real database is
	// unavailable and the caller skips.
	probe, err := New(cfg)
	if err != nil {
		t.Logf("dbIntegrationConfig: real PostgreSQL not reachable: %v", err)
		return cfg, false
	}
	probe.Close()
	return cfg, true
}

// setupStressTable creates a dedicated test table and registers a cleanup that
// DROPs it. Every query string comes pre-built and pre-validated from tableQueries.
func setupStressTable(t *testing.T, db *Database, suffix string) tableQueries {
	t.Helper()
	q := newTableQueries(t, suffix)
	ctx := context.Background()
	if _, err := db.Exec(ctx, q.create); err != nil {
		t.Fatalf("setup: create table %s: %v", q.table, err)
	}
	// Truncate in case a prior crashed run left rows behind.
	if _, err := db.Exec(ctx, q.truncate); err != nil {
		t.Fatalf("setup: truncate table %s: %v", q.table, err)
	}
	t.Cleanup(func() {
		// Use a fresh background context — the test context may be cancelled.
		if _, err := db.Exec(context.Background(), q.drop); err != nil {
			t.Logf("cleanup: drop table %s: %v", q.table, err)
		}
	})
	return q
}

// TestDatabase_Stress_SustainedInsertSelectUpdate drives the real Exec/QueryRow
// lifecycle (INSERT -> SELECT-back -> UPDATE) under sustained load (N>=100),
// recording per-call latency. Every iteration writes a real row, reads it back
// with a parameterised SELECT, and updates it — proving genuine round-trip work.
func TestDatabase_Stress_SustainedInsertSelectUpdate(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	q := setupStressTable(t, db, "sustained")
	ctx := context.Background()

	var written int64
	stresschaos.RunSustainedLoad(t, "database_sustained_insert_select_update",
		stresschaos.SustainedConfig{N: 400, MaxErrorRate: 0.0},
		func(i int) error {
			var id int64
			payload := fmt.Sprintf("stress-payload-%d", i)
			if err := db.QueryRow(ctx, q.insert, payload, i).Scan(&id); err != nil {
				return fmt.Errorf("insert: %w", err)
			}
			var gotPayload string
			var gotN int
			if err := db.QueryRow(ctx, q.selectByID, id).Scan(&gotPayload, &gotN); err != nil {
				return fmt.Errorf("select: %w", err)
			}
			if gotPayload != payload || gotN != i {
				return fmt.Errorf("read-back mismatch: got (%q,%d) want (%q,%d)", gotPayload, gotN, payload, i)
			}
			tag, err := db.Exec(ctx, q.updateInc, id)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}
			if tag.RowsAffected() != 1 {
				return fmt.Errorf("update affected %d rows, want 1", tag.RowsAffected())
			}
			atomic.AddInt64(&written, 1)
			return nil
		})

	if atomic.LoadInt64(&written) == 0 {
		t.Fatal("zero rows written under sustained load — not real work")
	}
	var count int64
	if err := db.QueryRow(ctx, q.countAll).Scan(&count); err != nil {
		t.Fatalf("final count: %v", err)
	}
	if count != atomic.LoadInt64(&written) {
		t.Fatalf("persisted row count %d != writes %d", count, atomic.LoadInt64(&written))
	}
	t.Logf("database sustained: %d rows inserted+selected+updated, %d persisted", written, count)
}

// TestDatabase_Stress_ConcurrentPoolContention hammers the real pool from
// N>=10 goroutines per §11.4.85(A)(2). Each goroutine inserts and reads back its
// own rows concurrently, exercising real connection-pool checkout/return under
// contention. Run under -race to catch data races in the pool/query layer.
func TestDatabase_Stress_ConcurrentPoolContention(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	q := setupStressTable(t, db, "concurrent")
	ctx := context.Background()

	var total int64
	rep := stresschaos.RunConcurrent(t, "database_concurrent_pool_contention",
		stresschaos.ConcurrencyConfig{Parallelism: 16, IterationsPerGoroutine: 40, Timeout: 60 * time.Second},
		func(g, it int) error {
			var id int64
			payload := fmt.Sprintf("g%d-it%d", g, it)
			if err := db.QueryRow(ctx, q.insert, payload, g*1000+it).Scan(&id); err != nil {
				return fmt.Errorf("g%d it%d insert: %w", g, it, err)
			}
			var gotPayload string
			if err := db.QueryRow(ctx, q.selectPay, id).Scan(&gotPayload); err != nil {
				return fmt.Errorf("g%d it%d select: %w", g, it, err)
			}
			if gotPayload != payload {
				return fmt.Errorf("g%d it%d read-back mismatch: got %q want %q", g, it, gotPayload, payload)
			}
			atomic.AddInt64(&total, 1)
			return nil
		})

	var count int64
	if err := db.QueryRow(ctx, q.countAll).Scan(&count); err != nil {
		t.Fatalf("final count: %v", err)
	}
	if count != int64(rep.TotalCalls) {
		t.Fatalf("persisted count %d != concurrent calls %d (lost writes under contention)", count, rep.TotalCalls)
	}
	t.Logf("database concurrent: %d calls across %d goroutines, %d rows persisted, gDelta=%d",
		rep.TotalCalls, rep.Parallelism, count, rep.GoroutineDelta)
}

// TestDatabase_Stress_TransactionContention runs N>=10 goroutines each opening a
// real pgx transaction, performing INSERT+UPDATE, and committing — exercising the
// pool's transaction-scoped connection acquisition under contention.
func TestDatabase_Stress_TransactionContention(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	q := setupStressTable(t, db, "tx")
	ctx := context.Background()

	rep := stresschaos.RunConcurrent(t, "database_transaction_contention",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 30, Timeout: 60 * time.Second},
		func(g, it int) error {
			tx, err := db.Pool.Begin(ctx)
			if err != nil {
				return fmt.Errorf("g%d it%d begin: %w", g, it, err)
			}
			committed := false
			defer func() {
				if !committed {
					_ = tx.Rollback(ctx)
				}
			}()
			var id int64
			if err := tx.QueryRow(ctx, q.insert, fmt.Sprintf("tx-g%d-it%d", g, it), 0).Scan(&id); err != nil {
				return fmt.Errorf("g%d it%d tx insert: %w", g, it, err)
			}
			if _, err := tx.Exec(ctx, q.updateSetN, id); err != nil {
				return fmt.Errorf("g%d it%d tx update: %w", g, it, err)
			}
			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("g%d it%d commit: %w", g, it, err)
			}
			committed = true
			return nil
		})

	var count int64
	if err := db.QueryRow(ctx, q.countN1).Scan(&count); err != nil {
		t.Fatalf("final count: %v", err)
	}
	if count != int64(rep.TotalCalls) {
		t.Fatalf("committed-row count %d != tx count %d", count, rep.TotalCalls)
	}
	t.Logf("database tx contention: %d committed transactions, %d rows with n=1", rep.TotalCalls, count)
}

// TestDatabase_Stress_BoundaryConditions exercises §11.4.85(A)(3) boundary cases
// against real PostgreSQL: empty result set, a very large row payload, a no-op
// UPDATE (zero rows affected), and an empty-string payload.
func TestDatabase_Stress_BoundaryConditions(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	q := setupStressTable(t, db, "boundary")
	ctx := context.Background()

	// Boundary 1: empty result set — QueryRow on no-match must return ErrNoRows.
	var dummy string
	err = db.QueryRow(ctx, q.selectPay, int64(-1)).Scan(&dummy)
	if err != pgx.ErrNoRows {
		t.Fatalf("boundary empty-result: want pgx.ErrNoRows, got %v", err)
	}

	// Boundary 2: large row payload (~1 MiB TEXT) — must round-trip intact.
	large := make([]byte, 1<<20)
	for i := range large {
		large[i] = byte('A' + (i % 26))
	}
	largeStr := string(large)
	var largeID int64
	if err := db.QueryRow(ctx, q.insert, largeStr, 1).Scan(&largeID); err != nil {
		t.Fatalf("boundary large-row insert: %v", err)
	}
	var readBack string
	if err := db.QueryRow(ctx, q.selectPay, largeID).Scan(&readBack); err != nil {
		t.Fatalf("boundary large-row select: %v", err)
	}
	if len(readBack) != len(largeStr) || readBack != largeStr {
		t.Fatalf("boundary large-row mismatch: got len=%d want len=%d", len(readBack), len(largeStr))
	}

	// Boundary 3: no-op UPDATE — zero rows affected, no error.
	tag, err := db.Exec(ctx, q.updateSet99, int64(-12345))
	if err != nil {
		t.Fatalf("boundary no-op update: %v", err)
	}
	if tag.RowsAffected() != 0 {
		t.Fatalf("boundary no-op update affected %d rows, want 0", tag.RowsAffected())
	}

	// Boundary 4: empty-string payload is valid (NOT NULL but length 0 allowed).
	var emptyID int64
	if err := db.QueryRow(ctx, q.insert, "", 0).Scan(&emptyID); err != nil {
		t.Fatalf("boundary empty-string insert: %v", err)
	}
	t.Logf("database boundary: empty-result, %d-byte large row, no-op update, empty-string row all handled", len(largeStr))
}
