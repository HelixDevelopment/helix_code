//go:build integration

package integration

// project_idor_realpg_test.go — REAL-PostgreSQL integration guard for the
// project-IDOR fix (commit b2c5c066, internal/project/manager_db.go
// GetProjectForUser owner-scoped).
//
// WHY THIS EXISTS (§11.4.118 coverage gap):
// Stream A's IDOR fix is proven by a hermetic UNIT test (mock DB) + source
// review. But the cross-user NEGATIVE — user B reading user A's project must be
// DENIED at the real SQL layer (WHERE id=$1 AND owner_id=$2 AND status='active'
// → pgx.ErrNoRows → ErrProjectNotFound, no existence leak) — was NEVER exercised
// against a REAL database. A mock cannot prove the `owner_id = $2` predicate is
// actually enforced by PostgreSQL: the mock just returns whatever the test wired
// it to return. This test closes that gap by seeding TWO real owner rows + a real
// project in real PostgreSQL and asserting the real query denies the cross-user
// read.
//
// ANTI-BLUFF (CONST-050(A) / §11.4 / §11.4.95): NO mock, NO stub, NO fake. It
// connects to the REAL PostgreSQL the full-test stack (nezha) provides, registers
// TWO REAL active users (real UUIDs), inserts a REAL project owned by user A, and
// drives GetProjectForUser against the live pgx pool. ErrProjectNotFound here is a
// REAL pgx.ErrNoRows from a REAL SELECT, not a mock return.
//
// HONEST SKIP (§11.4.3): if no real database is reachable, the test SKIPs with a
// documented reason — it never bluffs a PASS and never silently bypasses the IDOR
// check. Reuses the shared realDBConfigFromEnv() helper (realdb_auth_helper_test.go).
//
// §11.4.115 RED-on-broken→GREEN POLARITY:
//   - default (RED_IDOR_REALPG unset)  → GREEN guard: assert the FIX — user B is
//     DENIED (ErrProjectNotFound), user A is ALLOWED. This is the standing
//     regression guard (§11.4.135).
//   - RED_IDOR_REALPG=1                 → assert the DEFECT shape the PRE-fix code
//     produced — user B CAN read user A's project. Against the FIXED code this
//     mode FAILs (because the fix correctly denies B), proving the test genuinely
//     discriminates a regression rather than rubber-stamping.
//
// §11.4.98 fully-automatic: no human step after startup; re-runnable at -count=3
// because every seeded row is uniquely-suffixed and torn down in t.Cleanup.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/auth"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/project"
	"github.com/google/uuid"
)

// redIDORMode reports whether the test runs in §11.4.115 RED reproduce-the-defect
// polarity (RED_IDOR_REALPG=1) vs the default GREEN regression-guard polarity.
func redIDORMode() bool {
	return strings.TrimSpace(os.Getenv("RED_IDOR_REALPG")) == "1"
}

// seedRealUser registers a unique active user against the real auth backend and
// returns its real UUID owner-id string. Uniqueness avoids ErrUserExists across
// repeated -count runs. The user row is torn down in t.Cleanup.
func seedRealUser(t *testing.T, ctx context.Context, db *database.Database, tag string) (ownerID string) {
	t.Helper()
	authConfig := auth.AuthConfig{
		JWTSecret:   authJWTSecret,
		TokenExpiry: time.Hour,
		BcryptCost:  4,
	}
	authService := auth.NewAuthService(authConfig, auth.NewAuthDB(db.Pool))

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	username := fmt.Sprintf("idor_%s_%s", tag, suffix)
	email := fmt.Sprintf("%s@idor.test", username)
	user, err := authService.Register(ctx, username, email, "idor-password-123", "IDOR Test User")
	if err != nil {
		t.Skipf("SKIP-OK: could not register real test user %q (schema migrated?): %v", username, err) //nolint
	}
	t.Cleanup(func() {
		// Hard-delete the seeded user so repeated -count runs stay clean. Best-effort.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = db.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, user.ID)
	})
	return user.ID.String()
}

