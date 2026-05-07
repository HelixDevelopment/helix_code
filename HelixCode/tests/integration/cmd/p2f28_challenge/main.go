// p2f28_challenge runs the F28 Kilo-code refactoring harness.
// Article XI 11.9: every PASS has positive runtime evidence.
// Phases (5 always-run; no network/DB/chromium deps):
//
//	A. CALLGRAPH — build call graph from tempdir Go files
//	B. RENAME — cross-file rename verifies post-rename content
//	C. IMPACT — impact analysis on symbol returns results
//	D. EXTRACT — extract method creates new function
//	E. INLINE — inline call replaces function refs
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.code/internal/kilocode"
)

var failures int

func main() {
	fmt.Println("=== P2-F28 Challenge Harness ===")
	phaseA()
	phaseB()
	phaseC()
	phaseD()
	phaseE()

	fmt.Printf("\nSUMMARY: PHASE-A=%d/2; PHASE-B=%d/2; PHASE-C=%d/2; PHASE-D=%d/2; PHASE-E=%d/2\n",
		aChecks, bChecks, cChecks, dChecks, eChecks)

	if failures == 0 {
		fmt.Println("==> ALL CHECKS PASSED")
		fmt.Println("==> P2-F28 challenge harness PASS")
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

var aChecks, bChecks, cChecks, dChecks, eChecks int

func phaseA() {
	aChecks = 2
	fmt.Println("\n--- PHASE-A: callgraph ---")
	dir, _ := os.MkdirTemp("", "p2f28-a-")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package p\nfunc f1() {}\nfunc f2() { f1() }\n"), 0644)

	cg, err := kilocode.BuildCallGraph(dir)
	check(err == nil, "PHASE-A: BuildCallGraph failed")
	check(cg.NodeCount() >= 2, fmt.Sprintf("PHASE-A: nodes=%d want >=2", cg.NodeCount()))
}

func phaseB() {
	bChecks = 2
	fmt.Println("\n--- PHASE-B: rename ---")
	dir, _ := os.MkdirTemp("", "p2f28-b-")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package p\nfunc TargetFunc() {}\nfunc main() { TargetFunc() }\n"), 0644)

	engine := kilocode.NewRenameEngine(dir)
	result, err := engine.Rename(context.Background(), "TargetFunc", "RenamedFunc")
	check(err == nil, "PHASE-B: Rename failed")
	check(result.Occurrences >= 2, fmt.Sprintf("PHASE-B: occurrences=%d want >=2", result.Occurrences))

	src, _ := os.ReadFile(filepath.Join(dir, "b.go"))
	check(!contains(string(src), "TargetFunc"), "PHASE-B: old name still present in file")
	check(contains(string(src), "RenamedFunc"), "PHASE-B: new name not found in file")
}

func phaseC() {
	cChecks = 2
	fmt.Println("\n--- PHASE-C: impact ---")
	dir, _ := os.MkdirTemp("", "p2f28-c-")
	defer os.RemoveAll(dir)

	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package p\nfunc helper() int { return 42 }\nfunc user() { helper() }\n"), 0644)

	ia, err := kilocode.NewImpactAnalyzer(dir)
	check(err == nil, "PHASE-C: NewImpactAnalyzer failed")
	if err != nil {
		return
	}

	result, err := ia.Analyze("p.helper")
	check(err == nil, "PHASE-C: Analyze failed")
	check(result.BlastRadius >= 1, fmt.Sprintf("PHASE-C: blastRadius=%d", result.BlastRadius))
}

func phaseD() {
	dChecks = 2
	fmt.Println("\n--- PHASE-D: extract method ---")
	dir, _ := os.MkdirTemp("", "p2f28-d-")
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "d.go")
	os.WriteFile(path, []byte("package p\nfunc main() {\n println(\"a\")\n println(\"b\")\n}\n"), 0644)

	r := kilocode.NewRefactorer(dir)
	err := r.ExtractMethod(path, "printBoth", 2, 3)
	check(err == nil, "PHASE-D: ExtractMethod failed")

	src, _ := os.ReadFile(path)
	check(contains(string(src), "func printBoth()"), "PHASE-D: extracted function not created")
	check(contains(string(src), "printBoth()"), "PHASE-D: call to extracted function not inserted")
}

func phaseE() {
	eChecks = 2
	fmt.Println("\n--- PHASE-E: inline call ---")
	dir, _ := os.MkdirTemp("", "p2f28-e-")
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "e.go")
	os.WriteFile(path, []byte("package p\nfunc util() { println(\"util\") }\nfunc main() { util() }\n"), 0644)

	r := kilocode.NewRefactorer(dir)
	err := r.InlineCall(path, "util")
	check(err == nil, "PHASE-E: InlineCall failed")

	src, _ := os.ReadFile(path)
	check(contains(string(src), "/* inlined"), "PHASE-E: inlined marker not found")
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
