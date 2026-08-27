package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// ProjectContextResolver implements usecase.ProjectContextResolver by
// dialing project-service's GetProjectContext RPC directly.
type ProjectContextResolver struct {
	client projectv1.ProjectServiceClient
}

func NewProjectContextResolver(client projectv1.ProjectServiceClient) *ProjectContextResolver {
	return &ProjectContextResolver{client: client}
}

func (r *ProjectContextResolver) GetProjectContext(ctx context.Context, projectID string) (usecase.ProjectContext, error) {
	// GetProjectContext is membership-gated on the server side
	// (project-service's requireProjectAccess, projectActionAnyMember) — it
	// needs BOTH tenant AND acting-user identity forwarded as outbound
	// metadata, not just tenant (see withTenantMetadata's doc comment,
	// scoped to infra-fleet-service calls only). The caller (SimpleExecutor)
	// is expected to have already set tenant.WithUserID(ctx, <target user>)
	// before reaching this resolver — see TASK-PRF-04-08.
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return usecase.ProjectContext{}, fmt.Errorf("grpcclient: project context resolver: %w", err)
	}
	userID, _ := tenant.UserID(ctx) // absent -> project-service denies with PROJECT_NO_USER, a legitimate fail-closed outcome
	outCtx := metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID, grpcmw.MetadataUserID, userID)

	resp, err := r.client.GetProjectContext(outCtx, &projectv1.GetProjectContextRequest{ProjectId: projectID})
	if err != nil {
		return usecase.ProjectContext{}, fmt.Errorf("grpcclient: project-service GetProjectContext: %w", err)
	}
	return usecase.ProjectContext{
		ProjectID: resp.GetProjectId(), ProjectName: resp.GetProjectName(), Description: resp.GetDescription(),
		RepoURL: resp.GetRepoUrl(), DevServerID: resp.GetDevServerId(), DevServerHostname: resp.GetDevServerHostname(),
	}, nil
}
