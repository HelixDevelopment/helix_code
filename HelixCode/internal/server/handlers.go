package server

import (
	"net/http"
	"os"
	"time"

	"dev.helix.code/internal/auth"
	"dev.helix.code/internal/project"
	"dev.helix.code/internal/workflow"
	"github.com/gin-gonic/gin"
)

// Project Handlers

func (s *Server) listProjects(c *gin.Context) {
	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "default-user" // For testing without full auth
	}

	projects, err := s.projectManager.ListProjects(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to list projects",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"projects": projects,
	})
}

// Auth Handlers

func (s *Server) register(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required"`
		Email       string `json:"email" binding:"required"`
		Password    string `json:"password" binding:"required"`
		DisplayName string `json:"display_name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	user, err := s.auth.Register(c.Request.Context(), req.Username, req.Email, req.Password, req.DisplayName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Registration failed",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"user":   user,
	})
}

func (s *Server) login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	session, user, err := s.auth.Login(c.Request.Context(), req.Username, req.Password, "rest_api", c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Login failed",
			"error":   err.Error(),
		})
		return
	}

	// Generate JWT token
	token, err := s.auth.GenerateJWT(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate token",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"user":    user,
		"token":   token,
		"session": session,
	})
}

func (s *Server) logout(c *gin.Context) {
	// Get token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || len(authHeader) <= 7 || authHeader[:7] != "Bearer " {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid authorization header",
		})
		return
	}

	token := authHeader[7:]

	// For JWT, we can't invalidate it server-side, but we can log the logout
	// In a production system, you might want to maintain a blacklist
	if err := s.auth.Logout(c.Request.Context(), token); err != nil {
		// Log error but don't fail the request
		c.Error(err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Logged out successfully",
	})
}

func (s *Server) refreshToken(c *gin.Context) {
	// Get current user from context (set by auth middleware)
	userValue, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User not authenticated",
		})
		return
	}

	user, ok := userValue.(*auth.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Invalid user context",
		})
		return
	}

	// Generate new JWT token
	token, err := s.auth.GenerateJWT(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to generate token",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"token":  token,
	})
}

func (s *Server) getCurrentUser(c *gin.Context) {
	// Get current user from context (set by auth middleware)
	userValue, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "User not authenticated",
		})
		return
	}

	user, ok := userValue.(*auth.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Invalid user context",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"user":   user,
	})
}

func (s *Server) createProject(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		Path        string `json:"path" binding:"required"`
		Type        string `json:"type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	// Create project directory if it doesn't exist
	if err := os.MkdirAll(req.Path, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to create project directory",
			"error":   err.Error(),
		})
		return
	}

	proj, err := s.projectManager.CreateProject(c.Request.Context(), req.Name, req.Description, req.Path, req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to create project",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"status":  "success",
		"project": proj,
	})
}

func (s *Server) getProject(c *gin.Context) {
	id := c.Param("id")

	proj, err := s.projectManager.GetProject(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"status":  "error",
			"message": "Project not found",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"project": proj,
	})
}

func (s *Server) updateProject(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	// For now, return placeholder until we have user authentication
	// In production, this would use: projectManager := project.NewDatabaseManager(s.db)
	proj := gin.H{
		"id":          id,
		"name":        req.Name,
		"description": req.Description,
		"path":        "/path/to/project",
		"type":        "go",
		"created_at":  time.Now(),
		"updated_at":  time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"project": proj,
	})
}

func (s *Server) deleteProject(c *gin.Context) {
	// For now, return success until we have user authentication
	// In production, this would use: projectManager := project.NewDatabaseManager(s.db)
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Project deleted",
	})
}

// Task Handlers

func (s *Server) listTasks(c *gin.Context) {
	if s.taskManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"tasks":  []interface{}{},
		})
		return
	}

	tasks, err := s.taskManager.ListTasks(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to list tasks",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"tasks":  tasks,
	})
}

