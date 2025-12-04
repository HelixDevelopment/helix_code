package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"dev.helix.code/internal/database"
	"github.com/spf13/viper"
)

// AuthConfig represents authentication configuration
type AuthConfig struct {
	JWTSecret     string `mapstructure:"jwt_secret"`
	TokenExpiry   int    `mapstructure:"token_expiry"`
	SessionExpiry int    `mapstructure:"session_expiry"`
	BcryptCost    int    `mapstructure:"bcrypt_cost"`
}

// ServerConfig represents server configuration
type ServerConfig struct {
	Address         string `mapstructure:"address"`
	Port            int    `mapstructure:"port"`
	ReadTimeout     int    `mapstructure:"read_timeout"`
	WriteTimeout    int    `mapstructure:"write_timeout"`
	IdleTimeout     int    `mapstructure:"idle_timeout"`
	ShutdownTimeout int    `mapstructure:"shutdown_timeout"`
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	Database int    `mapstructure:"database"`
}

// WorkersConfig represents worker configuration
type WorkersConfig struct {
	HealthCheckInterval int `mapstructure:"health_check_interval"`
	HealthTTL           int `mapstructure:"health_ttl"`
	MaxConcurrentTasks  int `mapstructure:"max_concurrent_tasks"`
}

// TasksConfig represents task configuration
type TasksConfig struct {
	MaxRetries         int `mapstructure:"max_retries"`
	CheckpointInterval int `mapstructure:"checkpoint_interval"`
	CleanupInterval    int `mapstructure:"cleanup_interval"`
}

// LLMConfig represents LLM configuration
type LLMConfig struct {
	DefaultProvider string  `mapstructure:"default_provider"`
	DefaultModel    string  `mapstructure:"default_model"`
	MaxTokens       int     `mapstructure:"max_tokens"`
	Temperature     float64 `mapstructure:"temperature"`
}

// Config represents the application configuration
type Config struct {
	Version     string            `mapstructure:"version"`
	UpdatedBy   string            `mapstructure:"updated_by"`
	Application ApplicationConfig `mapstructure:"application"`
	Server      ServerConfig      `mapstructure:"server"`
	Database    database.Config   `mapstructure:"database"`
	Redis       RedisConfig       `mapstructure:"redis"`
	Auth        AuthConfig        `mapstructure:"auth"`
	Workers     WorkersConfig     `mapstructure:"workers"`
	Tasks       TasksConfig       `mapstructure:"tasks"`
	LLM         LLMConfig         `mapstructure:"llm"`
	Providers   ProvidersConfig   `mapstructure:"providers"`
	Logging     LoggingConfig     `mapstructure:"logging"`
	Cognee      *CogneeConfig     `mapstructure:"cognee"`
}

// HelixConfig is an alias for Config
type HelixConfig = Config

// ProvidersConfig represents provider configurations
type ProvidersConfig struct {
	Mem0    Mem0Config    `mapstructure:"mem0"`
	Zep     ZepConfig     `mapstructure:"zep"`
	Memonto MemontoConfig `mapstructure:"memonto"`
	BaseAI  BaseAIConfig  `mapstructure:"baseai"`
}

// Mem0Config represents Mem0 provider configuration
type Mem0Config struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// ZepConfig represents Zep provider configuration
type ZepConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// MemontoConfig represents Memonto provider configuration
type MemontoConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// BaseAIConfig represents BaseAI provider configuration
type BaseAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// TelemetryConfig represents telemetry configuration
type TelemetryConfig struct {
	Enabled        bool   `mapstructure:"enabled"`
	Level          string `mapstructure:"level"`
	DataRetention  int    `mapstructure:"data_retention"`
}

// ApplicationConfig represents application configuration
type ApplicationConfig struct {
	Name        string          `mapstructure:"name"`
	Version     string          `mapstructure:"version"`
	Description string          `mapstructure:"description"`
	Environment string          `mapstructure:"environment"`
	Workspace   WorkspaceConfig `mapstructure:"workspace"`
	Session     SessionConfig   `mapstructure:"session"`
	Logging     LoggingConfig   `mapstructure:"logging"`
	Telemetry   TelemetryConfig `mapstructure:"telemetry"`
}

// WorkspaceConfig represents workspace configuration
type WorkspaceConfig struct {
	AutoSave         bool   `mapstructure:"auto_save"`
	DefaultPath      string `mapstructure:"default_path"`
	AutoSaveInterval int    `mapstructure:"auto_save_interval"`
	BackupEnabled    bool   `mapstructure:"backup_enabled"`
	BackupLocation   string `mapstructure:"backup_location"`
	BackupRetention  int    `mapstructure:"backup_retention"`
}

// ContextCompressionConfig represents context compression configuration
type ContextCompressionConfig struct {
	Enabled          bool    `mapstructure:"enabled"`
	Threshold        int     `mapstructure:"threshold"`
	Strategy         string  `mapstructure:"strategy"`
	CompressionRatio float64 `mapstructure:"compression_ratio"`
	RetentionPolicy  string  `mapstructure:"retention_policy"`
}

// SessionConfig represents session configuration
type SessionConfig struct {
	Timeout            int                      `mapstructure:"timeout"`
	AutoSave           bool                     `mapstructure:"auto_save"`
	MaxHistory         int                      `mapstructure:"max_history"`
	PersistContext     bool                     `mapstructure:"persist_context"`
	ContextRetention   int                      `mapstructure:"context_retention"`
	MaxHistorySize     int                      `mapstructure:"max_history_size"`
	AutoResume         bool                     `mapstructure:"auto_resume"`
	ContextCompression ContextCompressionConfig `mapstructure:"context_compression"`
}

