package domain

import "errors"

// AccessRequestStatus is the lifecycle state of a DevServerAccessRequest —
// mirrors orca.infrafleet.v1.DevServerAccessRequestStatus. See
// docs/crs/v2/dev-server/CR-DS-008-first-login-department-gate-and-access-request.md.
type AccessRequestStatus string

const (
	AccessRequestStatusPending  AccessRequestStatus = "pending"
	AccessRequestStatusApproved AccessRequestStatus = "approved"
	AccessRequestStatusRejected AccessRequestStatus = "rejected"
)

func (s AccessRequestStatus) Valid() bool {
	switch s {
	case AccessRequestStatusPending, AccessRequestStatusApproved, AccessRequestStatusRejected:
		return true
	default:
		return false
	}
}

var (
	ErrEmptyAccessRequestTenant  = errors.New("domain: tenant_id is required")
	ErrEmptyAccessRequestUser    = errors.New("domain: user_id is required")
	ErrEmptyAccessRequestGroupID = errors.New("domain: dev_server_group_id is required")
)

// DevServerAccessRequest is a user's ask to be granted access to a
// DevServerGroup — see CR-DS-008 §2.3. GranteeKind/GranteeID are captured
// at creation time (who the resulting grant, if approved, applies to) —
// see the proto message's doc comment for why this isn't re-derived at
// resolve time.
type DevServerAccessRequest struct {
	ID               string
	TenantID         string
	UserID           string
	DevServerGroupID string
	Status           AccessRequestStatus
	Message          string
	GranteeKind      GranteeKind
	GranteeID        string
	CreatedAtUnixMs  int64
}

// NewDevServerAccessRequest constructs a DevServerAccessRequest, always
// starting at AccessRequestStatusPending — a request is never created
// pre-resolved.
func NewDevServerAccessRequest(id, tenantID, userID, groupID, message string, kind GranteeKind, granteeID string, createdAtUnixMs int64) (DevServerAccessRequest, error) {
	if tenantID == "" {
		return DevServerAccessRequest{}, ErrEmptyAccessRequestTenant
	}
	if userID == "" {
		return DevServerAccessRequest{}, ErrEmptyAccessRequestUser
	}
	if groupID == "" {
		return DevServerAccessRequest{}, ErrEmptyAccessRequestGroupID
	}
	if !kind.Valid() {
		return DevServerAccessRequest{}, ErrInvalidGranteeKind
	}
	if granteeID == "" {
		return DevServerAccessRequest{}, ErrEmptyGranteeID
	}
	return DevServerAccessRequest{
		ID: id, TenantID: tenantID, UserID: userID, DevServerGroupID: groupID,
		Status: AccessRequestStatusPending, Message: message,
		GranteeKind: kind, GranteeID: granteeID, CreatedAtUnixMs: createdAtUnixMs,
	}, nil
}
