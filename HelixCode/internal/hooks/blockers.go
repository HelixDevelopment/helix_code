package hooks

// Blockers extracts non-nil errors from a result slice. Returns nil for an
// empty / nil-only / all-succeeded slice. Used by callers (registry.Execute,
// auto_compactor, agent.RequestPlanApproval) to decide whether any hook
// objected to the operation.
//
// A "blocker" is any result whose Error is non-nil, regardless of Status.
// (StatusFailed implies non-nil Error per the existing executor; checking
// Error directly is the more robust contract.)
func Blockers(results []*ExecutionResult) []error {
	var blockers []error
	for _, r := range results {
		if r == nil {
			continue
		}
		if r.Error != nil {
			blockers = append(blockers, r.Error)
		}
	}
	return blockers
}
