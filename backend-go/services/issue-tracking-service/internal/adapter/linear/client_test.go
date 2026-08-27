package linear

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/issue-tracking-service/internal/usecase"
)

// newFakeLinearServer starts an httptest.Server that always answers Linear's
// single GraphQL endpoint with body — the client under test is redirected
// to it via httpClient's Transport rewriting the request URL, since
// endpoint is a package-level const, not injectable.
func newFakeLinearServer(t *testing.T, body map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(body)
	}))
}

// newTestClient returns a Client whose httpClient redirects every request
// to srv, regardless of the URL the code under test dialed (endpoint is
// a package const pointing at the real api.linear.app).
func newTestClient(srv *httptest.Server) *Client {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		req.URL.Scheme = "http"
		req.URL.Host = srv.Listener.Addr().String()
		return srv.Client().Transport.RoundTrip(req)
	})
	return New(&http.Client{Transport: transport})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestClient_Whoami_ParsesGraphQLResponse(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"data": map[string]any{
			"viewer": map[string]any{"id": "u-1", "name": "Ada", "email": "ada@example.com"},
		},
	})
	defer srv.Close()
	c := newTestClient(srv)

	viewer, err := c.Whoami(context.Background(), usecase.Credential{Token: "tok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if viewer.ID != "u-1" || viewer.DisplayName != "Ada" || viewer.Email != "ada@example.com" {
		t.Fatalf("unexpected viewer: %+v", viewer)
	}
}

func TestClient_Whoami_PropagatesGraphQLError(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"errors": []map[string]any{{"message": "invalid token"}},
	})
	defer srv.Close()
	c := newTestClient(srv)

	_, err := c.Whoami(context.Background(), usecase.Credential{Token: "bad"})
	if err == nil {
		t.Fatal("expected an error for a GraphQL-level error response")
	}
}

func TestClient_ListTeams_ParsesGraphQLResponse(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"data": map[string]any{
			"teams": map[string]any{
				"nodes": []map[string]any{{"id": "team-1", "name": "Engineering", "key": "ENG"}},
			},
		},
	})
	defer srv.Close()
	c := newTestClient(srv)

	teams, err := c.ListTeams(context.Background(), usecase.Credential{Token: "tok"}, "ws-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(teams) != 1 || teams[0].Key != "ENG" || teams[0].WorkspaceID != "ws-1" {
		t.Fatalf("unexpected teams: %+v", teams)
	}
}

func TestClient_ListTeamLabels_ParsesGraphQLResponse(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"data": map[string]any{
			"team": map[string]any{
				"labels": map[string]any{
					"nodes": []map[string]any{{"id": "label-1", "name": "bug", "color": "#ff0000"}},
				},
			},
		},
	})
	defer srv.Close()
	c := newTestClient(srv)

	labels, err := c.ListTeamLabels(context.Background(), usecase.Credential{Token: "tok"}, "team-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(labels) != 1 || labels[0].Name != "bug" {
		t.Fatalf("unexpected labels: %+v", labels)
	}
}

func TestClient_ListTeamMembers_ParsesGraphQLResponse(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"data": map[string]any{
			"team": map[string]any{
				"members": map[string]any{
					"nodes": []map[string]any{{"id": "user-1", "name": "Ada", "avatarUrl": "https://x/ada.png"}},
				},
			},
		},
	})
	defer srv.Close()
	c := newTestClient(srv)

	members, err := c.ListTeamMembers(context.Background(), usecase.Credential{Token: "tok"}, "team-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(members) != 1 || members[0].DisplayName != "Ada" {
		t.Fatalf("unexpected members: %+v", members)
	}
}

func TestClient_GetCustomView_ParsesGraphQLResponse(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"data": map[string]any{
			"customView": map[string]any{"id": "view-1", "name": "My View", "team": map[string]any{"id": "team-1"}},
		},
	})
	defer srv.Close()
	c := newTestClient(srv)

	view, err := c.GetCustomView(context.Background(), usecase.Credential{Token: "tok"}, "view-1", "issue")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if view.ID != "view-1" || view.TeamID != "team-1" || view.Model != "issue" {
		t.Fatalf("unexpected view: %+v", view)
	}
}

func TestClient_ListWorkflowStates_ParsesGraphQLResponse(t *testing.T) {
	srv := newFakeLinearServer(t, map[string]any{
		"data": map[string]any{
			"team": map[string]any{
				"states": map[string]any{
					"nodes": []map[string]any{
						{"id": "state-1", "name": "Todo", "type": "unstarted", "position": 1.0},
						{"id": "state-2", "name": "Done", "type": "completed", "position": 2.0},
					},
				},
			},
		},
	})
	defer srv.Close()
	c := newTestClient(srv)

	states, err := c.ListWorkflowStates(context.Background(), usecase.Credential{Token: "tok"}, "team-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(states) != 2 || states[0].Name != "Todo" || states[1].Category != "completed" {
		t.Fatalf("unexpected states: %+v", states)
	}
}
