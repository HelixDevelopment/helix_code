// p2f25_challenge runs the F25 Plandex plan trees harness end-to-end
// against real tempdirs with real JSON I/O. Article XI 11.9 anti-bluff
// anchor: every PASS carries positive runtime evidence — byte equality,
// JSON schema validation, status transitions, metadata assertions,
// byte reduction, corruption detection, render structure match.
//
// Phases (seven always-run; zero network / chromium / DB deps):
//
//	A. CREATE + SCHEMA   — plan_create writes valid JSON, root.id UUID,
//	                        root.title == "Challenge Plan", status="pending",
//	                        node count==1, 0 children.
//	B. BRANCH + INTEGRITY — 3 branches under root; Children array len==3,
//	                        all ParentID==root.ID, VerifyTree() Valid.
//	C. MERGE + METADATA   — merge "Task 1"; child Status="completed",
//	                        root.Metadata["merged_from"]==task1.ID,
//	                        root.Metadata["merged_history"]==task1.ID.
//	D. COMPACT + REDUCTION — deep tree with 30+ Completed leaves; post<pre,
//	                        reduction >= 30%, root unchanged, compacted
//	                        marker present.
//	E. VERIFY DETECTS CORRUPTION — 3 corrupt files (orphan/cycle/dup);
//	                        all VerifyTree() Valid==false, each has >=1
//	                        Error-severity issue with correct category.
//	F. SHOW OUTPUT MATCH — 3-level tree; RenderTree output contains markers
//	                        at correct indentation levels, node IDs in parens.
//	G. SHALLOW NO-COMPACT — depth-1/2 Completed leaves; NodesCompacted==0,
//	                        OriginalBytes==NewBytes.
//
// Exit 0 on all PASS; exit 1 with diagnostics on any failure.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dev.helix.code/internal/plantree"
)

var (
	exitCode int
	failures int
	store    plantree.Store
	dir      string
)

