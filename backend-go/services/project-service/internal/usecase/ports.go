// Package usecase holds project-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per the
// Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/project-service/internal/domain"
)

// ProjectRepository is the persistence port for projects and project
// membership. Implemented by internal/adapter/postgres against
// project-service's own database — see
// specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
type ProjectRepository interface {
	Create(ctx context.Context, project domain.Project) (domain.Project, error)
	Get(ctx context.Context, tenantID, id string) (domain.Project, error)
	List(ctx context.Context, tenantID, pageToken string, pageSize int32) ([]domain.Project, string, error)
	AddMember(ctx context.Context, member domain.ProjectMember) error
	// UpdateDevServerID rebinds a project to a new dev server — the only
	// write path for dev_server_id (see usecase.RebindDevServer). Returns
	// domain.ErrProjectNotFound (wrapped) if no project matches.
	UpdateDevServerID(ctx context.Context, tenantID, projectID, devServerID string) (domain.Project, error)
}

// WorkflowExecutionChecker is the outbound port toward workflow-service —
// RebindDevServer's guard against rebinding a project with a running
// workflow. Implemented by internal/adapter/grpcclient. See
// project-service.md §3 ("closing the active-execution gap").
type WorkflowExecutionChecker interface {
	HasActiveExecutions(ctx context.Context, projectID string) (bool, error)
}

// TaskExecutionChecker is the outbound port toward task-service — the same
// guard as WorkflowExecutionChecker but for standalone task executions. Kept
// as a separate interface (not merged with WorkflowExecutionChecker) per
// project-service.md §6, so RebindDevServer's test fakes stay independent of
// whichever of the two services changes its contract first.
type TaskExecutionChecker interface {
	HasActiveExecutions(ctx context.Context, projectID string) (bool, error)
}
