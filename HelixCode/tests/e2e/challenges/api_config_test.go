package challenges

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAPIKeys_FileNotExists(t *testing.T) {
	// Should return empty APIKeys without error when file doesn't exist
	apiKeys, err := LoadAPIKeys("nonexistent-file.yaml")
	if err != nil {
		t.Errorf("Expected no error for nonexistent file, got: %v", err)
	}
	if apiKeys == nil {
		t.Fatal("Expected non-nil APIKeys")
	}
}

func TestLoadAPIKeys_ValidFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-api-keys.yaml")

	configContent := `
xai:
  api_key: "test-xai-key-123"

openai:
  api_key: "sk-test-openai-key"
  organization: "org-test"

anthropic:
  api_key: "sk-ant-test-key"

groq:
  api_key: "gsk-test-key"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	// Load the config
	apiKeys, err := LoadAPIKeys(configPath)
	if err != nil {
		t.Fatalf("Failed to load API keys: %v", err)
	}

	// Verify xAI
	if apiKeys.XAI == nil {
		t.Error("Expected xAI config to be loaded")
	} else if apiKeys.XAI.APIKey != "test-xai-key-123" {
		t.Errorf("Expected xAI key 'test-xai-key-123', got '%s'", apiKeys.XAI.APIKey)
	}

	// Verify OpenAI
	if apiKeys.OpenAI == nil {
		t.Error("Expected OpenAI config to be loaded")
	} else {
		if apiKeys.OpenAI.APIKey != "sk-test-openai-key" {
			t.Errorf("Expected OpenAI key 'sk-test-openai-key', got '%s'", apiKeys.OpenAI.APIKey)
		}
		if apiKeys.OpenAI.Organization != "org-test" {
			t.Errorf("Expected OpenAI org 'org-test', got '%s'", apiKeys.OpenAI.Organization)
		}
	}

	// Verify Anthropic
	if apiKeys.Anthropic == nil {
		t.Error("Expected Anthropic config to be loaded")
	} else if apiKeys.Anthropic.APIKey != "sk-ant-test-key" {
		t.Errorf("Expected Anthropic key 'sk-ant-test-key', got '%s'", apiKeys.Anthropic.APIKey)
	}

	// Verify Groq
	if apiKeys.Groq == nil {
		t.Error("Expected Groq config to be loaded")
	} else if apiKeys.Groq.APIKey != "gsk-test-key" {
		t.Errorf("Expected Groq key 'gsk-test-key', got '%s'", apiKeys.Groq.APIKey)
	}
}

func TestGetAPIKey_Success(t *testing.T) {
	apiKeys := &APIKeys{
		XAI:       &XAIConfig{APIKey: "xai-key"},
		OpenAI:    &OpenAIConfig{APIKey: "openai-key"},
		Anthropic: &AnthropicConfig{APIKey: "anthropic-key"},
		Groq:      &GroqConfig{APIKey: "groq-key"},
		Gemini:    &GeminiConfig{APIKey: "gemini-key"},
		Mistral:   &MistralConfig{APIKey: "mistral-key"},
		Cohere:    &CohereConfig{APIKey: "cohere-key"},
		Azure:     &AzureConfig{APIKey: "azure-key"},
	}

	tests := []struct {
		provider LLMProviderType
		expected string
	}{
		{ProviderXAI, "xai-key"},
		{ProviderOpenAI, "openai-key"},
		{ProviderAnthropic, "anthropic-key"},
		{ProviderGroq, "groq-key"},
		{ProviderGemini, "gemini-key"},
		{ProviderMistral, "mistral-key"},
		{ProviderCohere, "cohere-key"},
		{ProviderAzure, "azure-key"},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			key, err := apiKeys.GetAPIKey(tt.provider)
			if err != nil {
				t.Errorf("Expected no error, got: %v", err)
			}
			if key != tt.expected {
				t.Errorf("Expected key '%s', got '%s'", tt.expected, key)
			}
		})
	}
}

func TestGetAPIKey_NotConfigured(t *testing.T) {
	apiKeys := &APIKeys{} // Empty config

	tests := []LLMProviderType{
		ProviderXAI,
		ProviderOpenAI,
		ProviderAnthropic,
		ProviderGroq,
	}

	for _, provider := range tests {
		t.Run(string(provider), func(t *testing.T) {
			_, err := apiKeys.GetAPIKey(provider)
			if err == nil {
				t.Error("Expected error for unconfigured provider")
			}
		})
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard key",
			input:    "a977c8417a45457a83a897de82e4215b.lnHprFLE4TikOOjX",
			expected: "a977...OOjX",
		},
		{
			name:     "short key",
			input:    "short",
			expected: "***",
		},
		{
			name:     "exactly 8 chars",
			input:    "12345678",
			expected: "***",
		},
		{
			name:     "9 chars",
			input:    "123456789",
			expected: "1234...6789",
		},
		{
			name:     "openai key",
			input:    "sk-proj-abcdefghijklmnopqrstuvwxyz",
			expected: "sk-p...wxyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskAPIKey(tt.input)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestSanitizeForLogging(t *testing.T) {
	apiKeys := &APIKeys{
		XAI:       &XAIConfig{APIKey: "xai-secret-key-12345"},
		OpenAI:    &OpenAIConfig{APIKey: "sk-openai-secret"},
		Anthropic: &AnthropicConfig{APIKey: "sk-ant-secret"},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "text with xAI key",
			input:    "Using API key xai-secret-key-12345 for request",
			expected: "Using API key xai-...2345 for request",
		},
		{
			name:     "text with OpenAI key",
			input:    "Authorization: Bearer sk-openai-secret",
			expected: "Authorization: Bearer sk-o...cret",
		},
		{
			name:     "text with multiple keys",
			input:    "xAI: xai-secret-key-12345, OpenAI: sk-openai-secret",
			expected: "xAI: xai-...2345, OpenAI: sk-o...cret",
		},
		{
			name:     "text without keys",
			input:    "No API keys in this text",
			expected: "No API keys in this text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeForLogging(tt.input, apiKeys)
			if result != tt.expected {
				t.Errorf("Expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestIsCloudProvider(t *testing.T) {
	tests := []struct {
		provider LLMProviderType
		isCloud  bool
	}{
		{ProviderXAI, true},
		{ProviderOpenAI, true},
		{ProviderAnthropic, true},
		{ProviderGroq, true},
		{ProviderGemini, true},
		{ProviderMistral, true},
		{ProviderCohere, true},
		{ProviderAzure, true},
		{ProviderBedrock, true},
		{ProviderVertexAI, true},
		{ProviderOpenRouter, true},
		{ProviderOllama, false},
		{ProviderLlamaCpp, false},
		{ProviderVLLM, false},
		{ProviderLocalAI, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			result := IsCloudProvider(tt.provider)
			if result != tt.isCloud {
				t.Errorf("Expected IsCloudProvider(%s) = %v, got %v",
					tt.provider, tt.isCloud, result)
			}
		})
	}
}

func TestLoadAPIKeys_InvalidYAML(t *testing.T) {
	// Create temporary invalid YAML file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `
xai:
  api_key: "key1"
  invalid yaml here: [}
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Should return error for invalid YAML
	_, err = LoadAPIKeys(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoadAPIKeys_PartialConfig(t *testing.T) {
	// Create config with only some providers
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "partial.yaml")

	partialConfig := `
xai:
  api_key: "xai-only-key"
# No other providers configured
`

	err := os.WriteFile(configPath, []byte(partialConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	apiKeys, err := LoadAPIKeys(configPath)
	if err != nil {
		t.Fatalf("Failed to load partial config: %v", err)
	}

	// XAI should be configured
	if apiKeys.XAI == nil {
		t.Error("Expected XAI to be configured")
	}

	// Others should be nil
	if apiKeys.OpenAI != nil {
		t.Error("Expected OpenAI to be nil")
	}
	if apiKeys.Anthropic != nil {
		t.Error("Expected Anthropic to be nil")
	}

	// Getting unconfigured provider should return error
	_, err = apiKeys.GetAPIKey(ProviderOpenAI)
	if err == nil {
		t.Error("Expected error for unconfigured OpenAI")
	}
}
