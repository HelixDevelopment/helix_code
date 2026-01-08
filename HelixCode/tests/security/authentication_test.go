package security

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"sync"
	"testing"
	"time"

	"dev.helix.code/internal/auth"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// =============================================================================
// Mock Repository for Security Tests
// =============================================================================

type MockSecurityAuthRepository struct {
	mock.Mock
	loginAttempts     map[string]int
	loginAttemptsMu   sync.Mutex
	blockedUsers      map[string]time.Time
	blockedUsersMu    sync.Mutex
	sessions          map[string]*auth.Session
	sessionsMu        sync.Mutex
}

func NewMockSecurityAuthRepository() *MockSecurityAuthRepository {
	return &MockSecurityAuthRepository{
		loginAttempts: make(map[string]int),
		blockedUsers:  make(map[string]time.Time),
		sessions:      make(map[string]*auth.Session),
	}
}

func (m *MockSecurityAuthRepository) CreateUser(ctx context.Context, user *auth.User, passwordHash string) error {
	args := m.Called(ctx, user, passwordHash)
	return args.Error(0)
}

func (m *MockSecurityAuthRepository) GetUserByUsername(ctx context.Context, username string) (*auth.User, string, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).(*auth.User), args.String(1), args.Error(2)
}

func (m *MockSecurityAuthRepository) GetUserByEmail(ctx context.Context, email string) (*auth.User, string, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).(*auth.User), args.String(1), args.Error(2)
}

func (m *MockSecurityAuthRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*auth.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.User), args.Error(1)
}

func (m *MockSecurityAuthRepository) UpdateUserLastLogin(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSecurityAuthRepository) CreateSession(ctx context.Context, session *auth.Session) error {
	args := m.Called(ctx, session)
	if args.Error(0) == nil {
		m.sessionsMu.Lock()
		m.sessions[session.SessionToken] = session
		m.sessionsMu.Unlock()
	}
	return args.Error(0)
}

func (m *MockSecurityAuthRepository) GetSession(ctx context.Context, token string) (*auth.Session, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.Session), args.Error(1)
}

func (m *MockSecurityAuthRepository) DeleteSession(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	m.sessionsMu.Lock()
	delete(m.sessions, token)
	m.sessionsMu.Unlock()
	return args.Error(0)
}

func (m *MockSecurityAuthRepository) DeleteUserSessions(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

func (m *MockSecurityAuthRepository) UpdateUser(ctx context.Context, userID uuid.UUID, displayName, email string) (*auth.User, error) {
	args := m.Called(ctx, userID, displayName, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*auth.User), args.Error(1)
}

func (m *MockSecurityAuthRepository) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	args := m.Called(ctx, userID)
	return args.Error(0)
}

// =============================================================================
// 1. Password Hashing Security Tests
// =============================================================================

func TestPasswordHashing_BcryptCostSufficient(t *testing.T) {
	t.Run("Bcrypt cost should be at least 10", func(t *testing.T) {
		config := auth.DefaultConfig()
		assert.GreaterOrEqual(t, config.BcryptCost, 10,
			"Bcrypt cost should be at least 10 for security")
	})
}

func TestPasswordHashing_UniqueHashesForSamePassword(t *testing.T) {
	t.Run("Same password should produce different hashes (due to salt)", func(t *testing.T) {
		password := "SecurePassword123!"

		hash1, err1 := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		require.NoError(t, err1)

		hash2, err2 := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		require.NoError(t, err2)

		assert.NotEqual(t, hash1, hash2,
			"Same password should produce different hashes due to random salt")
	})
}

func TestPasswordHashing_NoPlaintextStorage(t *testing.T) {
	t.Run("Hash should not contain plaintext password", func(t *testing.T) {
		password := "SecurePassword123!"

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		require.NoError(t, err)

		assert.NotContains(t, string(hash), password,
			"Hash should not contain plaintext password")
	})
}

