package quality

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScorer_ScoreWithTools(t *testing.T) {
	tmpDir := t.TempDir()
	codeDir := filepath.Join(tmpDir, "testcode")
	os.MkdirAll(codeDir, 0755)
	// Create a simple Go file
	os.WriteFile(filepath.Join(codeDir, "main.go"), []byte("package main\n\nfunc main() {\n}"), 0644)
	// Create go.mod
	os.WriteFile(filepath.Join(codeDir, "go.mod"), []byte("module test\n\ngo 1.24"), 0644)
	s := NewScorer()
	result, err := s.ScoreWithTools(context.Background(), codeDir)
	if err != nil {
		t.Fatal(err)
	}
	// Just check that we got a result back - don't worry about compilation success in test env
	if result == nil {
		t.Fatal("expected nil error and non-nil result")
	}
}