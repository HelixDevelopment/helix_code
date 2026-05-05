package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/hooks"
)

// fakeTool is a minimal Tool that records its invocation count and lets the
// test seed a fixed result/error. Mocking is allowed at the unit-test layer.
type fakeTool struct {
	name          string
	executeCalled int
	resultValue   interface{}
	resultErr     error
}

func (f *fakeTool) Name() string                                               { return f.name }
func (f *fakeTool) Description() string                                        { return "fake" }
func (f *fakeTool) Schema() ToolSchema                                         { return ToolSchema{Type: "object"} }
func (f *fakeTool) Category() ToolCategory                                     { return CategoryShell }
func (f *fakeTool) Validate(map[string]interface{}) error                      { return nil }
func (f *fakeTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	f.executeCalled++
	return f.resultValue, f.resultErr
}

func TestRegistry_SetHooksManager_AcceptsManager(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	r.SetHooksManager(hooks.NewManager())
	assert.NotNil(t, r.hooksManager)
}

func TestRegistry_Execute_BeforeToolCallBlockPreventsExecute(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	tool := &fakeTool{name: "FakeTool"}
	r.Register(tool)

	hm := hooks.NewManager()
	blockHook := hooks.NewHook("blocker", hooks.HookTypeBeforeToolCall,
		func(ctx context.Context, e *hooks.Event) error {
			return assert.AnError // any non-nil error blocks
		})
	require.NoError(t, hm.Register(blockHook))
	r.SetHooksManager(hm)

	_, execErr := r.Execute(context.Background(), "FakeTool", map[string]interface{}{})
	require.Error(t, execErr)
	assert.Equal(t, 0, tool.executeCalled, "Execute must NOT run when before-hook blocks")
}

func TestRegistry_Execute_AfterToolCallFiresEvenOnError(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	tool := &fakeTool{name: "FakeTool", resultErr: assert.AnError}
	r.Register(tool)

	hm := hooks.NewManager()
	afterFireCount := 0
	afterHook := hooks.NewHook("after", hooks.HookTypeAfterToolCall,
		func(ctx context.Context, e *hooks.Event) error {
			afterFireCount++
			return nil
		})
	require.NoError(t, hm.Register(afterHook))
	r.SetHooksManager(hm)

	_, _ = r.Execute(context.Background(), "FakeTool", map[string]interface{}{})
	assert.Equal(t, 1, tool.executeCalled, "tool must have run")
	assert.Equal(t, 1, afterFireCount, "AfterToolCall must fire even on tool error")
}

func TestRegistry_Execute_BashFiresSpecialisedBeforeBashAndAfterBash(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	tool := &fakeTool{name: "Bash"}
	r.Register(tool)

	hm := hooks.NewManager()
	beforeBash, afterBash := 0, 0
	require.NoError(t, hm.Register(hooks.NewHook("bb", hooks.HookTypeBeforeBash,
		func(ctx context.Context, e *hooks.Event) error { beforeBash++; return nil })))
	require.NoError(t, hm.Register(hooks.NewHook("ab", hooks.HookTypeAfterBash,
		func(ctx context.Context, e *hooks.Event) error { afterBash++; return nil })))
	r.SetHooksManager(hm)

	_, err = r.Execute(context.Background(), "Bash", map[string]interface{}{"command": "ls"})
	require.NoError(t, err)
	assert.Equal(t, 1, beforeBash)
	assert.Equal(t, 1, afterBash)
}

func TestRegistry_Execute_EditFiresSpecialisedBeforeEditAndAfterEdit(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	tool := &fakeTool{name: "Edit"}
	r.Register(tool)

	hm := hooks.NewManager()
	beforeEdit, afterEdit := 0, 0
	require.NoError(t, hm.Register(hooks.NewHook("be", hooks.HookTypeBeforeEdit,
		func(ctx context.Context, e *hooks.Event) error { beforeEdit++; return nil })))
	require.NoError(t, hm.Register(hooks.NewHook("ae", hooks.HookTypeAfterEdit,
		func(ctx context.Context, e *hooks.Event) error { afterEdit++; return nil })))
	r.SetHooksManager(hm)

	_, err = r.Execute(context.Background(), "Edit", map[string]interface{}{"path": "/tmp/x"})
	require.NoError(t, err)
	assert.Equal(t, 1, beforeEdit)
	assert.Equal(t, 1, afterEdit)
}

func TestRegistry_Execute_NilHooksManagerIsPassthrough(t *testing.T) {
	r, err := NewToolRegistry(DefaultRegistryConfig())
	require.NoError(t, err)
	tool := &fakeTool{name: "X", resultValue: 42}
	r.Register(tool)
	// SetHooksManager not called → hooksManager is nil
	got, err := r.Execute(context.Background(), "X", map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, 42, got)
}
