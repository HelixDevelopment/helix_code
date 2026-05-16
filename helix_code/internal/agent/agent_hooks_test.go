package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/hooks"
)

func TestBaseAgent_SetHooksManager_AcceptsManager(t *testing.T) {
	a := &BaseAgent{}
	a.SetHooksManager(hooks.NewManager())
	assert.NotNil(t, a.hooksManager)
}

func TestBaseAgent_DispatchOnError_FiresEvent(t *testing.T) {
	a := &BaseAgent{}
	hm := hooks.NewManager()
	fired := 0
	require.NoError(t, hm.Register(hooks.NewHook("oe", hooks.HookTypeOnError,
		func(ctx context.Context, e *hooks.Event) error {
			fired++
			assert.NotEmpty(t, e.Data["error_message"])
			return nil
		})))
	a.SetHooksManager(hm)

	a.dispatchOnError(context.Background(), errors.New("kaboom"), "tool")
	// dispatchOnError uses TriggerEventAndWait so it's synchronous.
	// If the implementation uses async dispatch, this poll loop is robust.
	for i := 0; i < 20 && fired == 0; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, 1, fired)
}

func TestBaseAgent_RequestPlanApproval_FiresOnPlanApproval(t *testing.T) {
	a := &BaseAgent{}
	hm := hooks.NewManager()
	captured := ""
	require.NoError(t, hm.Register(hooks.NewHook("opa", hooks.HookTypeOnPlanApproval,
		func(ctx context.Context, e *hooks.Event) error {
			captured, _ = e.Data["plan_text"].(string)
			return nil
		})))
	a.SetHooksManager(hm)

	err := a.RequestPlanApproval(context.Background(), "plan: do X then Y")
	require.NoError(t, err)
	assert.Equal(t, "plan: do X then Y", captured)
}

func TestBaseAgent_RequestPlanApproval_BlockerSurfaces(t *testing.T) {
	a := &BaseAgent{}
	hm := hooks.NewManager()
	require.NoError(t, hm.Register(hooks.NewHook("opa", hooks.HookTypeOnPlanApproval,
		func(ctx context.Context, e *hooks.Event) error {
			return errors.New("plan rejected by policy")
		})))
	a.SetHooksManager(hm)

	err := a.RequestPlanApproval(context.Background(), "plan: ...")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rejected by policy")
}

func TestBaseAgent_NilHooksManagerIsSafe(t *testing.T) {
	a := &BaseAgent{}
	// No SetHooksManager call.
	a.dispatchOnError(context.Background(), errors.New("x"), "tool")          // must not panic
	require.NoError(t, a.RequestPlanApproval(context.Background(), "p")) // must not panic + return nil
}
