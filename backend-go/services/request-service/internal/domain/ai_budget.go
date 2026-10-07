package domain

type AIBudget struct {
	TenantID string
	Limit    float64
	Used     float64
}

func (b *AIBudget) CheckLimit() bool {
	return b.Used < b.Limit
}
