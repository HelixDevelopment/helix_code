package llm

import (
	"os"
	"testing"
)

func TestXiaomiKeyRecognition_Aliases(t *testing.T) {
	aliases := ProviderEnvAliases()
	xiaomiAliases, ok := aliases[ProviderTypeXiaomi]
	if !ok {
		t.Fatal("ProviderTypeXiaomi not found in ProviderEnvAliases")
	}
	expected := []string{"XIAOMI_MIMO_API_KEY", "ApiKey_Xiaomi_MiMo"}
	if len(xiaomiAliases) != len(expected) {
		t.Fatalf("expected %d aliases, got %d", len(expected), len(xiaomiAliases))
	}
	for i, a := range expected {
		if xiaomiAliases[i] != a {
			t.Errorf("alias[%d] = %q, want %q", i, xiaomiAliases[i], a)
		}
	}
}

func TestXiaomiKeyRecognition_Present(t *testing.T) {
	os.Setenv("XIAOMI_MIMO_API_KEY", "sk-test123")
	defer os.Unsetenv("XIAOMI_MIMO_API_KEY")

	present := PresentProviders()
	if !present[ProviderTypeXiaomi] {
		t.Fatal("Xiaomi should be present when XIAOMI_MIMO_API_KEY is set")
	}
}

func TestXiaomiKeyRecognition_Absent(t *testing.T) {
	os.Unsetenv("XIAOMI_MIMO_API_KEY")
	os.Unsetenv("ApiKey_Xiaomi_MiMo")

	present := PresentProviders()
	if present[ProviderTypeXiaomi] {
		t.Fatal("Xiaomi should NOT be present when no key is set")
	}
}
