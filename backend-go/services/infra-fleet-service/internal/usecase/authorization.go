package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// requireAdmin fails closed unless the caller's propagated role
// (common/tenant.Role, populated end-to-end only for the browser
// cookie/session auth path — see that function's doc comment) is exactly
// "admin". An absent role (bearer-JWT callers, or any caller from before
// CR-DS-006 Phase 2 wired this propagation) is treated as non-admin, never
// as an implicit allow — this is intentionally the OPPOSITE failure mode
// from project-service/annotation-service's inert callerGlobalRole stub
// (which always returns "", making their global-admin branch permanently
// unreachable but harmless): here, "unknown role" must deny, because these
// are the RPCs that actually enforce something (approve/reject a dev
// server, grant department/team access) — a silently-inert admin check
// would be a silent security hole, not a harmless no-op.
func requireAdmin(ctx context.Context) error {
	role, ok := tenant.Role(ctx)
	if !ok || role != "admin" {
		return apperrors.New(apperrors.KindPermissionDenied, "INFRA_NOT_ADMIN", "caller is not an admin", nil)
	}
	return nil
}
