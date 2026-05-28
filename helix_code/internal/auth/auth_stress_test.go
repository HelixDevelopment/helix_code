package auth

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(A) stress coverage for the REAL internal/auth surface.
//
// Everything below drives the production AuthService against a REAL, fully
// functional in-memory AuthRepository (memAuthRepo) — NOT a testify mock. The
// repo genuinely persists and retrieves users + sessions under a mutex, so the
// concurrent-session paths exercise real lock contention, real lookups, and real
// JWT/bcrypt crypto. The signing secret is an ephemeral per-run key generated in
// the test (never a committed secret, per CONST-042).
//
// Stress classes exercised:
//   - sustained load (N>=100) on the FAST JWT validate path (p50/p95/p99 captured);
//   - concurrent contention (>=10 goroutines) on JWT generate+validate;
//   - concurrent contention on the mutex-guarded session store (create/verify/logout);
//   - boundary conditions (expired token, empty token, max-size claims).

// memAuthRepo is a REAL, fully-implemented in-memory AuthRepository. It is not a
// mock: every method performs the genuine store/lookup it claims to, guarded by
// a RWMutex so the concurrency suites exercise real lock behaviour. (Unit-test
// scope per CONST-050(A): a real component standing in for PostgreSQL so the
// crypto + session paths can be hammered without external infrastructure.)
type memAuthRepo struct {
	mu        sync.RWMutex
	usersByID map[uuid.UUID]*User
	usersByNm map[string]*User
	hashByID  map[uuid.UUID]string
	sessions  map[string]*Session
}

func newMemAuthRepo() *memAuthRepo {
	return &memAuthRepo{
		usersByID: make(map[uuid.UUID]*User),
		usersByNm: make(map[string]*User),
		hashByID:  make(map[uuid.UUID]string),
		sessions:  make(map[string]*Session),
	}
}

func (r *memAuthRepo) CreateUser(_ context.Context, user *User, passwordHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.usersByNm[user.Username]; ok {
		return ErrUserExists
	}
	cp := *user
	r.usersByID[user.ID] = &cp
	r.usersByNm[user.Username] = &cp
	r.hashByID[user.ID] = passwordHash
	return nil
}

func (r *memAuthRepo) GetUserByUsername(_ context.Context, username string) (*User, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.usersByNm[username]
	if !ok {
		return nil, "", ErrUserNotFound
	}
	return u, r.hashByID[u.ID], nil
}

func (r *memAuthRepo) GetUserByEmail(_ context.Context, email string) (*User, string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.usersByID {
		if u.Email == email {
			return u, r.hashByID[u.ID], nil
		}
	}
	return nil, "", ErrUserNotFound
}

func (r *memAuthRepo) GetUserByID(_ context.Context, id uuid.UUID) (*User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.usersByID[id]
	if !ok {
		return nil, ErrUserNotFound
	}
	return u, nil
}

func (r *memAuthRepo) UpdateUserLastLogin(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.usersByID[id]
	if !ok {
		return ErrUserNotFound
	}
	u.LastLogin = time.Now()
	return nil
}

func (r *memAuthRepo) UpdateUser(_ context.Context, userID uuid.UUID, displayName, email string) (*User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.usersByID[userID]
	if !ok {
		return nil, ErrUserNotFound
	}
	if displayName != "" {
		u.DisplayName = displayName
	}
	if email != "" {
		u.Email = email
	}
	u.UpdatedAt = time.Now()
	return u, nil
}

func (r *memAuthRepo) DeleteUser(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.usersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	delete(r.usersByID, userID)
	delete(r.usersByNm, u.Username)
	delete(r.hashByID, userID)
	return nil
}

func (r *memAuthRepo) CreateSession(_ context.Context, session *Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *session
	r.sessions[session.SessionToken] = &cp
	return nil
}

func (r *memAuthRepo) GetSession(_ context.Context, token string) (*Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[token]
	if !ok {
		return nil, ErrTokenInvalid
	}
	return s, nil
}

func (r *memAuthRepo) DeleteSession(_ context.Context, token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, token)
	return nil
}

func (r *memAuthRepo) DeleteUserSessions(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tok, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, tok)
		}
	}
	return nil
}

