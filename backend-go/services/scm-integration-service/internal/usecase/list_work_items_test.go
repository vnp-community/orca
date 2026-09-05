package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func timeAt(unixSeconds int64) time.Time { return time.Unix(unixSeconds, 0) }

// noWorkItemsProvider satisfies ScmProvider (by embedding the interface,
// left nil — never actually invoked below) without implementing
// WorkItemProvider, exercising ListWorkItems' SCM_WORK_ITEMS_UNSUPPORTED
// path for a provider that hasn't added the feature.
type noWorkItemsProvider struct{ ScmProvider }

func TestListWorkItems(t *testing.T) {
	t.Run("missing tenant_id is rejected", func(t *testing.T) {
		uc := NewListWorkItems(&fakeCredentialResolver{}, &fakeRegistry{})
		_, err := uc.Execute(context.Background(), ListWorkItemsInput{Repo: "acme/widgets"})
		if err == nil {
			t.Fatal("expected error for missing tenant_id")
		}
	})

	t.Run("missing repo is rejected", func(t *testing.T) {
		uc := NewListWorkItems(&fakeCredentialResolver{}, &fakeRegistry{})
		_, err := uc.Execute(context.Background(), ListWorkItemsInput{TenantID: "t1"})
		if err == nil {
			t.Fatal("expected error for missing repo")
		}
	})

	t.Run("provider without WorkItemProvider support is rejected", func(t *testing.T) {
		registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{
			domain.ScmProviderGitLab: noWorkItemsProvider{},
		}}
		uc := NewListWorkItems(&fakeCredentialResolver{token: "tok"}, registry)
		_, err := uc.Execute(context.Background(), ListWorkItemsInput{TenantID: "t1", Provider: domain.ScmProviderGitLab, Repo: "acme/widgets"})
		if err == nil {
			t.Fatal("expected SCM_WORK_ITEMS_UNSUPPORTED error")
		}
	})

	t.Run("credential resolve failure is reported", func(t *testing.T) {
		registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{
			domain.ScmProviderGitHub: &fakeProvider{},
		}}
		creds := &fakeCredentialResolver{err: errors.New("no token")}
		uc := NewListWorkItems(creds, registry)
		_, err := uc.Execute(context.Background(), ListWorkItemsInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "acme/widgets"})
		if err == nil {
			t.Fatal("expected error when credential resolve fails")
		}
	})

	t.Run("dispatches to the resolved provider with the parsed filter and default limit", func(t *testing.T) {
		provider := &fakeProvider{workItems: []domain.WorkItem{
			{ID: "issue:1", Type: "issue", Title: "a"},
		}}
		registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
		creds := &fakeCredentialResolver{token: "tok"}
		uc := NewListWorkItems(creds, registry)

		items, err := uc.Execute(context.Background(), ListWorkItemsInput{
			TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "acme/widgets",
			Query: "is:issue label:bug assignee:alice",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 || items[0].ID != "issue:1" {
			t.Fatalf("got %+v, want the fake provider's one item", items)
		}
		if provider.lastRepo != "acme/widgets" {
			t.Errorf("got repo %q, want acme/widgets", provider.lastRepo)
		}
		if provider.lastCred.Token != "tok" {
			t.Errorf("got token %q, want tok", provider.lastCred.Token)
		}
		f := provider.lastWorkItemFilter
		if f.Scope != "issue" || f.Assignee != "alice" || len(f.Labels) != 1 || f.Labels[0] != "bug" {
			t.Errorf("got filter %+v, want scope=issue assignee=alice labels=[bug]", f)
		}
		if f.Limit != defaultWorkItemLimit {
			t.Errorf("got limit %d, want default %d", f.Limit, defaultWorkItemLimit)
		}
	})

	t.Run("provider failure is reported", func(t *testing.T) {
		provider := &fakeProvider{workItemsErr: errors.New("boom")}
		registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
		uc := NewListWorkItems(&fakeCredentialResolver{token: "tok"}, registry)
		_, err := uc.Execute(context.Background(), ListWorkItemsInput{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "acme/widgets"})
		if err == nil {
			t.Fatal("expected error when the provider call fails")
		}
	})

	t.Run("results beyond the limit are truncated after sorting by most-recently-updated", func(t *testing.T) {
		older, _ := domain.NewWorkItem("issue:1", "issue", 1, "older", "open", "u", nil, timeAt(1), "")
		newer, _ := domain.NewWorkItem("issue:2", "issue", 2, "newer", "open", "u", nil, timeAt(2), "")
		provider := &fakeProvider{workItems: []domain.WorkItem{older, newer}}
		registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
		uc := NewListWorkItems(&fakeCredentialResolver{token: "tok"}, registry)

		items, err := uc.Execute(context.Background(), ListWorkItemsInput{
			TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "acme/widgets", Limit: 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(items) != 1 || items[0].ID != "issue:2" {
			t.Fatalf("got %+v, want only the newer item", items)
		}
	})
}

func TestParseWorkItemQuery(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  WorkItemFilter
	}{
		{"empty query defaults to recent/open", "", WorkItemFilter{Scope: "all", State: "open"}},
		{"is:issue narrows scope", "is:issue", WorkItemFilter{Scope: "issue", State: "open"}},
		{"is:pr narrows scope", "is:pr", WorkItemFilter{Scope: "pr", State: "open"}},
		{"is:merged implies pr scope", "is:merged", WorkItemFilter{Scope: "pr", State: "merged"}},
		{"label is repeatable", "label:bug label:p0", WorkItemFilter{Scope: "all", State: "open", Labels: []string{"bug", "p0"}}},
		{"assignee and author", "assignee:alice author:bob", WorkItemFilter{Scope: "all", State: "open", Assignee: "alice", Author: "bob"}},
		{"unsupported qualifiers are ignored, not errors", "review-requested:alice free text @me", WorkItemFilter{Scope: "all", State: "open"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseWorkItemQuery(tc.query)
			if got.Scope != tc.want.Scope || got.State != tc.want.State || got.Assignee != tc.want.Assignee || got.Author != tc.want.Author || len(got.Labels) != len(tc.want.Labels) {
				t.Errorf("parseWorkItemQuery(%q) = %+v, want %+v", tc.query, got, tc.want)
			}
			for i := range got.Labels {
				if got.Labels[i] != tc.want.Labels[i] {
					t.Errorf("parseWorkItemQuery(%q).Labels = %v, want %v", tc.query, got.Labels, tc.want.Labels)
				}
			}
		})
	}
}
