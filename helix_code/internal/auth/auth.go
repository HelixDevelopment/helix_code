package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Errors
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrTokenExpired       = errors.New("token expired")
	ErrTokenInvalid       = errors.New("invalid token")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	// ErrAuthBackendUnavailable is returned when the authentication
	// service has no usable data store (HXC-043). The HelixCode server
	// boots with db=nil on its documented "continuing without database"
	// path; every db-touching AuthService method MUST surface this clean
	// error instead of dereferencing a nil s.db and panicking. The server
	// maps it to HTTP 503 (auth backend down) — the request fails cleanly
	// rather than crashing the connection with an empty 500.
	ErrAuthBackendUnavailable = errors.New("authentication backend unavailable")
)

// User represents an authenticated user
type User struct {
	ID          uuid.UUID `json:"id"`
	Username    string    `json:"username"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	IsActive    bool      `json:"is_active"`
	IsVerified  bool      `json:"is_verified"`
	MFAEnabled  bool      `json:"mfa_enabled"`
	LastLogin   time.Time `json:"last_login"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Session represents a user session
type Session struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	SessionToken string    `json:"session_token"`
	ClientType   string    `json:"client_type"`
	IPAddress    net.IP    `json:"ip_address"`
	UserAgent    string    `json:"user_agent"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret       string
	TokenExpiry     time.Duration
	SessionExpiry   time.Duration
	BcryptCost      int
	Argon2Time      uint32
	Argon2Memory    uint32
	Argon2Threads   uint8
	Argon2KeyLength uint32
}

// DefaultConfig returns a default authentication configuration
func DefaultConfig() AuthConfig {
	return AuthConfig{
		JWTSecret:       "default-secret-change-in-production",
		TokenExpiry:     24 * time.Hour,
		SessionExpiry:   7 * 24 * time.Hour,
		BcryptCost:      bcrypt.DefaultCost,
		Argon2Time:      1,
		Argon2Memory:    64 * 1024,
		Argon2Threads:   4,
		Argon2KeyLength: 32,
	}
}

// AuthService provides authentication and authorization services
type AuthService struct {
	config AuthConfig
	db     AuthRepository
}

// AuthRepository defines the interface for authentication data storage
type AuthRepository interface {
	CreateUser(ctx context.Context, user *User, passwordHash string) error
	GetUserByUsername(ctx context.Context, username string) (*User, string, error)
	GetUserByEmail(ctx context.Context, email string) (*User, string, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdateUserLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateUser(ctx context.Context, userID uuid.UUID, displayName, email string) (*User, error)
	DeleteUser(ctx context.Context, userID uuid.UUID) error
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, token string) (*Session, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteUserSessions(ctx context.Context, userID uuid.UUID) error
}

// NewAuthService creates a new authentication service
func NewAuthService(config AuthConfig, db AuthRepository) *AuthService {
	return &AuthService{
		config: config,
		db:     db,
	}
}

// dbAvailable reports whether this AuthService has a usable data store.
// It is nil-receiver safe: the HelixCode server may invoke a method on a
// nil *AuthService (server boots with db=nil → authService stays nil, and
// a method call on a nil pointer receiver is legal in Go until the body
// dereferences a field). Every db-touching method calls this first so a
// missing backend yields ErrAuthBackendUnavailable instead of a
// nil-pointer panic (HXC-043).
func (s *AuthService) dbAvailable() bool {
	return s != nil && s.db != nil
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, username, email, password, displayName string) (*User, error) {
	// HXC-043: no data store → cannot persist a new user. Fail cleanly
	// instead of panicking on a nil s.db.
	if !s.dbAvailable() {
		return nil, ErrAuthBackendUnavailable
	}

	// Validate input
	if err := s.validateRegistration(username, email, password); err != nil {
		return nil, err
	}

	// Check if user already exists
	if _, _, err := s.db.GetUserByUsername(ctx, username); err == nil {
		return nil, ErrUserExists
	}

	if _, _, err := s.db.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrUserExists
	}

	// Hash password
	passwordHash, err := s.hashPassword(password)
	if err != nil {
		return nil, errors.New(tr(ctx, "internal_auth_failed_hash_password", map[string]any{"Err": err.Error()}))
	}

	// Create user
	user := &User{
		ID:          uuid.New(),
		Username:    strings.ToLower(username),
		Email:       strings.ToLower(email),
		DisplayName: displayName,
		IsActive:    true,
		IsVerified:  false,
		MFAEnabled:  false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// Save user to database
	if err := s.db.CreateUser(ctx, user, passwordHash); err != nil {
		return nil, errors.New(tr(ctx, "internal_auth_failed_create_user", map[string]any{"Err": err.Error()}))
	}

	return user, nil
}

// Login authenticates a user and creates a session
func (s *AuthService) Login(ctx context.Context, username, password, clientType, ipAddress, userAgent string) (*Session, *User, error) {
	// HXC-043: with no data store there is no user to authenticate against.
	// Return the existing invalid-credentials sentinel (handler maps it to
	// 401 "Login failed") rather than dereferencing a nil s.db and panicking
	// into an empty HTTP 500. A login attempt can never succeed without a
	// user store, so 401 is the honest, scheme-preserving verdict.
	if !s.dbAvailable() {
		return nil, nil, ErrInvalidCredentials
	}

	// Get user and password hash
	user, passwordHash, err := s.db.GetUserByUsername(ctx, strings.ToLower(username))
	if err != nil {
		// Try email
		user, passwordHash, err = s.db.GetUserByEmail(ctx, strings.ToLower(username))
		if err != nil {
			return nil, nil, ErrInvalidCredentials
		}
	}

	// Check if user is active
	if !user.IsActive {
		return nil, nil, errors.New(tr(ctx, "internal_auth_account_deactivated", nil))
	}

	// Verify password
	if !s.verifyPassword(password, passwordHash) {
		return nil, nil, ErrInvalidCredentials
	}

	// Update last login
	if err := s.db.UpdateUserLastLogin(ctx, user.ID); err != nil {
		return nil, nil, errors.New(tr(ctx, "internal_auth_failed_update_last_login", map[string]any{"Err": err.Error()}))
	}

	// Create session
	sessionToken, err := s.generateSessionToken()
	if err != nil {
		return nil, nil, errors.New(tr(ctx, "internal_auth_failed_generate_session_token", map[string]any{"Err": err.Error()}))
	}

	var ip net.IP
	if ipAddress != "" {
		ip = net.ParseIP(ipAddress)
	}

	session := &Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		SessionToken: sessionToken,
		ClientType:   clientType,
		IPAddress:    ip,
		UserAgent:    userAgent,
		ExpiresAt:    time.Now().Add(s.config.SessionExpiry),
		CreatedAt:    time.Now(),
	}

	if err := s.db.CreateSession(ctx, session); err != nil {
		return nil, nil, errors.New(tr(ctx, "internal_auth_failed_create_session", map[string]any{"Err": err.Error()}))
	}

	return session, user, nil
}

// VerifySession verifies a session token and returns the associated user
func (s *AuthService) VerifySession(ctx context.Context, sessionToken string) (*User, error) {
	// HXC-043: no session store → token cannot be verified. Treat as an
	// invalid token (handler maps to 401) rather than panicking on nil s.db.
	if !s.dbAvailable() {
		return nil, ErrTokenInvalid
	}

	session, err := s.db.GetSession(ctx, sessionToken)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	if time.Now().After(session.ExpiresAt) {
		// Delete expired session
		_ = s.db.DeleteSession(ctx, sessionToken)
		return nil, ErrTokenExpired
	}

	user, err := s.db.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if !user.IsActive {
		return nil, errors.New(tr(ctx, "internal_auth_account_deactivated", nil))
	}

	return user, nil
}

// Logout invalidates a session
func (s *AuthService) Logout(ctx context.Context, sessionToken string) error {
	// HXC-043: no session store → nothing to invalidate. Fail cleanly.
	if !s.dbAvailable() {
		return ErrAuthBackendUnavailable
	}
	return s.db.DeleteSession(ctx, sessionToken)
}

// LogoutAll invalidates all sessions for a user
func (s *AuthService) LogoutAll(ctx context.Context, userID uuid.UUID) error {
	// HXC-043: no session store → nothing to invalidate. Fail cleanly.
	if !s.dbAvailable() {
		return ErrAuthBackendUnavailable
	}
	return s.db.DeleteUserSessions(ctx, userID)
}

// UpdateUser updates user profile information
func (s *AuthService) UpdateUser(ctx context.Context, userID uuid.UUID, displayName, email string) (*User, error) {
	// HXC-043: no user store → nothing to update. Fail cleanly.
	if !s.dbAvailable() {
		return nil, ErrAuthBackendUnavailable
	}

	// Validate email if provided
	if email != "" && (len(email) < 5 || len(email) > 255 || !strings.Contains(email, "@")) {
		return nil, errors.New(tr(ctx, "internal_auth_invalid_email", nil))
	}

	return s.db.UpdateUser(ctx, userID, displayName, email)
}

// DeleteUser soft-deletes a user account
func (s *AuthService) DeleteUser(ctx context.Context, userID uuid.UUID) error {
	// HXC-043: no user store → nothing to delete. Fail cleanly.
	if !s.dbAvailable() {
		return ErrAuthBackendUnavailable
	}
	return s.db.DeleteUser(ctx, userID)
}

// GenerateJWT generates a JWT token for a user
func (s *AuthService) GenerateJWT(user *User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":  user.ID.String(),
		"username": user.Username,
		"email":    user.Email,
		"exp":      time.Now().Add(s.config.TokenExpiry).Unix(),
		"iat":      time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// VerifyJWT verifies a JWT token and returns a minimal user from claims.
// Use VerifyJWTWithDB for a complete user object from database.
func (s *AuthService) VerifyJWT(tokenString string) (*User, error) {
	ctx := context.Background()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New(tr(ctx, "internal_auth_unexpected_signing_method", map[string]any{"Alg": token.Header["alg"]}))
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			return nil, ErrTokenInvalid
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil, ErrTokenInvalid
		}

		// Extract the remaining claims with checked type assertions. A
		// validly-signed token may still carry a missing / non-string
		// username or email claim (forged or corrupted); an unchecked
		// assertion here would PANIC on attacker-controlled input and crash
		// the process. Reject such tokens cleanly instead.
		username, ok := claims["username"].(string)
		if !ok {
			return nil, ErrTokenInvalid
		}
		email, ok := claims["email"].(string)
		if !ok {
			return nil, ErrTokenInvalid
		}

		// Return minimal user object from JWT claims
		// For complete user data, use VerifyJWTWithDB
		return &User{
			ID:       userID,
			Username: username,
			Email:    email,
		}, nil
	}

	return nil, ErrTokenInvalid
}

// VerifyJWTWithDB verifies a JWT token and fetches the complete user from database.
// This is slower than VerifyJWT but returns complete user data including
// IsActive, IsVerified, MFAEnabled, DisplayName, LastLogin, and timestamps.
func (s *AuthService) VerifyJWTWithDB(ctx context.Context, tokenString string) (*User, error) {
	// HXC-043: no user store → cannot fetch the full user record. Fail
	// cleanly instead of panicking on nil s.db after a successful parse.
	if !s.dbAvailable() {
		return nil, ErrAuthBackendUnavailable
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New(tr(ctx, "internal_auth_unexpected_signing_method", map[string]any{"Alg": token.Header["alg"]}))
		}
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userIDStr, ok := claims["user_id"].(string)
		if !ok {
			return nil, ErrTokenInvalid
		}

		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil, ErrTokenInvalid
		}

		// Fetch complete user from database
		user, err := s.db.GetUserByID(ctx, userID)
		if err != nil {
			return nil, ErrUserNotFound
		}

		// Verify user is still active
		if !user.IsActive {
			return nil, errors.New(tr(ctx, "internal_auth_account_deactivated", nil))
		}

		return user, nil
	}

	return nil, ErrTokenInvalid
}

// Helper methods

func (s *AuthService) validateRegistration(username, email, password string) error {
	ctx := context.Background()
	if len(username) < 3 || len(username) > 255 {
		return errors.New(tr(ctx, "internal_auth_username_length", nil))
	}

	if len(email) < 5 || len(email) > 255 || !strings.Contains(email, "@") {
		return errors.New(tr(ctx, "internal_auth_invalid_email", nil))
	}

	if len(password) < 8 {
		return errors.New(tr(ctx, "internal_auth_password_too_short", nil))
	}

	return nil
}

func (s *AuthService) hashPassword(password string) (string, error) {
	// Use bcrypt for password hashing
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.config.BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *AuthService) verifyPassword(password, hash string) bool {
	// Try bcrypt first
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return true
	}

	// Fallback to argon2 if bcrypt fails
	return s.verifyArgon2Password(password, hash)
}

func (s *AuthService) verifyArgon2Password(password, hash string) bool {
	// Parse Argon2 hash format: $argon2id$v=19$m=65536,t=1,p=4$salt$hash
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		return false
	}

	// Validate algorithm type (support argon2id, argon2i, argon2d)
	algorithm := parts[1]
	if algorithm != "argon2id" && algorithm != "argon2i" && algorithm != "argon2d" {
		return false
	}

	// Parse version (v=19 for Argon2 v1.3)
	if !strings.HasPrefix(parts[2], "v=") {
		return false
	}

	// Parse parameters: m=memory,t=time,p=parallelism
	params := strings.Split(parts[3], ",")
	if len(params) != 3 {
		return false
	}

	var memory, time uint32
	var parallelism uint8
	for _, param := range params {
		kv := strings.SplitN(param, "=", 2)
		if len(kv) != 2 {
			return false
		}
		var val uint64
		_, err := fmt.Sscanf(kv[1], "%d", &val)
		if err != nil {
			return false
		}
		switch kv[0] {
		case "m":
			memory = uint32(val)
		case "t":
			time = uint32(val)
		case "p":
			parallelism = uint8(val)
		}
	}

	// Decode salt and hash
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}

	// Compute hash with the same parameters
	var computedHash []byte
	keyLen := uint32(len(expectedHash))
	switch algorithm {
	case "argon2id":
		computedHash = argon2.IDKey([]byte(password), salt, time, memory, parallelism, keyLen)
	case "argon2i":
		computedHash = argon2.Key([]byte(password), salt, time, memory, parallelism, keyLen)
	default:
		return false
	}

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(computedHash, expectedHash) == 1
}

func (s *AuthService) generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