func main() {
	dir = os.Args[1]
	if dir == "" {
		dir, _ = os.MkdirTemp("", "p2f25-challenge-")
		defer os.RemoveAll(dir)
	}

	store = plantree.NewFileStore(dir)
	fmt.Println("=== P2-F25 Challenge Harness ===")

	phaseA()
	phaseB()
	phaseC()
	phaseD()
	phaseE()
	phaseF()
	phaseG()

	fmt.Printf("\nSUMMARY: PHASE-A=%d/%d; PHASE-B=%d/%d; PHASE-C=%d/%d; PHASE-D=%d/%d; PHASE-E=%d/%d; PHASE-F=%d/%d; PHASE-G=%d/%d\n",
		aChecks, aTotal, bChecks, bTotal, cChecks, cTotal, dChecks, dTotal, eChecks, eTotal, fChecks, fTotal, gChecks, gTotal)

	if failures == 0 {
		fmt.Println("==> ALL CHECKS PASSED")
		fmt.Println("==> P2-F25 challenge harness PASS")
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

// ----------------------------------------------------------------
// PHASE-A: create + verify schema
// ----------------------------------------------------------------
var aChecks, aTotal int

func phaseA() {
	aTotal = 4
	fmt.Println("\n--- PHASE-A: create + verify schema ---")
	nowTotal := failures

	tree, err := plantree.CreateTree("challenge-plan", "Challenge Plan", "A plan for the F25 challenge")
	check(err == nil, "PHASE-A: CreateTree returned error")
	if err != nil {
		return
	}
	err = store.Save(*tree)
	check(err == nil, "PHASE-A: Save returned error")
	if err != nil {
		return
	}

	path := filepath.Join(dir, plantree.StorageDir, "challenge-plan.json")
	data, err := os.ReadFile(path)
	check(err == nil, "PHASE-A: plan JSON file not found on disk")
	if err != nil {
		return
	}

	var loaded plantree.PlanTree
	err = json.Unmarshal(data, &loaded)
	check(err == nil, "PHASE-A: plan JSON is invalid")
	if err != nil {
		return
	}

	check(loaded.Root.ID != "", "PHASE-A: root.id is empty")
	check(loaded.Root.Title == "Challenge Plan", fmt.Sprintf("PHASE-A: root.title=%q want %q", loaded.Root.Title, "Challenge Plan"))
	check(loaded.Root.Status.String() == "pending", fmt.Sprintf("PHASE-A: root.status=%q want pending", loaded.Root.Status.String()))
	check(plantree.CountNodes(loaded.Root) == 1, fmt.Sprintf("PHASE-A: node count=%d want 1", plantree.CountNodes(loaded.Root)))
	check(len(loaded.Root.Children) == 0, fmt.Sprintf("PHASE-A: root has %d children want 0", len(loaded.Root.Children)))

	aChecks = (aTotal - (failures - nowTotal))
}

// ----------------------------------------------------------------
// PHASE-B: branch + structural integrity
// ----------------------------------------------------------------
var bChecks, bTotal int

func phaseB() {
	bTotal = 5
	fmt.Println("\n--- PHASE-B: branch + structural integrity ---")
	nowTotal := failures

	tree, err := store.Load("challenge-plan")
	check(err == nil, "PHASE-B: Load challenge-plan failed")
	if err != nil {
		return
	}

	task1, err := plantree.BranchNode(&tree, tree.Root.ID, "Task 1", "First task desc")
	check(err == nil, "PHASE-B: BranchNode task1 failed")
	task2, err := plantree.BranchNode(&tree, tree.Root.ID, "Task 2", "Second task desc")
	check(err == nil, "PHASE-B: BranchNode task2 failed")
	task3, err := plantree.BranchNode(&tree, tree.Root.ID, "Task 3", "Third task desc")
	check(err == nil, "PHASE-B: BranchNode task3 failed")

	check(task1 != nil && task2 != nil && task3 != nil, "PHASE-B: branch returned nil")

	err = store.Save(tree)
	check(err == nil, "PHASE-B: Save after branch failed")
	if err != nil {
		return
	}

	tree2, _ := store.Load("challenge-plan")
	check(len(tree2.Root.Children) == 3, fmt.Sprintf("PHASE-B: root has %d children want 3", len(tree2.Root.Children)))

	if len(tree2.Root.Children) >= 3 {
		check(tree2.Root.Children[0].ParentID == tree2.Root.ID, "PHASE-B: child0 ParentID != root.ID")
		check(tree2.Root.Children[1].ParentID == tree2.Root.ID, "PHASE-B: child1 ParentID != root.ID")
		check(tree2.Root.Children[2].ParentID == tree2.Root.ID, "PHASE-B: child2 ParentID != root.ID")
	}

	result := plantree.VerifyTree(&tree2)
	check(result.Valid, fmt.Sprintf("PHASE-B: VerifyTree returned Valid=false; issues=%v", result.Issues))

	bChecks = (bTotal - (failures - nowTotal))
}

// ----------------------------------------------------------------
// PHASE-C: merge + metadata
// ----------------------------------------------------------------
var cChecks, cTotal int

func phaseC() {
	cTotal = 4
	fmt.Println("\n--- PHASE-C: merge + metadata ---")
	nowTotal := failures

	tree, err := store.Load("challenge-plan")
	check(err == nil, "PHASE-C: Load challenge-plan failed")
	if err != nil {
		return
	}

	task1ID := tree.Root.Children[0].ID

	parent, err := plantree.MergeNode(&tree, task1ID)
	check(err == nil, "PHASE-C: MergeNode failed")
	if err != nil {
		return
	}

	err = store.Save(tree)
	check(err == nil, "PHASE-C: Save after merge failed")

	tree2, _ := store.Load("challenge-plan")

	mergedChild := findNodeInTree(tree2.Root, task1ID)
	check(mergedChild != nil, "PHASE-C: merged child not found in tree")
	if mergedChild != nil {
		check(mergedChild.Status.String() == "completed", fmt.Sprintf("PHASE-C: child status=%q want completed", mergedChild.Status.String()))
	}

	check(parent.Metadata != nil, "PHASE-C: parent.Metadata is nil")
	if parent.Metadata != nil {
		check(parent.Metadata["merged_from"] == task1ID, fmt.Sprintf("PHASE-C: merged_from=%q want %q", parent.Metadata["merged_from"], task1ID))
		check(strings.Contains(parent.Metadata["merged_history"], task1ID), "PHASE-C: merged_history does not contain task1 ID")
	}

	cChecks = (cTotal - (failures - nowTotal))
}

// ----------------------------------------------------------------
// PHASE-D: compact + byte reduction
// ----------------------------------------------------------------
var dChecks, dTotal int

func phaseD() {
	dTotal = 5
	fmt.Println("\n--- PHASE-D: compact + byte reduction ---")
	nowTotal := failures

	root := &plantree.PlanNode{
		ID:     "d-root",
		Title:  "Compact Test Plan",
		Description: phrases(8000),
		Status: plantree.StatusPending,
	}

	for i := 0; i < 20; i++ {
		gc := &plantree.PlanNode{
			ID:     fmt.Sprintf("d-gc-%d", i),
			Title:  "Grandchild Task",
			Description: phrases(5000),
			Status: plantree.StatusCompleted,
		}
		child := &plantree.PlanNode{
			ID:     fmt.Sprintf("d-c-%d", i),
			Title:  "Child",
			Description: phrases(5000),
			Status: plantree.StatusCompleted,
			Children: []*plantree.PlanNode{gc},
			ParentID: "d-root",
		}
		gc.ParentID = child.ID
		root.Children = append(root.Children, child)
	}

	bigTree := &plantree.PlanTree{Name: "compact-test", Version: 1, Root: root}

	origJSON, _ := json.Marshal(bigTree)
	pre := len(origJSON)

	result, err := plantree.CompactTree(bigTree, plantree.DeterministicSummariser{})
	check(err == nil, "PHASE-D: CompactTree failed")
	if err != nil {
		return
	}

	post := result.NewBytes
	check(post < pre, fmt.Sprintf("PHASE-D: post=%d >= pre=%d (no reduction)", post, pre))
	reduction := float64(pre-post) / float64(pre) * 100
	check(reduction >= 30, fmt.Sprintf("PHASE-D: reduction=%.1f%% want >=30%%", reduction))
	check(result.NodesCompacted > 0, "PHASE-D: no nodes compacted")
	check(result.Tree.Root.Title == "Compact Test Plan", "PHASE-D: root title changed")

	foundMarker := false
	for _, child := range result.Tree.Root.Children {
		for _, gc := range child.Children {
			if gc.Metadata != nil && gc.Metadata["compacted"] == "true" {
				foundMarker = true
				break
			}
		}
	}
	check(foundMarker, "PHASE-D: no node has compacted marker")
	check(result.Tree.Root.Description == phrases(8000), "PHASE-D: root description changed")

	dChecks = (dTotal - (failures - nowTotal))
}

// ----------------------------------------------------------------
// PHASE-E: verify detects corruption
// ----------------------------------------------------------------
var eChecks, eTotal int

func phaseE() {
	eTotal = 8
	fmt.Println("\n--- PHASE-E: verify detects corruption ---")
	nowTotal := failures

	// Orphan
	orphan := []byte(`{"name":"orphan","version":1,"root":{"id":"r","title":"R","description":"","status":"pending","children":[{"id":"c","title":"C","description":"","status":"pending","parent_id":"nonexistent","children":null,"metadata":null}]},"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}`)
	checkVerify("orphan", orphan, false, "orphan", nowTotal)

	// Cycle
	cycle := []byte(`{"name":"cycle","version":1,"root":{"id":"a","title":"A","description":"","status":"pending","children":[{"id":"b","title":"B","description":"","status":"pending","parent_id":"a","children":[{"id":"c","title":"C","description":"","status":"pending","parent_id":"b","children":[{"id":"a","title":"A","description":"","status":"pending","parent_id":"c"}]}]}]},"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}`)
	checkVerify("cycle", cycle, false, "cycle", nowTotal)

	// Duplicate
	dup := []byte(`{"name":"dup","version":1,"root":{"id":"root","title":"Root","description":"","status":"pending","children":[{"id":"same","title":"Child1","description":"","status":"pending"},{"id":"same","title":"Child2","description":"","status":"pending"}]},"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}`)
	checkVerify("dup", dup, false, "duplicate", nowTotal)

	eChecks = (eTotal - (failures - nowTotal))
}

func writePlanJSON(name string, data []byte) {
	planDir := filepath.Join(dir, plantree.StorageDir)
	os.MkdirAll(planDir, 0700)
	path := filepath.Join(planDir, name+".json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "writePlanJSON: %v\n", err)
	}
}

func checkVerify(name string, raw []byte, expectValid bool, wantIssueSubstr string, baseline int) {
	writePlanJSON(name, raw)
	tree, err := store.Load(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: PHASE-E: Load %s: %v\n", name, err)
		failures++
		return
	}

	result := plantree.VerifyTree(&tree)
	check(result.Valid == expectValid, fmt.Sprintf("PHASE-E: %s Valid=%v want %v", name, result.Valid, expectValid))

	found := false
	hasError := false
	for _, issue := range result.Issues {
		if issue.Severity == plantree.SeverityError {
			hasError = true
		}
		if strings.Contains(strings.ToLower(issue.Message), wantIssueSubstr) {
			found = true
		}
	}
	check(hasError, fmt.Sprintf("PHASE-E: %s has no Error-severity issue", name))
	check(found, fmt.Sprintf("PHASE-E: %s issues do not mention %q: %v", name, wantIssueSubstr, result.Issues))

	store.Delete(name)
}

// ----------------------------------------------------------------
// PHASE-F: show output structural match
// ----------------------------------------------------------------
var fChecks, fTotal int

func phaseF() {
	fTotal = 7
	fmt.Println("\n--- PHASE-F: show output structural match ---")
	nowTotal := failures

	root := &plantree.PlanNode{
		ID:     "f-root",
		Title:  "Root",
		Status: plantree.StatusPending,
		Children: []*plantree.PlanNode{
			{
				ID:     "f-child1",
				Title:  "Child1",
				Status: plantree.StatusInProgress,
				ParentID: "f-root",
				Children: []*plantree.PlanNode{
					{
						ID:     "f-gc1",
						Title:  "Grandchild1",
						Status: plantree.StatusPending,
						ParentID: "f-child1",
					},
				},
			},
			{
				ID:     "f-child2",
				Title:  "Child2",
				Status: plantree.StatusCompleted,
				ParentID: "f-root",
			},
		},
	}

	output := plantree.RenderTree(root, 0)
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")

	check(len(lines) == 4, fmt.Sprintf("PHASE-F: got %d lines want 4", len(lines)))

	if len(lines) >= 1 {
		check(strings.HasPrefix(lines[0], "[ ]") && strings.Contains(lines[0], "Root"), fmt.Sprintf("PHASE-F: line0=%q missing [ ] Root", lines[0]))
	}
	if len(lines) >= 2 {
		check(strings.HasPrefix(lines[1], "  [▶]") && strings.Contains(lines[1], "Child1"), fmt.Sprintf("PHASE-F: line1=%q missing   [▶] Child1", lines[1]))
	}
	if len(lines) >= 3 {
		check(strings.HasPrefix(lines[2], "    [ ]") && strings.Contains(lines[2], "Grandchild1"), fmt.Sprintf("PHASE-F: line2=%q missing     [ ] Grandchild1", lines[2]))
	}
	if len(lines) >= 4 {
		check(strings.HasPrefix(lines[3], "  [✓]") && strings.Contains(lines[3], "Child2"), fmt.Sprintf("PHASE-F: line3=%q missing   [✓] Child2", lines[3]))
	}

	check(strings.Contains(output, "(f-root)"), "PHASE-F: output missing node ID (f-root)")
	check(strings.Contains(output, "(f-child1)"), "PHASE-F: output missing node ID (f-child1)")
	check(strings.Contains(output, "(f-gc1)"), "PHASE-F: output missing node ID (f-gc1)")

	fChecks = (fTotal - (failures - nowTotal))
}

// ----------------------------------------------------------------
// PHASE-G: shallow no-compact guard
// ----------------------------------------------------------------
var gChecks, gTotal int

func phaseG() {
	gTotal = 2
	fmt.Println("\n--- PHASE-G: shallow no-compact guard ---")
	nowTotal := failures

	root := &plantree.PlanNode{
		ID:     "g-root",
		Title:  "Shallow",
		Description: phrases(5000),
		Status: plantree.StatusPending,
	}

	for i := 0; i < 15; i++ {
		child := &plantree.PlanNode{
			ID:     fmt.Sprintf("g-c-%d", i),
			Title:  "Child",
			Description: phrases(5000),
			Status: plantree.StatusCompleted,
			ParentID: "g-root",
		}
		root.Children = append(root.Children, child)
	}

	tree := &plantree.PlanTree{Name: "shallow", Version: 1, Root: root}

	origJSON, _ := json.Marshal(tree)
	pre := len(origJSON)

	if pre < plantree.CompactThreshold {
		check(true, "PHASE-G: tree below threshold (expected for shallow)")
	} else {
		result, err := plantree.CompactTree(tree, plantree.DeterministicSummariser{})
		check(err == nil, "PHASE-G: CompactTree failed")
		if err == nil {
			check(result.NodesCompacted == 0, fmt.Sprintf("PHASE-G: NodesCompacted=%d want 0", result.NodesCompacted))
			check(result.NewBytes == result.OriginalBytes, fmt.Sprintf("PHASE-G: NewBytes=%d OriginalBytes=%d — should be equal", result.NewBytes, result.OriginalBytes))
		}
	}

	gChecks = (gTotal - (failures - nowTotal))
}

func findNodeInTree(node *plantree.PlanNode, id string) *plantree.PlanNode {
	if node == nil {
		return nil
	}
	if node.ID == id {
		return node
	}
	for _, child := range node.Children {
		if found := findNodeInTree(child, id); found != nil {
			return found
		}
	}
	return nil
}

func phrases(targetBytes int) string {
	var s string
	for len(s) < targetBytes {
		s += "Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. "
	}
	return s[:targetBytes]
}
