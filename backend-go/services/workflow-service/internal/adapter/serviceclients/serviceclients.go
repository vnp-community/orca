// Package serviceclients holds workflow-service's outbound gRPC adapters
// toward project-service, git-gateway-service, and automation-service —
// three new dependency edges CleanupWorktreesStepExecutor (BL-AT-04) needs
// that this service didn't have before. Kept in one package (rather than
// three, one per remote service) since each client is a single-method thin
// wrapper — splitting further would add package-boilerplate without a
// matching increase in cohesion.
package serviceclients

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"

	automationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/automation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"

	"github.com/stablyai/orca-go/services/workflow-service/internal/usecase"
)

// Dial opens a gRPC client connection to addr. The dial is lazy
// (grpc.NewClient doesn't block on connect), so the remote service being
// down doesn't fail workflow-service's startup — mirrors
// infrafleetclient.Dial. Insecure transport credentials — local-dev only,
// see that function's doc comment for the production mTLS gap this shares.
func Dial(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// withTenantMetadata stamps the caller's already-validated tenant ID onto
// ctx as outbound gRPC metadata — see infrafleetclient/tenant_forwarding.go's
// identical helper for the full rationale (workflow-service only forwards
// what its own inbound TenantExtractionInterceptor already resolved, never
// invents or re-validates a tenant).
func withTenantMetadata(ctx context.Context) (context.Context, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, grpcmw.MetadataTenantID, tenantID), nil
}

// ProjectClient implements usecase.ProjectClient against project-service.
type ProjectClient struct {
	client projectv1.ProjectServiceClient
}

func NewProjectClient(client projectv1.ProjectServiceClient) *ProjectClient {
	return &ProjectClient{client: client}
}

func (c *ProjectClient) ListWorktrees(ctx context.Context, projectID string, statusIn []string, olderThan time.Time) ([]usecase.WorktreeInfo, error) {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.ListWorktrees(ctx, &projectv1.ListWorktreesRequest{
		ProjectId: projectID,
		StatusIn:  statusIn,
		OlderThan: timestamppb.New(olderThan),
	})
	if err != nil {
		return nil, err
	}
	out := make([]usecase.WorktreeInfo, 0, len(resp.GetWorktrees()))
	for _, wt := range resp.GetWorktrees() {
		out = append(out, usecase.WorktreeInfo{ID: wt.GetId(), Branch: wt.GetBranch()})
	}
	return out, nil
}

// GitGatewayClient implements usecase.GitGatewayClient against
// git-gateway-service.
type GitGatewayClient struct {
	client gitgatewayv1.GitGatewayServiceClient
}

func NewGitGatewayClient(client gitgatewayv1.GitGatewayServiceClient) *GitGatewayClient {
	return &GitGatewayClient{client: client}
}

func (c *GitGatewayClient) RemoveWorktree(ctx context.Context, worktreeID string, force, allowOpenPR bool) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	_, err = c.client.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{
		WorktreeId:  worktreeID,
		Force:       force,
		AllowOpenPr: allowOpenPR,
	})
	if err != nil {
		// A FailedPrecondition status means git-gateway-service rejected
		// the removal on BR-AT-11/BR-AT-12 — translate to the
		// transport-agnostic sentinel usecase.CleanupWorktreesStepExecutor
		// checks for, so a caught-and-recorded "skip" is distinguishable
		// from a genuine transport/removal failure without usecase/
		// importing grpc codes.
		if status.Code(err) == codes.FailedPrecondition {
			return fmt.Errorf("%w: %v", usecase.ErrWorktreeRemovalBlocked, err)
		}
		return err
	}
	return nil
}

// CleanupAuditClient implements usecase.CleanupAuditWriter against
// automation-service's WriteCleanupReport RPC.
type CleanupAuditClient struct {
	client automationv1.AutomationServiceClient
}

func NewCleanupAuditClient(client automationv1.AutomationServiceClient) *CleanupAuditClient {
	return &CleanupAuditClient{client: client}
}

func (c *CleanupAuditClient) WriteCleanupReport(ctx context.Context, runID string, entries []usecase.CleanupEntry) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}
	protoEntries := make([]*automationv1.CleanupLogEntry, 0, len(entries))
	for _, e := range entries {
		protoEntries = append(protoEntries, &automationv1.CleanupLogEntry{WorktreeId: e.WorktreeID, Action: e.Action, Reason: e.Reason})
	}
	_, err = c.client.WriteCleanupReport(ctx, &automationv1.WriteCleanupReportRequest{RunId: runID, Entries: protoEntries})
	return err
}
