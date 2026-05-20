// execute_i18n_test.go — CONST-046 round-355 §11.4 anti-bluff sweep
// (2026-05-20). Call-site paired-mutation guards for the runtime
// CommandResult.Message literals migrated out of the built-in slash
// commands' Execute() methods.
//
// Round-353 migrated the context-free Description()/Usage() metadata
// (covered by translator_test.go). Round-355 migrates the runtime
// status/error messages that Execute() returns in CommandResult.Message
// — these are surfaced directly to the operator in the CLI/TUI
// command-result panel, so they are genuine (C) user-facing UI text.
//
// Each test wires a sentinelTranslator (defined in translator_test.go)
// so that if a future refactor inlines a hardcoded literal back into
// Execute(), the asserted sentinel-wrapped message ID will no longer
// appear and the test fails — exactly the §11.4 paired-mutation
// guarantee. The real on-disk bundle is restored after each test via
// withRealBundleTranslator so the positive-evidence assertions below
// still see genuine user text.
//
// Mocks ALLOWED here per CONST-050(A) — unit-test-only file
// (_test.go, no integration build tag).
package builtin

import (
	"context"
	"strings"
	"testing"

	"dev.helix.code/internal/commands"
)

// TestExecute_RuntimeMessagesGoThroughTranslator is the round-355
// call-site paired-mutation: with the sentinel translator wired, every
// built-in command's Execute() MUST return a CommandResult.Message
// that is the sentinel-wrapped form of the expected message ID. A
// hardcoded literal anywhere on the path would surface its raw text
// instead and fail the assertion.
func TestExecute_RuntimeMessagesGoThroughTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	defer withRealBundleTranslator(t)

	ctx := context.Background()

	cases := []struct {
		name   string
		run    func() *commands.CommandResult
		wantID string
	}{
		{
			name: "condense_no_history",
			run: func() *commands.CommandResult {
				r, _ := (&CondenseCommand{}).Execute(ctx, &commands.CommandContext{})
				return r
			},
			wantID: "builtin_condense_no_history",
		},
		{
			name: "condense_not_enough_history",
			run: func() *commands.CommandResult {
				// One message + default keep-last 5 → condenseCount <= 0.
				r, _ := (&CondenseCommand{}).Execute(ctx, &commands.CommandContext{
					ChatHistory: []commands.ChatMessage{{Role: "user", Content: "hi"}},
				})
				return r
			},
			wantID: "builtin_condense_not_enough_history",
		},
		{
			name: "newtask_description_required",
			run: func() *commands.CommandResult {
				r, _ := (&NewTaskCommand{}).Execute(ctx, &commands.CommandContext{})
				return r
			},
			wantID: "builtin_newtask_description_required",
		},
		{
			name: "newtask_created",
			run: func() *commands.CommandResult {
				r, _ := (&NewTaskCommand{}).Execute(ctx, &commands.CommandContext{
					Args: []string{"build", "feature"},
				})
				return r
			},
			wantID: "builtin_newtask_created",
		},
		{
			name: "deepplanning_topic_required",
			run: func() *commands.CommandResult {
				r, _ := (&DeepPlanningCommand{}).Execute(ctx, &commands.CommandContext{})
				return r
			},
			wantID: "builtin_deepplanning_topic_required",
		},
		{
			name: "deepplanning_starting",
			run: func() *commands.CommandResult {
				r, _ := (&DeepPlanningCommand{}).Execute(ctx, &commands.CommandContext{
					Args: []string{"new", "auth", "system"},
				})
				return r
			},
			wantID: "builtin_deepplanning_starting",
		},
		{
			name: "deepplanning_resuming",
			run: func() *commands.CommandResult {
				r, _ := (&DeepPlanningCommand{}).Execute(ctx, &commands.CommandContext{
					Flags: map[string]string{"resume": "plan-123"},
				})
				return r
			},
			wantID: "builtin_deepplanning_resuming",
		},
		{
			name: "newrule_generating",
			run: func() *commands.CommandResult {
				r, _ := (&NewRuleCommand{}).Execute(ctx, &commands.CommandContext{
					Args: []string{"coding-style"},
				})
				return r
			},
			wantID: "builtin_newrule_generating",
		},
		{
			name: "reportbug_prepared",
			run: func() *commands.CommandResult {
				r, _ := (&ReportBugCommand{}).Execute(ctx, &commands.CommandContext{
					Args: []string{"LLM", "timeout"},
				})
				return r
			},
			wantID: "builtin_reportbug_prepared",
		},
		// Round-358 §11.4 CONST-046 Phase-4: /workflows runtime
		// CommandResult.Message paired-mutation guards.
		{
			name: "workflows_found",
			run: func() *commands.CommandResult {
				r, _ := (&WorkflowsCommand{}).Execute(ctx, &commands.CommandContext{})
				return r
			},
			wantID: "builtin_workflows_found",
		},
		{
			name: "workflows_unknown",
			run: func() *commands.CommandResult {
				r, _ := (&WorkflowsCommand{}).Execute(ctx, &commands.CommandContext{
					Args: []string{"nonexistent-workflow"},
				})
				return r
			},
			wantID: "builtin_workflows_unknown",
		},
		{
			name: "workflows_executing",
			run: func() *commands.CommandResult {
				r, _ := (&WorkflowsCommand{}).Execute(ctx, &commands.CommandContext{
					Args: []string{"planning"},
				})
				return r
			},
			wantID: "builtin_workflows_executing",
		},
		{
			name: "workflows_checking_status",
			run: func() *commands.CommandResult {
				r, _ := (&WorkflowsCommand{}).Execute(ctx, &commands.CommandContext{
					Flags: map[string]string{"status": "workflow-123"},
				})
				return r
			},
			wantID: "builtin_workflows_checking_status",
		},
		{
			name: "workflows_cancelling",
			run: func() *commands.CommandResult {
				r, _ := (&WorkflowsCommand{}).Execute(ctx, &commands.CommandContext{
					Flags: map[string]string{"cancel": "workflow-123"},
				})
				return r
			},
			wantID: "builtin_workflows_cancelling",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := tc.run()
			if res == nil {
				t.Fatalf("%s: Execute returned nil CommandResult", tc.name)
			}
			want := "<TR:" + tc.wantID + ">"
			if !strings.Contains(res.Message, want) {
				t.Errorf("%s: Message = %q, want it to contain sentinel-wrapped %q — literal bypassed tr()",
					tc.name, res.Message, tc.wantID)
			}
		})
	}
}

