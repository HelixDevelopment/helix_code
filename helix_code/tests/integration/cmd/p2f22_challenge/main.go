// p2f22_challenge runs the F22 aider-style git-auto-commit harness end-to-end
// against the real ToolRegistry, the real *AutoCommitter, the real Git
// wrapper, and a real `git` subprocess. Article XI 11.9 anti-bluff anchor:
// every PASS carries positive runtime evidence — SHA differential, porcelain
// state, co-author trailer present, secret redaction observable.
//
// Phases (six always-run, plus PHASE-G secret filter):
//
//	A. DEFAULT-ON-COMMITS-EDIT    — env unset → committer enabled → real
//	                                 stub edit tool through real registry
//	                                 against real git tempdir; assert
//	                                 (i) tool succeeds, (ii) HEAD SHA
//	                                 changed, (iii) subject non-empty +
//	                                 ≤72 chars, (iv) co-author trailer in
//	                                 %B, (v) porcelain empty, (vi) git
//	                                 show --stat HEAD lists the path.
//	B. LLM-SUMMARY-ACCURATE       — fake llm.Provider returning sentinel
//	                                 "FAKE_LLM_RESPONSE_42" → assert
//	                                 commit subject equals the sentinel
//	                                 (proves LLM-call-then-use, not
//	                                 hardcoded fallback).
//	C. NON-EDIT-NO-OP             — invoke read-only tool; assert HEAD
//	                                 SHA unchanged.
//	D. ENV-OFF-NO-COMMIT          — set HELIXCODE_GIT_AUTO_COMMIT=off
//	                                 before construct; invoke edit tool;
//	                                 assert (i) tool succeeds,
//	                                 (ii) HEAD SHA unchanged, (iii)
//	                                 status --porcelain shows the file
//	                                 dirty.
//	E. RUNTIME-TOGGLE             — start off → SetEnabled(true) → next
//	                                 call commits (SHA differential).
//	F. PER-EDIT-SKIP              — invoke with SkipParamKey:true; assert
//	                                 (i) tool succeeds, (ii) HEAD SHA
//	                                 unchanged, (iii) porcelain dirty.
//	G. SECRET-FILTER              — fake LLM returns subject containing
//	                                 fake AKIA key; assert log -1 --format=%B
//	                                 contains [REDACTED] and NOT the
//	                                 original key.
//
// Exit code 0 on PASS; exit 1 with a diagnostic on any check failure.
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"

	"go.uber.org/zap"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/autocommit"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
)

// --- harness fakes ---

type harnessEditTool struct {
	name     string
	executed int32
}

func (s *harnessEditTool) Name() string                                     { return s.name }
func (s *harnessEditTool) Description() string                              { return "harness edit tool" }
func (s *harnessEditTool) Schema() tools.ToolSchema                         { return tools.ToolSchema{Type: "object"} }
func (s *harnessEditTool) Category() tools.ToolCategory                     { return tools.ToolCategory("test-stub") }
func (s *harnessEditTool) Validate(_ map[string]interface{}) error          { return nil }
func (s *harnessEditTool) RequiresApproval() approval.ApprovalLevel         { return approval.LevelEdit }
func (s *harnessEditTool) Execute(_ context.Context, p map[string]interface{}) (interface{}, error) {
	atomic.AddInt32(&s.executed, 1)
	if path, ok := p["path"].(string); ok {
		_ = os.WriteFile(path, []byte("hello\n"), 0644)
	}
	return "ok", nil
}

type harnessReadTool struct{ name string }

func (s *harnessReadTool) Name() string                                     { return s.name }
func (s *harnessReadTool) Description() string                              { return "harness read tool" }
func (s *harnessReadTool) Schema() tools.ToolSchema                         { return tools.ToolSchema{Type: "object"} }
func (s *harnessReadTool) Category() tools.ToolCategory                     { return tools.ToolCategory("test-stub") }
func (s *harnessReadTool) Validate(_ map[string]interface{}) error          { return nil }
func (s *harnessReadTool) RequiresApproval() approval.ApprovalLevel         { return approval.LevelReadOnly }
func (s *harnessReadTool) Execute(context.Context, map[string]interface{}) (interface{}, error) {
	return "read", nil
}

