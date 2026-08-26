package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/usecase"
)

func TestListAccessibleProjects_ParsesGraphQLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"viewer": map[string]any{
					"projectsV2": map[string]any{
						"nodes": []map[string]any{
							{"id": "PVT_1", "number": 7, "title": "Roadmap", "url": "https://github.com/orgs/acme/projects/7",
								"owner": map[string]any{"login": "acme"}},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	client.graphQLURL = server.URL + "/graphql"

	projects, err := client.ListAccessibleProjects(context.Background(), usecase.Credential{Token: "tok"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 1 || projects[0].Slug != "acme/7" {
		t.Fatalf("expected one project with slug acme/7, got %+v", projects)
	}
}

func TestUpdateProjectItemField_MapsFieldKindToGraphQLValue(t *testing.T) {
	cases := []struct {
		kind      string
		value     string
		wantField string
	}{
		{"text", "hello", "text"},
		{"number", "42", "number"},
		{"date", "2024-01-01", "date"},
		{"single_select", "opt-1", "singleSelectOptionId"},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			var gotBody map[string]any
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.Header().Set("Content-Type", "application/json")
				if callCount == 1 {
					// resolveProjectID's organization lookup.
					_ = json.NewEncoder(w).Encode(map[string]any{
						"data": map[string]any{
							"organization": map[string]any{
								"projectV2": map[string]any{"id": "PVT_1", "number": 7, "title": "Roadmap", "url": "u", "owner": map[string]any{"login": "acme"}},
							},
						},
					})
					return
				}
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &gotBody)
				_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"updateProjectV2ItemFieldValue": map[string]any{"projectV2Item": map[string]any{"id": "item-1"}}}})
			}))
			defer server.Close()

			client := New(server.Client(), server.URL)
			client.graphQLURL = server.URL + "/graphql"

			_, err := client.UpdateProjectItemField(context.Background(), usecase.Credential{Token: "tok"}, "acme/7", "item-1", usecase.ProjectFieldValue{
				FieldID: "f1", Kind: tc.kind, Value: tc.value,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			variables, ok := gotBody["variables"].(map[string]any)
			if !ok {
				t.Fatalf("expected variables in request body, got %+v", gotBody)
			}
			valueInput, ok := variables["value"].(map[string]any)
			if !ok {
				t.Fatalf("expected value input in variables, got %+v", variables)
			}
			if _, ok := valueInput[tc.wantField]; !ok {
				t.Errorf("expected value input to contain key %q, got %+v", tc.wantField, valueInput)
			}
		})
	}
}

func TestGetWorkItemDetailsBySlug_ParsesOwnerRepoNumberSlug(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"title": "Bug", "body": "desc", "state": "open", "html_url": "https://github.com/acme/repo/issues/42"})
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	details, err := client.GetWorkItemDetailsBySlug(context.Background(), usecase.Credential{Token: "tok"}, "acme/repo#42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/repos/acme/repo/issues/42" {
		t.Errorf("expected path /repos/acme/repo/issues/42, got %s", gotPath)
	}
	if details.Title != "Bug" || details.Slug != "acme/repo#42" {
		t.Fatalf("unexpected result: %+v", details)
	}
}

func TestGetWorkItemDetailsBySlug_RejectsInvalidSlug(t *testing.T) {
	client := New(nil, "https://example.invalid")
	if _, err := client.GetWorkItemDetailsBySlug(context.Background(), usecase.Credential{Token: "tok"}, "no-number-here"); err == nil {
		t.Fatal("expected an error for a slug missing the #number suffix")
	}
}

func TestBranchExists_ReturnsTrueOn200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New(server.Client(), server.URL)
	exists, err := client.BranchExists(context.Background(), usecase.Credential{Token: "tok"}, "o/r", "feature-x")
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
	exists, err := client.BranchExists(context.Background(), usecase.Credential{Token: "tok"}, "o/r", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if exists {
		t.Error("expected exists=false on 404")
	}
}