// TestExecute_RuntimeMessagesRealBundleProduceUserText is the
// positive runtime-evidence half: with the real on-disk bundle wired
// (via translator_test.go's TestMain), every Execute() runtime message
// is non-empty and never echoes a raw message ID — proving the
// migration preserved genuine user-facing output.
func TestExecute_RuntimeMessagesRealBundleProduceUserText(t *testing.T) {
	ctx := context.Background()

	checks := []struct {
		name    string
		message string
		rawID   string
	}{
		func() struct {
			name, message, rawID string
		} {
			r, _ := (&CondenseCommand{}).Execute(ctx, &commands.CommandContext{})
			return struct{ name, message, rawID string }{"condense_no_history", r.Message, "builtin_condense_no_history"}
		}(),
		func() struct {
			name, message, rawID string
		} {
			r, _ := (&NewTaskCommand{}).Execute(ctx, &commands.CommandContext{Args: []string{"x"}})
			return struct{ name, message, rawID string }{"newtask_created", r.Message, "builtin_newtask_created"}
		}(),
		func() struct {
			name, message, rawID string
		} {
			r, _ := (&DeepPlanningCommand{}).Execute(ctx, &commands.CommandContext{Args: []string{"topic"}})
			return struct{ name, message, rawID string }{"deepplanning_starting", r.Message, "builtin_deepplanning_starting"}
		}(),
		func() struct {
			name, message, rawID string
		} {
			r, _ := (&ReportBugCommand{}).Execute(ctx, &commands.CommandContext{Args: []string{"bug"}})
			return struct{ name, message, rawID string }{"reportbug_prepared", r.Message, "builtin_reportbug_prepared"}
		}(),
		func() struct {
			name, message, rawID string
		} {
			r, _ := (&WorkflowsCommand{}).Execute(ctx, &commands.CommandContext{})
			return struct{ name, message, rawID string }{"workflows_found", r.Message, "builtin_workflows_found"}
		}(),
		func() struct {
			name, message, rawID string
		} {
			r, _ := (&WorkflowsCommand{}).Execute(ctx, &commands.CommandContext{Args: []string{"building"}})
			return struct{ name, message, rawID string }{"workflows_executing", r.Message, "builtin_workflows_executing"}
		}(),
	}

	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			if strings.TrimSpace(c.message) == "" {
				t.Errorf("%s: runtime Message empty under real bundle", c.name)
			}
			if c.message == c.rawID {
				t.Errorf("%s: runtime Message echoed raw ID %q — bundle entry missing", c.name, c.rawID)
			}
		})
	}
}

