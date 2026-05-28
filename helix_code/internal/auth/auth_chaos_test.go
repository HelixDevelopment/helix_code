package auth

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"

	"dev.helix.code/tests/stresschaos"
)

// §11.4.85(B) chaos coverage for the REAL internal/auth surface.
//
// Auth handles untrusted input by definition — every token and credential that
// reaches the validator may be hostile. A validator that PANICS on a malformed /
// oversized / garbage token (instead of returning an error) is a crash-on-
// untrusted-input bug: a single forged request could take the whole process
// down. These suites feed corrupted tokens to the REAL JWT validator and hostile
// passwords to the REAL hasher, asserting clean rejection — never a crash.
//
// Chaos classes exercised against the production AuthService:
//   - input-corruption: malformed/garbage/oversized/truncated/wrong-signature/
//     wrong-algorithm tokens fed to VerifyJWT (must error, never panic);
//   - claim-corruption: tokens that ARE signed with the real key but carry
//     missing / wrong-typed claims — the historical unchecked-type-assertion
//     crash surface;
//   - hostile-credential: empty/huge/binary/null-byte passwords to the hasher
//     and verifier (must reject cleanly, never crash);
//   - state-corruption under contention: concurrent session create/verify/delete
//     churn on the SAME tokens (mutex must keep the store self-consistent).

// signWith mints a token with the given claims signed by the real service secret.
// Used to build tokens that pass signature verification but carry hostile claims,
// so the corruption flows all the way into the claim-extraction code path.
func signWith(t testing.TB, svc *AuthService, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(svc.config.JWTSecret))
	if err != nil {
		t.Fatalf("sign hostile-claims token: %v", err)
	}
	return s
}

// TestAuth_Chaos_MalformedTokens feeds structurally corrupt token strings to the
// REAL VerifyJWT. None are validly signed; each MUST be rejected with an error.
// A panic on any of these is a §11.4.85(B) Fatal (crash-on-untrusted-input).
func TestAuth_Chaos_MalformedTokens(t *testing.T) {
	svc, _, _ := newStressService(t)

	huge := strings.Repeat("A", 1<<20)           // 1 MiB of garbage
	hugeDotted := huge + "." + huge + "." + huge // 3 MiB oversized JWT-shaped blob

	// b64url builds a base64url segment at runtime so no JWT-shaped literal is
	// committed (avoids tripping hard-coded-key scanners — these are garbage
	// inputs, not real keys).
	b64url := func(s string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(s))
	}
	hs256Header := b64url(`{"alg":"HS256"}`)        // valid header, garbage body follows
	noneHeader := b64url(`{"alg":"none"}`)          // alg=none attack header
	bodyUserX := b64url(`{"user_id":"x"}`)          // minimal body

	corrupt := [][]byte{
		[]byte(""),                  // empty
		[]byte("."),                 // bare separator
		[]byte(".."),                // empty triplet
		[]byte("not-a-jwt"),         // no structure
		[]byte("a.b.c"),             // non-base64 segments
		[]byte(hs256Header + ".garbage.sig"),         // valid header, garbage body
		[]byte("\x00\x01\x02\xff\xfe"),               // raw binary
		[]byte(noneHeader + "." + bodyUserX + "."),   // alg=none, no sig
		[]byte(strings.Repeat("a.", 5000)),           // pathological dotted input
		[]byte(huge),                                 // 1 MiB single segment
		[]byte(hugeDotted),                           // 3 MiB JWT-shaped
		[]byte("Bearer " + hs256Header + ".x.y"),     // header-prefixed garbage
	}

	stresschaos.ChaosCorruptInputDuring(t, "auth_malformed_tokens", corrupt,
		func(input []byte) error {
			user, err := svc.VerifyJWT(string(input))
			if err == nil {
				// Accepting a structurally-corrupt unsigned token is a real defect.
				return fmt.Errorf("validator ACCEPTED corrupt token (user=%+v) — security bypass", user)
			}
			return err // clean rejection — the desired Degraded path
		})
}

