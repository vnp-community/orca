package grpc

import (
	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/adapter/broadcaster"
	"github.com/stablyai/orca-go/services/code-intel-service/internal/usecase"
)

// SetBroadcasterAndAuthz attaches push broadcaster and authorizer to the server.
func (s *CodeIntelServer) SetBroadcasterAndAuthz(b *broadcaster.CodeIntelPushBroadcaster, authz *usecase.PushAuthorization) {
	s.broadcaster = b
	s.pushAuthz = authz
}

// StreamCodeIntelEvents streams push events to clients (TASK-024-08).
func (s *CodeIntelServer) StreamCodeIntelEvents(
	req *codeintelv1.StreamCodeIntelEventsRequest,
	stream codeintelv1.CodeIntelService_StreamCodeIntelEventsServer,
) error {
	ctx := stream.Context()
	tenantID, ok := tenant.TenantID(ctx)
	if !ok || tenantID == "" {
		if req != nil && len(req.GetSelectors()) > 0 && req.GetSelectors()[0].GetProjectId() != "" {
			tenantID = req.GetSelectors()[0].GetProjectId()
		} else {
			return apperrors.ToGRPCStatus(apperrors.New(apperrors.KindUnauthenticated, "UNAUTHENTICATED", "missing tenant identity", nil))
		}
	}

	if s.pushAuthz != nil && req != nil && len(req.GetSelectors()) > 0 {
		if err := s.pushAuthz.AuthorizeSelectors(ctx, tenantID, req.GetSelectors()); err != nil {
			return apperrors.ToGRPCStatus(err)
		}
	}

	if s.broadcaster == nil {
		return nil
	}

	// Build filter set for requested worktrees
	filterMap := make(map[string]bool)
	hasFilter := false
	if req != nil && len(req.GetSelectors()) > 0 {
		hasFilter = true
		for _, sel := range req.GetSelectors() {
			filterMap[sel.WorktreeRef] = true
		}
	}

	ch, unsubscribe := s.broadcaster.Subscribe()
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case push, ok := <-ch:
			if !ok {
				return nil
			}
			if push == nil {
				continue
			}
			// Filter by selector if requested
			if hasFilter && !filterMap[push.WorktreeId] {
				continue
			}
			// If empty selector, check individual worktree permission
			if !hasFilter && s.pushAuthz != nil {
				if !s.pushAuthz.CanRead(ctx, tenantID, push.ProjectId, push.WorktreeId) {
					continue
				}
			}

			if err := stream.Send(push); err != nil {
				return err
			}
		}
	}
}
