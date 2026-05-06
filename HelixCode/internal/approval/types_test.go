package approval

import (
	"errors"
	"testing"
)

func TestApprovalMode_IsValid(t *testing.T) {
	valid := []ApprovalMode{ModeSuggest, ModeAutoEdit, ModeFullAuto, ModeDangerous}
	for _, m := range valid {
		if !m.IsValid() {
			t.Errorf("expected %q to be valid", m)
		}
	}
	invalid := []ApprovalMode{"", "banana", "AUTO", "auto_edit"}
	for _, m := range invalid {
		if m.IsValid() {
			t.Errorf("expected %q to be invalid", m)
		}
	}
}

func TestApprovalMode_String(t *testing.T) {
	cases := map[ApprovalMode]string{
		ModeSuggest:   "suggest",
		ModeAutoEdit:  "auto-edit",
		ModeFullAuto:  "full-auto",
		ModeDangerous: "dangerously-bypass",
	}
	for m, want := range cases {
		if got := m.String(); got != want {
			t.Errorf("ApprovalMode(%v).String() = %q, want %q", m, got, want)
		}
	}
}

func TestAllModes_ReturnsAllFour(t *testing.T) {
	got := AllModes()
	want := []ApprovalMode{ModeSuggest, ModeAutoEdit, ModeFullAuto, ModeDangerous}
	if len(got) != len(want) {
		t.Fatalf("AllModes() length = %d, want %d", len(got), len(want))
	}
	for i, m := range want {
		if got[i] != m {
			t.Errorf("AllModes()[%d] = %q, want %q (canonical safety order)", i, got[i], m)
		}
	}
}

func TestApprovalLevel_IsValid(t *testing.T) {
	valid := []ApprovalLevel{LevelReadOnly, LevelEdit, LevelRun, LevelAll}
	for _, l := range valid {
		if !l.IsValid() {
			t.Errorf("expected level %d to be valid", int(l))
		}
	}
	invalid := []ApprovalLevel{-1, 4, 99}
	for _, l := range invalid {
		if l.IsValid() {
			t.Errorf("expected level %d to be invalid", int(l))
		}
	}
}

func TestApprovalLevel_String(t *testing.T) {
	cases := map[ApprovalLevel]string{
		LevelReadOnly: "read-only",
		LevelEdit:     "edit",
		LevelRun:      "run",
		LevelAll:      "all",
	}
	for l, want := range cases {
		if got := l.String(); got != want {
			t.Errorf("ApprovalLevel(%d).String() = %q, want %q", int(l), got, want)
		}
	}
}

func TestDecision_String(t *testing.T) {
	cases := map[Decision]string{
		DecisionAllow:  "allow",
		DecisionDeny:   "deny",
		DecisionPrompt: "prompt",
	}
	for d, want := range cases {
		if got := d.String(); got != want {
			t.Errorf("Decision(%d).String() = %q, want %q", int(d), got, want)
		}
	}
}

func TestAction_String(t *testing.T) {
	cases := map[Action]string{
		ActionAllow:          "allow",
		ActionPromptUser:     "prompt-user",
		ActionDenyWithReason: "deny-with-reason",
	}
	for a, want := range cases {
		if got := a.String(); got != want {
			t.Errorf("Action(%d).String() = %q, want %q", int(a), got, want)
		}
	}
}

