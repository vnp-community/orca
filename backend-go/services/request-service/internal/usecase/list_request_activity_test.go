package usecase

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type fakeActivityReader struct {
	rows  []domain.ActivityRow // newest first
	calls []domain.ActivityQuery
}

func (f *fakeActivityReader) ListActivity(_ context.Context, _ string, q domain.ActivityQuery) ([]domain.ActivityRow, error) {
	f.calls = append(f.calls, q)
	var out []domain.ActivityRow
	for _, r := range f.rows {
		if q.Before != nil && !(r.OccurredAt.Before(q.Before.At) || r.OccurredAt.Equal(q.Before.At) && r.ID < q.Before.ID) {
			continue
		}
		if len(q.Kinds) > 0 && !slices.Contains(q.Kinds, r.Kind) {
			continue
		}
		if q.SubjectID != "" && r.SubjectID != q.SubjectID {
			continue
		}
		out = append(out, r)
		if q.Limit > 0 && len(out) == q.Limit {
			break
		}
	}
	return out, nil
}

func activityRig(t *testing.T, rows []domain.ActivityRow) (*ListRequestActivity, domain.Request, *fakeActivityReader) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeChangeRequest, domain.RequestSizeM, domain.RequestStatusExecuting)
	f := &fakeActivityReader{rows: rows}
	return &ListRequestActivity{Requests: r.store, Activity: f, Tasks: r.tasks, MaxAttempts: 2}, req, f
}

func infoRows(n int) []domain.ActivityRow {
	base := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	var rows []domain.ActivityRow
	for i := 0; i < n; i++ {
		rows = append(rows, domain.ActivityRow{
			Kind: domain.ActivityPhaseStarted, ID: fmt.Sprintf("row-%03d", n-i), OccurredAt: base.Add(-time.Duration(i) * time.Minute), ActorKind: "system", SubjectID: "p",
		})
	}
	return rows
}