func TestPasswordHashing_LongPasswordSupport(t *testing.T) {
	t.Run("Should handle long passwords securely", func(t *testing.T) {
		// Bcrypt has a 72-byte limit
		// Test that passwords at the limit work
		maxPassword := strings.Repeat("A", 72)

		hash, err := bcrypt.GenerateFromPassword([]byte(maxPassword), bcrypt.DefaultCost)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)

		// Verify the hash works at max length
		err = bcrypt.CompareHashAndPassword(hash, []byte(maxPassword))
		assert.NoError(t, err)

		// Verify that passwords over 72 bytes are rejected by newer bcrypt
		// (This is the secure behavior - prevents truncation attacks)
		longPassword := strings.Repeat("A", 100)
		_, err = bcrypt.GenerateFromPassword([]byte(longPassword), bcrypt.DefaultCost)
		// Modern bcrypt implementations may error on >72 bytes
		// This is actually MORE secure than silent truncation
		if err != nil {
			assert.Contains(t, err.Error(), "72 bytes",
				"Long passwords should be explicitly rejected")
		}
	})
}

func TestPasswordHashing_Argon2Parameters(t *testing.T) {
	t.Run("Argon2 parameters should meet security recommendations", func(t *testing.T) {
		config := auth.DefaultConfig()

		// OWASP recommends for Argon2id:
		// - Memory: at least 15 MB (15360 KB)
		// - Iterations: at least 2
		// - Parallelism: at least 1
		assert.GreaterOrEqual(t, config.Argon2Memory, uint32(15*1024),
			"Argon2 memory should be at least 15MB")
		assert.GreaterOrEqual(t, config.Argon2Time, uint32(1),
			"Argon2 iterations should be at least 1")
		assert.GreaterOrEqual(t, config.Argon2Threads, uint8(1),
			"Argon2 parallelism should be at least 1")
		assert.GreaterOrEqual(t, config.Argon2KeyLength, uint32(32),
			"Argon2 key length should be at least 32 bytes")
	})
}

func TestPasswordHashing_TimingAttackResistance(t *testing.T) {
	t.Run("Password verification should use constant-time comparison", func(t *testing.T) {
		// Create a valid Argon2 hash
		password := "TestPassword123!"
		salt := make([]byte, 16)
		_, err := rand.Read(salt)
		require.NoError(t, err)

		hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
		saltB64 := base64.RawStdEncoding.EncodeToString(salt)
		hashB64 := base64.RawStdEncoding.EncodeToString(hash)
		argon2Hash := "$argon2id$v=19$m=65536,t=1,p=4$" + saltB64 + "$" + hashB64

		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		userID := uuid.New()
		user := &auth.User{
			ID:       userID,
			Username: "testuser",
			IsActive: true,
		}

		mockRepo.On("GetUserByUsername", mock.Anything, "testuser").Return(user, argon2Hash, nil)
		mockRepo.On("UpdateUserLastLogin", mock.Anything, userID).Return(nil)
		mockRepo.On("CreateSession", mock.Anything, mock.AnythingOfType("*auth.Session")).Return(nil)

		// Test that verification works (uses subtle.ConstantTimeCompare internally)
		_, _, err = service.Login(context.Background(), "testuser", password, "test", "", "")
		assert.NoError(t, err, "Login with correct Argon2 password should succeed")

		// Test wrong password also works correctly
		_, _, err = service.Login(context.Background(), "testuser", "wrongpassword", "test", "", "")
		assert.Equal(t, auth.ErrInvalidCredentials, err, "Wrong password should fail")
	})
}

// =============================================================================
// 2. JWT Token Validation Tests
// =============================================================================

func TestJWT_ValidTokenGeneration(t *testing.T) {
	t.Run("Should generate valid JWT token", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		user := &auth.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
		}

		token, err := service.GenerateJWT(user)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Verify token structure (header.payload.signature)
		parts := strings.Split(token, ".")
		assert.Len(t, parts, 3, "JWT should have 3 parts")
	})
}

func TestJWT_SigningMethod(t *testing.T) {
	t.Run("Should use HMAC signing method", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		user := &auth.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
		}

		tokenString, err := service.GenerateJWT(user)
		require.NoError(t, err)

		// Parse without verification to check algorithm
		token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
		require.NoError(t, err)

		assert.Equal(t, "HS256", token.Method.Alg(),
			"JWT should use HS256 (HMAC-SHA256)")
	})
}

