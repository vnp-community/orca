package usecase

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestActivityReader reads the lines of one Request's feed from the tables that already record what happened to it.
type RequestActivityReader interface {
	// ListActivity returns rows newest first, at most q.Limit, after q.Before when set.
	ListActivity(ctx context.Context, requestID string, q domain.ActivityQuery) ([]domain.ActivityRow, error)
}

const (
	defaultActivityPage = 50
	maxActivityPage     = 100
	// activityReadFactor reads this many rows per wanted entry while a severity filter hides some, so one page rarely needs a second read.
	activityReadFactor = 3
	activityMaxReads   = 8
)

type ListRequestActivityInput struct {
	RequestID   string
	PageSize    int
	PageToken   string
	Kinds       []string
	MinSeverity string
	ActorKind   string
	SubjectID   string
	Since       *time.Time
}

type ListRequestActivityResult struct {
	Entries       []domain.ActivityEntry
	NextPageToken string
}

// ListRequestActivity is the "History" of a Request: type changes, analysis, approvals, clarifications, Phase starts, every task
// attempt and the trips to the backlog, newest first. It is a read model over tables the service already owns, so there is no
// event store to keep in step. Who may read the Request decides who may read this: the RPC interceptor checks that.
type ListRequestActivity struct {
	Requests RequestReader
	Activity RequestActivityReader
	// Tasks names the tasks of the Request; without it (or when it fails) lines name them by id.
	Tasks       TaskClient
	MaxAttempts int
	Log         *slog.Logger
}

func (uc *ListRequestActivity) Execute(ctx context.Context, in ListRequestActivityInput) (ListRequestActivityResult, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return ListRequestActivityResult{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(in.RequestID); err != nil {
		return ListRequestActivityResult{}, domain.ErrRequestNotFound(in.RequestID)
	}
	minSev, err := domain.ParseActivitySeverity(in.MinSeverity)
	if err != nil {
		return ListRequestActivityResult{}, err
	}
	kinds, err := cleanActivityKinds(in.Kinds)
	if err != nil {
		return ListRequestActivityResult{}, err
	}
	before, err := domain.DecodeActivityCursor(in.PageToken)
	if err != nil {
		return ListRequestActivityResult{}, err
	}
	size := in.PageSize
	switch {
	case size <= 0:
		size = defaultActivityPage
	case size > maxActivityPage:
		size = maxActivityPage
	}
	req, err := uc.Requests.Get(ctx, in.RequestID)
	if err != nil {
		return ListRequestActivityResult{}, err
	}
	tasks := uc.taskNames(ctx, req)
	maxAttempts := uc.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultExecutionSettings().MaxTaskAttempts
	}

	q := domain.ActivityQuery{Kinds: kinds, ActorKind: in.ActorKind, SubjectID: in.SubjectID, Since: in.Since, Before: before}
	var out []domain.ActivityEntry
	exhausted := false
	for reads := 0; reads < activityMaxReads && len(out) <= size && !exhausted; reads++ {
		q.Limit = (size + 1) * activityReadFactor
		if minSev == domain.ActivityInfo {
			q.Limit = size + 1
		}
		rows, err := uc.Activity.ListActivity(ctx, in.RequestID, q)
		if err != nil {
			return ListRequestActivityResult{}, err
		}
		for _, row := range rows {
			if e := domain.BuildActivityEntry(row, tasks, maxAttempts); e.Severity.AtLeast(minSev) {
				out = append(out, e)
			}
		}
		if len(rows) < q.Limit {
			exhausted = true
			break
		}
		last := rows[len(rows)-1]
		q.Before = &domain.ActivityCursor{At: last.OccurredAt, ID: last.ID}
	}
	res := ListRequestActivityResult{}
	if len(out) > size {
		out = out[:size]
		last := out[len(out)-1]
		res.NextPageToken = domain.ActivityCursor{At: last.OccurredAt, ID: last.ID}.Encode()
	}
	res.Entries = out
	return res, nil
}

func (uc *ListRequestActivity) taskNames(ctx context.Context, req domain.Request) map[string]domain.ActivityTask {
	names := map[string]domain.ActivityTask{}
	if uc.Tasks == nil {
		return names
	}
	tasks, err := uc.Tasks.ListTasks(ctx, ListTasksQuery{RequestIDs: []string{req.ID}, TaskTypes: executionTaskTypes})
	if err != nil {
		uc.log().WarnContext(ctx, "task titles unavailable for the activity feed", slog.String("request_id", req.ID), slog.Any("error", err))
		return names
	}
	for _, t := range tasks {
		names[t.ID] = domain.ActivityTask{Title: t.Title}
	}
	return names
}

func (uc *ListRequestActivity) log() *slog.Logger {
	if uc.Log != nil {
		return uc.Log
	}
	return slog.Default()
}

func cleanActivityKinds(in []string) ([]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	known := make(map[string]bool, len(domain.AllActivityKinds))
	for _, k := range domain.AllActivityKinds {
		known[k] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, k := range in {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] {
			continue
		}
		if !known[k] {
			return nil, domain.ErrActivityFilterInvalid("unknown kind " + k)
		}
		seen[k] = true
		out = append(out, k)
	}
	return out, nil
}
