package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"dev.helix.code/internal/auth"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/helixqa"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/mcp"
	"dev.helix.code/internal/notification"
	"dev.helix.code/internal/project"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/task"
	"dev.helix.code/internal/verifier"
	"dev.helix.code/internal/worker"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP server
type Server struct {
	config         *config.Config
	db             *database.Database
	redis          *redis.Client
	auth           *auth.AuthService
	llm            *llm.Provider
	mcp            *mcp.MCPServer
	notification   *notification.NotificationEngine
	taskManager    *task.DatabaseManager
	workerManager  *worker.DatabaseManager
	projectManager *project.DatabaseManager
	sessionManager *session.Manager
	server         *http.Server
	router         *gin.Engine
	startTime      time.Time
	verifierResult *verifier.BootstrapResult
	qaEngine       *helixqa.Engine
}

// New creates a new HTTP server
func New(cfg *config.Config, db *database.Database, rds *redis.Client) *Server {
	// Set Gin mode
	if cfg.Logging.Level == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Global middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(CORSMiddleware())
	router.Use(SecurityMiddleware())

	// Initialize auth service
	var authService *auth.AuthService
	if db != nil {
		authConfig := auth.AuthConfig{
			JWTSecret:     cfg.Auth.JWTSecret,
			TokenExpiry:   time.Duration(cfg.Auth.TokenExpiry) * time.Second,
			SessionExpiry: time.Duration(cfg.Auth.SessionExpiry) * time.Second,
			BcryptCost:    cfg.Auth.BcryptCost,
		}
		authDB := auth.NewAuthDB(db.Pool)
		authService = auth.NewAuthService(authConfig, authDB)
	}

	// Initialize MCP server
	mcpServer := mcp.NewMCPServer()

	// Initialize notification engine
	notificationEngine := notification.NewNotificationEngine()

	// Server.llm is intentionally left nil here. All LLM concerns
	// surfaced over HTTP (/api/v1/llm/providers, etc.) are answered by
	// the LLMsVerifier subsystem via verifierResult, NOT by a single
	// provider stored on the Server struct (CONST-036: LLMsVerifier is
	// the sole authoritative source). The llm field is retained on the
	// struct as a reserved seam for a future per-server pinned-provider
	// feature; until that lands handlers.go MUST NOT dereference s.llm
	// (round-33 §11.4 honest-init anchor — previous "skip LLM
	// initialization as it requires more complex setup" comment was a
	// PASS-bluff implying a missing wire-up; in fact the verifier
	// already supplies the data and the field is correctly nil;
	// CONST-035 / Article XI §11.9).

	// Initialize task and worker managers if database is available
	var taskMgr *task.DatabaseManager
	var workerMgr *worker.DatabaseManager
	var projectMgr *project.DatabaseManager
	if db != nil {
		taskMgr = task.NewDatabaseManager(db)
		workerMgr = worker.NewDatabaseManager(db)
		projectMgr = project.NewDatabaseManager(db)
	}

	// Initialize session manager
	sessionMgr := session.NewManager()

	// Initialize LLMsVerifier subsystem if enabled
	var verifierResult *verifier.BootstrapResult
	if cfg.Verifier != nil && cfg.Verifier.Enabled {
		var err error
		verifierResult, err = verifier.Bootstrap(cfg.Verifier)
		if err != nil {
			log.Printf("⚠️  Failed to bootstrap verifier: %v (continuing without)", err)
		}
	}

	// Initialize helix_qa engine if enabled
	var qaEngine *helixqa.Engine
	if cfg.QA.Enabled {
		var err error
		qaEngine, err = helixqa.NewEngine(cfg)
		if err != nil {
			log.Printf("⚠️  Failed to initialize helix_qa engine: %v (continuing without)", err)
		} else {
			log.Printf("✅ helix_qa engine initialized (output: %s)", cfg.QA.OutputDir)
		}
	}

	server := &Server{
		config:         cfg,
		db:             db,
		redis:          rds,
		auth:           authService,
		mcp:            mcpServer,
		notification:   notificationEngine,
		taskManager:    taskMgr,
		workerManager:  workerMgr,
		projectManager: projectMgr,
		sessionManager: sessionMgr,
		router:         router,
		startTime:      time.Now(),
		verifierResult: verifierResult,
		qaEngine:       qaEngine,
	}

	// Setup routes
	server.setupRoutes()

	// Create HTTP server
	server.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Address, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	return server
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("🚀 Starting HelixCode server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.verifierResult != nil {
		s.verifierResult.Shutdown()
	}
	return s.server.Shutdown(ctx)
}

