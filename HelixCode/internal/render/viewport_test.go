package render

import (
	"reflect"
	"testing"
)

func TestNewViewport_BlockID(t *testing.T) {
	v := NewViewport("blk-1")
	if got := v.BlockID(); got != "blk-1" {
		t.Fatalf("BlockID() = %q, want %q", got, "blk-1")
	}
	if got := v.LineCount(); got != 0 {
		t.Fatalf("initial LineCount() = %d, want 0", got)
	}
	if got := v.Lines(); len(got) != 0 {
		t.Fatalf("initial Lines() len = %d, want 0", len(got))
	}
}

func TestViewport_Apply_FirstFrame_AppendsAll(t *testing.T) {
	v := NewViewport("blk")
	d := v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "b", "c"}})
	if !reflect.DeepEqual(d.Appended, []int{0, 1, 2}) {
		t.Fatalf("Appended = %v, want [0,1,2]", d.Appended)
	}
	if len(d.Changed) != 0 {
		t.Fatalf("Changed = %v, want empty", d.Changed)
	}
	if d.Truncated != 0 {
		t.Fatalf("Truncated = %d, want 0", d.Truncated)
	}
	if got := v.LineCount(); got != 3 {
		t.Fatalf("after Apply LineCount() = %d, want 3", got)
	}
	if !reflect.DeepEqual(v.Lines(), []string{"a", "b", "c"}) {
		t.Fatalf("Lines() = %v, want [a b c]", v.Lines())
	}
}

func TestViewport_Apply_SecondFrame_NoChange_IsNoChange(t *testing.T) {
	v := NewViewport("blk")
	frame := Frame{BlockID: "blk", Lines: []string{"a", "b"}}
	_ = v.Apply(frame)
	d := v.Apply(frame)
	if !d.IsNoChange() {
		t.Fatalf("expected IsNoChange()==true on identical second frame; diff=%+v", d)
	}
}

func TestViewport_Apply_OneLineChange_DiffPrecise(t *testing.T) {
	v := NewViewport("blk")
	_ = v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "b", "c"}})
	d := v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "X", "c"}})
	if !reflect.DeepEqual(d.Changed, []int{1}) {
		t.Fatalf("Changed = %v, want [1]", d.Changed)
	}
	if len(d.Appended) != 0 {
		t.Fatalf("Appended = %v, want empty", d.Appended)
	}
	if d.Truncated != 0 {
		t.Fatalf("Truncated = %d, want 0", d.Truncated)
	}
}

func TestViewport_Apply_AppendLines(t *testing.T) {
	v := NewViewport("blk")
	_ = v.Apply(Frame{BlockID: "blk", Lines: []string{"a"}})
	d := v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "b"}})
	if len(d.Changed) != 0 {
		t.Fatalf("Changed = %v, want empty", d.Changed)
	}
	if !reflect.DeepEqual(d.Appended, []int{1}) {
		t.Fatalf("Appended = %v, want [1]", d.Appended)
	}
	if d.Truncated != 0 {
		t.Fatalf("Truncated = %d, want 0", d.Truncated)
	}
}

func TestViewport_Apply_TruncateLines(t *testing.T) {
	v := NewViewport("blk")
	_ = v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "b", "c"}})
	d := v.Apply(Frame{BlockID: "blk", Lines: []string{"a"}})
	if len(d.Changed) != 0 {
		t.Fatalf("Changed = %v, want empty", d.Changed)
	}
	if len(d.Appended) != 0 {
		t.Fatalf("Appended = %v, want empty", d.Appended)
	}
	if d.Truncated != 2 {
		t.Fatalf("Truncated = %d, want 2", d.Truncated)
	}
	if got := v.LineCount(); got != 1 {
		t.Fatalf("after truncate LineCount() = %d, want 1", got)
	}
}

func TestViewport_Apply_ReplaceAll(t *testing.T) {
	v := NewViewport("blk")
	_ = v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "b"}})
	d := v.Apply(Frame{BlockID: "blk", Lines: []string{"x", "y"}})
	if !reflect.DeepEqual(d.Changed, []int{0, 1}) {
		t.Fatalf("Changed = %v, want [0,1]", d.Changed)
	}
	if len(d.Appended) != 0 {
		t.Fatalf("Appended = %v, want empty", d.Appended)
	}
	if d.Truncated != 0 {
		t.Fatalf("Truncated = %d, want 0", d.Truncated)
	}
}

func TestViewport_Apply_LinesIsDefensiveCopy(t *testing.T) {
	v := NewViewport("blk")
	_ = v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "b"}})
	out := v.Lines()
	out[0] = "MUTATED"
	// Subsequent Apply with the same logical frame as before must register
	// no changes — i.e., the viewport's internal lines were not mutated by
	// the caller's modification of the slice we returned.
	d := v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "b"}})
	if !d.IsNoChange() {
		t.Fatalf("Lines() must return defensive copy; diff=%+v", d)
	}
}

func TestViewport_Apply_FrameLinesDefensiveCopy(t *testing.T) {
	v := NewViewport("blk")
	src := []string{"a", "b"}
	_ = v.Apply(Frame{BlockID: "blk", Lines: src})
	// Mutate the caller's source slice; the viewport must not be affected.
	src[0] = "MUTATED"
	d := v.Apply(Frame{BlockID: "blk", Lines: []string{"a", "b"}})
	if !d.IsNoChange() {
		t.Fatalf("input slice must be defensively copied; diff=%+v", d)
	}
}

func TestDiff_PureFunction(t *testing.T) {
	old := []string{"a", "b", "c"}
	nw := []string{"a", "X", "c"}
	d1 := Diff(old, nw)
	d2 := Diff(old, nw)
	d3 := Diff(old, nw)
	if !reflect.DeepEqual(d1, d2) || !reflect.DeepEqual(d2, d3) {
		t.Fatalf("Diff is not pure: %+v / %+v / %+v", d1, d2, d3)
	}
	if !reflect.DeepEqual(d1.Changed, []int{1}) {
		t.Fatalf("Diff Changed = %v, want [1]", d1.Changed)
	}
}

func TestDiff_EmptyOldAndNew_IsNoChange(t *testing.T) {
	d := Diff(nil, nil)
	if !d.IsNoChange() {
		t.Fatalf("Diff(nil,nil) must IsNoChange; got %+v", d)
	}
	d2 := Diff([]string{}, []string{})
	if !d2.IsNoChange() {
		t.Fatalf("Diff([],[]) must IsNoChange; got %+v", d2)
	}
}

func TestLineDiff_IsNoChange_AllZero(t *testing.T) {
	d := LineDiff{}
	if !d.IsNoChange() {
		t.Fatalf("zero-value LineDiff must IsNoChange()")
	}
}

func TestLineDiff_IsNoChange_AnyNonZero(t *testing.T) {
	cases := []LineDiff{
		{Changed: []int{0}},
		{Appended: []int{0}},
		{Truncated: 1},
	}
	for i, c := range cases {
		if c.IsNoChange() {
			t.Fatalf("case %d: %+v must not IsNoChange()", i, c)
		}
	}
}