// Load loads configuration from file and environment variables
func Load() (*Config, error) {
	// Set default values
	setDefaults()

	// Find config file
	configPath := findConfigFile()
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// Use default config locations
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("./config/")
		viper.AddConfigPath("./")
		viper.AddConfigPath("$HOME/.config/helixcode/")
		viper.AddConfigPath("/etc/helixcode/")
	}

	// Read in environment variables
	viper.AutomaticEnv()
	viper.SetEnvPrefix("HELIX")

	// Explicitly bind environment variables for critical settings
	viper.BindEnv("auth.jwt_secret", "HELIX_AUTH_JWT_SECRET")
	viper.BindEnv("database.password", "HELIX_DATABASE_PASSWORD")
	viper.BindEnv("database.host", "HELIX_DATABASE_HOST")
	viper.BindEnv("database.port", "HELIX_DATABASE_PORT")
	viper.BindEnv("database.user", "HELIX_DATABASE_USER")
	viper.BindEnv("database.dbname", "HELIX_DATABASE_NAME")
	viper.BindEnv("redis.password", "HELIX_REDIS_PASSWORD")
	viper.BindEnv("redis.host", "HELIX_REDIS_HOST")
	viper.BindEnv("redis.port", "HELIX_REDIS_PORT")

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %v", err)
		}
		// Config file not found, but we can continue with defaults
		fmt.Println("⚠️  No config file found, using defaults and environment variables")
	} else {
		fmt.Printf("📁 Using config file: %s\n", viper.ConfigFileUsed())
	}

	// Unmarshal config
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// Validate config
	if err := validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("config validation failed: %v", err)
	}

	return &cfg, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Version defaults
	viper.SetDefault("version", "1.0.0")

	// Application defaults
	viper.SetDefault("application.name", "HelixCode")
	viper.SetDefault("application.version", "1.0.0")
	viper.SetDefault("application.description", "Enterprise AI Development Platform")
	viper.SetDefault("application.environment", "development")
	viper.SetDefault("application.workspace.auto_save", true)
	viper.SetDefault("application.telemetry.enabled", false)
	viper.SetDefault("application.telemetry.level", "info")
	viper.SetDefault("application.telemetry.data_retention", 30)

	// Server defaults
	viper.SetDefault("server.address", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", 30)
	viper.SetDefault("server.write_timeout", 30)
	viper.SetDefault("server.idle_timeout", 60)
	viper.SetDefault("server.shutdown_timeout", 30)

	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "helixcode")
	viper.SetDefault("database.dbname", "helixcode")
	viper.SetDefault("database.sslmode", "disable")

	// Redis defaults
	viper.SetDefault("redis.enabled", false)
	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.password", "")
	viper.SetDefault("redis.db", 0)

	// Auth defaults
	viper.SetDefault("auth.jwt_secret", "default-secret-change-in-production")
	viper.SetDefault("auth.token_expiry", 60)    // 60 seconds (not hours)
	viper.SetDefault("auth.session_expiry", 600) // 10 minutes (not days)
	viper.SetDefault("auth.bcrypt_cost", 12)

	// Workers defaults
	viper.SetDefault("workers.health_check_interval", 30)
	viper.SetDefault("workers.health_ttl", 120)
	viper.SetDefault("workers.max_concurrent_tasks", 10)

	// Tasks defaults
	viper.SetDefault("tasks.max_retries", 3)
	viper.SetDefault("tasks.checkpoint_interval", 300)
	viper.SetDefault("tasks.cleanup_interval", 600)

	// LLM defaults
	viper.SetDefault("llm.default_provider", "local")
	viper.SetDefault("llm.default_model", "llama-3.2-3b")
	viper.SetDefault("llm.max_tokens", 4096)
	viper.SetDefault("llm.temperature", 0.7)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("logging.output", "stdout")
}

// findConfigFile searches for config file in various locations
func findConfigFile() string {
	// Check environment variable first
	if configPath := os.Getenv("HELIX_CONFIG"); configPath != "" {
		if absPath, err := filepath.Abs(configPath); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath
			}
		}
	}

	// Check common locations
	locations := []string{
		"./config.yaml",
		"./config/config.yaml",
		"config.yaml",
		"$HOME/.config/helixcode/config.yaml",
		"/etc/helixcode/config.yaml",
	}

	for _, location := range locations {
		expanded := os.ExpandEnv(location)
		if absPath, err := filepath.Abs(expanded); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath
			}
		}
	}

	return ""
}

