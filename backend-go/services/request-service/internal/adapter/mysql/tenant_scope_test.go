package mysql

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

const (
	dynamicWhere = "WHERE built from a slice whose first element is tenant_id = ?"
	clockOnly    = "reads the database clock, no table"
	relay        = "multi-tenant by design (relay or sweeper); the per-item work is tenant scoped"
)

// MySQL has no row level security, so the source itself is the isolation: every statement is bound to a tenant.
func TestTenantScope(t *testing.T) {
	contracttest.CheckTenantScope(t, ".", map[string]string{
		"approval_policy_repository.go:List":             dynamicWhere,
		"approval_repository.go:List":                    dynamicWhere,
		"backlog_requests.go:ListByStatus":               "WHERE is assembled in a variable and always starts with tenant_id = ?/$1",
		"approval_policy_repository.go:NowDB":            clockOnly,
		"approval_repository.go:NowDB":                   clockOnly,
		"approval_repository.go:Insert":                  "column list is the approvalColumns constant, which includes tenant_id",
		"approval_repository.go:claim":                   relay,
		"analysis_run_repository.go:ClaimExpired":        relay,
		"clarification_repository.go:ListDueRefs":        relay,
		"clarification_repository.go:ListRemindableRefs": relay,
		"metrics_samples.go:CountOutboxPending":          relay,
		"metrics_samples.go:CountStuck":                  relay,
		"metrics_samples.go:PendingApprovalsBySubject":   relay,
		"audit_outbox.go:ProcessDue":                     relay,
		"audit_outbox.go:PurgeDelivered":                 relay,
		"audit_outbox.go:CountPending":                   relay,
		"classification_run_repository.go:ClaimExpired":  relay,
		"outbox.go:FetchUnpublished":                     relay,
		"outbox.go:MarkPublished":                        relay,
		"processed_events.go:Prune":                      relay,
		"retention.go:ListTenants":                       "lists the tenants whose retention runs; every other retention query is scoped",
		"webhook_nonces.go:PruneExpired":                 relay,
	})
}
