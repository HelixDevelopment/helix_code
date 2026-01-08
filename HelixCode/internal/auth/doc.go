// Package auth provides authentication and authorization services for HelixCode.
//
// # Overview
//
// The auth package implements secure user authentication using JWT tokens and
// session management with support for multiple authentication methods. It provides
// password hashing using bcrypt and Argon2, session-based authentication, and
// JWT token generation and verification.
//
// # Architecture
//
// The package is organized around several core components:
//
//   - AuthService: Main authentication service handling login, registration, and verification
//   - AuthRepository: Interface for authentication data storage
//   - AuthDB: PostgreSQL implementation of AuthRepository
//   - User: User entity with account information
//   - Session: Session entity for session-based authentication
//
// # Password Security
//
// The package supports two password hashing algorithms:
//
//   - bcrypt: Primary hashing algorithm with configurable cost factor
//   - Argon2: Fallback algorithm supporting argon2id, argon2i, and argon2d variants
//
// Both algorithms use constant-time comparison to prevent timing attacks.
//
// # Basic Usage
//
// Creating an authentication service:
//
//	// Create configuration
//	config := auth.DefaultConfig()
//	config.JWTSecret = "your-secret-key"
//
//	// Create database repository
//	authDB := auth.NewAuthDB(db)
//
//	// Create service
//	authService := auth.NewAuthService(config, authDB)
//
// # User Registration
//
// Registering a new user:
//
//	user, err := authService.Register(ctx, "username", "email@example.com", "password", "Display Name")
//	if err != nil {
//	    if errors.Is(err, auth.ErrUserExists) {
//	        // Handle existing user
//	    }
//	    return err
//	}
//
// # User Login
//
// Authenticating a user and creating a session:
//
//	session, user, err := authService.Login(ctx, "username", "password", "cli", "192.168.1.1", "HelixCode CLI")
//	if err != nil {
//	    if errors.Is(err, auth.ErrInvalidCredentials) {
//	        // Handle invalid credentials
//	    }
//	    return err
//	}
//	// session.SessionToken can be used for subsequent requests
//
// # Session Verification
//
// Verifying a session token:
//
//	user, err := authService.VerifySession(ctx, sessionToken)
//	if err != nil {
//	    if errors.Is(err, auth.ErrTokenExpired) {
//	        // Handle expired session
//	    }
//	    return err
//	}
//
// # JWT Tokens
//
// Generating and verifying JWT tokens:
//
//	// Generate JWT for API access
//	token, err := authService.GenerateJWT(user)
//
//	// Verify JWT (minimal user from claims)
//	user, err := authService.VerifyJWT(token)
//
//	// Verify JWT with full database lookup
//	user, err := authService.VerifyJWTWithDB(ctx, token)
//
// # Logout
//
// Invalidating sessions:
//
//	// Logout single session
//	err := authService.Logout(ctx, sessionToken)
//
//	// Logout all sessions for a user
//	err := authService.LogoutAll(ctx, userID)
//
// # Configuration
//
// AuthConfig provides configuration options:
//
//	type AuthConfig struct {
//	    JWTSecret       string        // Secret key for JWT signing
//	    TokenExpiry     time.Duration // JWT token expiry duration
//	    SessionExpiry   time.Duration // Session expiry duration
//	    BcryptCost      int           // bcrypt cost factor (default: 12)
//	    Argon2Time      uint32        // Argon2 time parameter
//	    Argon2Memory    uint32        // Argon2 memory parameter (KB)
//	    Argon2Threads   uint8         // Argon2 parallelism
//	    Argon2KeyLength uint32        // Argon2 output key length
//	}
//
// # Error Handling
//
// The package defines standard errors for common failure cases:
//
//	var (
//	    ErrInvalidCredentials = errors.New("invalid credentials")
//	    ErrTokenExpired       = errors.New("token expired")
//	    ErrTokenInvalid       = errors.New("invalid token")
//	    ErrUserNotFound       = errors.New("user not found")
//	    ErrUserExists         = errors.New("user already exists")
//	)
//
// # User Model
//
// The User struct contains account information:
//
//	type User struct {
//	    ID          uuid.UUID // Unique identifier
//	    Username    string    // Login username
//	    Email       string    // Email address
//	    DisplayName string    // Display name
//	    IsActive    bool      // Account active status
//	    IsVerified  bool      // Email verification status
//	    MFAEnabled  bool      // Multi-factor authentication status
//	    LastLogin   time.Time // Last login timestamp
//	    CreatedAt   time.Time // Account creation time
//	    UpdatedAt   time.Time // Last update time
//	}
//
// # Session Model
//
// The Session struct tracks user sessions:
//
//	type Session struct {
//	    ID           uuid.UUID // Session identifier
//	    UserID       uuid.UUID // Associated user
//	    SessionToken string    // Token for authentication
//	    ClientType   string    // Client type (cli, rest_api, etc.)
//	    IPAddress    net.IP    // Client IP address
//	    UserAgent    string    // Client user agent
//	    ExpiresAt    time.Time // Session expiry time
//	    CreatedAt    time.Time // Session creation time
//	}
//
// # Thread Safety
//
// The AuthService is safe for concurrent use from multiple goroutines.
//
// # Database Requirements
//
// The package requires the following database tables:
//
//   - users: User accounts with password hashes
//   - user_sessions: Active user sessions
//
// See the database package for schema creation.
package auth