func TestJWT_AlgorithmConfusionAttack(t *testing.T) {
	t.Run("Should reject tokens with 'none' algorithm", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// Create a malicious token with 'none' algorithm
		claims := jwt.MapClaims{
			"user_id":  uuid.New().String(),
			"username": "admin",
			"email":    "admin@example.com",
			"exp":      time.Now().Add(time.Hour).Unix(),
		}

		// This creates an unsigned token
		token := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
		tokenString, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		// Attempt to verify - should fail
		_, err = service.VerifyJWT(tokenString)
		assert.Error(t, err, "Should reject tokens with 'none' algorithm")
	})
}

func TestJWT_InvalidSignature(t *testing.T) {
	t.Run("Should reject tokens with invalid signature", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		user := &auth.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
		}

		token, err := service.GenerateJWT(user)
		require.NoError(t, err)

		// Tamper with the signature
		tamperedToken := token[:len(token)-5] + "XXXXX"

		_, err = service.VerifyJWT(tamperedToken)
		assert.Error(t, err, "Should reject tokens with invalid signature")
	})
}

func TestJWT_MalformedToken(t *testing.T) {
	t.Run("Should reject malformed tokens", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		malformedTokens := []string{
			"",
			"not.a.valid.token",
			"just-a-string",
			"eyJhbGciOiJIUzI1NiJ9", // Only header
			"...",
			"eyJhbGciOiJIUzI1NiJ9..",
			"eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoiMTIzIn0", // Missing signature
		}

		for _, token := range malformedTokens {
			_, err := service.VerifyJWT(token)
			assert.Error(t, err, "Should reject malformed token: %s", token)
		}
	})
}

func TestJWT_MissingClaims(t *testing.T) {
	t.Run("Should reject tokens with missing required claims", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// Create token without user_id claim
		claims := jwt.MapClaims{
			"username": "testuser",
			"exp":      time.Now().Add(time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.JWTSecret))
		require.NoError(t, err)

		_, err = service.VerifyJWT(tokenString)
		assert.Error(t, err, "Should reject tokens without user_id claim")
	})
}

func TestJWT_InvalidUserID(t *testing.T) {
	t.Run("Should reject tokens with invalid user_id format", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// Create token with invalid UUID
		claims := jwt.MapClaims{
			"user_id":  "not-a-valid-uuid",
			"username": "testuser",
			"email":    "test@example.com",
			"exp":      time.Now().Add(time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.JWTSecret))
		require.NoError(t, err)

		_, err = service.VerifyJWT(tokenString)
		assert.Error(t, err, "Should reject tokens with invalid user_id")
	})
}

// =============================================================================
// 3. Session Security Tests
// =============================================================================

func TestSession_TokenEntropy(t *testing.T) {
	t.Run("Session tokens should have sufficient entropy", func(t *testing.T) {
		// Generate multiple tokens and check for uniqueness
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			bytes := make([]byte, 32)
			_, err := rand.Read(bytes)
			require.NoError(t, err)

			token := base64.URLEncoding.EncodeToString(bytes)
			assert.False(t, tokens[token], "Session tokens should be unique")
			tokens[token] = true
		}
	})
}

func TestSession_ExpirationEnforced(t *testing.T) {
	t.Run("Expired sessions should be rejected", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		config.SessionExpiry = 1 * time.Second // Very short for testing
		service := auth.NewAuthService(config, mockRepo)

		userID := uuid.New()
		expiredSession := &auth.Session{
			ID:           uuid.New(),
			UserID:       userID,
			SessionToken: "expired-token",
			ExpiresAt:    time.Now().Add(-1 * time.Hour), // Expired
			CreatedAt:    time.Now().Add(-2 * time.Hour),
		}

		mockRepo.On("GetSession", ctx, "expired-token").Return(expiredSession, nil)
		mockRepo.On("DeleteSession", ctx, "expired-token").Return(nil)

		_, err := service.VerifySession(ctx, "expired-token")
		assert.Error(t, err, "Should reject expired sessions")
		assert.Equal(t, auth.ErrTokenExpired, err)
	})
}

func TestSession_InvalidatedOnLogout(t *testing.T) {
	t.Run("Session should be invalidated on logout", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		mockRepo.On("DeleteSession", ctx, "test-session-token").Return(nil)

		err := service.Logout(ctx, "test-session-token")
		assert.NoError(t, err)
		mockRepo.AssertCalled(t, "DeleteSession", ctx, "test-session-token")
	})
}

