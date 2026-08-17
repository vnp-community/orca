// Package usecase holds task-service's application services and the ports
// they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

// TaskRepository is the persistence port for tasks. Implemented by
// internal/adapter/postgres against task-service's own database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
type TaskRepository interface {
	Create(ctx context.Context, task domain.Task) (domain.Task, error)
	Get(ctx context.Context, tenantID, id string) (domain.Task, error)
	// GetAncestors returns the chain from the given task up to its root,
	// walking tasks.parent_id (task-service.md §6's WITH RECURSIVE note).
	// The first element is the task itself; the last is the root. maxDepth
	// bounds the walk (task-service.md §8's max-depth guard) — 0 means the
	// repository's own default cap.
	GetAncestors(ctx context.Context, tenantID, id string, maxDepth int) ([]domain.Task, error)
	// UpdateStatus persists a task's status transition. Currently only ever
	// called to set StatusInProgress on dispatch (see ExecuteTask) — there is
	// no RPC surface yet to transition a task back out of in_progress. See
	// this service's README "Known gaps".
	UpdateStatus(ctx context.Context, tenantID, id, status string) error
	// HasActiveExecutions reports whether tenantID/projectID has any task
	// currently in_progress — see usecase.HasActiveExecutions's doc comment
	// for the one-way-transition caveat this answer is subject to today.
	HasActiveExecutions(ctx context.Context, tenantID, projectID string) (bool, error)
}

// EdgeRepository is the persistence port for task_edges rows. Cycle
// detection is NOT this port's job — the AddEdge usecase calls
// domain.DetectCycle itself, using edges this port fetched, per Clean
// Architecture's dependency rule (domain logic never lives in an adapter).
type EdgeRepository interface {
	// Add persists a single edge. Callers (the AddEdge usecase) are
	// responsible for running cycle detection before calling this for a
	// depends_on edge — see task-service.md §8's "Edge-mutation
	// consistency" note and this service's README for the known gap this
	// scaffold has around doing the check-then-write atomically.
	Add(ctx context.Context, tenantID string, edge domain.TaskEdge) error
	// ListByKind returns every edge of the given kind for the tenant — the
	// graph AddEdge's cycle check walks. Fetching the whole kind-scoped
	// edge set (rather than only the reachable subgraph) keeps this port's
	// contract simple; see this service's README for the scale follow-up.
	ListByKind(ctx context.Context, tenantID string, kind domain.EdgeKind) ([]domain.TaskEdge, error)
	// ListFrom returns the edges of the given kind originating at
	// fromTaskID — used by the Execute usecase's complexity branch (§3.1):
	// a task with any parent_child or depends_on edges from it is
	// "complex".
	ListFrom(ctx context.Context, tenantID, fromTaskID string, kind domain.EdgeKind) ([]domain.TaskEdge, error)
}

// GrantRepository is the persistence port for task_grants rows.
type GrantRepository interface {
	Grant(ctx context.Context, tenantID string, grant domain.Grant) error
	// ListGrantsForAncestors returns every grant recorded against any of
	// taskIDs, grouped by task ID — the input ResolveGrant's BFS walk
	// (domain/grant_resolution.go) consumes.
	ListGrantsForAncestors(ctx context.Context, tenantID string, taskIDs []string) (map[string][]domain.Grant, error)
}

// TeamScopeResolver resolves a user's team memberships by calling
// tenant-service — task-service never reads tenant-service's team_members
// table directly, per task-service.md §2/§9's bounded-context rule.
//
// STUB in this scaffold: internal/adapter/grpcclient's implementation
// always returns an empty team list rather than actually calling
// tenant-service. Team-scoped grants will silently never match until this
// is wired to a real tenant-service client — see this service's README.
type TeamScopeResolver interface {
	ResolveTeams(ctx context.Context, tenantID, userID string) ([]string, error)
}

// OPAClient is the authorization port ResolvePermission uses for the
// "does this resolved level authorize this action" decision (§9's
// domain-computes/OPA-decides split) — implemented by
// internal/adapter/opaclient against the shared embedded OPA evaluator
// (common/policy), consuming
// backend-go/policy/orca-authz/task_grant.rego's
// data.orca.authz.task.allow rule.
type OPAClient interface {
	// Decision reports whether level authorizes action for tenantID, per
	// task_grant.rego's level_actions table. Never called with
	// domain.GrantLevelUnspecified — ResolvePermission.Execute returns its
	// own PermissionDenied before reaching this call when domain.ResolveGrant
	// finds no match.
	Decision(ctx context.Context, level domain.GrantLevel, action, tenantID string) (bool, error)
}

// SimpleExecutor relays Execute's simple-path dispatch to
// infra-fleet-service (-> Dev Server Agent agent.exec), per
// task-service.md §3.1.
//
// STUB in this scaffold: internal/adapter/grpcclient's implementation
// returns a fixed placeholder execution ref without calling
// infra-fleet-service — see this service's README.
type SimpleExecutor interface {
	Execute(ctx context.Context, tenantID, taskID, requestID string) (executionRef string, err error)
}

// ComplexExecutor relays Execute's complex-path dispatch to
// orchestration-service's coordinator, per task-service.md §3.1.
//
// STUB in this scaffold: internal/adapter/grpcclient's implementation
// returns a fixed placeholder execution ref without calling
// orchestration-service — see this service's README.
type ComplexExecutor interface {
	Execute(ctx context.Context, tenantID, taskID, requestID string) (executionRef string, err error)
}
