package litellm

type CostTracker struct {
	TotalCost     float64
	TotalTokens   int64
	TotalRequests int64
	BudgetLimit   float64
}

func (c *CostTracker) TrackUsage(in, out int) {
	c.TotalRequests++
	c.TotalTokens += int64(in + out)
	c.TotalCost += float64(in)*0.000001 + float64(out)*0.000002
}

func (c *CostTracker) OverBudget() bool {
	return c.BudgetLimit > 0 && c.TotalCost > c.BudgetLimit
}

func (c *CostTracker) Reset() {
	c.TotalCost = 0
	c.TotalTokens = 0
	c.TotalRequests = 0
}