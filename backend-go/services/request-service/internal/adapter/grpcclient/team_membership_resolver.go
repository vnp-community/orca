package grpcclient

import (
	"context"
	"fmt"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	tenantv1 "github.com/stablyai/orca-go/proto/gen/go/orca/tenant/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

// TeamMembershipResolver answers team questions from tenant-service. ListTeamsForUser and ListTeamMembers take
// no tenant_id; the tenant travels in call metadata (withTenantMetadata), never in the request body.
type TeamMembershipResolver struct {
	client tenantv1.TenantServiceClient
	cache  *directoryCache
}

var _ usecase.TeamMembershipResolver = (*TeamMembershipResolver)(nil)

func NewTeamMembershipResolver(client tenantv1.TenantServiceClient, ttl time.Duration) *TeamMembershipResolver {
	return &TeamMembershipResolver{client: client, cache: newDirectoryCache(ttl)}
}

func (r *TeamMembershipResolver) TeamsForUser(ctx context.Context, userID string) ([]string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	key := "teams-of|" + tenantID + "|" + userID
	if v, ok := r.cache.get(key); ok {
		return v, nil
	}
	callCtx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.ListTeamsForUser(callCtx, &tenantv1.ListTeamsForUserRequest{UserId: userID})
	if err != nil {
		return nil, directoryUnavailable("list teams for user", err)
	}
	r.cache.put(key, resp.GetTeamIds())
	return resp.GetTeamIds(), nil
}

func (r *TeamMembershipResolver) MembersOfTeam(ctx context.Context, teamID string) ([]string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	key := "members-of|" + tenantID + "|" + teamID
	if v, ok := r.cache.get(key); ok {
		return v, nil
	}
	callCtx, err := withTenantMetadata(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.ListTeamMembers(callCtx, &tenantv1.ListTeamMembersRequest{TeamId: teamID})
	if err != nil {
		return nil, directoryUnavailable("list team members", err)
	}
	ids := make([]string, 0, len(resp.GetMembers()))
	for _, m := range resp.GetMembers() {
		ids = append(ids, m.GetUserId())
	}
	r.cache.put(key, ids)
	return ids, nil
}

// directoryUnavailable keeps the stable code for callers and the cause for logs.
func directoryUnavailable(op string, cause error) error {
	e := *domain.ErrApprovalDirectoryUnavailable
	e.Err = fmt.Errorf("%s: %w", op, cause)
	return &e
}

// UnavailableDirectory is bound when the directory address is not configured: every lookup fails closed.
type UnavailableDirectory struct{}

var (
	_ usecase.TeamMembershipResolver = UnavailableDirectory{}
	_ usecase.AdminDirectoryResolver = UnavailableDirectory{}
)

func (UnavailableDirectory) TeamsForUser(context.Context, string) ([]string, error) {
	return nil, domain.ErrApprovalDirectoryUnavailable
}

func (UnavailableDirectory) MembersOfTeam(context.Context, string) ([]string, error) {
	return nil, domain.ErrApprovalDirectoryUnavailable
}

func (UnavailableDirectory) ListAdmins(context.Context, string) ([]string, error) {
	return nil, domain.ErrApprovalDirectoryUnavailable
}