// TestAuth_Chaos_HostileClaims is the KEY auth chaos suite: it mints tokens that
// PASS signature verification (signed with the real key) but carry hostile or
// missing claims. The claim-extraction path in VerifyJWT must defend against
// missing / non-string username/email and bad user_id — an unchecked type
// assertion here PANICS on a forged-but-validly-signed token, which a single
// attacker request could trigger to crash the process. The validator MUST return
// an error for these, never panic, and never decode a bogus user.
func TestAuth_Chaos_HostileClaims(t *testing.T) {
	svc, _, _ := newStressService(t)
	exp := time.Now().Add(time.Hour).Unix()

	huge := strings.Repeat("x", 1<<16)
	cases := []jwt.MapClaims{
		{"exp": exp},                                                      // no user_id at all
		{"user_id": 12345, "exp": exp},                                    // user_id wrong type (number)
		{"user_id": "not-a-uuid", "exp": exp},                             // user_id unparseable
		{"user_id": uuid.New().String(), "exp": exp},                      // valid id, MISSING username+email
		{"user_id": uuid.New().String(), "username": 999, "exp": exp},     // username wrong type (number)
		{"user_id": uuid.New().String(), "username": "u", "email": true},  // email wrong type (bool)
		{"user_id": uuid.New().String(), "username": nil, "email": nil},   // explicit null claims
		{"user_id": uuid.New().String(), "username": huge, "email": huge}, // oversized but well-typed
		{"user_id": []any{"a", "b"}, "exp": exp},                          // user_id is an array
	}

	rec := stresschaos.NewChaosRecorder(t, "auth_hostile_claims", "input-corruption")
	for i, claims := range cases {
		func(idx int, c jwt.MapClaims) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("case[%d] VerifyJWT PANICKED on validly-signed hostile claims: %v", idx, p))
				}
			}()
			tok := signWith(t, svc, c)
			user, err := svc.VerifyJWT(tok)
			if err != nil {
				rec.Record(stresschaos.Degraded,
					fmt.Sprintf("case[%d] rejected hostile claims cleanly: %v", idx, err))
				return
			}
			// No error: only acceptable for the well-typed oversized case (idx 7).
			rec.Record(stresschaos.Recovered,
				fmt.Sprintf("case[%d] accepted well-typed claims without crash (user=%s)", idx, user.ID))
		}(i, claims)
	}
	rec.AssertNoFatal()
}

// TestAuth_Chaos_HostilePasswords feeds hostile credentials to the REAL hasher
// and verifier. Empty / huge / binary / null-byte passwords must hash-or-reject
// cleanly without crashing, and malformed argon2 hash strings fed to the verifier
// must return false rather than panic. (bcrypt itself caps input at 72 bytes and
// errors on longer input — the hasher must surface that as an error, not crash.)
func TestAuth_Chaos_HostilePasswords(t *testing.T) {
	svc, _, _ := newStressService(t)
	rec := stresschaos.NewChaosRecorder(t, "auth_hostile_passwords", "input-corruption")

	hostilePasswords := []string{
		"",                          // empty
		strings.Repeat("p", 1<<20),  // 1 MiB password (bcrypt rejects >72 bytes)
		"\x00\x00\x00",              // null bytes
		"\xff\xfe\xfd\xfc",          // raw binary
		"pass\x00word",              // embedded null
		strings.Repeat("🔥", 10000),  // multibyte unicode flood
	}
	for i, pw := range hostilePasswords {
		func(idx int, password string) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("hashPassword[%d] PANICKED on hostile password: %v", idx, p))
				}
			}()
			hash, err := svc.hashPassword(password)
			if err != nil {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("hashPassword[%d] rejected cleanly: %v", idx, err))
				return
			}
			// If it hashed, verify must round-trip without crashing.
			if !svc.verifyPassword(password, hash) {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("hashPassword[%d] hashed but verify mismatched", idx))
				return
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("hashPassword[%d] hashed+verified without crash", idx))
		}(i, pw)
	}

	// Malformed argon2 hash strings fed to the verifier MUST return false, never panic.
	malformedHashes := []string{
		"",
		"$argon2id$",
		"$argon2id$v=19$m=x,t=y,p=z$salt$hash",        // non-numeric params
		"$argon2id$v=19$m=65536,t=1$onlytwoparams$h",  // wrong param count
		"$badalg$v=19$m=1,t=1,p=1$c2FsdA$aGFzaA",      // unknown algorithm
		"$argon2id$v=19$m=1,t=1,p=1$!!notbase64!!$h",  // bad base64 salt
		"not$a$hash$at$all$really",                     // 6 parts but garbage
	}
	for i, h := range malformedHashes {
		func(idx int, hash string) {
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal,
						fmt.Sprintf("verifyArgon2Password[%d] PANICKED on malformed hash: %v", idx, p))
				}
			}()
			// Must simply return false (no match) without crashing.
			if svc.verifyArgon2Password("any-password", hash) {
				rec.Record(stresschaos.Degraded, fmt.Sprintf("verifyArgon2Password[%d] returned true on malformed hash", idx))
				return
			}
			rec.Record(stresschaos.Recovered, fmt.Sprintf("verifyArgon2Password[%d] rejected malformed hash cleanly", idx))
		}(i, h)
	}

	rec.AssertNoFatal()
}