// TestProjectIDOR_RealPG proves the project-IDOR fix against REAL PostgreSQL:
// user B cannot read user A's project; user A can.
func TestProjectIDOR_RealPG(t *testing.T) {
	dbCfg, ok := realDBConfigFromEnv()
	if !ok {
		t.Skip("SKIP-OK: no real database configured (set DB_HOST/HELIX_DATABASE_HOST); cannot exercise the project-IDOR negative against real infra") //nolint
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	realDB, err := database.New(dbCfg)
	if err != nil {
		t.Skipf("SKIP-OK: real database at %s:%d unreachable: %v; cannot exercise the project-IDOR negative", dbCfg.Host, dbCfg.Port, err) //nolint
	}
	// Prove the connection is genuinely live before relying on it (§11.4.5).
	if pingErr := realDB.Pool.Ping(ctx); pingErr != nil {
		realDB.Pool.Close()
		t.Skipf("SKIP-OK: real database at %s:%d did not answer ping: %v", dbCfg.Host, dbCfg.Port, pingErr) //nolint
	}
	t.Cleanup(func() { realDB.Pool.Close() })

	// REAL project manager backed by the REAL pgx pool (NO mock).
	mgr := project.NewDatabaseManager(realDB)

	// (a) Seed TWO distinct owners — real UUIDs in real pg.
	ownerA := seedRealUser(t, ctx, realDB, "a")
	ownerB := seedRealUser(t, ctx, realDB, "b")
	if ownerA == ownerB {
		t.Fatalf("seeded owners must be distinct: A=%s B=%s", ownerA, ownerB)
	}

	// (b) User A creates a REAL project.
	proj, err := mgr.CreateProjectWithUser(ctx, "IDOR Victim Project", "owned by A", t.TempDir(), "go", ownerA)
	if err != nil {
		t.Fatalf("user A must be able to create a real project: %v", err)
	}
	if proj.ID == "" {
		t.Fatalf("created project must have a real id")
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = realDB.Exec(cleanupCtx, `DELETE FROM projects WHERE id = $1`, proj.ID)
	})

	// (d) OWNER POSITIVE (always asserted, both polarities): user A reads its own
	// project. The fix must not break the legitimate read.
	gotA, err := mgr.GetProjectForUser(ctx, proj.ID, ownerA)
	if err != nil {
		t.Fatalf("OWNER POSITIVE: user A reading its own project must succeed, got err: %v", err)
	}
	if gotA == nil || gotA.ID != proj.ID {
		t.Fatalf("OWNER POSITIVE: user A must get back project %s, got %+v", proj.ID, gotA)
	}
	t.Logf("OWNER POSITIVE (real pg): user A (%s) read project %s OK", ownerA, proj.ID)

	// (c) IDOR NEGATIVE: user B attempts to read user A's project.
	_, errB := mgr.GetProjectForUser(ctx, proj.ID, ownerB)

	if redIDORMode() {
		// §11.4.115 RED polarity — assert the PRE-fix DEFECT shape: B CAN read A's
		// project (errB == nil). Against the FIXED code this assertion FAILs, which
		// is the discrimination proof: the guard genuinely catches the regression.
		if errB == nil {
			t.Logf("RED MODE: user B read user A's project (defect reproduced) — would indicate IDOR present")
		} else {
			t.Fatalf("RED MODE (RED_IDOR_REALPG=1) expected the DEFECT (B reads A's project, errB==nil) "+
				"but the live query DENIED B with: %v — this FAIL proves the fix is in place and the "+
				"guard discriminates a regression (the GREEN polarity is the standing guard).", errB)
		}
		return
	}

	// GREEN polarity (default) — assert the FIX: B is DENIED with ErrProjectNotFound
	// (no existence leak — same 404 as a truly-missing project).
	if errB == nil {
		t.Fatalf("IDOR NEGATIVE: user B (%s) MUST NOT be able to read user A's project %s, but the read SUCCEEDED — IDOR present", ownerB, proj.ID)
	}
	if !errors.Is(errB, project.ErrProjectNotFound) {
		t.Fatalf("IDOR NEGATIVE: user B's denied read must surface ErrProjectNotFound (no existence leak), got: %v", errB)
	}
	t.Logf("IDOR NEGATIVE (real pg): user B (%s) DENIED reading project %s → ErrProjectNotFound (no leak)", ownerB, proj.ID)
}
