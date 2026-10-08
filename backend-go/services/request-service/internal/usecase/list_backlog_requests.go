package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

const (
	defaultBacklogPageSize = 20
	maxBacklogPageSize     = 100
)

type ListBacklogRequestsInput struct {
	ProjectID  string
	Types      []string
	Categories []string
	PageToken  string
	PageSize   int
}

// ListBacklogRequests is the REQUEST view: Requests waiting in request_backlog. It never calls task-service.
type ListBacklogRequests struct {
	Reader     BacklogRequestReader
	Visibility RequestVisibility
}

func clampBacklogPageSize(n int) int {
	switch {
	case n <= 0:
		return defaultBacklogPageSize
	case n > maxBacklogPageSize:
		return maxBacklogPageSize
	}
	return n
}

func decodeBacklogCursor(token string) (*Cursor, error) {
	if token == "" {
		return nil, nil
	}
	at, id, err := domain.DecodePageToken(token)
	if err != nil {
		return nil, apperrors.New(apperrors.KindInvalidArgument, "REQUEST_BACKLOG_BAD_PAGE_TOKEN", "invalid page token", err)
	}
	return &Cursor{UpdatedAt: at, ID: id}, nil
}

// pageOf trims the extra row the reader fetched to learn whether another page exists.
func pageOf(reqs []domain.Request, limit int) (page []domain.Request, next string) {
	if len(reqs) > limit {
		last := reqs[limit-1]
		return reqs[:limit], domain.EncodePageToken(last.UpdatedAt, last.ID)
	}
	return reqs, ""
}

func (uc *ListBacklogRequests) Execute(ctx context.Context, in ListBacklogRequestsInput) ([]domain.BacklogRequestRow, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", err
	}
	limit := clampBacklogPageSize(in.PageSize)
	cursor, err := decodeBacklogCursor(in.PageToken)
	if err != nil {
		return nil, "", err
	}
	filter := BacklogRequestFilter{
		ProjectID: in.ProjectID, Types: in.Types, Categories: in.Categories, Cursor: cursor, Limit: limit,
	}
	if scope, ok := ProjectScope(ctx); ok && in.ProjectID == "" {
		if len(scope) == 0 {
			return nil, "", nil // a member of no project sees no backlog.
		}
		filter.ProjectIDs = scope
	}
	reqs, err := uc.Reader.ListReturnedRequests(ctx, tenantID, filter)
	if err != nil {
		return nil, "", err
	}
	page, next := pageOf(reqs, limit)
	visible, err := filterVisible(ctx, uc.Visibility, page)
	if err != nil {
		return nil, "", err
	}
	ids := make([]string, 0, len(visible))
	for _, r := range visible {
		ids = append(ids, r.ID)
	}
	parents, err := uc.Reader.ParentRequestIDs(ctx, tenantID, ids)
	if err != nil {
		return nil, "", err
	}
	returns, err := uc.Reader.LatestReturns(ctx, tenantID, ids)
	if err != nil {
		return nil, "", err
	}
	rows := make([]domain.BacklogRequestRow, 0, len(visible))
	for _, r := range visible {
		row := domain.BacklogRequestRow{Request: r, ParentRequestIDs: parents[r.ID]}
		if ev, ok := returns[r.ID]; ok {
			row.ReturnedBy, row.ReturnedAt = ev.ActorID, ev.At
		}
		rows = append(rows, row)
	}
	return rows, next, nil
}

// filterVisible applies the caller's view rights; a missing policy is an error, never "show everything".
func filterVisible(ctx context.Context, v RequestVisibility, reqs []domain.Request) ([]domain.Request, error) {
	if v == nil {
		return nil, apperrors.New(apperrors.KindInternal, "REQUEST_BACKLOG_NO_VISIBILITY_POLICY", "request visibility policy is not wired", nil)
	}
	userID, _ := tenant.UserID(ctx)
	role, _ := tenant.Role(ctx)
	return v.Filter(ctx, domain.DecisionActor{UserID: userID, Role: role, IsMachine: tenant.ActorType(ctx) == tenant.ActorAgent}, reqs)
}