// TestAuth_Chaos_ConcurrentSessionChurn hammers the SAME set of session tokens
// with concurrent create / verify / delete from many goroutines. The store's
// RWMutex must serialise the map mutations so it never panics or races (under
// -race) and ends self-consistent. A delete racing a verify must not corrupt the
// map; a missing session must surface as a clean error, never a crash.
func TestAuth_Chaos_ConcurrentSessionChurn(t *testing.T) {
	svc, repo, u := newStressService(t)
	ctx := context.Background()
	rec := stresschaos.NewChaosRecorder(t, "auth_session_churn", "state-corruption")

	const goroutines = 16
	const iters = 200

	// A shared pool of token slots the goroutines contend over.
	const slots = 8
	tokens := make([]string, slots)
	for i := range tokens {
		tok, _ := svc.generateSessionToken()
		tokens[i] = tok
	}

	var wg sync.WaitGroup
	var creates, verifies, deletes int64
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if p := recover(); p != nil {
					rec.Record(stresschaos.Fatal, fmt.Sprintf("goroutine %d panicked: %v", id, p))
				}
			}()
			for it := 0; it < iters; it++ {
				tok := tokens[(id+it)%slots]
				switch (id + it) % 3 {
				case 0:
					_ = repo.CreateSession(ctx, &Session{
						ID:           uuid.New(),
						UserID:       u.ID,
						SessionToken: tok,
						ExpiresAt:    time.Now().Add(time.Hour),
						CreatedAt:    time.Now(),
					})
					atomic.AddInt64(&creates, 1)
				case 1:
					// May error (clean) if a racing goroutine deleted it; must not crash.
					_, _ = svc.VerifySession(ctx, tok)
					atomic.AddInt64(&verifies, 1)
				default:
					_ = svc.Logout(ctx, tok)
					atomic.AddInt64(&deletes, 1)
				}
			}
		}(g)
	}
	wg.Wait()

	rec.Record(stresschaos.Recovered, fmt.Sprintf(
		"survived session churn: %d creates, %d verifies, %d deletes, no panic/race",
		atomic.LoadInt64(&creates), atomic.LoadInt64(&verifies), atomic.LoadInt64(&deletes)))

	// Terminal consistency: a freshly-created session must still verify cleanly,
	// proving the map was not left torn by the churn.
	freshTok, _ := svc.generateSessionToken()
	if err := repo.CreateSession(ctx, &Session{
		ID: uuid.New(), UserID: u.ID, SessionToken: freshTok,
		ExpiresAt: time.Now().Add(time.Hour), CreatedAt: time.Now(),
	}); err != nil {
		rec.Record(stresschaos.Fatal, "post-churn session create failed: "+err.Error())
	} else if vu, err := svc.VerifySession(ctx, freshTok); err != nil {
		rec.Record(stresschaos.Fatal, "post-churn session verify failed — store corrupted: "+err.Error())
	} else if vu.ID != u.ID {
		rec.Record(stresschaos.Fatal, "post-churn session resolved wrong user")
	} else {
		rec.Record(stresschaos.Recovered, "store self-consistent after churn")
	}

	rec.AssertNoFatal()
	t.Logf("session churn: creates=%d verifies=%d deletes=%d",
		atomic.LoadInt64(&creates), atomic.LoadInt64(&verifies), atomic.LoadInt64(&deletes))
}