// setupRoutes sets up all HTTP routes
func (s *Server) setupRoutes() {
	// Health check
	s.router.GET("/health", s.healthCheck)

	// API routes
	api := s.router.Group("/api/v1")
	{
		// API health alias for /health — matches README §API surface and
		// the anti-bluff challenge harness (ddos_health_flood etc.) which
		// probe /api/v1/health as the API-namespaced endpoint per
		// CONST-035 (documented surface must actually exist).
		api.GET("/health", s.healthCheck)

		// Authentication routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", s.register)
			auth.POST("/login", s.login)
			auth.POST("/logout", s.logout)
			auth.POST("/refresh", s.refreshToken)
		}

		// User routes
		users := api.Group("/users")
		users.Use(s.authMiddleware())
		{
			users.GET("/me", s.getCurrentUser)
			users.PUT("/me", s.updateCurrentUser)
			users.DELETE("/me", s.deleteCurrentUser)
		}

		// Worker routes
		workers := api.Group("/workers")
		workers.Use(s.authMiddleware())
		{
			workers.GET("", s.listWorkers)
			workers.POST("", s.createWorker)
			workers.GET("/:id", s.getWorker)
			workers.PUT("/:id", s.updateWorker)
			workers.DELETE("/:id", s.deleteWorker)
			workers.POST("/:id/heartbeat", s.workerHeartbeat)
			workers.GET("/:id/metrics", s.getWorkerMetrics)
		}

		// Task routes
		tasks := api.Group("/tasks")
		tasks.Use(s.authMiddleware())
		{
			tasks.GET("", s.listTasks)
			tasks.POST("", s.createTask)
			tasks.GET("/:id", s.getTask)
			tasks.PUT("/:id", s.updateTask)
			tasks.DELETE("/:id", s.deleteTask)
			tasks.POST("/:id/assign", s.assignTask)
			tasks.POST("/:id/start", s.startTask)
			tasks.POST("/:id/complete", s.completeTask)
			tasks.POST("/:id/fail", s.failTask)
			tasks.POST("/:id/checkpoint", s.createTaskCheckpoint)
			tasks.GET("/:id/checkpoints", s.getTaskCheckpoints)
			tasks.POST("/:id/retry", s.retryTask)
		}

		// Project routes — fully authenticated.
		//
		// Previously POST / + the 4 workflow endpoints were registered
		// under a `publicProjects` group with the comment
		// "no auth for testing" — i.e., anyone could create projects
		// or trigger workflow executions against any projectId without
		// any credential. That was a real production security hole AND
		// inconsistent with createProject's own requirements (the
		// handler now pulls `*auth.User` from context to determine
		// project owner, which only works through authMiddleware).
		// Consolidated into a single authenticated group.
		projects := api.Group("/projects")
		projects.Use(s.authMiddleware())
		{
			projects.GET("", s.listProjects)
			projects.POST("", s.createProject)
			projects.GET("/:id", s.getProject)
			projects.PUT("/:id", s.updateProject)
			projects.DELETE("/:id", s.deleteProject)
			projects.GET("/:id/sessions", s.getProjectSessions)
			projects.POST("/:projectId/workflows/planning", s.executePlanningWorkflow)
			projects.POST("/:projectId/workflows/building", s.executeBuildingWorkflow)
			projects.POST("/:projectId/workflows/testing", s.executeTestingWorkflow)
			projects.POST("/:projectId/workflows/refactoring", s.executeRefactoringWorkflow)
		}

		// Session routes
		sessions := api.Group("/sessions")
		sessions.Use(s.authMiddleware())
		{
			sessions.GET("", s.listSessions)
			sessions.POST("", s.createSession)
			sessions.GET("/:id", s.getSession)
			sessions.PUT("/:id", s.updateSession)
			sessions.DELETE("/:id", s.deleteSession)
		}

		// System routes
		system := api.Group("/system")
		system.Use(s.authMiddleware())
		{
			system.GET("/stats", s.getSystemStats)
			system.GET("/status", s.getSystemStatus)
		}

		// Server info routes (public)
		api.GET("/server/info", s.getServerInfo)
		api.GET("/metrics", s.getMetrics)

		// LLM routes
		llmRoutes := api.Group("/llm")
		{
			llmRoutes.GET("/providers", s.listLLMProviders)
			llmRoutes.GET("/providers/:id", s.getLLMProvider)
			llmRoutes.GET("/models", s.listLLMModels)
		}

		// Memory routes
		memory := api.Group("/memory")
		{
			memory.GET("/systems", s.listMemorySystems)
			memory.GET("/stats", s.getMemoryStats)
		}

		// QA routes
		qa := api.Group("/qa")
		qa.Use(s.authMiddleware())
		{
			qa.POST("/session", s.startQASession)
			qa.GET("/sessions", s.listQASessions)
			qa.GET("/session/:id/status", s.getQASessionStatus)
			qa.GET("/session/:id/report", s.getQASessionReport)
			qa.GET("/session/:id/screenshot/:name", s.getQASessionScreenshot)
			qa.DELETE("/session/:id", s.cancelQASession)
		}

		// Screenshot routes (standalone)
		screenshot := api.Group("/screenshot")
		screenshot.Use(s.authMiddleware())
		{
			screenshot.GET("/engines", s.listScreenshotEngines)
			screenshot.GET("/:platform", s.captureScreenshot)
		}
	}

	// WebSocket routes
	s.router.GET("/ws", s.handleWebSocket)

	// Static file serving for web interface
	s.router.Static("/static", "./web/frontend/static")
	s.router.StaticFile("/", "./web/frontend/index.html")
	s.router.StaticFile("/favicon.ico", "./assets/icons/icon-32x32.png")
}