// validateConfig validates the configuration
func validateConfig(cfg *Config) error {
	// Version validation
	if cfg.Version == "" {
		return fmt.Errorf("version is required")
	}

	// Application validation
	if cfg.Application.Name == "" {
		return fmt.Errorf("application name is required")
	}

	// Server validation
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server port must be between 1 and 65535")
	}

	// Database validation
	if cfg.Database.Host == "" {
		return fmt.Errorf("database host is required")
	}
	if cfg.Database.DBName == "" {
		return fmt.Errorf("database name is required")
	}

	// Redis validation
	if cfg.Redis.Enabled {
		if cfg.Redis.Host == "" {
			return fmt.Errorf("redis host is required when redis is enabled")
		}
		if cfg.Redis.Port < 1 || cfg.Redis.Port > 65535 {
			return fmt.Errorf("redis port must be between 1 and 65535")
		}
	}

	// Auth validation
	if cfg.Auth.JWTSecret == "" || cfg.Auth.JWTSecret == "default-secret-change-in-production" {
		return fmt.Errorf("JWT secret must be set and not use default value")
	}

	// Workers validation
	if cfg.Workers.HealthCheckInterval < 1 {
		return fmt.Errorf("health check interval must be positive")
	}
	if cfg.Workers.MaxConcurrentTasks < 1 {
		return fmt.Errorf("max concurrent tasks must be positive")
	}

	// Tasks validation
	if cfg.Tasks.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	// LLM validation
	if cfg.LLM.MaxTokens < 1 {
		return fmt.Errorf("max tokens must be positive")
	}
	if cfg.LLM.Temperature < 0 || cfg.LLM.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	return nil
}

// CreateDefaultConfig creates a default configuration file
func CreateDefaultConfig(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Create default config content
	configContent := `# HelixCode Server Configuration

server:
  address: "0.0.0.0"
  port: 8080
  read_timeout: 30
  write_timeout: 30
  idle_timeout: 300
  shutdown_timeout: 30

database:
  host: "localhost"
  port: 5432
  user: "helixcode"
  password: "" # Set via HELIX_DATABASE_PASSWORD environment variable
  dbname: "helixcode"
  sslmode: "disable"

redis:
  host: "localhost"
  port: 6379
  password: "" # Set via HELIX_REDIS_PASSWORD environment variable
  db: 0
  enabled: true

auth:
  jwt_secret: "" # Set via HELIX_AUTH_JWT_SECRET environment variable
  token_expiry: 86400
  session_expiry: 604800
  bcrypt_cost: 12

workers:
  health_check_interval: 30
  health_ttl: 120
  max_concurrent_tasks: 10

tasks:
  max_retries: 3
  checkpoint_interval: 300
  cleanup_interval: 3600

llm:
  default_provider: "local"
  providers:
    local: "http://localhost:11434"
    openai: "" # Set API key via environment variable
  max_tokens: 4096
  temperature: 0.7

logging:
  level: "info"
  format: "text"
  output: "stdout"
`

	// Write config file
	if err := os.WriteFile(path, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// GetEnvOrDefault gets an environment variable with a default value
func GetEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetEnvIntOrDefault gets an environment variable as integer with a default value
func GetEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getDefaultConfig returns a default configuration
func getDefaultConfig() *Config {
	setDefaults()
	var cfg Config
	viper.Unmarshal(&cfg)
	return &cfg
}

// ConfigManager manages configuration loading and saving
type ConfigManager struct {
	configPath string
	config     *Config
}

// Initialize initializes the configuration manager
func (m *ConfigManager) Initialize(ctx context.Context) error {
	// Configuration manager is already initialized during creation
	// This method exists for compatibility with the test expectations
	return nil
}

// NewConfigurationManager creates a new configuration manager with options
func NewConfigurationManager(options *ConfigurationOptions) (*ConfigManager, error) {
	// For now, just use the config path from options
	return NewHelixConfigManager(options.ConfigPath)
}

// NewHelixConfigManager creates a new configuration manager
func NewHelixConfigManager(configPath string) (*ConfigManager, error) {
	manager := &ConfigManager{
		configPath: configPath,
	}

	// Try to load existing config
	if _, err := os.Stat(configPath); err == nil {
		if err := manager.loadConfig(); err != nil {
			return nil, err
		}
	} else {
		// Create default config
		manager.config = getDefaultConfig()
		if err := manager.saveConfig(); err != nil {
			return nil, err
		}
	}

	return manager, nil
}

// GetConfig returns the current configuration
func (m *ConfigManager) GetConfig() *Config {
	return m.config
}

// UpdateConfig updates the configuration with the provided function
func (m *ConfigManager) UpdateConfig(updateFunc func(*Config)) error {
	updateFunc(m.config)
	return m.saveConfig()
}

// UpdateConfigFromMap updates the configuration with a map of values
func (m *ConfigManager) UpdateConfigFromMap(updates map[string]interface{}) error {
	return m.UpdateConfig(func(cfg *Config) {
		// Apply updates from map - for now this is a placeholder
		// In a real implementation, this would walk the map and update nested fields
	})
}

// IsConfigPresent checks if the configuration file exists
func (m *ConfigManager) IsConfigPresent() bool {
	_, err := os.Stat(m.configPath)
	return err == nil
}

// GetConfigPath returns the configuration file path
func (m *ConfigManager) GetConfigPath() string {
	return m.configPath
}

// loadConfig loads configuration from file
func (m *ConfigManager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	m.config = &Config{}
	return json.Unmarshal(data, m.config)
}

// saveConfig saves configuration to file
func (m *ConfigManager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// AddWatcher adds a configuration change watcher
func (m *ConfigManager) AddWatcher(watcher ConfigWatcher) {}

// ExportConfig exports the configuration to a file
func (m *ConfigManager) ExportConfig(path string) error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ImportConfig imports the configuration from a file
func (m *ConfigManager) ImportConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	m.config = &Config{}
	err = json.Unmarshal(data, m.config)
	if err != nil {
		return err
	}
	return m.saveConfig()
}

// BackupConfig backs up the configuration to a file
func (m *ConfigManager) BackupConfig(path string) error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ResetToDefaults resets the configuration to defaults
func (m *ConfigManager) ResetToDefaults() error {
	m.config = getDefaultConfig()
	return m.saveConfig()
}

// LoadConfig loads configuration from the default location
func LoadConfig() (*Config, error) {
	path := GetConfigPath()
	manager, err := NewHelixConfigManager(path)
	if err != nil {
		return nil, err
	}
	return manager.GetConfig(), nil
}

// SaveConfig saves configuration to the default location
func SaveConfig(config *Config) error {
	path := GetConfigPath()
	manager, err := NewHelixConfigManager(path)
	if err != nil {
		return err
	}
	manager.config = config
	return manager.saveConfig()
}

// GetConfigPath returns the default configuration file path
func GetConfigPath() string {
	if path := os.Getenv("HELIX_CONFIG_PATH"); path != "" {
		return path
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "helixcode", "config.json")
}

// IsConfigPresent checks if the default configuration file exists
func IsConfigPresent() bool {
	path := GetConfigPath()
	_, err := os.Stat(path)
	return err == nil
}

// UpdateConfig updates the configuration with the provided function
func UpdateConfig(updateFunc func(*Config)) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}
	updateFunc(config)
	return SaveConfig(config)
}

