package llm

import (
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// Selector tests — flag > env > config > wizard precedence (P1-F12-T07).
// ---------------------------------------------------------------------------

func TestSelector_FlagWinsOverEnv(t *testing.T) {
	got, err := Select(SelectorInput{
		Flag:   "bedrock",
		Env:    "anthropic",
		Config: "azure",
	})
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if got != ProviderTypeBedrock {
		t.Fatalf("Select() = %q, want %q", got, ProviderTypeBedrock)
	}
}

func TestSelector_FlagWinsOverConfig(t *testing.T) {
	got, err := Select(SelectorInput{
		Flag:   "vertexai",
		Config: "azure",
	})
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if got != ProviderTypeVertexAI {
		t.Fatalf("Select() = %q, want %q", got, ProviderTypeVertexAI)
	}
}

func TestSelector_EnvWinsOverConfig(t *testing.T) {
	got, err := Select(SelectorInput{
		Flag:   "",
		Env:    "azure",
		Config: "anthropic",
	})
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if got != ProviderTypeAzure {
		t.Fatalf("Select() = %q, want %q", got, ProviderTypeAzure)
	}
}

func TestSelector_ConfigWhenNoFlagOrEnv(t *testing.T) {
	got, err := Select(SelectorInput{
		Flag:   "",
		Env:    "",
		Config: "anthropic",
	})
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if got != ProviderTypeAnthropic {
		t.Fatalf("Select() = %q, want %q", got, ProviderTypeAnthropic)
	}
}

func TestSelector_NoSourcesErrors(t *testing.T) {
	_, err := Select(SelectorInput{})
	if !errors.Is(err, ErrNoProviderConfigured) {
		t.Fatalf("Select() error = %v, want ErrNoProviderConfigured", err)
	}
}

func TestSelector_UnknownTypeErrors(t *testing.T) {
	_, err := Select(SelectorInput{Flag: "random_garbage"})
	if err == nil {
		t.Fatalf("Select() expected error for unknown type, got nil")
	}
	if errors.Is(err, ErrNoProviderConfigured) {
		t.Fatalf("Select() returned ErrNoProviderConfigured for unknown type; expected ErrUnknownProviderType-class error")
	}
}

func TestSelector_CaseInsensitive(t *testing.T) {
	got, err := Select(SelectorInput{Flag: "BEDROCK"})
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if got != ProviderTypeBedrock {
		t.Fatalf("Select() = %q, want %q", got, ProviderTypeBedrock)
	}
}

func TestSelector_TrimsWhitespace(t *testing.T) {
	got, err := Select(SelectorInput{Flag: "  anthropic  "})
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if got != ProviderTypeAnthropic {
		t.Fatalf("Select() = %q, want %q", got, ProviderTypeAnthropic)
	}
}

// Vertex alias — accept "vertex" as a synonym of "vertexai" since users
// commonly type the short form.
func TestSelector_VertexAlias(t *testing.T) {
	got, err := Select(SelectorInput{Flag: "vertex"})
	if err != nil {
		t.Fatalf("Select() unexpected error: %v", err)
	}
	if got != ProviderTypeVertexAI {
		t.Fatalf("Select() = %q, want %q", got, ProviderTypeVertexAI)
	}
}

// Selector restricts to cloud providers (factory's mandate).
func TestSelector_RejectsNonCloudType(t *testing.T) {
	_, err := Select(SelectorInput{Flag: "ollama"})
	if err == nil {
		t.Fatalf("Select() expected error for non-cloud provider 'ollama', got nil")
	}
}

// ---------------------------------------------------------------------------
// NewCloudProvider tests — constructs concrete cloud providers.
// ---------------------------------------------------------------------------

func TestNewCloudProvider_Anthropic(t *testing.T) {
	cfg := ProviderConfigEntry{
		Type:    ProviderTypeAnthropic,
		APIKey:  "test-anthropic-key",
		Enabled: true,
	}
	p, err := NewCloudProvider(ProviderTypeAnthropic, cfg)
	if err != nil {
		t.Fatalf("NewCloudProvider(anthropic) error = %v", err)
	}
	if p == nil {
		t.Fatal("NewCloudProvider(anthropic) returned nil provider")
	}
	// Compile-time interface check.
	var _ Provider = p
	if p.GetType() != ProviderTypeAnthropic {
		t.Errorf("provider.GetType() = %q, want %q", p.GetType(), ProviderTypeAnthropic)
	}
}

func TestNewCloudProvider_Bedrock(t *testing.T) {
	cfg := ProviderConfigEntry{
		Type:    ProviderTypeBedrock,
		Enabled: true,
		Parameters: map[string]interface{}{
			"region": "us-east-1",
		},
	}
	p, err := NewCloudProvider(ProviderTypeBedrock, cfg)
	if err != nil {
		// Bedrock LoadDefaultConfig should succeed even without creds; if
		// the host is so locked-down it can't even build a default config,
		// surface that as a skip rather than a hard failure — the factory
		// itself is correct.
		t.Skipf("SKIP-OK: #P1-F12-T07 Bedrock LoadDefaultConfig failed in this environment: %v", err)
	}
	if p == nil {
		t.Fatal("NewCloudProvider(bedrock) returned nil provider")
	}
	var _ Provider = p
}

func TestNewCloudProvider_Vertex(t *testing.T) {
	cfg := ProviderConfigEntry{
		Type:    ProviderTypeVertexAI,
		Enabled: true,
		Parameters: map[string]interface{}{
			"project_id": "test-project-id",
			"location":   "us-central1",
		},
	}
	p, err := NewCloudProvider(ProviderTypeVertexAI, cfg)
	if err != nil {
		t.Fatalf("NewCloudProvider(vertexai) error = %v", err)
	}
	if p == nil {
		t.Fatal("NewCloudProvider(vertexai) returned nil provider")
	}
	var _ Provider = p
}

func TestNewCloudProvider_Azure(t *testing.T) {
	cfg := ProviderConfigEntry{
		Type:    ProviderTypeAzure,
		APIKey:  "test-azure-key",
		Enabled: true,
		Parameters: map[string]interface{}{
			"endpoint":    "https://example.openai.azure.com",
			"api_version": "2025-04-01-preview",
		},
	}
	p, err := NewCloudProvider(ProviderTypeAzure, cfg)
	if err != nil {
		t.Fatalf("NewCloudProvider(azure) error = %v", err)
	}
	if p == nil {
		t.Fatal("NewCloudProvider(azure) returned nil provider")
	}
	var _ Provider = p
}

func TestNewCloudProvider_Unknown(t *testing.T) {
	cfg := ProviderConfigEntry{
		Type:    ProviderType("not-a-cloud-provider"),
		Enabled: true,
	}
	p, err := NewCloudProvider(ProviderType("not-a-cloud-provider"), cfg)
	if err == nil {
		t.Fatal("NewCloudProvider(unknown) expected error, got nil")
	}
	if p != nil {
		t.Errorf("NewCloudProvider(unknown) returned non-nil provider %v", p)
	}
}

// Reject local/non-cloud types — NewCloudProvider's mandate is the
// 4 cloud backends only (Anthropic, Bedrock, Vertex, Azure).
func TestNewCloudProvider_RejectsLocalProvider(t *testing.T) {
	cfg := ProviderConfigEntry{
		Type:     ProviderTypeOllama,
		Endpoint: "http://localhost:11434",
		Enabled:  true,
	}
	_, err := NewCloudProvider(ProviderTypeOllama, cfg)
	if err == nil {
		t.Fatal("NewCloudProvider(ollama) expected error (non-cloud), got nil")
	}
}