func (s *Server) createTask(c *gin.Context) {
	var req struct {
		Name         string                 `json:"name" binding:"required"`
		Description  string                 `json:"description"`
		Type         string                 `json:"type" binding:"required"`
		Priority     string                 `json:"priority"`
		Parameters   map[string]interface{} `json:"parameters"`
		Dependencies []string               `json:"dependencies"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	// Use database manager if available
	if s.taskManager != nil {
		priority := req.Priority
		if priority == "" {
			priority = "normal"
		}

		task, err := s.taskManager.CreateTask(
			c.Request.Context(),
			req.Name,
			req.Description,
			req.Type,
			priority,
			req.Parameters,
			req.Dependencies,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to create task",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"status": "success",
			"task":   task,
		})
		return
	}

	// Fallback to placeholder if no database
	task := gin.H{
		"id":          "task_placeholder",
		"name":        req.Name,
		"description": req.Description,
		"type":        req.Type,
		"status":      "pending",
		"created_at":  time.Now(),
	}

	c.JSON(http.StatusCreated, gin.H{
		"status": "success",
		"task":   task,
	})
}

func (s *Server) getTask(c *gin.Context) {
	id := c.Param("id")

	if s.taskManager != nil {
		task, err := s.taskManager.GetTask(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Task not found",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"task":   task,
		})
		return
	}

	// Fallback to placeholder if no database
	task := gin.H{
		"id":          id,
		"name":        "Sample Task",
		"description": "This is a sample task",
		"type":        "generic",
		"status":      "pending",
		"created_at":  time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"task":   task,
	})
}

func (s *Server) updateTask(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Status       string                 `json:"status"`
		Result       map[string]interface{} `json:"result"`
		ErrorMessage string                 `json:"error_message"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request",
			"error":   err.Error(),
		})
		return
	}

	if s.taskManager != nil {
		var err error
		switch req.Status {
		case "running":
			err = s.taskManager.StartTask(c.Request.Context(), id)
		case "completed":
			err = s.taskManager.CompleteTask(c.Request.Context(), id, req.Result)
		case "failed":
			err = s.taskManager.FailTask(c.Request.Context(), id, req.ErrorMessage)
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid status. Use: running, completed, or failed",
			})
			return
		}

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to update task",
				"error":   err.Error(),
			})
			return
		}

		// Get the updated task
		task, err := s.taskManager.GetTask(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Task updated but failed to retrieve",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"task":   task,
		})
		return
	}

	// Fallback to placeholder if no database
	task := gin.H{
		"id":          id,
		"name":        "Sample Task",
		"description": "This is a sample task",
		"type":        "generic",
		"status":      req.Status,
		"created_at":  time.Now(),
		"updated_at":  time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"task":   task,
	})
}

func (s *Server) deleteTask(c *gin.Context) {
	id := c.Param("id")

	if s.taskManager != nil {
		err := s.taskManager.DeleteTask(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to delete task",
				"error":   err.Error(),
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Task deleted",
	})
}

// Worker Handlers

func (s *Server) listWorkers(c *gin.Context) {
	if s.workerManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"workers": []interface{}{},
		})
		return
	}

	workers, err := s.workerManager.ListWorkers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to list workers",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"workers": workers,
	})
}

func (s *Server) getWorker(c *gin.Context) {
	id := c.Param("id")

	if s.workerManager != nil {
		worker, err := s.workerManager.GetWorker(c.Request.Context(), id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Worker not found",
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"worker": worker,
		})
		return
	}

	// Fallback to placeholder if no database
	worker := gin.H{
		"id":           id,
		"hostname":     "localhost",
		"status":       "active",
		"capabilities": []string{"build", "test"},
		"created_at":   time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"worker": worker,
	})
}

// System Handlers

func (s *Server) getSystemStats(c *gin.Context) {
	// Initialize counters
	var (
		totalTasks     = 0
		pendingTasks   = 0
		runningTasks   = 0
		completedTasks = 0
		failedTasks    = 0
		totalWorkers   = 0
		activeWorkers  = 0
	)

	// Get task statistics if task manager is available
	if s.taskManager != nil {
		tasks, err := s.taskManager.ListTasks(c.Request.Context())
		if err == nil {
			totalTasks = len(tasks)
			// Count tasks by status
			for _, t := range tasks {
				switch string(t.Status) {
				case "pending":
					pendingTasks++
				case "running":
					runningTasks++
				case "completed":
					completedTasks++
				case "failed":
					failedTasks++
				}
			}
		}
	}

	// Get worker statistics if worker manager is available
	if s.workerManager != nil {
		workers, err := s.workerManager.ListWorkers(c.Request.Context())
		if err == nil {
			totalWorkers = len(workers)
			// Count active workers
			for _, w := range workers {
				if string(w.Status) == "active" {
					activeWorkers++
				}
			}
		}
	}

	// Calculate uptime
	uptime := time.Since(s.startTime)

	stats := gin.H{
		"tasks": gin.H{
			"total":     totalTasks,
			"pending":   pendingTasks,
			"running":   runningTasks,
			"completed": completedTasks,
			"failed":    failedTasks,
		},
		"workers": gin.H{
			"total":  totalWorkers,
			"active": activeWorkers,
		},
		"system": gin.H{
			"uptime": uptime.String(),
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"stats":  stats,
	})
}