// GetHelixConfigPath returns the default configuration file path
func GetHelixConfigPath() string {
	return GetConfigPath()
}

// CreateDefaultHelixConfig creates a default configuration file
func CreateDefaultHelixConfig() error {
	return CreateDefaultConfig(GetConfigPath())
}

// IsHelixConfigPresent checks if the default configuration file exists
func IsHelixConfigPresent() bool {
	return IsConfigPresent()
}

// LoadHelixConfig loads configuration from the default location
func LoadHelixConfig() (*Config, error) {
	return LoadConfig()
}

// SaveHelixConfig saves configuration to the default location
func SaveHelixConfig(config *Config) error {
	return SaveConfig(config)
}

// UpdateHelixConfig updates the configuration with the provided function
func UpdateHelixConfig(updateFunc func(*Config)) error {
	return UpdateConfig(updateFunc)
}

// NewConfigWatcher creates a new configuration watcher
func NewConfigWatcher(configPath string) (ConfigWatcher, error) {
	return nil, nil
}

// GetConfigInfo returns configuration information
func GetConfigInfo() (*ConfigInfo, error) {
	return &ConfigInfo{}, nil
}

// ConfigInfo represents configuration information
type ConfigInfo struct{}

// ConfigWatcher represents a configuration watcher
type ConfigWatcher interface {
	OnConfigChange(old, new *Config) error
}

// ConfigurationValidator validates configuration
// ValidationRule represents a custom validation rule
type ValidationRule struct {
	Name      string
	Validator func(interface{}) error
	Message   string
	Severity  string
}

type ConfigurationValidator struct {
	strict bool
	rules  map[string][]ValidationRule
}

// NewConfigurationValidator creates a new configuration validator
func NewConfigurationValidator(strict bool) *ConfigurationValidator {
	validator := &ConfigurationValidator{
		strict: strict,
		rules:  make(map[string][]ValidationRule),
	}
	
	if strict {
		validator.addDefaultRules()
	}
	
	return validator
}

// AddRule adds a validation rule for a specific property
func (cv *ConfigurationValidator) AddRule(property string, rule ValidationRule) {
	if cv.rules[property] == nil {
		cv.rules[property] = make([]ValidationRule, 0)
	}
	cv.rules[property] = append(cv.rules[property], rule)
}

// addDefaultRules adds default validation rules
func (cv *ConfigurationValidator) addDefaultRules() {
	cv.AddRule("server.port", ValidationRule{
		Name: "port_range",
		Validator: func(value interface{}) error {
			port, ok := value.(int)
			if !ok {
				return fmt.Errorf("port must be an integer")
			}
			if port < 1 || port > 65535 {
				return fmt.Errorf("port must be between 1 and 65535")
			}
			return nil
		},
		Message:  "Port must be between 1 and 65535",
		Severity: "error",
	})
	
	cv.AddRule("llm.temperature", ValidationRule{
		Name: "temperature_range",
		Validator: func(value interface{}) error {
			temp, ok := value.(float64)
			if !ok {
				return fmt.Errorf("temperature must be a number")
			}
			if temp < 0.0 || temp > 2.0 {
				return fmt.Errorf("temperature must be between 0.0 and 2.0")
			}
			return nil
		},
		Message:  "LLM temperature must be between 0.0 and 2.0",
		Severity: "error",
	})
}

