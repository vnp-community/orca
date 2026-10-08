package domain

import "time"

// RetentionSettings are the per-tenant retention days; 0 keeps the data forever.
type RetentionSettings struct {
	RequestDays, AITraceDays, LedgerDays int
	RedactPII                            bool
}

// DefaultRetention is the unmeasured proposal of CR-REQ-035 (no legal requirement behind it yet).
var DefaultRetention = RetentionSettings{RequestDays: 730, AITraceDays: 30, LedgerDays: 400}

// RetentionPlan holds the cutoffs for one tenant; a zero time means that group is never purged.
type RetentionPlan struct {
	TenantID                                 string
	RequestCutoff, TraceCutoff, LedgerCutoff time.Time
}

// PlanFor turns settings into UTC cutoffs.
func PlanFor(tenantID string, s RetentionSettings, now time.Time) RetentionPlan {
	cut := func(days int) time.Time {
		if days <= 0 {
			return time.Time{}
		}
		return now.UTC().AddDate(0, 0, -days)
	}
	return RetentionPlan{TenantID: tenantID, RequestCutoff: cut(s.RequestDays), TraceCutoff: cut(s.AITraceDays), LedgerCutoff: cut(s.LedgerDays)}
}