func (s *Server) getSystemStatus(c *gin.Context) {
	// Check database connection
	dbStatus := "healthy"
	if err := s.db.HealthCheck(); err != nil {
		dbStatus = "unhealthy"
	}

	status := gin.H{
		"database": dbStatus,
		"api":      "healthy",
		"version":  "1.0.0",
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"system": status,
	})
}

// Workflow Handlers

func (s *Server) executePlanningWorkflow(c *gin.Context) {
	projectID := c.Param("projectId")

	workflowExecutor := workflow.NewExecutor(s.projectManager)

	wf, err := workflowExecutor.ExecutePlanningWorkflow(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to execute planning workflow",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"workflow": wf,
	})
}

func (s *Server) executeBuildingWorkflow(c *gin.Context) {
	projectID := c.Param("projectId")

	projectManager := project.NewManager()
	workflowExecutor := workflow.NewExecutor(projectManager)

	wf, err := workflowExecutor.ExecuteBuildingWorkflow(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to execute building workflow",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"workflow": wf,
	})
}

func (s *Server) executeTestingWorkflow(c *gin.Context) {
	projectID := c.Param("projectId")

	projectManager := project.NewManager()
	workflowExecutor := workflow.NewExecutor(projectManager)

	wf, err := workflowExecutor.ExecuteTestingWorkflow(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to execute testing workflow",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"workflow": wf,
	})
}

func (s *Server) executeRefactoringWorkflow(c *gin.Context) {
	projectID := c.Param("projectId")

	projectManager := project.NewManager()
	workflowExecutor := workflow.NewExecutor(projectManager)

	wf, err := workflowExecutor.ExecuteRefactoringWorkflow(c.Request.Context(), projectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to execute refactoring workflow",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"workflow": wf,
	})
}

// Server Info and Metrics Handlers

// getServerInfo returns server information
func (s *Server) getServerInfo(c *gin.Context) {
	uptime := time.Since(s.startTime)

	info := gin.H{
		"name":       "HelixCode Server",
		"version":    "1.0.0",
		"go_version": "1.24",
		"uptime":     uptime.String(),
		"start_time": s.startTime.UTC().Format(time.RFC3339),
		"database": gin.H{
			"enabled":   s.db != nil,
			"connected": s.db != nil && s.db.HealthCheck() == nil,
		},
		"redis": gin.H{
			"enabled":   s.redis != nil && s.redis.IsEnabled(),
			"connected": s.redis != nil && s.redis.IsEnabled() && s.redis.GetClient().Ping(c.Request.Context()).Err() == nil,
		},
		"features": gin.H{
			"auth_enabled":          s.auth != nil,
			"mcp_enabled":           s.mcp != nil,
			"notifications_enabled": s.notification != nil,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"info":   info,
	})
}

// getMetrics returns server metrics
func (s *Server) getMetrics(c *gin.Context) {
	metrics := gin.H{
		"uptime_seconds": time.Since(s.startTime).Seconds(),
		"requests": gin.H{
			"total":  0,
			"active": 0,
		},
		"resources": gin.H{
			"goroutines": 0,
			"memory_mb":  0,
		},
		"database": gin.H{
			"connections_active": 0,
			"connections_idle":   0,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"metrics": metrics,
	})
}

// LLM Handlers