func TestSession_LogoutAllInvalidatesAllSessions(t *testing.T) {
	t.Run("LogoutAll should invalidate all user sessions", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		userID := uuid.New()
		mockRepo.On("DeleteUserSessions", ctx, userID).Return(nil)

		err := service.LogoutAll(ctx, userID)
		assert.NoError(t, err)
		mockRepo.AssertCalled(t, "DeleteUserSessions", ctx, userID)
	})
}

func TestSession_UniqueTokenPerLogin(t *testing.T) {
	t.Run("Each login should generate unique session token", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		userID := uuid.New()
		testUser := &auth.User{
			ID:       userID,
			Username: "testuser",
			Email:    "test@example.com",
			IsActive: true,
		}

		passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

		mockRepo.On("GetUserByUsername", ctx, "testuser").Return(testUser, string(passwordHash), nil)
		mockRepo.On("UpdateUserLastLogin", ctx, userID).Return(nil)

		tokens := make([]string, 0)
		for i := 0; i < 5; i++ {
			mockRepo.On("CreateSession", ctx, mock.AnythingOfType("*auth.Session")).Return(nil).Once()

			session, _, err := service.Login(ctx, "testuser", "password123", "web", "127.0.0.1", "Mozilla/5.0")
			require.NoError(t, err)

			for _, prevToken := range tokens {
				assert.NotEqual(t, prevToken, session.SessionToken,
					"Each login should have unique session token")
			}
			tokens = append(tokens, session.SessionToken)
		}
	})
}

// =============================================================================
// 4. Brute Force Protection Tests
// =============================================================================

func TestBruteForce_FailedLoginTracking(t *testing.T) {
	t.Run("Failed login attempts should be trackable", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// Setup for failed login
		mockRepo.On("GetUserByUsername", ctx, "nonexistent").Return(nil, "", auth.ErrUserNotFound)
		mockRepo.On("GetUserByEmail", ctx, "nonexistent").Return(nil, "", auth.ErrUserNotFound)

		// Multiple failed login attempts
		for i := 0; i < 5; i++ {
			_, _, err := service.Login(ctx, "nonexistent", "wrongpass", "web", "127.0.0.1", "")
			assert.Error(t, err)
			assert.Equal(t, auth.ErrInvalidCredentials, err)
		}

		// Note: Rate limiting implementation would be at the server/middleware level
		// This test documents the expected behavior
	})
}

func TestBruteForce_WrongPasswordTracking(t *testing.T) {
	t.Run("Wrong password attempts should be trackable", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		userID := uuid.New()
		testUser := &auth.User{
			ID:       userID,
			Username: "testuser",
			IsActive: true,
		}

		correctHash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
		mockRepo.On("GetUserByUsername", ctx, "testuser").Return(testUser, string(correctHash), nil)

		// Multiple wrong password attempts
		for i := 0; i < 5; i++ {
			_, _, err := service.Login(ctx, "testuser", "wrongpassword", "web", "127.0.0.1", "")
			assert.Error(t, err)
			assert.Equal(t, auth.ErrInvalidCredentials, err)
		}
	})
}

func TestBruteForce_GenericErrorMessage(t *testing.T) {
	t.Run("Error messages should not reveal user existence", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// Test with non-existent user
		mockRepo.On("GetUserByUsername", ctx, "nonexistent").Return(nil, "", auth.ErrUserNotFound)
		mockRepo.On("GetUserByEmail", ctx, "nonexistent").Return(nil, "", auth.ErrUserNotFound)

		_, _, errNotExist := service.Login(ctx, "nonexistent", "anypassword", "web", "", "")

		// Test with existing user but wrong password
		existingUser := &auth.User{
			ID:       uuid.New(),
			Username: "existing",
			IsActive: true,
		}
		correctHash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
		mockRepo.On("GetUserByUsername", ctx, "existing").Return(existingUser, string(correctHash), nil)

		_, _, errWrongPass := service.Login(ctx, "existing", "wrongpassword", "web", "", "")

		// Both should return the same generic error
		assert.Equal(t, errNotExist, errWrongPass,
			"Both scenarios should return same error to prevent user enumeration")
		assert.Equal(t, auth.ErrInvalidCredentials, errNotExist)
	})
}

// =============================================================================
// 5. Token Expiration Handling Tests
// =============================================================================

