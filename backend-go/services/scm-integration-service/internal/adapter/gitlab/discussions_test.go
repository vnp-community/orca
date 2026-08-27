package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

func TestResolveDiscussion_SendsResolvedQueryParam(t *testing.T) {
	var gotPath, gotQuery, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotMethod = r.URL.Path, r.URL.RawQuery, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"disc-1","notes":[{"resolved":true,"resolved_by":{"username":"alice"}}]}`))
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	disc, err := client.ResolveDiscussion(context.Background(), usecase.Credential{Token: "tok"}, "group/project", 42, "disc-1", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("expected PUT, got %s", gotMethod)
	}
	// r.URL.Path is the decoded path (net/http decodes %2F back to "/" here);
	// the %2F-escaped project id is only visible on the wire / in RawPath.
	if gotPath != "/projects/group/project/merge_requests/42/discussions/disc-1" {
		t.Errorf("unexpected path: %s", gotPath)
	}
	if gotQuery != "resolved=true" {
		t.Errorf("expected resolved=true query param, got %q", gotQuery)
	}
	if !disc.Resolved || disc.ResolvedBy != "alice" {
		t.Errorf("unexpected discussion: %+v", disc)
	}
}

func TestListMergeRequests_FiltersByStateAndSourceBranch(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	_, err := client.ListMergeRequests(context.Background(), usecase.Credential{Token: "tok"}, "group/project", usecase.MRFilter{State: "opened", SourceBranch: "feature-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotQuery != "source_branch=feature-x&state=opened" {
		t.Errorf("unexpected query: %q", gotQuery)
	}
}

func TestGetWorkItemDetails_SelectsEndpointByItemType(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"iid":42,"title":"t","description":"d","state":"opened","web_url":"u","labels":[]}`))
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)

	if _, err := client.GetWorkItemDetails(context.Background(), usecase.Credential{Token: "tok"}, "group/project", 42, "issue"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/projects/group/project/issues/42" {
		t.Errorf("expected issues endpoint for item_type=issue, got %s", gotPath)
	}

	if _, err := client.GetWorkItemDetails(context.Background(), usecase.Credential{Token: "tok"}, "group/project", 42, "merge_request"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/projects/group/project/merge_requests/42" {
		t.Errorf("expected merge_requests endpoint for any non-issue item_type, got %s", gotPath)
	}
}

func TestBranchExists_ReturnsTrueOn200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	exists, err := client.BranchExists(context.Background(), usecase.Credential{Token: "tok"}, "group/project", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Error("expected exists=true on 200")
	}
}

func TestBranchExists_ReturnsFalseOn404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	exists, err := client.BranchExists(context.Background(), usecase.Credential{Token: "tok"}, "group/project", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected exists=false on 404")
	}
}