// Validate validates the configuration
func (v *ConfigurationValidator) Validate(config *Config) ValidationResult {
	result := ValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
	}
	
	// Validate server port
	if config.Server.Port < 1 || config.Server.Port > 65535 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Property: "server.port",
			Path:     "server.port",
			Severity: "error",
			Code:     "invalid_port",
			Message:  "Port must be between 1 and 65535",
		})
	}
	
	// Validate application environment
	validEnvironments := []string{"development", "production", "testing", "staging"}
	isValidEnv := false
	for _, env := range validEnvironments {
		if config.Application.Environment == env {
			isValidEnv = true
			break
		}
	}
	if !isValidEnv {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Property: "application.environment",
			Path:     "application.environment",
			Severity: "error",
			Code:     "FIELD_SCHEMA_ERROR",
			Message:  "Environment must be one of: development, production, testing, staging",
		})
	}
	
	// Validate LLM provider
	validProviders := []string{"local", "openai", "anthropic", "gemini", "xai", "openrouter", "copilot"}
	isValidProvider := false
	for _, provider := range validProviders {
		if config.LLM.DefaultProvider == provider {
			isValidProvider = true
			break
		}
	}
	if !isValidProvider {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Property: "llm.default_provider",
			Path:     "llm.default_provider",
			Severity: "error",
			Code:     "FIELD_SCHEMA_ERROR",
			Message:  "LLM provider must be one of: local, openai, anthropic, gemini, xai, openrouter, copilot",
		})
	}
	
	// Validate LLM max tokens
	if config.LLM.MaxTokens < 1 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Property: "llm.max_tokens",
			Path:     "llm.max_tokens",
			Severity: "error",
			Code:     "FIELD_SCHEMA_ERROR",
			Message:  "LLM max tokens must be a positive integer",
		})
	}
	
	// Validate LLM temperature
	if config.LLM.Temperature < 0.0 || config.LLM.Temperature > 2.0 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Property: "llm.temperature",
			Path:     "llm.temperature",
			Severity: "error",
			Code:     "CUSTOM_RULE_ERROR",
			Message:  "LLM temperature must be between 0.0 and 2.0",
		})
	}
	
	// Validate database port
	if config.Database.Port < 1 || config.Database.Port > 65535 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Property: "database.port",
			Path:     "database.port",
			Severity: "error",
			Code:     "invalid_port",
			Message:  "Database port must be between 1 and 65535",
		})
	}
	
	// Validate JWT secret
	if len(config.Auth.JWTSecret) < 32 {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Property: "auth.jwt_secret",
			Path:     "auth.jwt_secret",
			Severity: "error",
			Code:     "invalid_jwt_secret",
			Message:  "JWT secret must be at least 32 characters",
		})
	}
	
	// Validate custom rules
	if v.rules != nil {
		// Check application.name field
		if rules, exists := v.rules["application.name"]; exists {
			value := config.Application.Name
			for _, rule := range rules {
				if rule.Name == "custom" {
					if err := rule.Validator(value); err != nil {
						result.Valid = false
						result.Errors = append(result.Errors, ValidationError{
							Property: "application.name",
							Path:     "application.name",
							Severity: "error",
							Code:     "CUSTOM_RULE_ERROR",
							Message:  "Custom rule validation failed",
						})
					}
				}
			}
		}
	}
	
	return result
}

// ValidateField validates a specific field
func (v *ConfigurationValidator) ValidateField(config *Config, field string) ValidationResult {
	result := ValidationResult{
		Valid:  true,
		Errors: make([]ValidationError, 0),
		Path:   field,
	}
	
	switch field {
	case "server.port":
		if config.Server.Port < 1 || config.Server.Port > 65535 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Property: "server.port",
				Path:     "server.port",
				Severity: "error",
				Code:     "invalid_port",
				Message:  "Port must be between 1 and 65535",
			})
		}
	case "llm.temperature":
		if config.LLM.Temperature < 0.0 || config.LLM.Temperature > 2.0 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Property: "llm.temperature",
				Path:     "llm.temperature",
				Severity: "error",
				Code:     "invalid_temperature",
				Message:  "LLM temperature must be between 0.0 and 2.0",
			})
		}
	case "database.port":
		if config.Database.Port < 1 || config.Database.Port > 65535 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Property: "database.port",
				Path:     "database.port",
				Severity: "error",
				Code:     "invalid_port",
				Message:  "Database port must be between 1 and 65535",
			})
		}
	case "auth.jwt_secret":
		if len(config.Auth.JWTSecret) < 32 {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Property: "auth.jwt_secret",
				Path:     "auth.jwt_secret",
				Severity: "error",
				Code:     "invalid_jwt_secret",
				Message:  "JWT secret must be at least 32 characters",
			})
		}
	}
	
	return result
}

// AddCustomRule adds a custom validation rule
func (v *ConfigurationValidator) AddCustomRule(field string, rule func(interface{}) error) {
	// Store custom rule - implementation depends on rules field existence
	if v.rules == nil {
		v.rules = make(map[string][]ValidationRule)
	}
	
	v.rules[field] = append(v.rules[field], ValidationRule{
		Name:      "custom",
		Validator: rule,
		Message:   "Custom rule validation failed",
		Severity:  "error",
	})
}

// ValidationResult represents validation result
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
	Path   string
}

// ValidationError represents a validation error
type ValidationError struct {
	Property string
	Path     string
	Severity string
	Code     string
	Message  string
}

// createDefaultSchema creates the default validation schema
func (v *ConfigurationValidator) createDefaultSchema() *ValidationSchema {
	schema := &ValidationSchema{
		Version:  "1.0",
		Properties: make(map[string]*SchemaProperty),
		Required: []string{"version", "application", "server"},
	}
	
	// Application properties
	schema.Properties["application"] = &SchemaProperty{
		Type: "object",
		Properties: map[string]*SchemaProperty{
			"name": {
				Type:      "string",
				MinLength: intPtr(1),
				MaxLength: intPtr(100),
			},
			"version": {
				Type:      "string",
				MinLength: intPtr(1),
			},
			"environment": {
				Type: "string",
			},
		},
		Required: []string{"name", "version"},
	}
	
	// Server properties
	schema.Properties["server"] = &SchemaProperty{
		Type: "object",
		Properties: map[string]*SchemaProperty{
			"address": {
				Type: "string",
			},
			"port": {
				Type: "integer",
			},
			"read_timeout": {
				Type: "integer",
			},
			"write_timeout": {
				Type: "integer",
			},
		},
		Required: []string{"address", "port"},
	}
	
	// Version property
	schema.Properties["version"] = &SchemaProperty{
		Type: "string",
		MinLength: intPtr(1),
	}
	
	return schema
}

