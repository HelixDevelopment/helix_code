// Package config provides comprehensive configuration management for HelixCode,
// supporting multiple configuration sources, validation, migration, and templates.
//
// # Overview
//
// The config package handles all aspects of application configuration, from loading
// YAML/JSON configuration files to validating settings, migrating between versions,
// and providing runtime configuration updates via a REST API.
//
// # Architecture
//
// The package is organized around several core components:
//
//   - Config: Main configuration struct containing all settings
//   - ConfigManager: Manages configuration lifecycle (load, save, update)
//   - ConfigurationValidator: Validates configuration against rules and schemas
//   - ConfigurationMigrator: Migrates configuration between versions
//   - ConfigurationTransformer: Transforms configuration with variable substitution
//   - ConfigurationTemplateManager: Manages configuration templates
//   - ConfigAPI: REST API for runtime configuration management
//
// # Configuration Structure
//
// The main Config struct contains sections for all application components:
//
//	type Config struct {
//	    Version     string            // Configuration version
//	    Application ApplicationConfig // Application settings
//	    Server      ServerConfig      // HTTP server settings
//	    Database    database.Config   // PostgreSQL settings
//	    Redis       RedisConfig       // Redis cache settings
//	    Auth        AuthConfig        // Authentication settings
//	    Workers     WorkersConfig     // Worker pool settings
//	    Tasks       TasksConfig       // Task management settings
//	    LLM         LLMConfig         // LLM provider settings
//	    Providers   ProvidersConfig   // External provider settings
//	    Logging     LoggingConfig     // Logging settings
//	    Cognee      *CogneeConfig     // Cognee integration settings
//	}
//
// # Loading Configuration
//
// Load configuration using Viper with multiple sources:
//
//	// Load from default locations
//	cfg, err := config.Load()
//
//	// Load using ConfigManager
//	manager, err := config.NewHelixConfigManager("/path/to/config.yaml")
//	cfg := manager.GetConfig()
//
// Configuration is loaded from these sources (in priority order):
//   - CLI flag for config path
//   - HELIX_CONFIG environment variable
//   - ./config/config.yaml
//   - ./config.yaml
//   - ~/.config/helixcode/config.yaml
//   - /etc/helixcode/config.yaml
//
// # Environment Variables
//
// Configuration can be overridden with environment variables:
//
//	HELIX_AUTH_JWT_SECRET        // JWT signing secret
//	HELIX_DATABASE_PASSWORD      // Database password
//	HELIX_DATABASE_HOST          // Database host
//	HELIX_DATABASE_PORT          // Database port
//	HELIX_DATABASE_USER          // Database user
//	HELIX_DATABASE_NAME          // Database name
//	HELIX_REDIS_PASSWORD         // Redis password
//	HELIX_REDIS_HOST             // Redis host
//	HELIX_REDIS_PORT             // Redis port
//	HELIX_CONFIG                 // Config file path
//
// # Configuration Validation
//
// Validate configuration with strict or lenient modes:
//
//	validator := config.NewConfigurationValidator(true) // strict mode
//
//	// Validate entire configuration
//	result := validator.Validate(cfg)
//	if !result.Valid {
//	    for _, err := range result.Errors {
//	        fmt.Printf("Error: %s - %s\n", err.Path, err.Message)
//	    }
//	}
//
//	// Validate specific field
//	result := validator.ValidateField(cfg, "server.port")
//
//	// Add custom validation rules
//	validator.AddCustomRule("application.name", func(value interface{}) error {
//	    name := value.(string)
//	    if len(name) < 3 {
//	        return errors.New("name too short")
//	    }
//	    return nil
//	})
//
// # Configuration Migration
//
// Migrate configuration between versions:
//
//	migrator := config.NewConfigurationMigrator("1.2.0")
//
//	// Get available versions
//	versions := migrator.GetAvailableVersions()
//
//	// Migrate to target version
//	err := migrator.Migrate(cfg, "1.2.0")
//
// # Configuration Templates
//
// Use templates for common configuration scenarios:
//
//	templateMgr := config.NewConfigurationTemplateManager("/templates")
//
//	// Create template from existing config
//	template, err := templateMgr.CreateTemplateFromConfig(cfg, "development", "Dev config", variables)
//
//	// Apply template with variables
//	cfg, err := templateMgr.ApplyTemplate("development", map[string]interface{}{
//	    "port": 8080,
//	    "debug": true,
//	})
//
//	// Use default templates
//	templates := config.CreateDefaultTemplates()
//	// Available: "basic", "development", "production", "testing"
//
// # Configuration Transformation
//
// Transform configuration with variable substitution:
//
//	transformer := config.NewConfigurationTransformer()
//
//	// Add mapping rules
//	transformer.AddMapping(config.TransformMapping{
//	    Source:    "server_port",
//	    Target:    "server.port",
//	    Transform: "copy",
//	    Priority:  1,
//	})
//
//	// Transform with variables
//	newCfg, err := transformer.Transform(cfg, map[string]interface{}{
//	    "server_port": 9000,
//	})
//
// # Runtime Configuration
//
// Update configuration at runtime using ConfigManager:
//
//	// Update with function
//	err := manager.UpdateConfig(func(cfg *config.Config) {
//	    cfg.Server.Port = 9000
//	    cfg.Logging.Level = "debug"
//	})
//
//	// Export configuration
//	err := manager.ExportConfig("/path/to/export.json")
//
//	// Import configuration
//	err := manager.ImportConfig("/path/to/import.json")
//
//	// Reset to defaults
//	err := manager.ResetToDefaults()
//
// # Configuration API
//
// Expose configuration via REST API:
//
//	api := config.NewConfigAPI(manager)
//	api.SetupRoutes(ginRouter.Group("/api"))
//
//	// Endpoints:
//	// GET  /api/config        - Get current configuration
//	// PUT  /api/config        - Update configuration
//	// POST /api/config/reload - Reload from disk
//	// POST /api/config/restore - Restore defaults/backup
//	// GET  /api/config/ws     - WebSocket for live updates
//
// # Configuration Watching
//
// Watch for configuration changes:
//
//	manager.AddWatcher(myWatcher)
//
//	// Implement ConfigWatcher interface
//	type myWatcher struct{}
//	func (w *myWatcher) OnConfigChange(old, new *config.Config) error {
//	    // Handle configuration change
//	    return nil
//	}
//
// # Validation Modes
//
// The package supports different validation modes:
//
//	const (
//	    ValidationModeStrict   // All rules enforced
//	    ValidationModeLenient  // Warnings only
//	    ValidationModeDisabled // No validation
//	    ValidationModeSchema   // Schema-based validation
//	)
//
// # Default Values
//
// The package provides sensible defaults for all settings:
//
//   - Server: port 8080, 30s timeouts
//   - Database: localhost:5432, helixcode database
//   - Redis: disabled by default, localhost:6379
//   - Auth: 12 bcrypt cost, token expiry configured
//   - Workers: 30s health check, 10 max concurrent tasks
//   - Tasks: 3 max retries, 300s checkpoint interval
//   - LLM: local provider, llama-3.2-3b model
//   - Logging: info level, text format, stdout
//
// # Thread Safety
//
// ConfigManager and ConfigAPI are safe for concurrent use. Configuration
// updates are synchronized to prevent race conditions.
//
// # Helper Functions
//
// Utility functions for configuration:
//
//	// Get environment variable with default
//	value := config.GetEnvOrDefault("KEY", "default")
//
//	// Get environment variable as integer
//	port := config.GetEnvIntOrDefault("PORT", 8080)
//
//	// Get default config path
//	path := config.GetConfigPath()
//
//	// Check if config exists
//	exists := config.IsConfigPresent()
//
//	// Create default config file
//	err := config.CreateDefaultConfig("/path/to/config.yaml")
package config