// newStressService builds a REAL AuthService with an ephemeral random signing
// key (CONST-042: never a committed secret) and a low bcrypt cost so the
// password paths stay test-fast while still exercising real bcrypt. It returns
// the service, its backing repo, and a freshly persisted user.
func newStressService(t testing.TB) (*AuthService, *memAuthRepo, *User) {
	t.Helper()
	key, err := randomSecret(48)
	if err != nil {
		t.Fatalf("generate ephemeral signing key: %v", err)
	}
	cfg := DefaultConfig()
	cfg.JWTSecret = key
	cfg.TokenExpiry = time.Hour
	cfg.SessionExpiry = time.Hour
	cfg.BcryptCost = bcrypt.MinCost // keep hashing fast in tests; real bcrypt path

	repo := newMemAuthRepo()
	svc := NewAuthService(cfg, repo)

	hash, err := svc.hashPassword("Str3ss-Ch@os-Passw0rd!")
	if err != nil {
		t.Fatalf("hash seed password: %v", err)
	}
	u := &User{
		ID:        uuid.New(),
		Username:  "stress_user",
		Email:     "stress@example.com",
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := repo.CreateUser(context.Background(), u, hash); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return svc, repo, u
}

// randomSecret returns a base64-url ephemeral secret of n random bytes. It uses
// the same crypto/rand path the production session-token generator uses, so the
// test secret is genuinely random per run and never a hardcoded literal.
func randomSecret(n int) (string, error) {
	tmp := &AuthService{}
	// generateSessionToken always emits 32 random bytes; for n<=32 we slice it,
	// otherwise concatenate two draws. This keeps us on the real crypto path.
	a, err := tmp.generateSessionToken()
	if err != nil {
		return "", err
	}
	b, err := tmp.generateSessionToken()
	if err != nil {
		return "", err
	}
	s := a + b
	if len(s) < n {
		return s, nil
	}
	return s[:n], nil
}

// TestAuth_Stress_SustainedJWTValidate drives the FAST JWT validate path under
// sustained load (N>=100) and captures p50/p95/p99. JWT HMAC validation is the
// cheap path (no bcrypt), so 100 iterations complete quickly per the harness
// guidance. Every validation MUST succeed for a freshly-minted valid token.
func TestAuth_Stress_SustainedJWTValidate(t *testing.T) {
	svc, _, u := newStressService(t)

	token, err := svc.GenerateJWT(u)
	if err != nil {
		t.Fatalf("generate JWT: %v", err)
	}

	rep := stresschaos.RunSustainedLoad(t, "auth_jwt_validate_sustained",
		stresschaos.SustainedConfig{N: 500, MaxErrorRate: 0.0},
		func(i int) error {
			got, verr := svc.VerifyJWT(token)
			if verr != nil {
				return fmt.Errorf("iter %d: verify failed: %w", i, verr)
			}
			if got.ID != u.ID {
				return fmt.Errorf("iter %d: wrong user id %s != %s", i, got.ID, u.ID)
			}
			return nil
		})

	if rep.N < stresschaos.MinSustainedN {
		t.Fatalf("sustained run did not meet §11.4.85 floor: N=%d", rep.N)
	}
	t.Logf("JWT validate sustained: N=%d p50=%.3fms p95=%.3fms p99=%.3fms",
		rep.N, rep.P50Ms, rep.P95Ms, rep.P99Ms)
}

// TestAuth_Stress_ConcurrentGenerateValidate hammers JWT generate + validate
// from >=10 goroutines. Each goroutine mints its own token from a distinct user
// and validates it, so the run exercises concurrent reads of the shared signing
// secret and concurrent jwt parsing. Run under -race to catch any shared-state
// data race in the signing/parsing paths.
func TestAuth_Stress_ConcurrentGenerateValidate(t *testing.T) {
	svc, repo, _ := newStressService(t)

	// Seed a distinct active user per goroutine so each thread owns its data.
	const goroutines = 16
	users := make([]*User, goroutines)
	for g := 0; g < goroutines; g++ {
		u := &User{
			ID:        uuid.New(),
			Username:  fmt.Sprintf("concurrent_user_%d", g),
			Email:     fmt.Sprintf("c%d@example.com", g),
			IsActive:  true,
			CreatedAt: time.Now(),
		}
		hash, _ := svc.hashPassword("pw-" + u.Username)
		if err := repo.CreateUser(context.Background(), u, hash); err != nil {
			t.Fatalf("seed concurrent user %d: %v", g, err)
		}
		users[g] = u
	}

	stresschaos.RunConcurrent(t, "auth_jwt_generate_validate_concurrent",
		stresschaos.ConcurrencyConfig{Parallelism: goroutines, IterationsPerGoroutine: 60},
		func(goroutine, iter int) error {
			u := users[goroutine]
			tok, err := svc.GenerateJWT(u)
			if err != nil {
				return fmt.Errorf("g%d/i%d generate: %w", goroutine, iter, err)
			}
			got, err := svc.VerifyJWT(tok)
			if err != nil {
				return fmt.Errorf("g%d/i%d verify: %w", goroutine, iter, err)
			}
			if got.ID != u.ID {
				return fmt.Errorf("g%d/i%d cross-talk: got %s want %s", goroutine, iter, got.ID, u.ID)
			}
			return nil
		})
}

// TestAuth_Stress_ConcurrentSessionStore exercises the mutex-guarded session
// store under contention: many goroutines concurrently create, verify, and
// delete sessions through the REAL AuthService + repo. The store's RWMutex must
// keep the sessions map self-consistent — under -race a torn map write would be
// reported, and a lock defect would deadlock (caught by the harness timeout).
func TestAuth_Stress_ConcurrentSessionStore(t *testing.T) {
	svc, repo, u := newStressService(t)
	ctx := context.Background()

	stresschaos.RunConcurrent(t, "auth_session_store_concurrent",
		stresschaos.ConcurrencyConfig{Parallelism: 12, IterationsPerGoroutine: 80},
		func(goroutine, iter int) error {
			// Real session-token generation (crypto/rand) + real persistence.
			tok, err := svc.generateSessionToken()
			if err != nil {
				return fmt.Errorf("g%d/i%d token gen: %w", goroutine, iter, err)
			}
			sess := &Session{
				ID:           uuid.New(),
				UserID:       u.ID,
				SessionToken: tok,
				ClientType:   "stress",
				IPAddress:    net.ParseIP("127.0.0.1"),
				ExpiresAt:    time.Now().Add(time.Hour),
				CreatedAt:    time.Now(),
			}
			if err := repo.CreateSession(ctx, sess); err != nil {
				return fmt.Errorf("g%d/i%d create session: %w", goroutine, iter, err)
			}
			// Verify through the real path (looks up store + active-user check).
			vu, err := svc.VerifySession(ctx, tok)
			if err != nil {
				return fmt.Errorf("g%d/i%d verify session: %w", goroutine, iter, err)
			}
			if vu.ID != u.ID {
				return fmt.Errorf("g%d/i%d session user mismatch", goroutine, iter)
			}
			// Real logout — deletes from the store under the write lock.
			if err := svc.Logout(ctx, tok); err != nil {
				return fmt.Errorf("g%d/i%d logout: %w", goroutine, iter, err)
			}
			return nil
		})
}

// TestAuth_Stress_BoundaryConditions exercises §11.4.85(A)(3) boundary cases on
// the validate path: an expired token MUST be rejected, an empty token MUST be
// rejected, and a token carrying a maximally large (but well-formed) set of
// claims MUST still validate. Each boundary is categorised and asserted.
func TestAuth_Stress_BoundaryConditions(t *testing.T) {
	svc, _, u := newStressService(t)

	t.Run("expired_token_rejected", func(t *testing.T) {
		// Build a service whose tokens are already expired.
		expiredCfg := svc.config
		expiredCfg.TokenExpiry = -time.Hour
		expiredSvc := NewAuthService(expiredCfg, newMemAuthRepo())
		tok, err := expiredSvc.GenerateJWT(u)
		if err != nil {
			t.Fatalf("generate expired token: %v", err)
		}
		if _, err := expiredSvc.VerifyJWT(tok); err == nil {
			t.Fatal("BOUNDARY VIOLATION: expired token was accepted")
		}
	})

	t.Run("empty_token_rejected", func(t *testing.T) {
		if _, err := svc.VerifyJWT(""); err == nil {
			t.Fatal("BOUNDARY VIOLATION: empty token was accepted")
		}
	})

	t.Run("max_claims_token_valid", func(t *testing.T) {
		// A user with maximal-length username/email still produces a valid token.
		big := make([]byte, 4096)
		for i := range big {
			big[i] = 'a'
		}
		bigUser := &User{
			ID:       uuid.New(),
			Username: string(big),
			Email:    string(big) + "@example.com",
			IsActive: true,
		}
		tok, err := svc.GenerateJWT(bigUser)
		if err != nil {
			t.Fatalf("generate max-claims token: %v", err)
		}
		got, err := svc.VerifyJWT(tok)
		if err != nil {
			t.Fatalf("BOUNDARY VIOLATION: max-claims token rejected: %v", err)
		}
		if got.ID != bigUser.ID {
			t.Fatalf("max-claims token decoded wrong user")
		}
	})
}