// Handler methods

func (s *Server) healthCheck(c *gin.Context) {
	// Check database connection
	if s.db != nil {
		if err := s.db.HealthCheck(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "error",
				"message": "Database connection failed",
				"error":   err.Error(),
			})
			return
		}
	}

	// Check Redis connection if enabled
	if s.redis.IsEnabled() {
		if _, err := s.redis.Get(c.Request.Context(), "health_check"); err != nil && err.Error() != "redis: nil" {
			// Try to ping Redis
			if s.redis.GetClient().Ping(c.Request.Context()).Err() != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status":  "error",
					"message": "Redis connection failed",
					"error":   err.Error(),
				})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"version":   "1.0.0",
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"status":  "error",
		"message": "Not implemented yet",
	})
}

// Middleware

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Check for Bearer token
		const bearerPrefix = "Bearer "
		if len(authHeader) <= len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		token := authHeader[len(bearerPrefix):]

		// Verify JWT token AND fetch the complete user from the database.
		//
		// VerifyJWT (the cheap variant) returns a stub User with ONLY
		// {ID, Username, Email} populated from JWT claims — every other
		// field (IsActive, IsVerified, MFAEnabled, DisplayName, LastLogin,
		// timestamps) is zero-valued. That stub then propagates into
		// /api/v1/users/me responses with `is_active:false` and
		// `created_at:"0001-01-01T00:00:00Z"`, which is a CONST-035 bluff:
		// callers think the user was "created in year 0001" or is "not
		// active" while in reality the DB has the right state.
		//
		// VerifyJWTWithDB performs a single indexed UUID lookup AND
		// rejects deactivated accounts (defense-in-depth — a JWT issued
		// before account deactivation must not continue to authenticate).
		user, err := s.auth.VerifyJWTWithDB(c.Request.Context(), token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid or expired token",
				"error":   err.Error(),
			})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Next()
	}
}

// handleWebSocket handles WebSocket connections for MCP
func (s *Server) handleWebSocket(c *gin.Context) {
	s.mcp.HandleWebSocket(c.Writer, c.Request)
}

// CORSMiddleware provides CORS headers
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// SecurityMiddleware provides security headers
func SecurityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Set("X-Frame-Options", "DENY")
		c.Writer.Header().Set("X-XSS-Protection", "1; mode=block")
		c.Writer.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Next()
	}
}
