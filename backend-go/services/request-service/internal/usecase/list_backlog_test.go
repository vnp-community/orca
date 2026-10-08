package usecase

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// exBacklogReader serves keyset pages over an in-memory list, with the same ordering the SQL adapters use.
type exBacklogReader struct {
	reqs    []domain.Request
	parents map[string][]string
	returns map[string]domain.ReturnEvent
	pages   int
}

func (f *exBacklogReader) ListReturnedRequests(ctx context.Context, tenantID string, flt BacklogRequestFilter) ([]domain.Request, error) {
	return f.ListByStatus(ctx, tenantID, []domain.RequestStatus{domain.RequestStatusRequestBacklog}, flt)
}

func (f *exBacklogReader) ListByStatus(_ context.Context, _ string, statuses []domain.RequestStatus, flt BacklogRequestFilter) ([]domain.Request, error) {
	f.pages++
	var rows []domain.Request
	for _, r := range f.reqs {
		ok := false
		for _, s := range statuses {
			ok = ok || r.Status == s
		}
		if !ok || (flt.ProjectID != "" && r.ProjectID != flt.ProjectID) || (flt.RequestID != "" && r.ID != flt.RequestID) {
			continue
		}
		if len(flt.Types) > 0 && !slices.Contains(flt.Types, string(r.Type)) {
			continue
		}
		if len(flt.Categories) > 0 && !slices.Contains(flt.Categories, string(r.ReturnedCategory)) {
			continue
		}
		if flt.Cursor != nil && !(r.UpdatedAt.Before(flt.Cursor.UpdatedAt) || (r.UpdatedAt.Equal(flt.Cursor.UpdatedAt) && r.ID < flt.Cursor.ID)) {
			continue
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if !rows[i].UpdatedAt.Equal(rows[j].UpdatedAt) {
			return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
		}
		return rows[i].ID > rows[j].ID
	})
	if len(rows) > flt.Limit+1 {
		rows = rows[:flt.Limit+1]
	}
	return rows, nil
}

func (f *exBacklogReader) ParentRequestIDs(_ context.Context, _ string, ids []string) (map[string][]string, error) {
	out := map[string][]string{}
	for _, id := range ids {
		if p, ok := f.parents[id]; ok {
			out[id] = p
		}
	}
	return out, nil
}

func (f *exBacklogReader) LatestReturns(_ context.Context, _ string, ids []string) (map[string]domain.ReturnEvent, error) {
	out := map[string]domain.ReturnEvent{}
	for _, id := range ids {
		if e, ok := f.returns[id]; ok {
			out[id] = e
		}
	}
	return out, nil
}

type exMembers map[string]bool // "project/user"

func (m exMembers) IsMember(_ context.Context, project, user string) (bool, error) {
	return m[project+"/"+user], nil
}

type backlogRig struct {
	*exRig
	reader *exBacklogReader
	uc     *ListBacklog
}

func newBacklogRig(t *testing.T) *backlogRig {
	r := newExRig(t)
	reader := &exBacklogReader{parents: map[string][]string{}, returns: map[string]domain.ReturnEvent{}}
	vis := &MemberRequestVisibility{Members: exMembers{}}
	b := &backlogRig{exRig: r, reader: reader}
	b.uc = &ListBacklog{
		Requests: &ListBacklogRequests{Reader: reader, Visibility: vis},
		Tasks:    &ListBacklogTasks{Requests: reader, Approvals: r.gates, Tasks: r.tasks, Outcomes: r.outcomes, Visibility: vis},
	}
	return b
}

// seed adds a Request to both the request store and the backlog reader, newest first by minutes.
func (b *backlogRig) seed(typ domain.RequestType, size domain.RequestSize, status domain.RequestStatus, minutes int) domain.Request {
	req := b.store.seed(func(r *domain.Request) {
		r.Type, r.Size, r.Status = typ, size, status
		r.UpdatedAt = time.Date(2026, 10, 1, 0, minutes, 0, 0, time.UTC)
		if status == domain.RequestStatusRequestBacklog {
			r.ReturnedCategory, r.ReturnedFromStage = domain.ReturnCategoryOther, domain.ReturnStageTask
		}
	})
	b.reader.reqs = append(b.reader.reqs, req)
	return req
}