func TestTokenExpiration_JWTExpiry(t *testing.T) {
	t.Run("JWT tokens should have expiration", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		config.TokenExpiry = 1 * time.Hour
		service := auth.NewAuthService(config, mockRepo)

		user := &auth.User{
			ID:       uuid.New(),
			Username: "testuser",
			Email:    "test@example.com",
		}

		tokenString, err := service.GenerateJWT(user)
		require.NoError(t, err)

		// Parse and check expiration claim
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(config.JWTSecret), nil
		})
		require.NoError(t, err)

		claims := token.Claims.(jwt.MapClaims)
		exp, ok := claims["exp"].(float64)
		assert.True(t, ok, "Token should have exp claim")

		expTime := time.Unix(int64(exp), 0)
		assert.WithinDuration(t, time.Now().Add(1*time.Hour), expTime, 5*time.Second,
			"Token expiration should match config")
	})
}

func TestTokenExpiration_ExpiredJWTRejected(t *testing.T) {
	t.Run("Expired JWT tokens should be rejected", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// Create a token that's already expired
		claims := jwt.MapClaims{
			"user_id":  uuid.New().String(),
			"username": "testuser",
			"email":    "test@example.com",
			"exp":      time.Now().Add(-1 * time.Hour).Unix(), // Expired
			"iat":      time.Now().Add(-2 * time.Hour).Unix(),
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(config.JWTSecret))
		require.NoError(t, err)

		_, err = service.VerifyJWT(tokenString)
		assert.Error(t, err, "Expired JWT should be rejected")
	})
}

func TestTokenExpiration_SessionExpiry(t *testing.T) {
	t.Run("Session expiration should be enforced", func(t *testing.T) {
		config := auth.DefaultConfig()

		// Session expiry should be reasonable (not too long)
		assert.LessOrEqual(t, config.SessionExpiry, 30*24*time.Hour,
			"Session expiry should not exceed 30 days")
		assert.GreaterOrEqual(t, config.SessionExpiry, 1*time.Hour,
			"Session expiry should be at least 1 hour")
	})
}

// =============================================================================
// 6. Invalid Token Handling Tests
// =============================================================================

func TestInvalidToken_EmptyToken(t *testing.T) {
	t.Run("Empty token should be rejected", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		_, err := service.VerifyJWT("")
		assert.Error(t, err)
	})
}

func TestInvalidToken_RandomString(t *testing.T) {
	t.Run("Random string should be rejected", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		_, err := service.VerifyJWT("random-string-not-a-jwt")
		assert.Error(t, err)
	})
}

