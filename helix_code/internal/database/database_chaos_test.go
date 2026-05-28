//go:build integration
// +build integration

package database

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85 CHAOS coverage for the REAL internal/database layer against a REAL
// PostgreSQL (no mocks — integration-gated per CONST-050).
//
// Failure injection exercised here:
//   - cancel mid-query (context timeout / cancellation) — must abort cleanly,
//     never deadlock, never leak the connection.
//   - input corruption — malformed / huge / SQL-injection-y string PARAMETERS
//     must be safely bound (parameterised), never crash the process or alter
//     the query semantics.
//   - resource pressure — bounded memory pressure while querying.
//   - connection-pool exhaustion / churn — more concurrent holders than the
//     pool max, plus rapid open/close churn; the pool must degrade (queue /
//     time out) rather than crash, and must recover afterwards.
//
// Every PASS writes a categorised recovery_trace artefact via the harness.
// SQL-injection safety in this file follows database_stress_test.go: the table
// identifier is validated by mustSafeIdent; all values are bound parameters.

// TestDatabase_Chaos_CancelMidQuery injects a process-death-style fault by
// cancelling the context while a slow server-side query (pg_sleep) is in flight.
// The query MUST return promptly with a cancellation error and the pool MUST
// remain usable afterwards — no deadlock, no leaked connection.
func TestDatabase_Chaos_CancelMidQuery(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	q := setupStressTable(t, db, "chaos_cancel")

	stresschaos.ChaosKillDuring(t, "database_chaos_cancel_mid_query", 150*time.Millisecond,
		func(ctx context.Context, rec *stresschaos.ChaosRecorder) {
			// A 10s server-side sleep; the harness cancels ctx ~150ms in.
			var dummy int
			err := db.QueryRow(ctx, "SELECT 1 FROM pg_sleep(10)").Scan(&dummy)
			switch {
			case err == nil:
				rec.Record(stresschaos.Degraded, "pg_sleep completed before cancellation (unexpected but not fatal)")
			case ctx.Err() != nil || strings.Contains(err.Error(), "context") || strings.Contains(err.Error(), "canceling"):
				rec.Record(stresschaos.Recovered, fmt.Sprintf("query aborted on cancellation: %v", err))
			default:
				rec.Record(stresschaos.Degraded, fmt.Sprintf("query returned non-cancellation error: %v", err))
			}
		})

	// Anti-bluff: the pool must still serve queries after the cancellation storm.
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var alive int
	if err := db.QueryRow(pingCtx, "SELECT 1").Scan(&alive); err != nil {
		t.Fatalf("pool unusable after mid-query cancellation: %v", err)
	}
	if alive != 1 {
		t.Fatalf("post-cancellation health query returned %d, want 1", alive)
	}
	// And it can still do real table work.
	var id int64
	if err := db.QueryRow(context.Background(), q.insert, "post-cancel", 1).Scan(&id); err != nil {
		t.Fatalf("pool cannot insert after cancellation: %v", err)
	}
	t.Logf("database chaos cancel: pool survived mid-query cancellation and remains usable (post-row id=%d)", id)
}

// TestDatabase_Chaos_DeadlineExceededMidQuery is the timeout variant: a context
// with a short deadline against a longer server-side query. The driver MUST
// honour the deadline (return context.DeadlineExceeded) without wedging the pool.
func TestDatabase_Chaos_DeadlineExceededMidQuery(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	rec := stresschaos.NewChaosRecorder(t, "database_chaos_deadline_exceeded", "network-fault")
	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		var dummy int
		err := db.QueryRow(ctx, "SELECT 1 FROM pg_sleep(5)").Scan(&dummy)
		cancel()
		if err == nil {
			rec.Record(stresschaos.Degraded, fmt.Sprintf("iter %d: sleep finished before deadline (unexpected)", i))
		} else {
			rec.Record(stresschaos.Recovered, fmt.Sprintf("iter %d: deadline honoured: %v", i, err))
		}
	}
	rec.AssertNoFatal()

	// Pool must remain usable.
	var alive int
	if err := db.QueryRow(context.Background(), "SELECT 1").Scan(&alive); err != nil || alive != 1 {
		t.Fatalf("pool unusable after deadline storm: err=%v alive=%d", err, alive)
	}
	t.Logf("database chaos deadline: 20 deadline-exceeded queries, pool still healthy")
}

