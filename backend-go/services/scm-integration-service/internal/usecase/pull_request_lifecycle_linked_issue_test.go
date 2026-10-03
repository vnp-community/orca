package usecase

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func decodeLifecyclePayload(t *testing.T, outbox *fakeOutboxEnqueuer, subject string) map[string]any {
	t.Helper()
	if outbox.calls != 1 || outbox.lastEvent.Subject != subject {
		t.Fatalf("want one %s event, got calls=%d subject=%q", subject, outbox.calls, outbox.lastEvent.Subject)
	}
	var m map[string]any
	if err := json.Unmarshal(outbox.lastEvent.PayloadJSON, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func createPRHarness(t *testing.T) (*CreatePullRequest, *fakeOutboxEnqueuer) {
	t.Helper()
	pr, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "t", "open", "url", "head", "main")
	pr.Number = 42
	provider := &fakeProvider{pr: pr, branchExists: true}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	creds := &fakeCredentialResolver{token: "tok"}
	outbox := &fakeOutboxEnqueuer{}
	return NewCreatePullRequest(creds, registry, NewUpdateIssue(creds, registry), outbox, nil), outbox
}

func TestCreatePullRequest_EventCarriesParsedJiraIssueAndActor(t *testing.T) {
	uc, outbox := createPRHarness(t)
	ctx := tenant.WithUserID(context.Background(), "user-7")

	_, err := uc.Execute(ctx, CreatePullRequestParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r",
		Title: "add login", HeadBranch: "feature/ENG-123-login", BaseBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := decodeLifecyclePayload(t, outbox, subjectPullRequestCreated)
	if m["linked_issue_provider"] != "jira" || m["linked_issue_ref"] != "ENG-123" || m["actor_user_id"] != "user-7" || m["pr_number"] != float64(42) {
		t.Errorf("unexpected payload %v", m)
	}
}

func TestCreatePullRequest_CallerProvidedLinkWinsAndNoActorIsOmitted(t *testing.T) {
	uc, outbox := createPRHarness(t)

	_, err := uc.Execute(context.Background(), CreatePullRequestParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r",
		Title: "ENG-1 thing", HeadBranch: "ENG-1-x", BaseBranch: "main",
		LinkedIssueProvider: "linear", LinkedIssueRef: "LIN-9",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := decodeLifecyclePayload(t, outbox, subjectPullRequestCreated)
	if m["linked_issue_provider"] != "linear" || m["linked_issue_ref"] != "LIN-9" {
		t.Errorf("caller-provided link must win, got %v", m)
	}
	if _, present := m["actor_user_id"]; present {
		t.Errorf("actor_user_id must be omitted when unknown, got %v", m)
	}
}

func TestCreatePullRequest_NoIssueReferenceOmitsLinkFields(t *testing.T) {
	uc, outbox := createPRHarness(t)

	_, err := uc.Execute(context.Background(), CreatePullRequestParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r",
		Title: "support UTF-8", HeadBranch: "utf8", BaseBranch: "main",
	})
	if err != nil {
		t.Fatal(err)
	}
	m := decodeLifecyclePayload(t, outbox, subjectPullRequestCreated)
	if _, present := m["linked_issue_ref"]; present {
		t.Errorf("no key expected, got %v", m)
	}
}

func TestMergePullRequest_EventCarriesParsedJiraIssueAndActor(t *testing.T) {
	pr, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "ENG-5 add login", "closed", "url", "feature/ENG-5-login", "main")
	pr.Number = 42
	provider := &fakeProvider{mergedPR: pr, merged: true, mergeSHA: "abc"}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	outbox := &fakeOutboxEnqueuer{}
	uc := NewMergePullRequest(&fakeCredentialResolver{token: "tok"}, registry, outbox, nil)
	ctx := tenant.WithUserID(context.Background(), "user-7")

	if _, err := uc.Execute(ctx, MergePullRequestParams{TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 42}); err != nil {
		t.Fatal(err)
	}
	m := decodeLifecyclePayload(t, outbox, subjectPullRequestMerged)
	if m["linked_issue_provider"] != "jira" || m["linked_issue_ref"] != "ENG-5" || m["actor_user_id"] != "user-7" {
		t.Errorf("unexpected payload %v", m)
	}
}

func TestMergePullRequest_CallerProvidedLinkWins(t *testing.T) {
	pr, _ := domain.NewPullRequest("1", domain.ScmProviderGitHub, "o/r", "ENG-5 x", "closed", "url", "ENG-5-x", "main")
	provider := &fakeProvider{mergedPR: pr, merged: true}
	registry := &fakeRegistry{providers: map[domain.ScmProvider]ScmProvider{domain.ScmProviderGitHub: provider}}
	outbox := &fakeOutboxEnqueuer{}
	uc := NewMergePullRequest(&fakeCredentialResolver{token: "tok"}, registry, outbox, nil)

	_, err := uc.Execute(context.Background(), MergePullRequestParams{
		TenantID: "t1", Provider: domain.ScmProviderGitHub, Repo: "o/r", Number: 1,
		LinkedIssueProvider: "jira", LinkedIssueRef: "OPS-2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if m := decodeLifecyclePayload(t, outbox, subjectPullRequestMerged); m["linked_issue_ref"] != "OPS-2" {
		t.Errorf("caller-provided ref must win, got %v", m)
	}
}
