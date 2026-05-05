//go:build !linux

package sandbox

import (
	"context"
	"strings"
	"testing"
)

func TestNativeBackend_NonLinux_Kind(t *testing.T) {
	n, err := NewNativeBackend("/work")
	if err != nil {
		t.Fatalf("NewNativeBackend: %v", err)
	}
	if got := n.Kind(); got != BackendNative {
		t.Fatalf("Kind() = %v, want %v", got, BackendNative)
	}
}

func TestNativeBackend_NonLinux_Run_Errors(t *testing.T) {
	n, err := NewNativeBackend("/work")
	if err != nil {
		t.Fatalf("NewNativeBackend: %v", err)
	}
	_, err = n.Run(context.Background(), "true", DefaultSandboxPolicy())
	if err == nil {
		t.Fatalf("Run on non-Linux must return an unavailability error")
	}
	if !strings.Contains(err.Error(), "non-Linux") {
		t.Fatalf("error must mention non-Linux unavailability; got: %v", err)
	}
}

func TestIsHelperInvocation_NonLinux_AlwaysFalse(t *testing.T) {
	if IsHelperInvocation() {
		t.Fatalf("IsHelperInvocation() must be false on non-Linux")
	}
}