func backlogAdminCtx() context.Context { return lcCtxWithUser("admin-1", "admin") }

func (b *backlogRig) view(v domain.BacklogView, mod ...func(*ListBacklogInput)) ListBacklogOutput {
	b.t.Helper()
	in := ListBacklogInput{View: v}
	for _, m := range mod {
		m(&in)
	}
	out, err := b.uc.Execute(backlogAdminCtx(), in)
	if err != nil {
		b.t.Fatalf("ListBacklog: %v", err)
	}
	return out
}

func groupIDs(groups []BacklogGroup) []string {
	var ids []string
	for _, g := range groups {
		for _, t := range g.Tasks {
			ids = append(ids, t.Task.ID)
		}
	}
	return ids
}

func TestListBacklog_InvalidView(t *testing.T) {
	b := newBacklogRig(t)
	if _, err := b.uc.Execute(backlogAdminCtx(), ListBacklogInput{}); !errorHasCode(err, "REQUEST_BACKLOG_INVALID_VIEW") {
		t.Fatalf("got %v", err)
	}
	if _, err := b.uc.Execute(context.Background(), ListBacklogInput{View: domain.BacklogViewRequest}); err == nil {
		t.Fatal("a tenant is required")
	}
}

func TestListBacklog_RequestView_NoTaskServiceCall(t *testing.T) {
	b := newBacklogRig(t)
	b.reader.parents = map[string][]string{}
	first := b.seed(domain.RequestTypeBug, domain.RequestSizeS, domain.RequestStatusRequestBacklog, 5)
	b.seed(domain.RequestTypeBug, domain.RequestSizeS, domain.RequestStatusExecuting, 6) // not in the backlog
	b.reader.parents[first.ID] = []string{"parent-1"}
	b.reader.returns[first.ID] = domain.ReturnEvent{ActorID: "u-9", At: time.Date(2026, 10, 1, 0, 4, 0, 0, time.UTC)}
	out := b.view(domain.BacklogViewRequest)
	if len(out.RequestRows) != 1 || out.RequestRows[0].Request.ID != first.ID {
		t.Fatalf("only request_backlog rows: %+v", out.RequestRows)
	}
	row := out.RequestRows[0]
	if row.ReturnedBy != "u-9" || row.ReturnedAt.IsZero() || len(row.ParentRequestIDs) != 1 {
		t.Fatalf("row details: %+v", row)
	}
	if b.tasks.listCalls != 0 || len(b.tasks.calls) != 0 {
		t.Fatal("the request view must never call task-service")
	}
}

func TestListBacklogRequests_PageSizeClamp(t *testing.T) {
	for in, want := range map[int]int{-1: 20, 0: 20, 7: 7, 100: 100, 500: 100} {
		if got := clampBacklogPageSize(in); got != want {
			t.Errorf("clamp(%d)=%d, want %d", in, got, want)
		}
	}
}

func TestListBacklogRequests_BadToken(t *testing.T) {
	b := newBacklogRig(t)
	_, err := b.uc.Execute(backlogAdminCtx(), ListBacklogInput{View: domain.BacklogViewRequest, PageToken: "###"})
	if !errorHasCode(err, "REQUEST_BACKLOG_BAD_PAGE_TOKEN") {
		t.Fatalf("got %v", err)
	}
}

func TestListBacklogRequests_NextTokenOnlyWhenMore_AndPagesAreComplete(t *testing.T) {
	b := newBacklogRig(t)
	var want []string
	for i := 0; i < 7; i++ {
		// Equal updated_at on purpose: the id breaks ties.
		r := b.seed(domain.RequestTypeBug, domain.RequestSizeS, domain.RequestStatusRequestBacklog, 10+i/3)
		want = append(want, r.ID)
	}
	var seen []string
	token := ""
	for page := 0; ; page++ {
		out := b.view(domain.BacklogViewRequest, func(in *ListBacklogInput) { in.PageSize = 3; in.PageToken = token })
		for _, row := range out.RequestRows {
			seen = append(seen, row.Request.ID)
		}
		if out.NextPageToken == "" {
			if page != 2 {
				t.Fatalf("7 rows at 3 per page end on page 3, ended on %d", page+1)
			}
			break
		}
		if len(out.RequestRows) != 3 {
			t.Fatalf("a full page precedes every token: %d", len(out.RequestRows))
		}
		token = out.NextPageToken
	}
	sort.Strings(want)
	sort.Strings(seen)
	if fmt.Sprint(seen) != fmt.Sprint(want) {
		t.Fatalf("paging lost or repeated rows:\n got %v\nwant %v", seen, want)
	}
}