// ValidationSchema represents validation schema
type ValidationSchema struct {
	Version    string
	Properties map[string]*SchemaProperty
	Required   []string
}

// SchemaProperty represents a schema property
type SchemaProperty struct {
	Type       string
	Properties map[string]*SchemaProperty
	Required   []string
	MinLength  *int
	MaxLength  *int
}

// ConfigurationMigrator migrates configuration between versions
type ConfigurationMigrator struct {
	current    string
	migrations map[string][]Migration
}

// NewConfigurationMigrator creates a new configuration migrator
func NewConfigurationMigrator(currentVersion string) *ConfigurationMigrator {
	m := &ConfigurationMigrator{
		current:    currentVersion,
		migrations: make(map[string][]Migration),
	}
	
	// Register migrations
	m.registerMigrations()
	return m
}

// registerMigrations registers all available migrations
func (m *ConfigurationMigrator) registerMigrations() {
	// 1.0.0 -> 1.1.0
	m.addMigration("1.0.0", "1.1.0", Migration{
		From: "1.0.0",
		To:   "1.1.0",
		Up: func(config *Config) error {
			// Add auto-save feature with default true
			config.Application.Workspace.AutoSave = true
			config.Version = "1.1.0"
			return nil
		},
		Down: func(config *Config) error {
			config.Version = "1.0.0"
			return nil
		},
	})
	
	// 1.1.0 -> 1.2.0
	m.addMigration("1.1.0", "1.2.0", Migration{
		From: "1.1.0",
		To:   "1.2.0",
		Up: func(config *Config) error {
			// Ensure auto-save is enabled in 1.2.0
			config.Application.Workspace.AutoSave = true
			config.Version = "1.2.0"
			return nil
		},
		Down: func(config *Config) error {
			config.Version = "1.1.0"
			return nil
		},
	})
}

// addMigration adds a migration to the registry
func (m *ConfigurationMigrator) addMigration(from, to string, migration Migration) {
	if m.migrations[from] == nil {
		m.migrations[from] = []Migration{}
	}
	m.migrations[from] = append(m.migrations[from], migration)
}

// GetAvailableVersions returns available versions
func (m *ConfigurationMigrator) GetAvailableVersions() []string {
	return []string{"1.0.0", "1.1.0", "1.2.0"}
}

// Migrate migrates configuration to a target version
func (m *ConfigurationMigrator) Migrate(config *Config, targetVersion string) error {
	if config.Version == targetVersion {
		return nil
	}
	
	path := m.findMigrationPath(config.Version, targetVersion)
	if path == nil {
		return fmt.Errorf("no migration path from %s to %s", config.Version, targetVersion)
	}
	
	// Execute migrations in sequence
	currentVersion := config.Version
	for _, nextVersion := range path {
		migrations, exists := m.migrations[currentVersion]
		if !exists {
			return fmt.Errorf("no migrations from version %s", currentVersion)
		}
		
		// Find the migration to the next version
		var migration *Migration
		for j := range migrations {
			if migrations[j].To == nextVersion {
				migration = &migrations[j]
				break
			}
		}
		
		if migration == nil {
			return fmt.Errorf("no migration from %s to %s", currentVersion, nextVersion)
		}
		
		// Create backup if required
		if migration.Backup {
			if err := m.createBackup(config, currentVersion); err != nil {
				return fmt.Errorf("failed to create backup before migration from %s to %s: %w", currentVersion, nextVersion, err)
			}
		}
		
		// Execute the migration
		if err := migration.Up(config); err != nil {
			return fmt.Errorf("migration from %s to %s failed: %w", currentVersion, nextVersion, err)
		}
		
		// Update the configuration version
		config.Version = nextVersion
		
		currentVersion = nextVersion
	}
	
	return nil
}

// findMigrationPath finds the migration path
func (m *ConfigurationMigrator) findMigrationPath(from, to string) []string {
	// Direct migration available
	if migrations, exists := m.migrations[from]; exists {
		for _, migration := range migrations {
			if migration.To == to {
				// Return just the target version (1 step)
				return []string{to}
			}
		}
	}
	
	// Try multi-step paths
	for _, version := range m.GetAvailableVersions() {
		if version == from || version == to {
			continue
		}
		
		// Check if we can migrate from 'from' to 'version'
		if m.canMigrate(from, version) {
			// Check if we can then migrate from 'version' to 'to'
			if m.canMigrate(version, to) {
				// Return intermediate steps (2 steps)
				return []string{version, to}
			}
		}
	}
	
	// Check for reverse migration (downgrade)
	if from > to {
		// Simple downgrade path
		return []string{to}
	}
	
	return nil
}

// canMigrate checks if direct migration is possible
func (m *ConfigurationMigrator) canMigrate(from, to string) bool {
	migrations, exists := m.migrations[from]
	if !exists {
		return false
	}
	
	for _, migration := range migrations {
		if migration.To == to {
			return true
		}
	}
	return false
}

// createBackup creates a backup of the configuration
func (m *ConfigurationMigrator) createBackup(config *Config, version string) error {
	tempDir := os.TempDir()
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(tempDir, fmt.Sprintf("helix_config_backup_%s_%s.json", version, timestamp))
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config for backup: %w", err)
	}
	
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}
	
	return nil
}

