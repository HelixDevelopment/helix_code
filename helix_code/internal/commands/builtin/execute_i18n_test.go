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
