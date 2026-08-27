package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestUpdateProjectItemField_Success(t *testing.T) {
	gh := &fakeGitHubProjects{item: ProjectItem{ID: "item-1"}}
	uc := NewUpdateProjectItemField(&fakeCredentialResolver{token: "tok"}, gh)

	got, err := uc.Execute(context.Background(), UpdateProjectItemFieldParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, ProjectSlug: "acme/7", ItemID: "item-1",
		Field: ProjectFieldValue{FieldID: "f1", Kind: "text", Value: "v"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "item-1" || gh.calls != 1 {
		t.Fatalf("unexpected result: %+v calls=%d", got, gh.calls)
	}
}

func TestUpdateProjectItemField_RejectsNonGitHubProvider(t *testing.T) {
	gh := &fakeGitHubProjects{}
	uc := NewUpdateProjectItemField(&fakeCredentialResolver{token: "tok"}, gh)

	_, err := uc.Execute(context.Background(), UpdateProjectItemFieldParams{
		TenantID: "t1", Provider: domain.ScmProviderGitLab, ProjectSlug: "acme/7", ItemID: "item-1",
	})
	if err == nil {
		t.Fatal("expected SCM_PROVIDER_UNSUPPORTED rejection for a non-GitHub provider")
	}
	if gh.calls != 0 {
		t.Errorf("expected the port not to be called at all, got %d calls", gh.calls)
	}
}

func TestUpdateProjectItemField_PropagatesPortFailure(t *testing.T) {
	gh := &fakeGitHubProjects{itemErr: errors.New("graphql error")}
	uc := NewUpdateProjectItemField(&fakeCredentialResolver{token: "tok"}, gh)

	_, err := uc.Execute(context.Background(), UpdateProjectItemFieldParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ProjectSlug: "acme/7", ItemID: "item-1"})
	if err == nil {
		t.Fatal("expected an error when the port call fails")
	}
}

func TestListAccessibleProjects_Success(t *testing.T) {
	gh := &fakeGitHubProjects{projects: []Project{{ID: "p1", Slug: "acme/1"}}}
	uc := NewListAccessibleProjects(&fakeCredentialResolver{token: "tok"}, gh)

	got, err := uc.Execute(context.Background(), ListAccessibleProjectsParams{TenantID: "t1", Provider: domain.ScmProviderGitHub})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Slug != "acme/1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestListAccessibleProjects_RejectsNonGitHubProvider(t *testing.T) {
	uc := NewListAccessibleProjects(&fakeCredentialResolver{}, &fakeGitHubProjects{})
	if _, err := uc.Execute(context.Background(), ListAccessibleProjectsParams{TenantID: "t1", Provider: domain.ScmProviderBitbucket}); err == nil {
		t.Fatal("expected a rejection for a non-GitHub provider")
	}
}

func TestResolveProjectRef_Success(t *testing.T) {
	gh := &fakeGitHubProjects{project: Project{ID: "p1", Slug: "acme/7"}}
	uc := NewResolveProjectRef(&fakeCredentialResolver{token: "tok"}, gh)

	got, err := uc.Execute(context.Background(), ResolveProjectRefParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, Owner: "acme", Number: 7})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Slug != "acme/7" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestResolveProjectRef_PropagatesPortFailure(t *testing.T) {
	gh := &fakeGitHubProjects{projectErr: errors.New("not found")}
	uc := NewResolveProjectRef(&fakeCredentialResolver{token: "tok"}, gh)
	if _, err := uc.Execute(context.Background(), ResolveProjectRefParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, Owner: "acme", Number: 7}); err == nil {
		t.Fatal("expected an error when the port call fails")
	}
}

