package config

import (
	"context"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ConfigAPI handles configuration API requests
type ConfigAPI struct {
	manager  *ConfigManager
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
	mu       sync.Mutex
}

// NewConfigAPI creates a new configuration API handler
func NewConfigAPI(manager *ConfigManager) *ConfigAPI {
	return &ConfigAPI{
		manager: manager,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" || origin == "http://localhost:8080" || origin == "http://127.0.0.1:8080" {
					return true
				}
				return strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:")
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

// SetupRoutes sets up configuration routes
func (api *ConfigAPI) SetupRoutes(router *gin.RouterGroup) {
	configGroup := router.Group("/config")
	{
		configGroup.GET("", api.GetConfig)
		configGroup.PUT("", api.UpdateConfig)
		configGroup.POST("/reload", api.ReloadConfig)
		configGroup.POST("/restore", api.RestoreConfig)
		configGroup.GET("/ws", api.HandleWebSocket)
	}
}

// GetConfig returns the current configuration
func (api *ConfigAPI) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, api.manager.GetConfig())
}

// UpdateConfig updates the configuration
func (api *ConfigAPI) UpdateConfig(c *gin.Context) {
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := api.manager.UpdateConfigFromMap(updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tr(context.Background(), "internal_config_api_update_failed", map[string]any{"Error": err.Error()})})
		return
	}

	// Notify clients
	api.broadcastConfigUpdate()

	c.JSON(http.StatusOK, api.manager.GetConfig())
}

// ReloadConfig reloads configuration from disk
func (api *ConfigAPI) ReloadConfig(c *gin.Context) {
	if err := api.manager.loadConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": tr(context.Background(), "internal_config_api_reload_failed", map[string]any{"Error": err.Error()})})
		return
	}

	// Notify clients
	api.broadcastConfigUpdate()

	c.JSON(http.StatusOK, gin.H{"status": "reloaded", "config": api.manager.GetConfig()})
}

// RestoreConfig restores configuration to defaults or backup
func (api *ConfigAPI) RestoreConfig(c *gin.Context) {
	var req struct {
		BackupPath string `json:"backup_path"`
		Defaults   bool   `json:"defaults"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		// If no body, assume defaults
		req.Defaults = true
	}

	if req.Defaults {
		if err := api.manager.ResetToDefaults(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": tr(context.Background(), "internal_config_api_reset_failed", map[string]any{"Error": err.Error()})})
			return
		}
	} else if req.BackupPath != "" {
		if err := api.manager.ImportConfig(req.BackupPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": tr(context.Background(), "internal_config_api_restore_failed", map[string]any{"Error": err.Error()})})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": tr(context.Background(), "internal_config_api_invalid_restore_request", nil)})
		return
	}

	// Notify clients
	api.broadcastConfigUpdate()

	c.JSON(http.StatusOK, gin.H{"status": "restored", "config": api.manager.GetConfig()})
}

// HandleWebSocket handles WebSocket connections for live config updates
func (api *ConfigAPI) HandleWebSocket(c *gin.Context) {
	conn, err := api.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	api.mu.Lock()
	api.clients[conn] = true
	api.mu.Unlock()

	// Send initial config
	conn.WriteJSON(gin.H{"type": "config_update", "data": api.manager.GetConfig()})

	// Keep connection alive
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			api.mu.Lock()
			delete(api.clients, conn)
			api.mu.Unlock()
			conn.Close()
			break
		}
	}
}

func (api *ConfigAPI) broadcastConfigUpdate() {
	api.mu.Lock()
	defer api.mu.Unlock()

	config := api.manager.GetConfig()
	msg := gin.H{"type": "config_update", "data": config}

	for client := range api.clients {
		if err := client.WriteJSON(msg); err != nil {
			client.Close()
			delete(api.clients, client)
		}
	}
}
