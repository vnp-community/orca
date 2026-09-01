package domain

import "errors"

// GranteeKind is who a DevServerGroupGrant grants access to — mirrors
// orca.infrafleet.v1.DevServerGroupGranteeKind. See
// docs/crs/v2/dev-server/CR-DS-007-department-based-access-control.md.
type GranteeKind string

const (
	GranteeKindDepartment GranteeKind = "department"
	GranteeKindTeam       GranteeKind = "team"
)

// Valid reports whether k is one of the known enum values.
func (k GranteeKind) Valid() bool {
	switch k {
	case GranteeKindDepartment, GranteeKindTeam:
		return true
	default:
		return false
	}
}

var (
	ErrEmptyGrantTenant   = errors.New("domain: tenant_id is required")
	ErrEmptyGrantGroupID  = errors.New("domain: dev_server_group_id is required")
	ErrInvalidGranteeKind = errors.New("domain: invalid grantee kind")
	ErrEmptyGranteeID     = errors.New("domain: grantee_id is required")
)

// DevServerGroupGrant grants every member of a department or team access to
// every DevServer in a DevServerGroup (and, per usecase.ListDevServersForUser's
// doc comment, every descendant group in that group's tree). grantee_id is
// a logical FK into tenant-service's departments/teams — never validated
// for existence here (this service has no dependency on tenant-service; see
// CR-DS-007 §2's "resolve at the edge" note).
type DevServerGroupGrant struct {
	ID               string
	TenantID         string
	DevServerGroupID string
	GranteeKind      GranteeKind
	GranteeID        string
}

// NewDevServerGroupGrant constructs a DevServerGroupGrant, enforcing the
// same non-empty-required-field invariant pattern as NewDevServer/
// NewDevServerGroup.
func NewDevServerGroupGrant(id, tenantID, groupID string, kind GranteeKind, granteeID string) (DevServerGroupGrant, error) {
	if tenantID == "" {
		return DevServerGroupGrant{}, ErrEmptyGrantTenant
	}
	if groupID == "" {
		return DevServerGroupGrant{}, ErrEmptyGrantGroupID
	}
	if !kind.Valid() {
		return DevServerGroupGrant{}, ErrInvalidGranteeKind
	}
	if granteeID == "" {
		return DevServerGroupGrant{}, ErrEmptyGranteeID
	}
	return DevServerGroupGrant{
		ID: id, TenantID: tenantID, DevServerGroupID: groupID,
		GranteeKind: kind, GranteeID: granteeID,
	}, nil
}
