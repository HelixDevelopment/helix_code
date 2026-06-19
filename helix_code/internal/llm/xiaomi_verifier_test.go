package llm

import (
	"testing"
)

func TestXiaomiIsNativelyWired(t *testing.T) {
	if !dynamicNativelyWiredProviders["xiaomi"] {
		t.Fatal("xiaomi should be in dynamicNativelyWiredProviders to prevent double-registration")
	}
}