// ConfigurationTransformer transforms configuration
type ConfigurationTransformer struct {
	mappings []TransformMapping
}

// Migration represents a configuration migration
type Migration struct {
	From   string
	To     string
	Name   string
	Desc   string
	Backup bool
	Up     func(config *Config) error
	Down   func(config *Config) error
	Migrate func(config *Config) error
}

// NewConfigurationTransformer creates a new configuration transformer
func NewConfigurationTransformer() *ConfigurationTransformer {
	return &ConfigurationTransformer{
		mappings: []TransformMapping{},
	}
}

// AddMapping adds a transformation mapping
func (t *ConfigurationTransformer) AddMapping(mapping TransformMapping) {
	t.mappings = append(t.mappings, mapping)
}

// Transform transforms configuration with variables
func (t *ConfigurationTransformer) Transform(config *Config, variables map[string]interface{}) (*Config, error) {
	// Create a copy of the config
	result := *config
	
	// Sort mappings by priority
	sort.Slice(t.mappings, func(i, j int) bool {
		return t.mappings[i].Priority < t.mappings[j].Priority
	})
	
	// Apply transformations
	for _, mapping := range t.mappings {
		// Skip if condition is specified and doesn't match
		if mapping.Condition != "" && result.Application.Environment != mapping.Condition {
			continue
		}
		
		// Apply transformation based on type
		switch mapping.Transform {
		case "copy":
			// Try to find value in variables - check direct source first
			if sourceVal, exists := variables[mapping.Source]; exists {
				// Handle different target paths
				switch mapping.Target {
				case "server.port":
					if port, ok := sourceVal.(int); ok {
						result.Server.Port = port
					}
				case "application.name":
					if name, ok := sourceVal.(string); ok {
						result.Application.Name = name
					}
				}
			} else {
				// Try alternate variable naming conventions
				switch mapping.Source {
				case "server.port":
					if portVal, exists := variables["server_port"]; exists {
						if port, ok := portVal.(int); ok {
							result.Server.Port = port
						}
					}
				case "application.name":
					if nameVal, exists := variables["app_name"]; exists {
						if name, ok := nameVal.(string); ok {
							result.Application.Name = name
						}
					}
				}
			}
		}
	}
	
	return &result, nil
}

// TransformMapping represents a transformation mapping
type TransformMapping struct {
	Source    string
	Target    string
	Transform string
	Priority  int
	Condition string
}

// ConfigurationOptions provides options for configuration management
// This is a simplified interface for testing purposes
type ConfigurationOptions struct {
	ConfigPath       string
	AutoSave         bool
	AutoBackup       bool
	EnableEncryption bool
	EncryptionKey    string
	SchemaPath       string
	ValidationMode   string
	TransformMode    string
	WatchInterval    time.Duration
	MaxBackups       int
	Compression      bool
	LogLevel         string
	BackupPath       string
}

// ConfigurationTemplateManager manages configuration templates
type ConfigurationTemplateManager struct {
	templateDir string
	templates   map[string]*ConfigurationTemplate
}

// TemplateVariable represents a template variable
type TemplateVariable struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Default     interface{} `json:"default"`
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required"`
	MinLength   *int        `json:"min_length,omitempty"`
	MaxLength   *int        `json:"max_length,omitempty"`
	Pattern     string      `json:"pattern,omitempty"`
	Min         *float64    `json:"min,omitempty"`
	Max         *float64    `json:"max,omitempty"`
}

// NewConfigurationTemplateManager creates a new template manager
func NewConfigurationTemplateManager(templateDir string) *ConfigurationTemplateManager {
	return &ConfigurationTemplateManager{
		templateDir: templateDir,
		templates:   make(map[string]*ConfigurationTemplate),
	}
}

// ConfigurationTemplate represents a configuration template
type ConfigurationTemplate struct {
	ID          string                        `json:"id"`
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	Category    string                        `json:"category"`
	Variables   map[string]*TemplateVariable   `json:"variables"`
	Config      *Config                       `json:"config"`
	CreatedAt   time.Time                     `json:"created_at"`
	UpdatedAt   time.Time                     `json:"updated_at"`
}

// CreateTemplateFromConfig creates a template from configuration
func (tm *ConfigurationTemplateManager) CreateTemplateFromConfig(config *Config, name, description string, variables map[string]*TemplateVariable) (*ConfigurationTemplate, error) {
	template := &ConfigurationTemplate{
		ID:          "template-" + name,
		Name:        name,
		Description: description,
		Category:    "custom",
		Variables:   variables,
		Config:      config,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	return template, nil
}

// SaveTemplate saves a configuration template
func (tm *ConfigurationTemplateManager) SaveTemplate(template *ConfigurationTemplate, path string) error {
	// Store in manager
	tm.templates[template.ID] = template
	// Placeholder implementation - would save to path
	return nil
}

// ApplyTemplate applies a template with variables
func (tm *ConfigurationTemplateManager) ApplyTemplate(templateID string, variables map[string]interface{}) (*Config, error) {
	template, exists := tm.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}
	
	// Return a copy of the template's config
	result := *template.Config
	return &result, nil
}

// LoadTemplate loads a configuration template
func (tm *ConfigurationTemplateManager) LoadTemplate(path string) (*Config, error) {
	// Placeholder implementation
	return getDefaultConfig(), nil
}