// TestWorkflows_ListDescriptionsGoThroughTranslator is the round-358
// paired-mutation guard for the six built-in workflow descriptions
// surfaced in the /workflows listing metadata. With the sentinel
// translator wired, every description in the listing MUST be the
// sentinel-wrapped form of its message ID — a re-inlined literal
// would surface raw text and fail the assertion.
func TestWorkflows_ListDescriptionsGoThroughTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	defer withRealBundleTranslator(t)

	res, err := (&WorkflowsCommand{}).Execute(context.Background(), &commands.CommandContext{
		Flags: map[string]string{"list": ""},
	})
	if err != nil || res == nil {
		t.Fatalf("workflows --list: err=%v res=%v", err, res)
	}
	wfs, ok := res.Metadata["workflows"].([]map[string]string)
	if !ok {
		t.Fatalf("workflows metadata missing or wrong type: %T", res.Metadata["workflows"])
	}
	wantIDs := map[string]string{
		"planning":    "builtin_workflows_wf_planning_description",
		"building":    "builtin_workflows_wf_building_description",
		"testing":     "builtin_workflows_wf_testing_description",
		"refactoring": "builtin_workflows_wf_refactoring_description",
		"debugging":   "builtin_workflows_wf_debugging_description",
		"deployment":  "builtin_workflows_wf_deployment_description",
	}
	for _, wf := range wfs {
		want := "<TR:" + wantIDs[wf["name"]] + ">"
		if !strings.Contains(wf["description"], want) {
			t.Errorf("workflow %q: description = %q, want sentinel-wrapped %q — literal bypassed tr()",
				wf["name"], wf["description"], wantIDs[wf["name"]])
		}
	}
}

// TestWorkflows_ListDescriptionsRealBundleProduceUserText is the
// round-358 positive runtime-evidence half: under the real on-disk
// bundle every workflow description is non-empty and never echoes a
// raw message ID.
func TestWorkflows_ListDescriptionsRealBundleProduceUserText(t *testing.T) {
	res, err := (&WorkflowsCommand{}).Execute(context.Background(), &commands.CommandContext{
		Flags: map[string]string{"list": ""},
	})
	if err != nil || res == nil {
		t.Fatalf("workflows --list: err=%v res=%v", err, res)
	}
	wfs, _ := res.Metadata["workflows"].([]map[string]string)
	if len(wfs) != 6 {
		t.Fatalf("expected 6 workflows, got %d", len(wfs))
	}
	for _, wf := range wfs {
		desc := wf["description"]
		if strings.TrimSpace(desc) == "" {
			t.Errorf("workflow %q: description empty under real bundle", wf["name"])
		}
		if strings.HasPrefix(desc, "builtin_workflows_wf_") {
			t.Errorf("workflow %q: description echoed raw ID %q — bundle entry missing", wf["name"], desc)
		}
	}
}

