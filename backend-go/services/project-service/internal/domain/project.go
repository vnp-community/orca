// Package domain holds project-service's entities and value objects. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md, this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework.
package domain

import (
	"errors"
	"time"
)

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
	// ErrInvalidVisibility is returned when a visibility string isn't one of
	// the closed private|team|department|company enum — mirrors the
	// project.projects visibility CHECK constraint at the domain layer so a
	// bad value surfaces as apperrors.KindInvalidArgument, not a raw SQL
	// constraint-violation error.
	ErrInvalidVisibility = errors.New("domain: visibility must be one of private, team, department, company")
	// ErrProjectNotFound is the sentinel a Repository implementation returns
	// (wrapped, per errors.Is convention) when a project doesn't exist for
	// the given tenant — usecase/ maps this to apperrors.KindNotFound.
	ErrProjectNotFound = errors.New("domain: project not found")
)

// DefaultBranch and DefaultVisibility are applied by usecase.CreateProject
// when the corresponding request field is empty — matching the database
// column defaults in migrations/0002 (belt-and-suspenders: the domain layer
// is the source of truth callers observe before any row is written).
const (
	DefaultBranch     = "main"
	DefaultVisibility = "private"
)

// Visibility enum values — see project.proto's Project.visibility doc
// comment. Kept as plain strings (not a distinct Go type) on Project itself
// so the postgres/gRPC boundary layers don't need a conversion step; Valid
// below is the single place the closed set is enforced.
const (
	VisibilityPrivate    = "private"
	VisibilityTeam       = "team"
	VisibilityDepartment = "department"
	VisibilityCompany    = "company"
)

// ValidVisibility reports whether v is one of the closed enum values.
func ValidVisibility(v string) bool {
	switch v {
	case VisibilityPrivate, VisibilityTeam, VisibilityDepartment, VisibilityCompany:
		return true
	default:
		return false
	}
}

// Project is a workspace-organization record: which dev server a tenant's
// project is currently bound to, plus the descriptive/ownership metadata
// added alongside UpdateProject/DeleteProject (see project-service.md §4).
type Project struct {
	ID       string
	TenantID string
	Name     string
	// DevServerID is empty until the first RebindDevServer call — unlike the
	// full design doc (dev_server_id "never empty after create"), this
	// service's CreateProject RPC doesn't accept a dev_server_id, so a freshly
	// created project starts unbound.
	DevServerID string
	Description string
	// DefaultBranch/Visibility default to DefaultBranch/DefaultVisibility
	// when CreateProject's request leaves them empty — never left blank in
	// a persisted row.
	DefaultBranch string
	Visibility    string
	// CreatedBy is the authenticated caller's user id at creation time
	// (common/tenant, populated by the gRPC auth interceptor) — never
	// trusted from the request body. Logical FK -> tenant-service user id.
	CreatedBy string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ProjectUpdatePatch carries UpdateProject's field-mask semantics: an empty
// string means "leave unchanged", per project.proto's UpdateProjectRequest
// doc comment. Deliberately has no DevServerID field — RebindDevServer (with
// its active-execution guard) stays the sole path that may change it.
type ProjectUpdatePatch struct {
	Name          string
	Description   string
	DefaultBranch string
	Visibility    string
}

// NewProject constructs a Project, enforcing the invariants a record must
// satisfy to be meaningful — this is where "project-service owns this data's
// correctness" actually lives, not scattered validation in the gRPC handler.
// Description/DefaultBranch/Visibility/CreatedBy are set by the caller
// (usecase.CreateProject) after construction, not accepted here — NewProject
// only enforces the shape invariants that predate those fields, keeping this
// constructor's signature stable for existing callers.
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

// ProjectContext is GetProjectContext's read-only view — a subset of
// Project plus a best-effort-resolved dev server hostname, per
// project-service.md §2's Boundary decision.
type ProjectContext struct {
	ProjectID, ProjectName, Description     string
	RepoURL, DevServerID, DevServerHostname string
}