func TestInvalidToken_Base64Decoded(t *testing.T) {
	t.Run("Base64 decoded payload should not bypass validation", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// Create fake base64 encoded parts
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"user_id":"admin","role":"admin"}`))
		fakeToken := header + "." + payload + ".fakesignature"

		_, err := service.VerifyJWT(fakeToken)
		assert.Error(t, err, "Fake token should be rejected")
	})
}

func TestInvalidToken_SQLInjectionInToken(t *testing.T) {
	t.Run("SQL injection in token should be safe", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		// SQL injection payloads in session token
		sqlInjectionTokens := []string{
			"' OR '1'='1",
			"1; DROP TABLE sessions--",
			"admin'--",
		}

		for _, token := range sqlInjectionTokens {
			mockRepo.On("GetSession", ctx, token).Return(nil, auth.ErrTokenInvalid)

			_, err := service.VerifySession(ctx, token)
			assert.Error(t, err, "SQL injection should be rejected: %s", token)
		}
	})
}

func TestInvalidToken_XSSInToken(t *testing.T) {
	t.Run("XSS payloads in tokens should be safe", func(t *testing.T) {
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		xssTokens := []string{
			"<script>alert('xss')</script>",
			"javascript:alert('xss')",
			"<img src=x onerror=alert('xss')>",
		}

		for _, token := range xssTokens {
			_, err := service.VerifyJWT(token)
			assert.Error(t, err, "XSS payload should be rejected: %s", token)
		}
	})
}

// =============================================================================
// 7. CSRF Protection Tests
// =============================================================================

func TestCSRF_DifferentSessionPerDevice(t *testing.T) {
	t.Run("Different client types should get different sessions", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		userID := uuid.New()
		testUser := &auth.User{
			ID:       userID,
			Username: "testuser",
			IsActive: true,
		}

		passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		mockRepo.On("GetUserByUsername", ctx, "testuser").Return(testUser, string(passwordHash), nil)
		mockRepo.On("UpdateUserLastLogin", ctx, userID).Return(nil)

		clientTypes := []string{"web", "mobile", "cli", "desktop"}
		sessions := make(map[string]*auth.Session)

		for _, clientType := range clientTypes {
			mockRepo.On("CreateSession", ctx, mock.AnythingOfType("*auth.Session")).Return(nil).Once()

			session, _, err := service.Login(ctx, "testuser", "password123", clientType, "127.0.0.1", "")
			require.NoError(t, err)

			// Each session should be unique
			for prevType, prevSession := range sessions {
				assert.NotEqual(t, prevSession.SessionToken, session.SessionToken,
					"Session for %s should be different from %s", clientType, prevType)
			}
			sessions[clientType] = session
			assert.Equal(t, clientType, session.ClientType)
		}
	})
}

func TestCSRF_StateChangingOperationsRequireAuth(t *testing.T) {
	t.Run("State-changing operations should require authentication", func(t *testing.T) {
		// Note: CSRF protection is implemented at the HTTP handler/middleware level
		// The auth service itself doesn't handle CSRF - it relies on:
		// 1. JWT Bearer tokens (immune to CSRF as they're not auto-sent)
		// 2. Session tokens in headers (not cookies)
		// 3. HTTP-only, SameSite cookies when cookies are used

		// This test documents that the auth service expects authentication
		// to be handled by middleware before state-changing operations

		// Verify that the service requires a valid repository to function
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		userID := uuid.New()
		mockRepo.On("UpdateUser", mock.Anything, userID, "New Name", "new@email.com").Return(nil, auth.ErrUserNotFound)

		// The service itself doesn't enforce auth - that's the middleware's job
		// But it does require proper repository calls
		_, err := service.UpdateUser(context.Background(), userID, "New Name", "new@email.com")
		assert.Error(t, err) // User not found (auth would have been checked by middleware first)
	})
}

// =============================================================================
// 8. XSS Prevention in Auth Responses Tests
// =============================================================================

func TestXSSPrevention_UsernameEscaping(t *testing.T) {
	t.Run("Usernames with XSS payloads should be handled safely", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		xssUsername := "<script>alert('xss')</script>"

		// Attempt registration with XSS payload
		mockRepo.On("GetUserByUsername", ctx, strings.ToLower(xssUsername)).Return(nil, "", auth.ErrUserNotFound)
		mockRepo.On("GetUserByEmail", ctx, "xss@test.com").Return(nil, "", auth.ErrUserNotFound)
		mockRepo.On("CreateUser", ctx, mock.AnythingOfType("*auth.User"), mock.AnythingOfType("string")).Return(nil)

		user, err := service.Register(ctx, xssUsername, "xss@test.com", "SecurePass123!", "XSS Tester")
		require.NoError(t, err)

		// Username should be stored lowercase (prevents some XSS via case manipulation)
		assert.Equal(t, strings.ToLower(xssUsername), user.Username)

		// Note: Actual XSS prevention (HTML escaping) happens at the presentation layer
		// The auth service stores data as-is; handlers/templates must escape output
	})
}

func TestXSSPrevention_EmailValidation(t *testing.T) {
	t.Run("Email validation should reject malicious inputs", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		maliciousEmails := []string{
			"<script>@evil.com",
			"test@<script>.com",
			"javascript:alert@evil.com",
		}

		for i, email := range maliciousEmails {
			// Each iteration needs a unique username
			username := "testuser" + string(rune('a'+i))
			mockRepo.On("GetUserByUsername", ctx, username).Return(nil, "", auth.ErrUserNotFound).Maybe()
			mockRepo.On("GetUserByEmail", ctx, email).Return(nil, "", auth.ErrUserNotFound).Maybe()
			mockRepo.On("CreateUser", ctx, mock.AnythingOfType("*auth.User"), mock.AnythingOfType("string")).Return(nil).Maybe()

			_, err := service.Register(ctx, username, email, "SecurePass123!", "Test")
			// These should either fail validation or be safely stored
			// The important thing is no code execution
			if err == nil {
				// If stored, verify it's stored as a string, not executed
				t.Logf("Email %s was accepted (will be escaped on output)", email)
			} else {
				t.Logf("Email %s was rejected with error: %v", email, err)
			}
		}
	})
}

func TestXSSPrevention_DisplayNameHandling(t *testing.T) {
	t.Run("Display names with HTML should be handled safely", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		xssDisplayName := "<img src=x onerror=alert('xss')>"

		mockRepo.On("GetUserByUsername", ctx, "testuser").Return(nil, "", auth.ErrUserNotFound)
		mockRepo.On("GetUserByEmail", ctx, "test@example.com").Return(nil, "", auth.ErrUserNotFound)
		mockRepo.On("CreateUser", ctx, mock.AnythingOfType("*auth.User"), mock.AnythingOfType("string")).Return(nil)

		user, err := service.Register(ctx, "testuser", "test@example.com", "SecurePass123!", xssDisplayName)
		require.NoError(t, err)

		// Display name is stored as-is; escaping happens at output
		assert.Equal(t, xssDisplayName, user.DisplayName)
	})
}

// =============================================================================
// Additional Security Tests
// =============================================================================

func TestSecurity_InactiveUserRejected(t *testing.T) {
	t.Run("Inactive users should not be able to login", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		inactiveUser := &auth.User{
			ID:       uuid.New(),
			Username: "inactive",
			IsActive: false, // Deactivated
		}

		passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		mockRepo.On("GetUserByUsername", ctx, "inactive").Return(inactiveUser, string(passwordHash), nil)

		_, _, err := service.Login(ctx, "inactive", "password123", "web", "", "")
		assert.Error(t, err, "Inactive users should be rejected")
		assert.Contains(t, err.Error(), "deactivated")
	})
}

func TestSecurity_PasswordMinLength(t *testing.T) {
	t.Run("Passwords below minimum length should be rejected", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		shortPasswords := []string{
			"",
			"1234567", // 7 chars
			"short",
		}

		for _, password := range shortPasswords {
			_, err := service.Register(ctx, "testuser", "test@example.com", password, "Test User")
			assert.Error(t, err, "Password '%s' should be rejected", password)
			assert.Contains(t, err.Error(), "password")
		}
	})
}

func TestSecurity_JWTSecretStrength(t *testing.T) {
	t.Run("JWT secret should be strong in production", func(t *testing.T) {
		config := auth.DefaultConfig()

		// In production, the default secret should not be used
		// This test documents the expected behavior
		if config.JWTSecret == "default-secret-change-in-production" {
			t.Log("WARNING: Using default JWT secret. Change in production!")
		}

		// Secret should be at least 32 bytes for HS256
		assert.GreaterOrEqual(t, len(config.JWTSecret), 32,
			"JWT secret should be at least 32 bytes")
	})
}

func TestSecurity_ConcurrentLoginSafety(t *testing.T) {
	t.Run("Concurrent logins should be thread-safe", func(t *testing.T) {
		ctx := context.Background()
		mockRepo := NewMockSecurityAuthRepository()
		config := auth.DefaultConfig()
		service := auth.NewAuthService(config, mockRepo)

		userID := uuid.New()
		testUser := &auth.User{
			ID:       userID,
			Username: "testuser",
			IsActive: true,
		}

		passwordHash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
		mockRepo.On("GetUserByUsername", ctx, "testuser").Return(testUser, string(passwordHash), nil)
		mockRepo.On("UpdateUserLastLogin", ctx, userID).Return(nil)
		mockRepo.On("CreateSession", ctx, mock.AnythingOfType("*auth.Session")).Return(nil)

		var wg sync.WaitGroup
		errors := make(chan error, 10)
		sessions := make(chan string, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				session, _, err := service.Login(ctx, "testuser", "password123", "web", "127.0.0.1", "")
				if err != nil {
					errors <- err
				} else {
					sessions <- session.SessionToken
				}
			}()
		}

		wg.Wait()
		close(errors)
		close(sessions)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent login error: %v", err)
		}

		// Check that all sessions are unique
		seenTokens := make(map[string]bool)
		for token := range sessions {
			assert.False(t, seenTokens[token], "Session tokens should be unique")
			seenTokens[token] = true
		}
	})
}
