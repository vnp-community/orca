package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ClarificationView is a clarification plus what the caller may see of its answers.
type ClarificationView struct {
	Clarification domain.Clarification
	RequestNumber int64
	// AnswersVisible is false for readers who are neither assignee, asker nor admin: answers can hold sensitive detail.
	AnswersVisible bool
}

// UserTeamsResolver lists the teams a user belongs to (tenant-service in production; nil means no team assignments match).
type UserTeamsResolver interface {
	TeamsForUser(ctx context.Context, userID string) ([]string, error)
}

// ClarificationQueries serves the read RPCs.
type ClarificationQueries struct {
	repo           RequestRepository
	clarifications ClarificationRepository
	teams          TeamMembershipResolver
	userTeams      UserTeamsResolver
}

func NewClarificationQueries(repo RequestRepository, clarifications ClarificationRepository) *ClarificationQueries {
	return &ClarificationQueries{repo: repo, clarifications: clarifications}
}

func (q *ClarificationQueries) WithTeams(t TeamMembershipResolver) *ClarificationQueries {
	q.teams = t
	return q
}

func (q *ClarificationQueries) WithUserTeams(t UserTeamsResolver) *ClarificationQueries {
	q.userTeams = t
	return q
}

func (q *ClarificationQueries) view(ctx context.Context, r domain.Request, c domain.Clarification) ClarificationView {
	return ClarificationView{Clarification: c, RequestNumber: r.Number, AnswersVisible: q.answersVisible(ctx, r, c)}
}

func (q *ClarificationQueries) answersVisible(ctx context.Context, r domain.Request, c domain.Clarification) bool {
	if callerIsAdmin(ctx) {
		return true
	}
	me := callerID(ctx)
	if me == "" {
		return false
	}
	if me == c.CreatedBy {
		return true
	}
	probe := &AnswerClarification{teams: q.teams}
	return probe.canAnswer(ctx, r, c)
}

func (q *ClarificationQueries) Get(ctx context.Context, id string) (ClarificationView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ClarificationView{}, domain.ErrRequestTenantRequired()
	}
	c, err := q.clarifications.Get(ctx, id)
	if err != nil {
		return ClarificationView{}, err
	}
	r, err := q.repo.Get(ctx, c.RequestID)
	if err != nil {
		return ClarificationView{}, domain.ErrClarificationNotFound(id)
	}
	return q.view(ctx, r, c), nil
}

type ClarificationPage struct {
	Items     []ClarificationView
	NextAfter int // seq to continue from, 0 when finished
}

func (q *ClarificationQueries) List(ctx context.Context, requestID string, status domain.ClarificationStatus, afterSeq, pageSize int) (ClarificationPage, error) {
	r, err := loadReadableRequest(ctx, q.repo, requestID)
	if err != nil {
		return ClarificationPage{}, err
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	got, err := q.clarifications.List(ctx, ClarificationListFilter{RequestID: requestID, Status: status, AfterSeq: afterSeq, Limit: pageSize + 1})
	if err != nil {
		return ClarificationPage{}, err
	}
	page := ClarificationPage{}
	if len(got) > pageSize {
		got = got[:pageSize]
		page.NextAfter = got[pageSize-1].Seq
	}
	for _, c := range got {
		page.Items = append(page.Items, q.view(ctx, r, c))
	}
	return page, nil
}

type PendingPage struct {
	Items         []ClarificationView
	NextPageToken string
}

// ListPending lists what the caller can answer, using the caller's own id, role and teams.
func (q *ClarificationQueries) ListPending(ctx context.Context, pageSize int, pageToken string) (PendingPage, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return PendingPage{}, domain.ErrRequestTenantRequired()
	}
	me := callerID(ctx)
	if me == "" {
		return PendingPage{}, errRequestForbidden("a user identity is required")
	}
	var teams []string
	if q.userTeams != nil {
		var err error
		if teams, err = q.userTeams.TeamsForUser(ctx, me); err != nil {
			return PendingPage{}, err
		}
	}
	role, _ := tenant.Role(ctx)
	var roles []string
	if role != "" {
		roles = []string{role}
	}
	got, next, err := q.clarifications.ListPendingForUser(ctx, PendingFilter{
		UserID: me, Teams: teams, Roles: roles, IsAdmin: callerIsAdmin(ctx), PageSize: pageSize, PageToken: pageToken,
	})
	if err != nil {
		return PendingPage{}, err
	}
	page := PendingPage{NextPageToken: next}
	for _, c := range got {
		r, err := q.repo.Get(ctx, c.RequestID)
		if err != nil {
			continue // the request vanished (erased); nothing to answer
		}
		page.Items = append(page.Items, q.view(ctx, r, c))
	}
	return page, nil
}
