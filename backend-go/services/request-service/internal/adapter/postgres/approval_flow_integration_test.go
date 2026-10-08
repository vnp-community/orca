//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/contracttest"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// contracttestSeed inserts one pending approval through the real repositories and returns its id and tenant.
func contracttestSeed(t *testing.T, env contracttest.ApprovalEnv) (approvalID, tenantID string) {
	t.Helper()
	tenantID = uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tenantID)
	r, err := domain.NewRequest(domain.NewRequestInput{TenantID: tenantID, ProjectID: uuid.NewString(), Title: "t", SourceProvider: "manual", ReporterID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	r.Status, r.Type = domain.RequestStatusAwaitingPlanApproval, domain.RequestTypeChangeRequest
	if err := env.Tx.InTx(ctx, func(txCtx context.Context) error {
		n, err := env.Requests.NextNumber(txCtx)
		if err != nil {
			return err
		}
		r.Number = n
		return env.Requests.Create(txCtx, r)
	}); err != nil {
		t.Fatal(err)
	}
	a := domain.Approval{ID: uuid.NewString(), TenantID: tenantID, RequestID: r.ID, SubjectType: domain.SubjectPlan, SubjectID: "s", Stage: string(r.Status),
		Status: domain.ApprovalStatusPending, RequestedBy: "u", Version: 1, SubjectDigest: "d", CreatedAt: r.CreatedAt, UpdatedAt: r.CreatedAt}
	if err := env.Approvals.Insert(ctx, a); err != nil {
		t.Fatal(err)
	}
	return a.ID, tenantID
}

func TestPostgres_ApprovalFlowContract(t *testing.T) {
	f := newMigratedPostgres(t)
	contracttest.RunApprovalFlowContract(t, func(*testing.T) contracttest.ApprovalEnv { return f.approvalEnv() })
}
