package plantree

// Standing regression guard (§11.4.135) for the wouldCycle infinite-loop
// defect (operations.go wouldCycle).
//
// Defect: wouldCycle walks the ParentID chain upward without a visited
// guard. A malformed tree (e.g. loaded from corrupt JSON via FileStore.Load,
// which performs no structural validation) can contain a ParentID cycle that
// does NOT include the descendant being checked. In that case the original
// loop never terminated — an infinite-loop / hang reachable through
// MergeNode and, transitively, PlanMergeTool.Execute.
//
// Counterexample (structural tree root->p1->p2->c, with corrupt ParentID
// pointers p1.ParentID="p2", p2.ParentID="p1", c.ParentID="p1"):
// MergeNode(tree,"c") -> wouldCycle(root, "p1", "c") walks p1->p2->p1->...
// forever because neither p1 nor p2 ever equals descendant "c".
//
// §11.4.115 polarity switch via RED_MODE:
//   RED_MODE=1 -> reproduce the defect against the PRE-FIX (unbounded) logic
//                 inlined below; asserts it hangs (PASSES on the broken logic).
//   RED_MODE=0 (default) -> drive the REAL, fixed wouldCycle; asserts it
//                 terminates quickly (the standing GREEN regression guard).

import (
	"os"
	"testing"
	"time"
)

// corruptParentIDCycleTree builds the reproduction fixture: a tree whose
// ParentID pointers form a p1<->p2 cycle that EXCLUDES leaf "c".
func corruptParentIDCycleTree() *PlanTree {
	root := &PlanNode{ID: "root", Title: "R"}
	p1 := &PlanNode{ID: "p1", ParentID: "p2"}
	p2 := &PlanNode{ID: "p2", ParentID: "p1"}
	c := &PlanNode{ID: "c", ParentID: "p1"}
	root.Children = []*PlanNode{p1}
	p1.Children = []*PlanNode{p2}
	p2.Children = []*PlanNode{c}
	return &PlanTree{Name: "t", Root: root}
}

// wouldCyclePreFix is the verbatim PRE-FIX implementation (no visited guard).
// Used ONLY by the RED_MODE=1 reproduction to prove the defect was real.
func wouldCyclePreFix(node *PlanNode, ancestorID, descendantID string) bool {
	current := findNode(node, ancestorID)
	for current != nil && current.ParentID != "" {
		if current.ParentID == descendantID {
			return true
		}
		current = findNode(node, current.ParentID)
	}
	return false
}

func TestGuard_WouldCycle_NoInfiniteLoopOnCorruptParentChain(t *testing.T) {
	tree := corruptParentIDCycleTree()
	// Mirror MergeNode's call shape: child=c, child.ParentID="p1"=parent,
	// so wouldCycle is invoked with ancestorID="p1", descendantID="c".
	const ancestorID, descendantID = "p1", "c"

	red := os.Getenv("RED_MODE") == "1"

	done := make(chan bool, 1)
	go func() {
		if red {
			// PRE-FIX logic — expected to hang on the corrupt chain.
			_ = wouldCyclePreFix(tree.Root, ancestorID, descendantID)
		} else {
			// REAL fixed logic — expected to terminate.
			_ = wouldCycle(tree.Root, ancestorID, descendantID)
		}
		done <- true
	}()

	select {
	case <-done:
		if red {
			t.Fatalf("RED_MODE=1: pre-fix wouldCycle terminated, expected it to HANG on corrupt ParentID cycle — defect not reproduced")
		}
		// GREEN guard: fixed code terminated as required.
	case <-time.After(2 * time.Second):
		if red {
			// GREEN-in-RED: defect reproduced (pre-fix logic hung). Leaking
			// goroutine is acceptable for the bounded reproduction run.
			t.Logf("RED_MODE=1: defect reproduced — pre-fix wouldCycle hung as expected")
			return
		}
		t.Fatalf("fixed wouldCycle did NOT terminate within 2s on corrupt ParentID cycle — regression: the infinite-loop bug is back")
	}
}

// End-to-end guard at the MergeNode boundary (the actual reachable surface):
// MergeNode on the corrupt tree must return quickly, never hang.
func TestGuard_MergeNode_TerminatesOnCorruptTree(t *testing.T) {
	tree := corruptParentIDCycleTree()
	done := make(chan struct{}, 1)
	go func() {
		_, _ = MergeNode(tree, "c")
		close(done)
	}()
	select {
	case <-done:
		// Required: MergeNode returns (cycle is detected and reported, or
		// it completes) without hanging.
	case <-time.After(2 * time.Second):
		t.Fatalf("MergeNode hung on a tree with a corrupt ParentID cycle — infinite-loop regression in wouldCycle")
	}
}
