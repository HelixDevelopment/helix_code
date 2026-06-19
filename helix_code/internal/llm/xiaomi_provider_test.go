package llm

import (
	"testing"
)

func TestNewXiaomiProvider_WithKey(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXiaomi,
		APIKey:  "sk-test123",
		Enabled: true,
		Models:  []string{"mimo-v2.5"},
	}
	provider, err := NewXiaomiProvider(config)
	if err != nil {
		t.Fatalf("NewXiaomiProvider failed: %v", err)
	}
	if provider == nil {
		t.Fatal("provider is nil")
	}
	if provider.GetType() != ProviderTypeXiaomi {
		t.Errorf("GetType() = %q, want %q", provider.GetType(), ProviderTypeXiaomi)
	}
	if provider.GetName() != "xiaomi" {
		t.Errorf("GetName() = %q, want %q", provider.GetName(), "xiaomi")
	}
}

func TestNewXiaomiProvider_DefaultBaseURL(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXiaomi,
		APIKey:  "sk-test123",
		Enabled: true,
	}
	provider, err := NewXiaomiProvider(config)
	if err != nil {
		t.Fatalf("NewXiaomiProvider failed: %v", err)
	}
	if provider.baseURL != "https://api.xiaomimimo.com/v1" {
		t.Errorf("baseURL = %q, want %q", provider.baseURL, "https://api.xiaomimimo.com/v1")
	}
}

func TestXiaomiProvider_Models(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXiaomi,
		APIKey:  "sk-test123",
		Enabled: true,
	}
	provider, err := NewXiaomiProvider(config)
	if err != nil {
		t.Fatalf("NewXiaomiProvider failed: %v", err)
	}
	models := provider.GetModels()
	if len(models) == 0 {
		t.Fatal("expected at least 1 model from seed list")
	}
	found := false
	for _, m := range models {
		if m.Name == "mimo-v2.5" {
			found = true
			if m.ContextSize != 1000000 {
				t.Errorf("mimo-v2.5 ContextSize = %d, want 1000000", m.ContextSize)
			}
			if m.MaxTokens != 128000 {
				t.Errorf("mimo-v2.5 MaxTokens = %d, want 128000", m.MaxTokens)
			}
			break
		}
	}
	if !found {
		t.Fatal("mimo-v2.5 not found in models")
	}
}

func TestXiaomiProvider_Capabilities(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXiaomi,
		APIKey:  "sk-test123",
		Enabled: true,
	}
	provider, err := NewXiaomiProvider(config)
	if err != nil {
		t.Fatalf("NewXiaomiProvider failed: %v", err)
	}
	caps := provider.GetCapabilities()
	if len(caps) == 0 {
		t.Fatal("expected at least 1 capability")
	}
	foundText := false
	for _, c := range caps {
		if c == CapabilityTextGeneration {
			foundText = true
			break
		}
	}
	if !foundText {
		t.Error("expected CapabilityTextGeneration in capabilities")
	}
}

func TestXiaomiProvider_ContextWindow(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXiaomi,
		APIKey:  "sk-test123",
		Enabled: true,
	}
	provider, err := NewXiaomiProvider(config)
	if err != nil {
		t.Fatalf("NewXiaomiProvider failed: %v", err)
	}
	ctx := provider.GetContextWindow()
	if ctx < 256000 {
		t.Errorf("GetContextWindow() = %d, want >= 256000", ctx)
	}
}