func TestListProjectViews_Success(t *testing.T) {
	gh := &fakeGitHubProjects{views: []ProjectView{{ID: "v1", Name: "Board"}}}
	uc := NewListProjectViews(&fakeCredentialResolver{token: "tok"}, gh)

	got, err := uc.Execute(context.Background(), ListProjectViewsParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ProjectSlug: "acme/7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Board" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestViewProjectTable_Success(t *testing.T) {
	gh := &fakeGitHubProjects{items: []ProjectItem{{ID: "item-1"}}, nextPageToken: "cursor-2"}
	uc := NewViewProjectTable(&fakeCredentialResolver{token: "tok"}, gh)

	got, err := uc.Execute(context.Background(), ViewProjectTableParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ProjectSlug: "acme/7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Items) != 1 || got.NextPageToken != "cursor-2" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestClearProjectItemField_Success(t *testing.T) {
	gh := &fakeGitHubProjects{item: ProjectItem{ID: "item-1"}}
	uc := NewClearProjectItemField(&fakeCredentialResolver{token: "tok"}, gh)

	got, err := uc.Execute(context.Background(), ClearProjectItemFieldParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ProjectSlug: "acme/7", ItemID: "item-1", FieldID: "f1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "item-1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestGetWorkItemDetailsBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{details: WorkItemDetails{Slug: "acme/repo#1", Title: "Bug"}}
	uc := NewGetWorkItemDetailsBySlug(&fakeCredentialResolver{token: "tok"}, gh)

	got, err := uc.Execute(context.Background(), GetWorkItemDetailsBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Bug" || gh.lastItemSlug != "acme/repo#1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestUpdateIssueBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{details: WorkItemDetails{Slug: "acme/repo#1"}}
	uc := NewUpdateIssueBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	title := "new title"
	if _, err := uc.Execute(context.Background(), UpdateIssueBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1", Patch: WorkItemPatch{Title: &title}}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdatePullRequestBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{details: WorkItemDetails{Slug: "acme/repo#1"}}
	uc := NewUpdatePullRequestBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	if _, err := uc.Execute(context.Background(), UpdatePullRequestBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateIssueTypeBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{details: WorkItemDetails{Slug: "acme/repo#1"}}
	uc := NewUpdateIssueTypeBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	if _, err := uc.Execute(context.Background(), UpdateIssueTypeBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1", IssueType: "Bug"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListIssueTypesBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{issueTypes: []IssueType{{ID: "1", Name: "Bug"}}}
	uc := NewListIssueTypesBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	got, err := uc.Execute(context.Background(), ListIssueTypesBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "Bug" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestListAssignableUsersBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{users: []AssignableUser{{Login: "alice"}}}
	uc := NewListAssignableUsersBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	got, err := uc.Execute(context.Background(), ListAssignableUsersBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Login != "alice" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestListLabelsBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{labels: []Label{{Name: "bug"}}}
	uc := NewListLabelsBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	got, err := uc.Execute(context.Background(), ListLabelsBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Name != "bug" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestAddIssueCommentBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{comment: ProjectComment{ID: "c1", Body: "hi"}}
	uc := NewAddIssueCommentBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	got, err := uc.Execute(context.Background(), AddIssueCommentBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1", Body: "hi"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "c1" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestUpdateIssueCommentBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{comment: ProjectComment{ID: "c1", Body: "updated"}}
	uc := NewUpdateIssueCommentBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	got, err := uc.Execute(context.Background(), UpdateIssueCommentBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1", CommentID: "c1", Body: "updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Body != "updated" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestDeleteIssueCommentBySlug_Success(t *testing.T) {
	gh := &fakeGitHubProjects{}
	uc := NewDeleteIssueCommentBySlug(&fakeCredentialResolver{token: "tok"}, gh)
	if err := uc.Execute(context.Background(), DeleteIssueCommentBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, ItemSlug: "acme/repo#1", CommentID: "c1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gh.calls != 1 {
		t.Errorf("expected the port called exactly once, got %d", gh.calls)
	}
}

func TestDeleteIssueCommentBySlug_RejectsNonGitHubProvider(t *testing.T) {
	uc := NewDeleteIssueCommentBySlug(&fakeCredentialResolver{}, &fakeGitHubProjects{})
	if err := uc.Execute(context.Background(), DeleteIssueCommentBySlugParams{TenantID: "t1", Provider: domain.ScmProviderGitea, ItemSlug: "acme/repo#1", CommentID: "c1"}); err == nil {
		t.Fatal("expected a rejection for a non-GitHub provider")
	}
}
