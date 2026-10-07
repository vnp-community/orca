package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type ListBacklogRequestsInput struct {
	ProjectID  string
	Types      []string
	Categories []string
	PageToken  string
	PageSize   int
}

type ListBacklogRequests struct {
	Reader BacklogRequestReader
}

func (uc *ListBacklogRequests) Execute(ctx context.Context, in ListBacklogRequestsInput) ([]domain.BacklogRequestRow, string, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, "", err
	}

	limit := in.PageSize
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	filter := BacklogRequestFilter{
		ProjectID:  in.ProjectID,
		Types:      in.Types,
		Categories: in.Categories,
		Limit:      limit,
	}

	if in.PageToken != "" {
		t, i, err := domain.DecodePageToken(in.PageToken)
		if err != nil {
			return nil, "", apperrors.New(apperrors.KindInvalidArgument, "REQUEST_BACKLOG_BAD_PAGE_TOKEN", "invalid page token", err)
		}
		filter.Cursor = &Cursor{UpdatedAt: t, ID: i}
	}

	reqs, err := uc.Reader.ListReturnedRequests(ctx, tenantID, filter)
	if err != nil {
		return nil, "", err
	}

	var nextToken string
	if len(reqs) > limit {
		lastReq := reqs[limit-1]
		nextToken = domain.EncodePageToken(lastReq.UpdatedAt, lastReq.ID)
		reqs = reqs[:limit]
	}

	var reqIDs []string
	for _, r := range reqs {
		reqIDs = append(reqIDs, r.ID)
	}

	parentsMap, err := uc.Reader.ParentRequestIDs(ctx, tenantID, reqIDs)
	if err != nil {
		return nil, "", err
	}

	latestReturns, err := uc.Reader.LatestReturns(ctx, tenantID, reqIDs)
	if err != nil {
		return nil, "", err
	}

	var rows []domain.BacklogRequestRow
	for _, r := range reqs {
		row := domain.BacklogRequestRow{
			Request:          r,
			ParentRequestIDs: parentsMap[r.ID],
		}
		if ev, ok := latestReturns[r.ID]; ok {
			row.ReturnedBy = ev.ActorID
			row.ReturnedAt = ev.At
		}
		rows = append(rows, row)
	}

	return rows, nextToken, nil
}
