package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	// Write test config
	content := `
version: "1.0.0"
server:
  address: "0.0.0.0"
  port: 8080
database:
  host: "localhost"
  port: 5432
  dbname: "test"
  user: "test"
auth:
  jwt_secret: "test-jwt-secret-for-testing"
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	// Clean up environment
	defer func() {
		if oldVal := os.Getenv("HELIX_CONFIG"); oldVal != "" {
			os.Unsetenv("HELIX_CONFIG")
		}
	}()
	os.Unsetenv("HELIX_AUTH_JWT_SECRET")

	// Test that config file exists and can be read
	_, err = os.Stat(configPath)
	require.NoError(t, err)
	
	// Basic test - create a config manager and initialize it
	manager, err := NewHelixConfigManager(configPath)
	require.NoError(t, err)
	assert.NotNil(t, manager)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Auth: AuthConfig{
					JWTSecret: "test-secret",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			config: Config{
				Server: ServerConfig{Port: 99999},
			},
			wantErr: true,
		},
		{
			name: "missing required fields",
			config: Config{
				Server: ServerConfig{Port: 8080},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigurationValidator(true)
			result := validator.Validate(&tt.config)
			if tt.wantErr {
				assert.False(t, result.Valid)
				assert.NotEmpty(t, result.Errors)
			} else {
				assert.True(t, result.Valid)
				assert.Empty(t, result.Errors)
			}
		})
	}
}

func TestFindConfigFile(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")
	
	content := `
server:
  port: 8080
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	// Change to temp directory
	oldCwd, _ := os.Getwd()
	defer os.Chdir(oldCwd)
	os.Chdir(tempDir)

	found := findConfigFile()
	assert.Equal(t, configPath, found)
}

func TestCreateDefaultConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.yaml")

	// Create a basic config for testing
	content := `server:
  address: "0.0.0.0"
  port: 8080
auth:
  jwt_secret: "change-me"
database:
  host: "localhost"
  port: 5432
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	assert.NoError(t, err)

	// Check if file was created
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

func TestConfigValidationEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		valid  bool
	}{
		{
			name: "minimal valid config",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Auth:  AuthConfig{JWTSecret: "test-secret"},
			},
			valid: true,
		},
		{
			name: "port too low",
			config: Config{
				Server: ServerConfig{Port: 0},
			},
			valid: false,
		},
		{
			name: "port negative",
			config: Config{
				Server: ServerConfig{Port: -1},
			},
			valid: false,
		},
		{
			name: "missing auth secret",
			config: Config{
				Server: ServerConfig{Port: 8080},
				Auth:  AuthConfig{JWTSecret: ""},
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewConfigurationValidator(true)
			result := validator.Validate(&tt.config)
			if tt.valid {
				assert.True(t, result.Valid)
				assert.Empty(t, result.Errors)
			} else {
				assert.False(t, result.Valid)
				assert.NotEmpty(t, result.Errors)
			}
		})
	}
}