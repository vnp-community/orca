package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

type ListGrantsInput struct {
	AllUsers bool   // admin view of every user's grants
	UserID   string // optional filter, AllUsers only
}

type GrantView struct {
	Grant    domain.Grant
	UserName string // best effort, admin listing only
}

// ListGrants returns active grants: the caller's own, or (admin only) the
// whole tenant's. Revoked grants are history and are not listed.
type ListGrants struct {
	repo AuthorizationRepository
	as   AuthorizationServer
}

func NewListGrants(repo AuthorizationRepository, as AuthorizationServer) *ListGrants {
	return &ListGrants{repo: repo, as: as}
}

func (uc *ListGrants) Execute(ctx context.Context, in ListGrantsInput) ([]GrantView, error) {
	who, err := userCaller(ctx)
	if err != nil {
		return nil, err
	}
	filter := who.UserID
	if in.AllUsers {
		if err := who.requireAdmin(); err != nil {
			return nil, err
		}
		filter = in.UserID
	}
	grants, err := uc.repo.ListActiveGrants(ctx, who.TenantID, filter)
	if err != nil {
		return nil, domain.ErrInternal("failed to list grants", err)
	}
	var names map[string]string
	if in.AllUsers && len(grants) > 0 {
		// Names are decoration: a directory outage must not hide the grants.
		names, _ = uc.as.MemberNames(ctx)
	}
	out := make([]GrantView, 0, len(grants))
	for _, g := range grants {
		out = append(out, GrantView{Grant: g, UserName: names[g.UserID]})
	}
	return out, nil
}
