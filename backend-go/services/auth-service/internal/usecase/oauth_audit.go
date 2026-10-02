package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
)

// oauthSystemTenantID tags audit rows for events with no tenant yet
// (anonymous dynamic client registration). auth.audit_log.tenant_id is NOT
// NULL; the nil UUID keeps the row without pretending it belongs to a tenant.
const oauthSystemTenantID = "00000000-0000-0000-0000-000000000000"

// oauthAuditor appends best-effort audit entries (a failed audit write never
// fails the already-committed OAuth operation, the IssueServiceToken posture).
// Metadata must never contain codes, tokens or verifiers.
type oauthAuditor struct {
	audit AuditRepository
	clock Clock
}

func (a oauthAuditor) record(ctx context.Context, tenantID, actorID, action, targetType, targetID string, meta map[string]any, outcome domain.Outcome) {
	if tenantID == "" {
		tenantID = oauthSystemTenantID
	}
	ip, _ := tenant.ClientIP(ctx)
	entry, err := domain.NewAuditEntry(uuid.NewString(), tenantID, actorID, action, "", targetType, targetID, meta, outcome, ip, a.clock.Now())
	if err != nil {
		return
	}
	_ = a.audit.Append(ctx, entry)
}