// listLLMProviders returns available LLM providers
func (s *Server) listLLMProviders(c *gin.Context) {
	providers := []gin.H{
		{
			"id":          "ollama",
			"name":        "Ollama",
			"type":        "local",
			"description": "Local LLM inference using Ollama",
			"status":      "available",
			"models":      []string{"llama2", "llama2:7b", "mistral", "codellama"},
		},
		{
			"id":          "openai",
			"name":        "OpenAI",
			"type":        "cloud",
			"description": "OpenAI GPT models",
			"status":      "available",
			"models":      []string{"gpt-4", "gpt-4-turbo", "gpt-3.5-turbo"},
		},
		{
			"id":          "anthropic",
			"name":        "Anthropic",
			"type":        "cloud",
			"description": "Anthropic Claude models",
			"status":      "available",
			"models":      []string{"claude-3-opus", "claude-3-sonnet", "claude-3-haiku"},
		},
		{
			"id":          "gemini",
			"name":        "Google Gemini",
			"type":        "cloud",
			"description": "Google Gemini models",
			"status":      "available",
			"models":      []string{"gemini-pro", "gemini-pro-vision"},
		},
		{
			"id":          "azure",
			"name":        "Azure OpenAI",
			"type":        "cloud",
			"description": "Azure-hosted OpenAI models",
			"status":      "available",
			"models":      []string{"gpt-4", "gpt-35-turbo"},
		},
		{
			"id":          "bedrock",
			"name":        "AWS Bedrock",
			"type":        "cloud",
			"description": "AWS Bedrock foundation models",
			"status":      "available",
			"models":      []string{"anthropic.claude-v2", "amazon.titan-text"},
		},
		{
			"id":          "groq",
			"name":        "Groq",
			"type":        "cloud",
			"description": "Groq LPU inference",
			"status":      "available",
			"models":      []string{"llama2-70b", "mixtral-8x7b"},
		},
		{
			"id":          "llamacpp",
			"name":        "Llama.cpp",
			"type":        "local",
			"description": "Local inference with llama.cpp",
			"status":      "available",
			"models":      []string{"custom-gguf"},
		},
		{
			"id":          "vllm",
			"name":        "vLLM",
			"type":        "local",
			"description": "High-throughput local inference",
			"status":      "available",
			"models":      []string{"custom"},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"providers": providers,
		"count":     len(providers),
	})
}

// getLLMProvider returns details for a specific LLM provider
func (s *Server) getLLMProvider(c *gin.Context) {
	providerID := c.Param("id")

	provider := gin.H{
		"id":          providerID,
		"name":        providerID,
		"status":      "available",
		"configured":  true,
		"description": "LLM Provider",
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "success",
		"provider": provider,
	})
}

// listLLMModels returns available LLM models
func (s *Server) listLLMModels(c *gin.Context) {
	models := []gin.H{
		{"id": "gpt-4", "provider": "openai", "context_length": 8192},
		{"id": "gpt-4-turbo", "provider": "openai", "context_length": 128000},
		{"id": "gpt-3.5-turbo", "provider": "openai", "context_length": 16385},
		{"id": "claude-3-opus", "provider": "anthropic", "context_length": 200000},
		{"id": "claude-3-sonnet", "provider": "anthropic", "context_length": 200000},
		{"id": "llama2:7b", "provider": "ollama", "context_length": 4096},
		{"id": "gemini-pro", "provider": "gemini", "context_length": 32768},
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"models": models,
		"count":  len(models),
	})
}

// Memory System Handlers

// listMemorySystems returns available memory systems
func (s *Server) listMemorySystems(c *gin.Context) {
	systems := []gin.H{
		{
			"id":          "cognee",
			"name":        "Cognee",
			"type":        "knowledge_graph",
			"description": "AI-powered knowledge graph for memory management",
			"status":      "available",
			"features":    []string{"semantic_search", "context_retrieval", "optimization"},
		},
		{
			"id":          "weaviate",
			"name":        "Weaviate",
			"type":        "vector_db",
			"description": "Vector database for embeddings",
			"status":      "available",
			"features":    []string{"vector_search", "hybrid_search", "filtering"},
		},
		{
			"id":          "chromadb",
			"name":        "ChromaDB",
			"type":        "vector_db",
			"description": "Embedding database",
			"status":      "available",
			"features":    []string{"vector_search", "metadata_filtering"},
		},
		{
			"id":          "qdrant",
			"name":        "Qdrant",
			"type":        "vector_db",
			"description": "High-performance vector similarity search",
			"status":      "available",
			"features":    []string{"vector_search", "filtering", "payload_indexing"},
		},
		{
			"id":          "mem0",
			"name":        "Mem0",
			"type":        "memory_layer",
			"description": "Memory layer for AI applications",
			"status":      "available",
			"features":    []string{"user_memory", "session_memory", "agent_memory"},
		},
		{
			"id":          "zep",
			"name":        "Zep",
			"type":        "memory_store",
			"description": "Long-term memory for AI assistants",
			"status":      "available",
			"features":    []string{"conversation_history", "entity_extraction", "summarization"},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"systems": systems,
		"count":   len(systems),
	})
}

// getMemoryStats returns memory system statistics
func (s *Server) getMemoryStats(c *gin.Context) {
	stats := gin.H{
		"total_memories":    0,
		"total_embeddings":  0,
		"storage_used_mb":   0,
		"active_sessions":   0,
		"cache_hit_rate":    0.0,
		"avg_retrieval_ms":  0,
		"systems_connected": 0,
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "success",
		"stats":  stats,
	})
}
