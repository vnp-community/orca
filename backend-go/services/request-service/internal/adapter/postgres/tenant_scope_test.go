package postgres

import (
	"os"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
)

const (
	dynamicWhere = "WHERE built from a slice whose first element is tenant_id = $n"
	clockOnly    = "reads the database clock, no table"
	relay        = "multi-tenant by design (relay or sweeper) under the app.relay policy; the per-item work is tenant scoped"
)

// Second layer next to RLS: application filters keep working if a role ever bypasses the policies.
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
		"tx.go:InTx":                                     "sets the tenant GUC, touches no table",
		"tx.go:withRelayTx":                              "sets the relay GUC, touches no table",
	})
}

// Every query goes through InTx/scoped/withRelayTx, which set app.tenant_id or app.relay; a bare pool call would
// run without a tenant and see nothing under FORCE RLS (or everything for a bypassing role).
func TestNoDirectPoolQuery(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "tx.go" {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, call := range []string{"r.db.Query(", "r.db.Exec(", "r.db.QueryRow(", "pool.Query(", "pool.Exec(", "pool.QueryRow("} {
			if strings.Contains(src, call) {
				t.Errorf("%s: direct pool access (%s); go through scoped/InTx/withRelayTx", name, call)
			}
		}
	}
}
