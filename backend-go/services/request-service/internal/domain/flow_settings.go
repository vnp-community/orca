package domain

import "github.com/stablyai/orca-go/common/apperrors"

// FlowSettings is the per-tenant half of the request_flow_enabled switch (CR-REQ-025).
type FlowSettings struct {
	TenantID string
	Enabled  bool
}

// ErrFlowDisabled is returned for flow-advancing RPCs while the flag is off; also used when the flag
// cannot be read, so an unknown state never lets the flow run (fail closed).
func ErrFlowDisabled() error {
	return apperrors.New(apperrors.KindFailedPrecondition, "REQUEST_FLOW_DISABLED", "the request flow is not enabled for this tenant", nil)
}

func ErrFlowAdminOnly() error {
	return apperrors.New(apperrors.KindPermissionDenied, "REQUEST_FLOW_ADMIN_ONLY", "only a tenant admin can change request flow settings", nil)
}