func TestListBacklogRequests_FiltersByTypeAndCategory(t *testing.T) {
	b := newBacklogRig(t)
	bug := b.seed(domain.RequestTypeBug, domain.RequestSizeS, domain.RequestStatusRequestBacklog, 1)
	task := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusRequestBacklog, 2)
	for i, r := range b.reader.reqs {
		if r.ID == task.ID {
			r.ReturnedCategory = domain.ReturnCategoryRejected
			b.reader.reqs[i] = r
		}
	}
	out := b.view(domain.BacklogViewRequest, func(in *ListBacklogInput) { in.Types = []string{"task"} })
	if len(out.RequestRows) != 1 || out.RequestRows[0].Request.ID != task.ID {
		t.Fatalf("type filter: %+v", out.RequestRows)
	}
	out = b.view(domain.BacklogViewRequest, func(in *ListBacklogInput) { in.Categories = []string{"other"} })
	if len(out.RequestRows) != 1 || out.RequestRows[0].Request.ID != bug.ID {
		t.Fatalf("category filter: %+v", out.RequestRows)
	}
}

func TestListBacklog_VisibilityFiltersAllViews(t *testing.T) {
	b := newBacklogRig(t)
	mine := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, 2)
	theirs := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusExecuting, 3)
	backlog := b.seed(domain.RequestTypeTask, domain.RequestSizeS, domain.RequestStatusRequestBacklog, 4)
	for i, r := range b.reader.reqs {
		if r.ID == mine.ID || r.ID == backlog.ID {
			r.ReporterID = "me"
			b.reader.reqs[i] = r
		}
	}
	for _, r := range []domain.Request{mine, theirs} {
		plan := b.plan(r)
		b.leaf(r, plan, "t-"+r.ID, domain.TaskStatusOpen)
		b.approved(r, domain.SubjectTaskList, plan.ID)
	}
	ctx := lcCtxWithUser("me", "")
	for _, v := range []domain.BacklogView{domain.BacklogViewTask, domain.BacklogViewExecute, domain.BacklogViewRequest} {
		out, err := b.uc.Execute(ctx, ListBacklogInput{View: v})
		if err != nil {
			t.Fatal(err)
		}
		for _, g := range append(out.TaskGroups, out.ExecuteGroups...) {
			if g.RequestID == theirs.ID {
				t.Fatalf("view %d leaks another reporter's tasks", v)
			}
		}
		for _, row := range out.RequestRows {
			if row.Request.ID != backlog.ID {
				t.Fatalf("view %d leaks request %s", v, row.Request.ID)
			}
		}
	}
	// The hidden request's tasks must not even be requested from task-service.
	for _, c := range b.tasks.calls {
		if c == "list:"+theirs.ID {
			t.Fatal("task-service was asked about a request the caller may not see")
		}
	}
}

func TestMemberRequestVisibility(t *testing.T) {
	v := &MemberRequestVisibility{Members: exMembers{"p1/u1": true}}
	mine := domain.Request{ID: "a", ReporterID: "u1"}
	member := domain.Request{ID: "b", ReporterID: "x", ProjectID: "p1"}
	other := domain.Request{ID: "c", ReporterID: "x", ProjectID: "p2"}
	noProject := domain.Request{ID: "d", ReporterID: "x"}
	all := []domain.Request{mine, member, other, noProject}
	got, _ := v.Filter(context.Background(), domain.DecisionActor{UserID: "u1"}, all)
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("reporter and project member: %+v", got)
	}
	if got, _ := v.Filter(context.Background(), domain.DecisionActor{UserID: "boss", Role: "admin"}, all); len(got) != 4 {
		t.Fatalf("admin sees all: %+v", got)
	}
	if got, _ := v.Filter(context.Background(), domain.DecisionActor{}, all); len(got) != 0 {
		t.Fatalf("no user, no rows: %+v", got)
	}
	if _, err := filterVisible(context.Background(), nil, all); err == nil {
		t.Fatal("a missing policy must be an error, not 'show all'")
	}
}
