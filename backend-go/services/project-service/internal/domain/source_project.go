package domain

import (
	"errors"
	"time"
)

var (
	// ErrEmptyContainerProjectID/ErrEmptySourceProjectID are NewSourceProject's
	// input-validation errors.
	ErrEmptyContainerProjectID = errors.New("domain: container project id must not be empty")
	ErrEmptySourceProjectID    = errors.New("domain: source project id must not be empty")
	// ErrSelfSourceProject mirrors the DB-level CHECK (container_project_id
	// != source_project_id, migrations/0014) — a project can't share
	// itself into itself.
	ErrSelfSourceProject = errors.New("domain: a project cannot be linked as its own source project")
	// ErrSourceProjectNotFound is the sentinel a SourceProjectRepository
	// implementation returns (wrapped, per errors.Is convention) when no
	// link row exists for the given container/source pair.
	ErrSourceProjectNotFound = errors.New("domain: source project link not found")
)

// SourceProject links source_project_id's repos/worktrees into
// container_project_id's shared view. Both sides are ordinary Project rows
// — there is no separate "OrcaProject" entity in this service (see
// project.proto's orcaProjects.* section doc comment). LinkedBy is an audit
// trail (who performed the link), never an ownership claim on either
// project — access is gated entirely through container_project_id's own
// project_members/OPA check (requireProjectAccess), not through this type.
type SourceProject struct {
	ID                 string
	ContainerProjectID string
	SourceProjectID    string
	LinkedBy           string
	LinkedAt           time.Time
}

// NewSourceProject constructs a SourceProject, enforcing its invariants. id
// is caller-generated (uuid.NewString(), matching usecase.AddRepo's own
// convention) — never left to the Postgres column default, which exists
// only as a belt-and-suspenders fallback.
func NewSourceProject(id, containerProjectID, sourceProjectID, linkedBy string) (SourceProject, error) {
	if containerProjectID == "" {
		return SourceProject{}, ErrEmptyContainerProjectID
	}
	if sourceProjectID == "" {
		return SourceProject{}, ErrEmptySourceProjectID
	}
	if containerProjectID == sourceProjectID {
		return SourceProject{}, ErrSelfSourceProject
	}
	return SourceProject{ID: id, ContainerProjectID: containerProjectID, SourceProjectID: sourceProjectID, LinkedBy: linkedBy}, nil
}
