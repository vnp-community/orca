package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The plan version arguments travel as query parameters of the same routes the UI uses.
func TestPlanRevisionCalls_SendTheVersionAsQueryParameters(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = append(got, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "tok")
	ctx := context.Background()
	for _, call := range []func() error{
		func() error { _, err := c.GetPlanProposalRevision(ctx, "r1", "", 2); return err },
		func() error { _, err := c.GetPlanProposalRevision(ctx, "r1", "run-9", 0); return err },
		func() error { _, err := c.GetPlanBundleFileRevision(ctx, "r1", "specs/ai/spec.md", 1); return err },
		func() error { _, err := c.GetPlanBundleFile(ctx, "r1", "tasks.md"); return err },
		func() error { _, err := c.ListPlanRevisions(ctx, "r1", "3", 5); return err },
		func() error { _, err := c.ListPlanRevisions(ctx, "r1", "", 0); return err },
	} {
		if err := call(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		"GET /v1/requests/r1/plan?revision=2",
		"GET /v1/requests/r1/plan?runId=run-9",
		"GET /v1/requests/r1/plan/bundle/file?path=specs%2Fai%2Fspec.md&revision=1",
		"GET /v1/requests/r1/plan/bundle/file?path=tasks.md",
		"GET /v1/requests/r1/plan/revisions?pageSize=5&pageToken=3",
		"GET /v1/requests/r1/plan/revisions?",
	}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Errorf("call %d = %q, want %q", i, got[i], want[i])
		}
	}
}
