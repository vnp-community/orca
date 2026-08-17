// Package domain holds project-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework.
package domain

import "errors"

var (
	// ErrEmptyTenantID is returned by NewProject when TenantID is empty — a
	// project with no owning tenant is never a valid domain state.
	ErrEmptyTenantID = errors.New("domain: tenant_id is required")
	// ErrEmptyName is returned by NewProject when Name is empty.
	ErrEmptyName = errors.New("domain: name is required")
	// ErrEmptyDevServerID is returned by Project.Rebind when the caller tries
	// to rebind to an empty dev server id — RebindDevServer must always move
	// to a concrete target, never "unbind".
	ErrEmptyDevServerID = errors.New("domain: dev_server_id is required for rebind")
	// ErrProjectNotFound is the sentinel a Repository implementation returns
	// (wrapped, per errors.Is convention) when a project doesn't exist for
	// the given tenant — usecase/ maps this to apperrors.KindNotFound.
	ErrProjectNotFound = errors.New("domain: project not found")
)

// Project is a workspace-organization record: which dev server a tenant's
// project is currently bound to. See project-service.md §4 — this scaffold
// carries the id/tenant_id/name/dev_server_id slice of the full design-doc
// model that the current proto surface (CreateProject/GetProject/
// ListProjects/AddMember/RebindDevServer) actually exercises; the richer
// shape (description, visibility, repos, worktrees, ...) is a documented
// follow-up, not implemented here.
type Project struct {
	ID       string
	TenantID string
	Name     string
	// DevServerID is empty until the first RebindDevServer call — unlike the
	// full design doc (dev_server_id "never empty after create"), this
	// service's CreateProject RPC doesn't accept a dev_server_id, so a freshly
	// created project starts unbound.
	DevServerID string
}

// NewProject constructs a Project, enforcing the invariants a record must
// satisfy to be meaningful — this is where "project-service owns this data's
// correctness" actually lives, not scattered validation in the gRPC handler.
func NewProject(id, tenantID, name, devServerID string) (Project, error) {
	if tenantID == "" {
		return Project{}, ErrEmptyTenantID
	}
	if name == "" {
		return Project{}, ErrEmptyName
	}
	return Project{ID: id, TenantID: tenantID, Name: name, DevServerID: devServerID}, nil
}

// Rebind returns a copy of p pointed at a new dev server — a pure,
// side-effect-free domain operation. The active-execution guard that decides
// whether a rebind is *allowed* lives in usecase.RebindDevServer, not here:
// this method only enforces the shape invariant (never rebind to empty).
func (p Project) Rebind(newDevServerID string) (Project, error) {
	if newDevServerID == "" {
		return Project{}, ErrEmptyDevServerID
	}
	p.DevServerID = newDevServerID
	return p, nil
}
