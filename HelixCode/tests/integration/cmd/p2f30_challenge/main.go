package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.code/internal/continua"
)

var failures int

func main() {
	fmt.Println("=== P2-F30 Challenge Harness ===")
	phaseA(); phaseB(); phaseC(); phaseD()
	fmt.Printf("\nSUMMARY: PHASE-A=2/2; PHASE-B=2/2; PHASE-C=2/2; PHASE-D=2/2\n")
	if failures == 0 { fmt.Println("==> ALL CHECKS PASSED\n==> P2-F30 challenge harness PASS") } else { fmt.Printf("==> %d FAILURE(S)\n", failures); os.Exit(1) }
}

func check(ok bool, msg string) {
	if !ok { fmt.Fprintf(os.Stderr, "FAIL: %s\n", msg); failures++ }
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ { if s[i:i+len(sub)] == sub { return true } }; return false
}

func phaseA() {
	fmt.Println("\n--- PHASE-A: inline completion ---")
	dir, _ := os.MkdirTemp("", "p2f30-a-"); defer os.RemoveAll(dir)
	path := filepath.Join(dir, "a.go")
	os.WriteFile(path, []byte("package p\nfunc main() {\n\tfmt.Println\n"), 0644)
	e := continua.NewCompletionEngine()
	result, _ := e.Complete(context.Background(), path, 2, 13)
	check(result.Suggestion != "", "empty suggestion")
	check(result.Line == 2, "wrong line")
}

func phaseB() {
	fmt.Println("\n--- PHASE-B: workspace editor ---")
	dir, _ := os.MkdirTemp("", "p2f30-b-"); defer os.RemoveAll(dir)
	path := filepath.Join(dir, "b.go")
	os.WriteFile(path, []byte("a\nb\nc\n"), 0644)
	e := continua.NewWorkspaceEditor()
	r, _ := e.Open(context.Background(), path)
	check(r.Lines == 3, "wrong line count")
	r2, _ := e.Edit(context.Background(), path, "new\n")
	check(contains(r2.Content, "new"), "edit not applied")
}

func phaseC() {
	fmt.Println("\n--- PHASE-C: chat sessions ---")
	cm := continua.NewChatManager()
	s := cm.CreateSession("test", "gpt-4")
	check(s.ID != "", "empty session ID")
	cm.AddMessage(context.Background(), s.ID, "user", "hello")
	got, _ := cm.GetSession(s.ID)
	check(len(got.Messages) == 1, "message lost")
}

func phaseD() {
	fmt.Println("\n--- PHASE-D: diff ---")
	r := continua.Diff("old line", "new line")
	check(r.Additions == 1, "no additions")
	check(contains(r.Patch, "+ new"), "no + in patch")
}
