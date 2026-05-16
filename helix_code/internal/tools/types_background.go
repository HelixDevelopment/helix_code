package tools

import (
	"context"
	"errors"
)

// LineSink is invoked by a BackgroundAware tool for each line of progress
// output. The sink is supplied by the BackgroundManager and routes lines
// into the BackgroundTask's bounded output ring.
type LineSink func(line string)

// BackgroundAware is implemented by tools that produce line-oriented
// progress output. Tools implementing this interface get streaming
// behavior under run_in_background:true. Tools that don't implement it
// fall back to "final result only" semantics.
type BackgroundAware interface {
	Tool
	ExecuteWithProgress(ctx context.Context, params map[string]interface{}, sink LineSink) (interface{}, error)
}

// ErrNoBackgroundMgr is returned by ToolRegistry.Execute when params include
// run_in_background:true but no BackgroundManager has been wired via
// ToolRegistry.SetBackgroundManager.
var ErrNoBackgroundMgr = errors.New("tools: run_in_background requested but no BackgroundManager wired")
