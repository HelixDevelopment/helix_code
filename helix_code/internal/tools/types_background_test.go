package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"dev.helix.code/internal/approval"
)

// fakeBgTool implements BackgroundAware for testing.
type fakeBgTool struct {
	approval.DefaultLevelEdit
	name string
}

func (f *fakeBgTool) Name() string                                 { return f.name }
func (f *fakeBgTool) Description() string                          { return "fake bg" }
func (f *fakeBgTool) Schema() ToolSchema                           { return ToolSchema{Type: "object"} }
func (f *fakeBgTool) Category() ToolCategory                       { return CategoryFileSystem }
func (f *fakeBgTool) Validate(params map[string]interface{}) error { return nil }
func (f *fakeBgTool) Execute(ctx context.Context, params map[string]interface{}) (interface{}, error) {
	return nil, nil
}
func (f *fakeBgTool) ExecuteWithProgress(ctx context.Context, params map[string]interface{}, sink LineSink) (interface{}, error) {
	return nil, nil
}

func TestBackgroundAware_InterfaceSatisfied(t *testing.T) {
	var _ BackgroundAware = (*fakeBgTool)(nil)
}

func TestLineSink_BasicCallback(t *testing.T) {
	var got []string
	var sink LineSink = func(line string) { got = append(got, line) }
	sink("a")
	sink("b")
	assert.Equal(t, []string{"a", "b"}, got)
}

func TestErrNoBackgroundMgr_IsExported(t *testing.T) {
	assert.NotNil(t, ErrNoBackgroundMgr)
	assert.Contains(t, ErrNoBackgroundMgr.Error(), "BackgroundManager")
}