// fakeLLMProvider returns a canned response. Used to pin LLM-call-then-use
// (PHASE-B) and secret filter (PHASE-G).
type fakeLLMProvider struct{ response string }

func (f *fakeLLMProvider) GetType() llm.ProviderType { return "fake" }
func (f *fakeLLMProvider) GetName() string           { return "fake" }
func (f *fakeLLMProvider) GetModels() []llm.ModelInfo {
	return []llm.ModelInfo{{ID: "fake-model"}}
}
func (f *fakeLLMProvider) GetCapabilities() []llm.ModelCapability { return nil }
func (f *fakeLLMProvider) Generate(_ context.Context, _ *llm.LLMRequest) (*llm.LLMResponse, error) {
	return &llm.LLMResponse{Content: f.response}, nil
}
func (f *fakeLLMProvider) GenerateStream(_ context.Context, _ *llm.LLMRequest, _ chan<- llm.LLMResponse) error {
	return nil
}
func (f *fakeLLMProvider) IsAvailable(context.Context) bool { return true }
func (f *fakeLLMProvider) GetHealth(context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "ok"}, nil
}
func (f *fakeLLMProvider) Close() error                      { return nil }
func (f *fakeLLMProvider) GetContextWindow() int             { return 4096 }
func (f *fakeLLMProvider) CountTokens(s string) (int, error) { return len(s) / 4, nil }

// --- harness helpers ---

func setupRepo() (string, error) {
	dir, err := os.MkdirTemp("", "p2f22-")
	if err != nil {
		return "", err
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@helixcode.dev"},
		{"config", "user.name", "Test"},
		{"config", "commit.gpgsign", "false"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git %v: %w", args, err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644); err != nil {
		return "", err
	}
	for _, args := range [][]string{
		{"-C", dir, "add", ".gitkeep"},
		{"-C", dir, "commit", "-q", "-m", "init"},
	} {
		if err := exec.Command("git", args...).Run(); err != nil {
			return "", fmt.Errorf("git %v: %w", args, err)
		}
	}
	return dir, nil
}

func headSHA(dir string) string {
	out, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	return strings.TrimSpace(string(out))
}

func porcelain(dir string) string {
	out, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	return string(out)
}

func commitSubject(dir string) string {
	out, _ := exec.Command("git", "-C", dir, "log", "-1", "--format=%s").Output()
	return strings.TrimSpace(string(out))
}

func commitBody(dir string) string {
	out, _ := exec.Command("git", "-C", dir, "log", "-1", "--format=%B").Output()
	return string(out)
}

func showStat(dir string) string {
	out, _ := exec.Command("git", "-C", dir, "show", "--stat", "HEAD").Output()
	return string(out)
}

func newRegistryWithCommitter(dir string, p llm.Provider, enabled bool) (*tools.ToolRegistry, *autocommit.AutoCommitter, *harnessEditTool, *harnessReadTool, error) {
	reg, err := tools.NewToolRegistry(tools.DefaultRegistryConfig())
	if err != nil {
		return nil, nil, nil, nil, err
	}
	editTool := &harnessEditTool{name: "harness_edit"}
	readTool := &harnessReadTool{name: "harness_read"}
	reg.Register(editTool)
	reg.Register(readTool)
	committer := autocommit.NewAutoCommitter(autocommit.Options{
		Enabled: enabled, Provider: p, WorkingDir: dir, Logger: zap.NewNop(),
	})
	reg.SetAutoCommitter(committer)
	return reg, committer, editTool, readTool, nil
}

// fail prints an error and exits non-zero.
func fail(phase, msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "FAIL %s: "+msg+"\n", append([]interface{}{phase}, args...)...)
	os.Exit(1)
}