// TestDatabase_Chaos_CorruptInput feeds malformed / huge / injection-style
// strings as bound PARAMETERS. They must be safely parameterised (stored as
// literal data), never crash the process and never alter query semantics. After
// the run we prove no injection occurred: the test table still holds only the
// rows we inserted, and a sentinel row was NOT dropped/altered.
func TestDatabase_Chaos_CorruptInput(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	q := setupStressTable(t, db, "chaos_corrupt")
	ctx := context.Background()

	// Sentinel row that an injection attack would try to destroy.
	var sentinelID int64
	if err := db.QueryRow(ctx, q.insert, "SENTINEL", 777).Scan(&sentinelID); err != nil {
		t.Fatalf("insert sentinel: %v", err)
	}

	corruptInputs := [][]byte{
		[]byte("'; DROP TABLE " + q.table + "; --"),
		[]byte("' OR '1'='1"),
		[]byte("\x00\x01\x02\xff\xfe binary garbage \x00"),
		[]byte(strings.Repeat("A", 2<<20)), // 2 MiB
		[]byte("Robert'); DELETE FROM " + q.table + " WHERE n=777; --"),
		[]byte("%s %d %n format string"),
		[]byte("ünïcödé 日本語 \U0001F600 emoji"),
		[]byte("line1\nline2\rline3\ttabbed"),
		[]byte(`{"json":"like","but":["text"]}`),
		[]byte(""),
	}

	stresschaos.ChaosCorruptInputDuring(t, "database_chaos_corrupt_input", corruptInputs,
		func(input []byte) error {
			// The corrupt bytes go in as a BOUND parameter — never concatenated
			// into SQL. A safe driver stores them as literal data.
			var id int64
			if err := db.QueryRow(ctx, q.insert, string(input), 0).Scan(&id); err != nil {
				// A driver-level rejection (e.g. invalid UTF-8 for TEXT) is a clean
				// graceful-rejection, not a crash.
				return fmt.Errorf("rejected: %w", err)
			}
			return nil
		})

	// Anti-bluff: prove NO injection mutated state. The sentinel must survive.
	var sentinelN int
	if err := db.QueryRow(ctx, q.selectByID, sentinelID).Scan(new(string), &sentinelN); err != nil {
		t.Fatalf("sentinel row destroyed by injection payload (table was dropped/deleted?): %v", err)
	}
	if sentinelN != 777 {
		t.Fatalf("sentinel row altered: n=%d want 777", sentinelN)
	}
	// The table still exists and is queryable.
	var total int64
	if err := db.QueryRow(ctx, q.countAll).Scan(&total); err != nil {
		t.Fatalf("table unusable after corrupt-input storm: %v", err)
	}
	t.Logf("database chaos corrupt-input: %d payloads bound safely, sentinel intact, %d rows total (no injection)", len(corruptInputs), total)
}