// TestReportBug_DocumentTemplateGoesThroughTranslator is the round-358
// paired-mutation guard for the /reportbug bug-report-document
// template strings. With the sentinel translator wired, the GitHub
// issue body produced by formatBugReport (via Execute) MUST embed the
// sentinel-wrapped section-heading + placeholder message IDs — a
// re-inlined literal would surface raw text and fail the assertion.
func TestReportBug_DocumentTemplateGoesThroughTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	defer withRealBundleTranslator(t)

	res, err := (&ReportBugCommand{}).Execute(context.Background(), &commands.CommandContext{
		Args: []string{"feature", "broken"},
	})
	if err != nil || res == nil {
		t.Fatalf("reportbug: err=%v res=%v", err, res)
	}
	body, _ := res.Metadata["body"].(string)
	if body == "" {
		// formatBugReport output is carried in the file_bug_report action.
		for _, a := range res.Actions {
			if b, ok := a.Data["body"].(string); ok {
				body = b
			}
		}
	}
	if body == "" {
		t.Fatal("reportbug: bug-report body not found in result")
	}
	wantIDs := []string{
		"builtin_reportbug_section_description",
		"builtin_reportbug_section_system_info",
		"builtin_reportbug_section_reproduction",
		"builtin_reportbug_section_expected",
		"builtin_reportbug_placeholder_expected",
		"builtin_reportbug_section_actual",
		"builtin_reportbug_placeholder_actual",
		"builtin_reportbug_placeholder_repro_steps",
		"builtin_reportbug_footer",
	}
	for _, id := range wantIDs {
		if !strings.Contains(body, "<TR:"+id+">") {
			t.Errorf("bug-report body missing sentinel-wrapped %q — literal bypassed tr()", id)
		}
	}
}

// TestReportBug_DocumentTemplateRealBundleProduceUserText is the
// round-358 positive runtime-evidence half: under the real on-disk
// bundle the bug-report document carries genuine localized section
// text, never raw message IDs.
func TestReportBug_DocumentTemplateRealBundleProduceUserText(t *testing.T) {
	res, err := (&ReportBugCommand{}).Execute(context.Background(), &commands.CommandContext{
		Args: []string{"feature", "broken"},
	})
	if err != nil || res == nil {
		t.Fatalf("reportbug: err=%v res=%v", err, res)
	}
	var body string
	for _, a := range res.Actions {
		if b, ok := a.Data["body"].(string); ok {
			body = b
		}
	}
	if strings.TrimSpace(body) == "" {
		t.Fatal("reportbug: bug-report body empty under real bundle")
	}
	for _, raw := range []string{
		"builtin_reportbug_section_description",
		"builtin_reportbug_footer",
		"builtin_reportbug_placeholder_repro_steps",
	} {
		if strings.Contains(body, raw) {
			t.Errorf("bug-report body echoed raw ID %q — bundle entry missing", raw)
		}
	}
	if !strings.Contains(body, "## Description") {
		t.Errorf("bug-report body missing expected localized section heading; got:\n%s", body)
	}
}

// TestReportBug_NoLogFilesBranchGoesThroughTranslator is the round-420
// paired-mutation guard for the collectRecentLogs no-log-files fallback
// messages. With a session ID that resolves to no on-disk log file the
// fallback branch fires; with the sentinel translator wired the bug
// report body MUST embed the sentinel-wrapped fallback message IDs — a
// re-inlined literal would surface raw English text and fail.
func TestReportBug_NoLogFilesBranchGoesThroughTranslator(t *testing.T) {
	SetTranslator(sentinelTranslator{})
	defer withRealBundleTranslator(t)

	res, err := (&ReportBugCommand{}).Execute(context.Background(), &commands.CommandContext{
		Args:      []string{"bug", "broken"},
		SessionID: "round420-no-such-log-session-xyz",
	})
	if err != nil || res == nil {
		t.Fatalf("reportbug: err=%v res=%v", err, res)
	}
	var body string
	for _, a := range res.Actions {
		if b, ok := a.Data["body"].(string); ok {
			body = b
		}
	}
	if body == "" {
		t.Fatal("reportbug: bug-report body not found in result")
	}
	for _, id := range []string{
		"builtin_reportbug_no_log_files",
		"builtin_reportbug_logger_configured",
		"builtin_reportbug_enable_file_logging",
	} {
		if !strings.Contains(body, "<TR:"+id+">") {
			t.Errorf("bug-report body missing sentinel-wrapped %q — literal bypassed tr()", id)
		}
	}
}