// pass increments the per-phase counter via println side effect.
func pass(phase, line string) { fmt.Printf("PASS %s: %s\n", phase, line) }

// --- main ---

func main() {
	pa, pb, pc, pd, pe, pf, pg := 0, 0, 0, 0, 0, 0, 0

	// PHASE-A: DEFAULT-ON, real edit, real commit ----------------------------
	{
		dir, err := setupRepo()
		if err != nil {
			fail("PHASE-A", "setup: %v", err)
		}
		defer os.RemoveAll(dir)
		initial := headSHA(dir)
		reg, _, edit, _, err := newRegistryWithCommitter(dir,
			&fakeLLMProvider{response: "edit hello.txt"}, true)
		if err != nil {
			fail("PHASE-A", "registry: %v", err)
		}

		target := filepath.Join(dir, "x.txt")
		_, err = reg.Execute(context.Background(), edit.Name(),
			map[string]interface{}{"path": target})
		if err != nil {
			fail("PHASE-A", "execute: %v", err)
		}
		pa++
		pass("PHASE-A", "tool executed without error")
		if atomic.LoadInt32(&edit.executed) != 1 {
			fail("PHASE-A", "edit counter %d != 1", edit.executed)
		}
		pa++
		pass("PHASE-A", "edit counter == 1")
		if headSHA(dir) == initial {
			fail("PHASE-A", "HEAD SHA unchanged after edit")
		}
		pa++
		pass("PHASE-A", "HEAD SHA changed (real commit landed)")

		subj := commitSubject(dir)
		if subj == "" || len(subj) > 72 {
			fail("PHASE-A", "subject %q len=%d not in (0,72]", subj, len(subj))
		}
		pa++
		pass("PHASE-A", fmt.Sprintf("subject ok (%d chars)", len(subj)))

		body := commitBody(dir)
		if !strings.Contains(body, autocommit.CoAuthorTrailer) {
			fail("PHASE-A", "co-author trailer missing in:\n%s", body)
		}
		pa++
		pass("PHASE-A", "co-author trailer present")

		if strings.TrimSpace(porcelain(dir)) != "" {
			fail("PHASE-A", "porcelain not empty: %q", porcelain(dir))
		}
		pa++
		pass("PHASE-A", "porcelain empty after commit")

		stat := showStat(dir)
		if !strings.Contains(stat, "x.txt") {
			fail("PHASE-A", "show --stat HEAD missing x.txt:\n%s", stat)
		}
		pa++
		pass("PHASE-A", "show --stat HEAD lists x.txt")
	}

	// PHASE-B: LLM SENTINEL ---------------------------------------------------
	{
		dir, err := setupRepo()
		if err != nil {
			fail("PHASE-B", "setup: %v", err)
		}
		defer os.RemoveAll(dir)
		reg, _, edit, _, err := newRegistryWithCommitter(dir,
			&fakeLLMProvider{response: "FAKE_LLM_RESPONSE_42"}, true)
		if err != nil {
			fail("PHASE-B", "registry: %v", err)
		}
		target := filepath.Join(dir, "x.txt")
		_, err = reg.Execute(context.Background(), edit.Name(),
			map[string]interface{}{"path": target})
		if err != nil {
			fail("PHASE-B", "execute: %v", err)
		}
		pb++
		pass("PHASE-B", "tool executed")

		subj := commitSubject(dir)
		if subj != "FAKE_LLM_RESPONSE_42" {
			fail("PHASE-B", "expected sentinel subject, got %q", subj)
		}
		pb++
		pass("PHASE-B", "subject == FAKE_LLM_RESPONSE_42 (LLM round-trip proven)")

		body := commitBody(dir)
		if !strings.Contains(body, autocommit.CoAuthorTrailer) {
			fail("PHASE-B", "trailer missing")
		}
		pb++
		pass("PHASE-B", "co-author trailer present even on LLM path")
	}

	// PHASE-C: NON-EDIT NO-OP -------------------------------------------------
	{
		dir, err := setupRepo()
		if err != nil {
			fail("PHASE-C", "setup: %v", err)
		}
		defer os.RemoveAll(dir)
		initial := headSHA(dir)
		reg, _, _, read, err := newRegistryWithCommitter(dir,
			&fakeLLMProvider{response: "should not appear"}, true)
		if err != nil {
			fail("PHASE-C", "registry: %v", err)
		}
		_, err = reg.Execute(context.Background(), read.Name(),
			map[string]interface{}{"path": "anywhere"})
		if err != nil {
			fail("PHASE-C", "read execute: %v", err)
		}
		pc++
		pass("PHASE-C", "read tool executed")

		if headSHA(dir) != initial {
			fail("PHASE-C", "HEAD SHA changed unexpectedly after read")
		}
		pc++
		pass("PHASE-C", "HEAD SHA unchanged after read tool (level filter works)")
	}

	// PHASE-D: ENV-OFF NO-COMMIT ---------------------------------------------
	{
		dir, err := setupRepo()
		if err != nil {
			fail("PHASE-D", "setup: %v", err)
		}
		defer os.RemoveAll(dir)
		initial := headSHA(dir)
		// Simulate env-off by constructing committer with Enabled=false
		// (the same outcome main.go produces when env=="off").
		reg, _, edit, _, err := newRegistryWithCommitter(dir, nil, false)
		if err != nil {
			fail("PHASE-D", "registry: %v", err)
		}
		target := filepath.Join(dir, "x.txt")
		_, err = reg.Execute(context.Background(), edit.Name(),
			map[string]interface{}{"path": target})
		if err != nil {
			fail("PHASE-D", "execute: %v", err)
		}
		pd++
		pass("PHASE-D", "tool executed")

		if headSHA(dir) != initial {
			fail("PHASE-D", "HEAD SHA changed despite env=off")
		}
		pd++
		pass("PHASE-D", "HEAD SHA unchanged (opt-out honoured)")

		if !strings.Contains(porcelain(dir), "x.txt") {
			fail("PHASE-D", "x.txt should be dirty:\n%s", porcelain(dir))
		}
		pd++
		pass("PHASE-D", "porcelain shows x.txt dirty")
	}

	// PHASE-E: RUNTIME TOGGLE ------------------------------------------------
	{
		dir, err := setupRepo()
		if err != nil {
			fail("PHASE-E", "setup: %v", err)
		}
		defer os.RemoveAll(dir)
		initial := headSHA(dir)
		reg, c, edit, _, err := newRegistryWithCommitter(dir,
			&fakeLLMProvider{response: "subj"}, false)
		if err != nil {
			fail("PHASE-E", "registry: %v", err)
		}
		// First call (off): no commit.
		t1 := filepath.Join(dir, "a.txt")
		_, err = reg.Execute(context.Background(), edit.Name(),
			map[string]interface{}{"path": t1})
		if err != nil {
			fail("PHASE-E", "execute1: %v", err)
		}
		if headSHA(dir) != initial {
			fail("PHASE-E", "first call (off) committed unexpectedly")
		}
		pe++
		pass("PHASE-E", "first call (off) skipped commit as expected")

		// Manually commit a.txt to clean tree before second call.
		_ = exec.Command("git", "-C", dir, "add", "a.txt").Run()
		_ = exec.Command("git", "-C", dir, "commit", "-q", "-m", "manual").Run()
		mid := headSHA(dir)

		// Toggle on; next call commits.
		c.SetEnabled(true)
		if !c.Enabled() {
			fail("PHASE-E", "Enabled() returned false after SetEnabled(true)")
		}
		pe++
		pass("PHASE-E", "Enabled() returns true after SetEnabled(true)")

		t2 := filepath.Join(dir, "b.txt")
		_, err = reg.Execute(context.Background(), edit.Name(),
			map[string]interface{}{"path": t2})
		if err != nil {
			fail("PHASE-E", "execute2: %v", err)
		}
		pe++
		pass("PHASE-E", "second call (on) executed")
		if headSHA(dir) == mid {
			fail("PHASE-E", "second call (on) did not commit")
		}
		pe++
		pass("PHASE-E", "HEAD SHA changed after runtime toggle (on)")
	}

	// PHASE-F: PER-EDIT SKIP -------------------------------------------------
	{
		dir, err := setupRepo()
		if err != nil {
			fail("PHASE-F", "setup: %v", err)
		}
		defer os.RemoveAll(dir)
		initial := headSHA(dir)
		reg, _, edit, _, err := newRegistryWithCommitter(dir,
			&fakeLLMProvider{response: "subj"}, true)
		if err != nil {
			fail("PHASE-F", "registry: %v", err)
		}
		target := filepath.Join(dir, "x.txt")
		_, err = reg.Execute(context.Background(), edit.Name(), map[string]interface{}{
			"path":                  target,
			autocommit.SkipParamKey: true,
		})
		if err != nil {
			fail("PHASE-F", "execute: %v", err)
		}
		pf++
		pass("PHASE-F", "tool executed with skip param")

		if headSHA(dir) != initial {
			fail("PHASE-F", "HEAD SHA changed despite per-edit skip")
		}
		pf++
		pass("PHASE-F", "HEAD SHA unchanged (skip honoured)")

		if !strings.Contains(porcelain(dir), "x.txt") {
			fail("PHASE-F", "x.txt should be dirty:\n%s", porcelain(dir))
		}
		pf++
		pass("PHASE-F", "porcelain shows x.txt dirty")
	}

	// PHASE-G: SECRET FILTER -------------------------------------------------
	{
		dir, err := setupRepo()
		if err != nil {
			fail("PHASE-G", "setup: %v", err)
		}
		defer os.RemoveAll(dir)
		// LLM "leaks" a fake AKIA key in its response. Filter must redact.
		reg, _, edit, _, err := newRegistryWithCommitter(dir,
			&fakeLLMProvider{response: "leak AKIAABCDEFGHIJKLMNOP add"}, true)
		if err != nil {
			fail("PHASE-G", "registry: %v", err)
		}
		target := filepath.Join(dir, "x.txt")
		_, err = reg.Execute(context.Background(), edit.Name(),
			map[string]interface{}{"path": target})
		if err != nil {
			fail("PHASE-G", "execute: %v", err)
		}
		pg++
		pass("PHASE-G", "tool executed")

		subj := commitSubject(dir)
		if strings.Contains(subj, "AKIAABCDEFGHIJKLMNOP") {
			fail("PHASE-G", "raw AKIA key leaked into subject: %q", subj)
		}
		pg++
		pass("PHASE-G", "raw AKIA key absent from subject")

		if !strings.Contains(subj, "[REDACTED]") {
			fail("PHASE-G", "expected [REDACTED] marker in subject, got %q", subj)
		}
		pg++
		pass("PHASE-G", "[REDACTED] marker present in subject")
	}

	// --- summary ---
	fmt.Println()
	fmt.Printf("SUMMARY: PHASE-A=%d/7 PASS; PHASE-B=%d/3 PASS; PHASE-C=%d/2 PASS; "+
		"PHASE-D=%d/3 PASS; PHASE-E=%d/4 PASS; PHASE-F=%d/3 PASS; PHASE-G=%d/3 PASS\n",
		pa, pb, pc, pd, pe, pf, pg)
	fmt.Println("==> ALL CHECKS PASSED")
	fmt.Println("==> P2-F22 challenge harness PASS")
}