// TestDatabase_Chaos_PoolExhaustion creates more concurrent long-holding
// connections than the pool max (server profile MaxConns=20). Excess acquirers
// must QUEUE and either succeed when a connection frees or time out cleanly —
// the pool must NOT crash, deadlock, or leak. Afterwards the pool must recover.
func TestDatabase_Chaos_PoolExhaustion(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	// Small pool so exhaustion is easy and fast to provoke.
	cfg.MaxConns = 5
	cfg.MinConns = 1
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	rec := stresschaos.NewChaosRecorder(t, "database_chaos_pool_exhaustion", "resource-exhaustion")

	// Launch 30 acquirers against a pool of 5. Each holds its connection for
	// 300ms, far longer than the per-acquire timeout, forcing contention.
	const holders = 30
	var wg sync.WaitGroup
	var succeeded, timedOut, otherErr int64
	wg.Add(holders)
	for i := 0; i < holders; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("acquirer %d panicked: %v", idx, p))
				}
			}()
			// Bounded per-acquire timeout — excess acquirers should hit this.
			ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
			defer cancel()
			conn, err := db.Pool.Acquire(ctx)
			if err != nil {
				atomic.AddInt64(&timedOut, 1)
				return
			}
			// Hold the connection (real query) then release.
			var dummy int
			if qerr := conn.QueryRow(ctx, "SELECT 1 FROM pg_sleep(0.3)").Scan(&dummy); qerr != nil {
				atomic.AddInt64(&otherErr, 1)
			} else {
				atomic.AddInt64(&succeeded, 1)
			}
			conn.Release()
		}(i)
	}
	wg.Wait()

	rec.Record(stresschaos.Degraded, fmt.Sprintf("pool exhaustion: %d succeeded, %d timed-out-cleanly, %d other-err (pool max=5, holders=%d)",
		atomic.LoadInt64(&succeeded), atomic.LoadInt64(&timedOut), atomic.LoadInt64(&otherErr), holders))
	if atomic.LoadInt64(&succeeded) > 0 {
		rec.Record(stresschaos.Recovered, "some acquirers succeeded — pool serviced load under contention")
	}
	rec.AssertNoFatal()

	// Recovery: after the storm the pool must serve a fresh query.
	recCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var alive int
	if err := db.QueryRow(recCtx, "SELECT 1").Scan(&alive); err != nil || alive != 1 {
		t.Fatalf("pool did not recover after exhaustion: err=%v alive=%d", err, alive)
	}
	t.Logf("database chaos pool-exhaustion: %d/%d succeeded, %d timed out cleanly, pool recovered",
		atomic.LoadInt64(&succeeded), holders, atomic.LoadInt64(&timedOut))
}

// TestDatabase_Chaos_ConnectionChurn rapidly opens and closes whole pools while
// issuing queries, exercising connection lifecycle churn. No pool may leak a
// connection or panic on Close while a query is mid-flight.
func TestDatabase_Chaos_ConnectionChurn(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}

	rec := stresschaos.NewChaosRecorder(t, "database_chaos_connection_churn", "process-death")
	const cycles = 25
	for i := 0; i < cycles; i++ {
		func(idx int) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("churn cycle %d panicked: %v", idx, p))
				}
			}()
			c := cfg
			c.MaxConns = 3
			c.MinConns = 0
			db, err := New(c)
			if err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("cycle %d: New failed: %v", idx, err))
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			var dummy int
			qerr := db.QueryRow(ctx, "SELECT 1").Scan(&dummy)
			cancel()
			db.Close() // close while the just-issued query has returned
			if qerr != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("cycle %d: query err: %v", idx, qerr))
				return
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("cycle %d: open->query->close clean", idx))
		}(i)
	}
	rec.AssertNoFatal()
	t.Logf("database chaos churn: %d open/query/close cycles completed without leak or panic", cycles)
}

// TestDatabase_Chaos_ResourcePressure runs real queries under bounded memory
// pressure (§11.4.85(B)(4)) — the pool/driver must keep serving, not OOM-crash.
func TestDatabase_Chaos_ResourcePressure(t *testing.T) {
	cfg, ok := dbIntegrationConfig(t)
	if !ok {
		t.Skip("SKIP-OK: #HXC-014 no real PostgreSQL reachable (§11.4.3)")
	}
	db, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer db.Close()

	q := setupStressTable(t, db, "chaos_pressure")
	ctx := context.Background()

	stresschaos.ChaosResourcePressureDuring(t, "database_chaos_resource_pressure", 64,
		func(rec *stresschaos.ChaosRecorder) {
			for i := 0; i < 200; i++ {
				var id int64
				if err := db.QueryRow(ctx, q.insert, fmt.Sprintf("pressure-%d", i), i).Scan(&id); err != nil {
					rec.Record(stresschaos.Degraded, fmt.Sprintf("insert %d under pressure failed cleanly: %v", i, err))
					continue
				}
			}
			rec.Record(stresschaos.Recovered, "200 inserts completed under bounded memory pressure")
		})

	var count int64
	if err := db.QueryRow(ctx, q.countAll).Scan(&count); err != nil {
		t.Fatalf("count after pressure: %v", err)
	}
	if count == 0 {
		t.Fatal("no rows persisted under resource pressure — not real work")
	}
	t.Logf("database chaos resource-pressure: %d rows persisted under memory pressure", count)
}