func TestListRequestActivity_PagesByCursorWithoutRepeatsOrGaps(t *testing.T) {
	uc, req, _ := activityRig(t, infoRows(7))
	var seen []string
	token := ""
	for page := 0; page < 10; page++ {
		res, err := uc.Execute(lcCtx(), ListRequestActivityInput{RequestID: req.ID, PageSize: 3, PageToken: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Entries {
			seen = append(seen, e.ID)
		}
		if res.NextPageToken == "" {
			break
		}
		token = res.NextPageToken
	}
	if len(seen) != 7 {
		t.Fatalf("saw %d entries, want 7: %v", len(seen), seen)
	}
	uniq := map[string]bool{}
	for _, id := range seen {
		uniq[id] = true
	}
	if len(uniq) != 7 {
		t.Fatalf("a row repeated: %v", seen)
	}
}

func TestListRequestActivity_RowsWithTheSameTimeKeepAStablePageEdge(t *testing.T) {
	at := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	rows := []domain.ActivityRow{
		{Kind: domain.ActivityPhaseStarted, ID: "c", OccurredAt: at},
		{Kind: domain.ActivityPhaseStarted, ID: "b", OccurredAt: at},
		{Kind: domain.ActivityPhaseStarted, ID: "a", OccurredAt: at},
	}
	uc, req, _ := activityRig(t, rows)
	first, _ := uc.Execute(lcCtx(), ListRequestActivityInput{RequestID: req.ID, PageSize: 2})
	second, _ := uc.Execute(lcCtx(), ListRequestActivityInput{RequestID: req.ID, PageSize: 2, PageToken: first.NextPageToken})
	var ids []string
	for _, e := range append(first.Entries, second.Entries...) {
		ids = append(ids, e.ID)
	}
	if fmt.Sprint(ids) != "[c b a]" {
		t.Fatalf("ids = %v", ids)
	}
}

func TestListRequestActivity_SeverityFilterKeepsPagesFull(t *testing.T) {
	base := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	var rows []domain.ActivityRow
	for i := 0; i < 30; i++ {
		kind, b := domain.ActivityPhaseStarted, ""
		if i%10 == 4 { // three failures hidden among info lines
			kind, b = domain.ActivityTaskFailed, "execution_failed"
		}
		rows = append(rows, domain.ActivityRow{Kind: kind, ID: fmt.Sprintf("r-%03d", 99-i), OccurredAt: base.Add(-time.Duration(i) * time.Minute), SubjectID: "t", B: b, N: 1})
	}
	uc, req, f := activityRig(t, rows)
	res, err := uc.Execute(lcCtx(), ListRequestActivityInput{RequestID: req.ID, PageSize: 2, MinSeverity: "attention"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Entries) != 2 || res.NextPageToken == "" {
		t.Fatalf("a page of two attention lines with more behind it: %d entries, token %q (reads=%d)", len(res.Entries), res.NextPageToken, len(f.calls))
	}
	for _, e := range res.Entries {
		if e.Kind != domain.ActivityTaskFailed {
			t.Fatalf("only the filtered kind: %+v", e)
		}
	}
	rest, _ := uc.Execute(lcCtx(), ListRequestActivityInput{RequestID: req.ID, PageSize: 2, MinSeverity: "attention", PageToken: res.NextPageToken})
	if len(rest.Entries) != 1 || rest.NextPageToken != "" {
		t.Fatalf("the last attention line and no more: %d entries, token %q", len(rest.Entries), rest.NextPageToken)
	}
}

func TestListRequestActivity_FiltersAreValidated(t *testing.T) {
	uc, req, _ := activityRig(t, nil)
	cases := []struct {
		name string
		in   ListRequestActivityInput
		code string
	}{
		{"unknown kind", ListRequestActivityInput{RequestID: req.ID, Kinds: []string{"task.exploded"}}, "REQUEST_ACTIVITY_FILTER_INVALID"},
		{"unknown severity", ListRequestActivityInput{RequestID: req.ID, MinSeverity: "loud"}, "REQUEST_ACTIVITY_FILTER_INVALID"},
		{"bad cursor", ListRequestActivityInput{RequestID: req.ID, PageToken: "###"}, "REQUEST_ACTIVITY_CURSOR_INVALID"},
		{"bad id", ListRequestActivityInput{RequestID: "nope"}, "REQUEST_NOT_FOUND"},
		{"missing request", ListRequestActivityInput{RequestID: "5f1c2a76-1111-4222-8333-944455556666"}, "REQUEST_NOT_FOUND"},
	}
	for _, c := range cases {
		if _, err := uc.Execute(lcCtx(), c.in); !errorHasCode(err, c.code) {
			t.Errorf("%s: got %v, want %s", c.name, err, c.code)
		}
	}
	if _, err := uc.Execute(context.Background(), ListRequestActivityInput{RequestID: req.ID}); err == nil {
		t.Fatal("a call without a tenant is refused")
	}
}

func TestListRequestActivity_PageSizeIsCappedAndKindsAreDeduplicated(t *testing.T) {
	uc, req, f := activityRig(t, infoRows(3))
	if _, err := uc.Execute(lcCtx(), ListRequestActivityInput{RequestID: req.ID, PageSize: 5000, Kinds: []string{domain.ActivityPhaseStarted, " " + domain.ActivityPhaseStarted, ""}}); err != nil {
		t.Fatal(err)
	}
	q := f.calls[0]
	if q.Limit != maxActivityPage+1 || len(q.Kinds) != 1 {
		t.Fatalf("limit=%d kinds=%v", q.Limit, q.Kinds)
	}
}

func TestListRequestActivity_NamesTasksFromTheTree(t *testing.T) {
	r := newExRig(t)
	req := r.request(domain.RequestTypeBug, domain.RequestSizeM, domain.RequestStatusExecuting)
	task := r.leaf(req, r.plan(req), "Fix the bound", domain.TaskStatusOpen)
	f := &fakeActivityReader{rows: []domain.ActivityRow{{Kind: domain.ActivityTaskStarted, ID: "o1", OccurredAt: time.Now(), SubjectID: task.ID, N: 1}}}
	uc := &ListRequestActivity{Requests: r.store, Activity: f, Tasks: r.tasks, MaxAttempts: 2}
	res, err := uc.Execute(lcCtx(), ListRequestActivityInput{RequestID: req.ID})
	if err != nil || len(res.Entries) != 1 || res.Entries[0].Details["taskTitle"] != "Fix the bound" {
		t.Fatalf("%+v %v", res.Entries, err)
	}
	r.tasks.listErr = fmt.Errorf("task-service is down")
	res, err = uc.Execute(lcCtx(), ListRequestActivityInput{RequestID: req.ID})
	if err != nil || len(res.Entries) != 1 {
		t.Fatalf("a task-service failure only costs the titles: %+v %v", res.Entries, err)
	}
	if _, ok := res.Entries[0].Details["taskTitle"]; ok {
		t.Fatal("no title without the tree")
	}
}
