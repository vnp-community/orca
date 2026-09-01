package domain

import "errors"

var (
	// ErrEmptyDevServerGroupTenant mirrors ErrEmptyDevServerTenant — a group
	// with no owning tenant is never a valid domain state.
	ErrEmptyDevServerGroupTenant = errors.New("domain: tenant_id is required")
	// ErrEmptyDevServerGroupName guards against an unnamed group — nothing
	// in the UI could ever refer to it meaningfully.
	ErrEmptyDevServerGroupName = errors.New("domain: name is required")
)

// DevServerGroup is a tenant-scoped, optionally-hierarchical folder for
// organizing DevServers — see
// docs/crs/v2/dev-server/CR-DS-006-dev-server-approval-and-grouping.md §3.2.
// Modeled after project-service's ProjectGroup (same parent_group_id
// self-reference shape) but kept as its own table/type: DevServerGroup and
// ProjectGroup organize different entities in different services, and nothing
// here should create a cross-service FK.
type DevServerGroup struct {
	ID            string
	TenantID      string
	Name          string
	ParentGroupID string // empty = root of the tree
}

// NewDevServerGroup constructs a DevServerGroup, enforcing the same
// non-empty tenant/name invariants NewDevServer enforces for its own fields.
// parentGroupID is not validated for existence here — the repository layer
// enforces the FK; a caller passing a stale/wrong id gets a Postgres FK
// violation, not a silent no-op.
func NewDevServerGroup(id, tenantID, name, parentGroupID string) (DevServerGroup, error) {
	if tenantID == "" {
		return DevServerGroup{}, ErrEmptyDevServerGroupTenant
	}
	if name == "" {
		return DevServerGroup{}, ErrEmptyDevServerGroupName
	}
	return DevServerGroup{ID: id, TenantID: tenantID, Name: name, ParentGroupID: parentGroupID}, nil
}
