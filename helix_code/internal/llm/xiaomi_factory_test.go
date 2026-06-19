package llm

import (
	"testing"
)

func TestFactory_CreatesXiaomiProvider(t *testing.T) {
	config := ProviderConfigEntry{
		Type:    ProviderTypeXiaomi,
		APIKey:  "sk-test123",
		Enabled: true,
		Models:  []string{"mimo-v2.5"},
	}
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}
	if provider.GetType() != ProviderTypeXiaomi {
		t.Errorf("GetType() = %q, want %q", provider.GetType(), ProviderTypeXiaomi)
	}
}
