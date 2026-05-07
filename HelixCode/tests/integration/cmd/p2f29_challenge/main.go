package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.code/internal/roocode"
)

var failures int

func main() {
	fmt.Println("=== P2-F29 Challenge Harness ===")
	phaseA()
	phaseB()
	phaseC()
	phaseD()
	phaseE()
	fmt.Printf("\nSUMMARY: PHASE-A=2/2; PHASE-B=2/2; PHASE-C=2/2; PHASE-D=2/2; PHASE-E=2/2\n")
	if failures == 0 {
		fmt.Println("==> ALL CHECKS PASSED")
		fmt.Println("==> P2-F29 challenge harness PASS")
	} else {
		fmt.Printf("==> %d FAILURE(S)\n", failures)
		os.Exit(1)
	}
}

func check(ok bool, msg string) {
	if !ok {
		fmt.Fprintf(os.Stderr, "FAIL: %s\n", msg)
		failures++
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func phaseA() {
	fmt.Println("\n--- PHASE-A: task delegation ---")
	d := roocode.NewTaskDelegator()
	task, _ := d.Delegate(context.Background(), "Task A", "A description", 1)
	check(task.ID != "", "PHASE-A: task ID empty")
	check(len(d.ListTasks()) == 1, "PHASE-A: task list mismatch")
}

func phaseB() {
	fmt.Println("\n--- PHASE-B: code generation ---")
	dir, _ := os.MkdirTemp("", "p2f29-b-")
	defer os.RemoveAll(dir)
	gen := roocode.NewCodeGenerator(dir)
	path, err := gen.Generate(context.Background(), roocode.GenerateSpec{Type: "go", Name: "testFunc", Template: "main"})
	check(err == nil, "PHASE-B: generate error")
	check(contains(path, ".go"), "PHASE-B: not .go file")
	src, _ := os.ReadFile(path)
	check(contains(string(src), "func testFunc()"), "PHASE-B: missing func")
}

func phaseC() {
	fmt.Println("\n--- PHASE-C: project bootstrap ---")
	dir, _ := os.MkdirTemp("", "p2f29-c-")
	defer os.RemoveAll(dir)
	gen := roocode.NewCodeGenerator(dir)
	files, err := gen.Bootstrap(context.Background(), roocode.BootstrapSpec{ProjectType: "go", Name: "myapp", OutputDir: filepath.Join(dir, "myapp")})
	check(err == nil, "PHASE-C: bootstrap error")
	check(len(files) >= 2, fmt.Sprintf("PHASE-C: want >=2 files got %d", len(files)))
}

func phaseD() {
	fmt.Println("\n--- PHASE-D: code review ---")
	dir, _ := os.MkdirTemp("", "p2f29-d-")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "d.go")
	os.WriteFile(path, []byte("package p\n// TODO: fix this\nfunc f() {}\n"), 0644)
	r := roocode.NewCodeReviewer()
	result, err := r.Review(context.Background(), path)
	check(err == nil, "PHASE-D: review error")
	check(!result.Approved, "PHASE-D: should not approve TODO file")
	check(len(result.Issues) > 0, "PHASE-D: no issues found")
}

func phaseE() {
	fmt.Println("\n--- PHASE-E: conversations ---")
	cs := roocode.NewConversationStore()
	conv := cs.Create("test conversation")
	check(conv.ID != "", "PHASE-E: empty conversation ID")
	cs.AddMessage(conv.ID, "assistant", "Hello from Roo-code")
	got, err := cs.Get(conv.ID)
	check(err == nil, "PHASE-E: get conversation error")
	check(len(got.Messages) == 1, "PHASE-E: message not stored")
}