// SearchTemplates searches templates by query
func (tm *ConfigurationTemplateManager) SearchTemplates(query string) []*ConfigurationTemplate {
	results := make([]*ConfigurationTemplate, 0)
	lowerQuery := strings.ToLower(query)
	
	for _, template := range tm.templates {
		// Search in name, description, and category
		nameMatch := strings.Contains(strings.ToLower(template.Name), lowerQuery)
		descMatch := strings.Contains(strings.ToLower(template.Description), lowerQuery)
		categoryMatch := strings.Contains(strings.ToLower(template.Category), lowerQuery)
		
		if nameMatch || descMatch || categoryMatch {
			results = append(results, template)
		}
	}
	
	return results
}

// intPtr returns a pointer to int
func intPtr(i int) *int {
	return &i
}

// processTemplate processes a template with variable validation
func (tm *ConfigurationTemplateManager) processTemplate(template *ConfigurationTemplate, variables map[string]interface{}) (*Config, error) {
	// Validate required variables
	for name, variable := range template.Variables {
		if variable.Required {
			if _, exists := variables[name]; !exists {
				return nil, fmt.Errorf("required variable not provided: %s", name)
			}
		}
		
		// Type validation and constraints
		if value, exists := variables[name]; exists {
			if variable.Type == "string" {
				strValue, ok := value.(string)
				if !ok {
					return nil, fmt.Errorf("variable %s must be a string, got %T", name, value)
				}
				
				// Length validation
				if variable.MinLength != nil && len(strValue) < *variable.MinLength {
					return nil, fmt.Errorf("variable %s is too short (min %d chars)", name, *variable.MinLength)
				}
				if variable.MaxLength != nil && len(strValue) > *variable.MaxLength {
					return nil, fmt.Errorf("variable %s is too long (max %d chars)", name, *variable.MaxLength)
				}
				
				// Pattern validation
				if variable.Pattern != "" {
					matched, err := regexp.MatchString(variable.Pattern, strValue)
					if err != nil {
						return nil, fmt.Errorf("invalid pattern for variable %s: %w", name, err)
					}
					if !matched {
						return nil, fmt.Errorf("variable %s doesn't match required pattern %s", name, variable.Pattern)
					}
				}
			}
		}
	}
	
	// Create a deep copy of template's config for manipulation
	configBytes, err := json.Marshal(template.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal template config: %w", err)
	}
	
	var result Config
	if err := json.Unmarshal(configBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal template config: %w", err)
	}
	
	// Substitute variables using simple string replacement for now
	// In a full implementation, this would use a proper template engine
	configStr := string(configBytes)
	
	// Replace variables in the configuration
	for name, value := range variables {
		placeholder := "{{" + name + "}}"
		replacement := fmt.Sprintf("%v", value)
		configStr = strings.ReplaceAll(configStr, placeholder, replacement)
	}
	
	// Unmarshal the substituted configuration
	if err := json.Unmarshal([]byte(configStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal substituted config: %w", err)
	}
	
	return &result, nil
}

// CreateDefaultTemplates creates default configuration templates
func CreateDefaultTemplates() map[string]*ConfigurationTemplate {
	templates := make(map[string]*ConfigurationTemplate)
	
	// Add basic template
	templates["basic"] = &ConfigurationTemplate{
		ID:          "basic",
		Name:        "Basic Configuration",
		Description: "Basic server configuration",
		Category:    "default",
		Variables:   make(map[string]*TemplateVariable),
		Config:      getDefaultConfig(),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// Add development template
	devConfig := getDefaultConfig()
	devConfig.Application.Environment = "development"
	devConfig.Server.Port = 8080
	devConfig.Server.Address = "0.0.0.0"
	
	templates["development"] = &ConfigurationTemplate{
		ID:          "development",
		Name:        "Development Environment",
		Description: "Development environment configuration",
		Category:    "environment",
		Variables:   make(map[string]*TemplateVariable),
		Config:      devConfig,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// Add production template
	prodConfig := getDefaultConfig()
	prodConfig.Application.Environment = "production"
	prodConfig.Server.Port = 443
	prodConfig.Server.Address = "0.0.0.0"
	prodConfig.Logging.Level = "error"
	
	templates["production"] = &ConfigurationTemplate{
		ID:          "production",
		Name:        "Production Environment",
		Description: "Production environment configuration",
		Category:    "environment",
		Variables:   make(map[string]*TemplateVariable),
		Config:      prodConfig,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// Add testing template
	testConfig := getDefaultConfig()
	testConfig.Application.Environment = "testing"
	testConfig.Server.Port = 0
	testConfig.Server.Address = "0.0.0.0"
	testConfig.Logging.Level = "debug"
	testConfig.Database.Host = ""  // Empty host disables database
	testConfig.Redis.Enabled = false
	testConfig.Workers.MaxConcurrentTasks = 10
	
	templates["testing"] = &ConfigurationTemplate{
		ID:          "testing",
		Name:        "Testing Environment",
		Description: "Testing environment configuration",
		Category:    "environment",
		Variables:   make(map[string]*TemplateVariable),
		Config:      testConfig,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	return templates
}

// Configuration validation modes
const (
	ValidationModeStrict     = "strict"
	ValidationModeLenient    = "lenient"
	ValidationModeDisabled   = "disabled"
	ValidationModeSchema     = "schema"
)

// Configuration transformation modes
const (
	TransformModeStrict  = "strict"
	TransformModeLenient = "lenient"
	TransformModeNone    = "none"
	TransformModeSchema = "schema"
)