func TestResolvedSource_String(t *testing.T) {
	cases := map[ResolvedSource]string{
		SourceFlag:    "flag",
		SourceEnv:     "env",
		SourceConfig:  "config",
		SourceDefault: "default",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("ResolvedSource(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}

func TestModeDescriptors_AllFourPresent(t *testing.T) {
	d := ModeDescriptors()
	for _, m := range AllModes() {
		desc, ok := d[m]
		if !ok {
			t.Errorf("ModeDescriptors() missing entry for %q", m)
			continue
		}
		if desc.Mode != m {
			t.Errorf("descriptor for %q has Mode field = %q", m, desc.Mode)
		}
		if desc.Description == "" {
			t.Errorf("descriptor for %q has empty Description", m)
		}
	}
	if len(d) != 4 {
		t.Errorf("ModeDescriptors() has %d entries, want 4", len(d))
	}
}

func TestModeDescriptors_FullAutoSandboxRequired(t *testing.T) {
	d := ModeDescriptors()
	if got := d[ModeFullAuto].SandboxRule; got != "required" {
		t.Errorf("ModeFullAuto SandboxRule = %q, want %q", got, "required")
	}
}

func TestModeDescriptors_FullAutoNetworkDenied(t *testing.T) {
	d := ModeDescriptors()
	if got := d[ModeFullAuto].NetworkRule; got != "denied" {
		t.Errorf("ModeFullAuto NetworkRule = %q, want %q", got, "denied")
	}
}

func TestModeDescriptors_DangerousSandboxSkipped(t *testing.T) {
	d := ModeDescriptors()
	if got := d[ModeDangerous].SandboxRule; got != "skipped" {
		t.Errorf("ModeDangerous SandboxRule = %q, want %q", got, "skipped")
	}
}

func TestModeDescriptors_SuggestSandboxNA(t *testing.T) {
	d := ModeDescriptors()
	if got := d[ModeSuggest].SandboxRule; got != "n/a" {
		t.Errorf("ModeSuggest SandboxRule = %q, want %q", got, "n/a")
	}
}

func TestModeDescriptors_SafetyOrder(t *testing.T) {
	d := ModeDescriptors()
	wantOrder := []ApprovalMode{ModeSuggest, ModeAutoEdit, ModeFullAuto, ModeDangerous}
	for i, m := range wantOrder {
		if d[m].SafetyOrder != i {
			t.Errorf("ModeDescriptors[%q].SafetyOrder = %d, want %d", m, d[m].SafetyOrder, i)
		}
	}
}

func TestParseMode_AcceptsCanonical(t *testing.T) {
	cases := map[string]ApprovalMode{
		"suggest":            ModeSuggest,
		"auto-edit":          ModeAutoEdit,
		"full-auto":          ModeFullAuto,
		"dangerously-bypass": ModeDangerous,
	}
	for in, want := range cases {
		got, err := ParseMode(in)
		if err != nil {
			t.Errorf("ParseMode(%q) returned err: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseMode_AcceptsCaseInsensitive(t *testing.T) {
	got, err := ParseMode("AUTO-EDIT")
	if err != nil {
		t.Fatalf("ParseMode(\"AUTO-EDIT\") err: %v", err)
	}
	if got != ModeAutoEdit {
		t.Errorf("ParseMode(\"AUTO-EDIT\") = %q, want %q", got, ModeAutoEdit)
	}
}

func TestParseMode_AcceptsUnderscores(t *testing.T) {
	got, err := ParseMode("auto_edit")
	if err != nil {
		t.Fatalf("ParseMode(\"auto_edit\") err: %v", err)
	}
	if got != ModeAutoEdit {
		t.Errorf("ParseMode(\"auto_edit\") = %q, want %q", got, ModeAutoEdit)
	}
	got, err = ParseMode("dangerously_bypass")
	if err != nil {
		t.Fatalf("ParseMode(\"dangerously_bypass\") err: %v", err)
	}
	if got != ModeDangerous {
		t.Errorf("ParseMode(\"dangerously_bypass\") = %q, want %q", got, ModeDangerous)
	}
}

func TestParseMode_RejectsGarbage(t *testing.T) {
	_, err := ParseMode("banana")
	if err == nil {
		t.Fatal("ParseMode(\"banana\") expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("ParseMode(\"banana\") err = %v, want errors.Is ErrInvalidMode", err)
	}
}

func TestParseMode_RejectsEmpty(t *testing.T) {
	_, err := ParseMode("")
	if err == nil {
		t.Fatal("ParseMode(\"\") expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidMode) {
		t.Errorf("ParseMode(\"\") err = %v, want errors.Is ErrInvalidMode", err)
	}
}

func TestSentinelErrors_Distinct(t *testing.T) {
	all := []error{
		ErrInvalidMode,
		ErrInvalidLevel,
		ErrApprovalDenied,
		ErrApprovalRequired,
		ErrUserCancelled,
	}
	for i, a := range all {
		if a == nil {
			t.Errorf("sentinel #%d is nil", i)
		}
		for j, b := range all {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Errorf("sentinel #%d (%v) is == sentinel #%d (%v); must be distinct", i, a, j, b)
			}
		}
	}
}

// Compile-time interface-shape check: any type embedding DefaultLevelEdit must
// expose RequiresApproval() ApprovalLevel returning LevelEdit.
type defaultLevelTool struct {
	DefaultLevelEdit
}

func TestDefaultLevelEdit_ReturnsLevelEdit(t *testing.T) {
	var d DefaultLevelEdit
	if got := d.RequiresApproval(); got != LevelEdit {
		t.Errorf("DefaultLevelEdit.RequiresApproval() = %v, want LevelEdit", got)
	}
	tool := defaultLevelTool{}
	if got := tool.RequiresApproval(); got != LevelEdit {
		t.Errorf("embedded DefaultLevelEdit.RequiresApproval() = %v, want LevelEdit", got)
	}
}
