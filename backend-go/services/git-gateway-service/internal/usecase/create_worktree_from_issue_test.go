package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// fakeIssueSourceClient is an in-memory IssueSourceClient — test against a
// fake, never a real scm-integration-service/issue-tracking-service call.
type fakeIssueSourceClient struct {
	issue   domain.Issue
	err     error
	lastRef domain.IssueRef
	calls   int
}

func (f *fakeIssueSourceClient) GetIssue(ctx context.Context, ref domain.IssueRef) (domain.Issue, error) {
	f.lastRef = ref
	f.calls++
	if f.err != nil {
		return domain.Issue{}, f.err
	}
	return f.issue, nil
}

// fakeAgentSpawner is an in-memory AgentSpawner.
type fakeAgentSpawner struct {
	sessionID string
	err       error

	calls          int
	lastWorktreeID string
	lastCwd        string
	lastPrompt     string
}

func (f *fakeAgentSpawner) SpawnAndInject(ctx context.Context, worktreeID, cwd, prompt string) (string, error) {
	f.calls++
	f.lastWorktreeID, f.lastCwd, f.lastPrompt = worktreeID, cwd, prompt
	if f.err != nil {
		return "", f.err
	}
	return f.sessionID, nil
}

func newHappyIssue() domain.Issue {
	return domain.Issue{
		Title: "Login button is broken", Description: "It does nothing on click.",
		AcceptanceCriteria: "Clicking it navigates to /login", Labels: []string{"bug"},
		Comments: []string{"can repro"}, Provider: "github", ExternalRef: "owner/repo#42",
	}
}

func TestCreateWorktreeFromIssue_HappyPath(t *testing.T) {
	issues := &fakeIssueSourceClient{issue: newHappyIssue()}
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-fix", HeadSHA: "sha1"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{
		recordCreatedResult:    domain.WorktreeRecord{ID: "wt-1", Path: "/repo-fix", Branch: "fix/login-button-is-broken-owner-repo-42"},
		issueStatusSyncEnabled: true,
	}
	createWT := NewCreateWorktree(resolver, projects, local, relay)
	agents := &fakeAgentSpawner{sessionID: "session-1"}
	uc := NewCreateWorktreeFromIssue(issues, createWT, agents, projects)

	result, err := uc.Execute(context.Background(), CreateWorktreeFromIssueInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main",
		IssueRef: domain.IssueRef{Provider: "github", Repo: "owner/repo", Number: 42},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if issues.calls != 1 {
		t.Fatalf("expected the issue to be fetched exactly once, got %d", issues.calls)
	}
	if result.BranchName != "fix/login-button-is-broken-owner-repo-42" {
		t.Fatalf("unexpected derived branch name: %q", result.BranchName)
	}
	if result.Worktree.WorktreeID != "wt-1" || result.Worktree.Path != "/repo-fix" {
		t.Fatalf("unexpected worktree result: %+v", result.Worktree)
	}
	if result.AgentSessionID != "session-1" || result.AgentStartError != "" {
		t.Fatalf("expected a successful agent spawn, got sessionID=%q startErr=%q", result.AgentSessionID, result.AgentStartError)
	}
	if !projects.calledRecordCreated {
		t.Fatal("expected RecordWorktreeCreated to be called")
	}
	if projects.gotRecordCreatedLineage.LinkedIssueProvider != "github" || projects.gotRecordCreatedLineage.LinkedIssueRef != "owner/repo#42" {
		t.Fatalf("expected the linked-issue lineage to reach CreateWorktree, got %+v", projects.gotRecordCreatedLineage)
	}
	if !result.StatusUpdateEnqueued {
		t.Error("expected StatusUpdateEnqueued=true when the link was recorded")
	}
	if agents.calls != 1 || agents.lastWorktreeID != "wt-1" || agents.lastCwd != "/repo-fix" {
		t.Fatalf("expected the agent to be spawned against the new worktree, got calls=%d worktreeID=%q cwd=%q", agents.calls, agents.lastWorktreeID, agents.lastCwd)
	}
}

// TestCreateWorktreeFromIssue_DuplicateBranch_SurfacesCreateWorktreeErrorUnchanged
// is the pre-flight-rejection regression guard: CreateWorktree's own git
// worktree add failure (e.g. branch already exists) must surface unchanged,
// with no agent spawn attempted.
func TestCreateWorktreeFromIssue_DuplicateBranch_SurfacesCreateWorktreeErrorUnchanged(t *testing.T) {
	issues := &fakeIssueSourceClient{issue: newHappyIssue()}
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeErr: errors.New("git worktree add failed: branch 'fix/login-button-is-broken-owner-repo-42' already exists")}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	createWT := NewCreateWorktree(resolver, projects, local, relay)
	agents := &fakeAgentSpawner{sessionID: "session-1"}
	uc := NewCreateWorktreeFromIssue(issues, createWT, agents, projects)

	_, err := uc.Execute(context.Background(), CreateWorktreeFromIssueInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main",
		IssueRef: domain.IssueRef{Provider: "github", Repo: "owner/repo", Number: 42},
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_CREATE_FAILED" {
		t.Fatalf("expected the saga to surface CreateWorktree's own WORKTREE_CREATE_FAILED unchanged, got %v", err)
	}
	if agents.calls != 0 {
		t.Error("expected no agent spawn attempt when the worktree was never created")
	}
	if projects.calledRecordCreated {
		t.Error("expected no bookkeeping call when git worktree add itself failed")
	}
}

// TestCreateWorktreeFromIssue_AgentSpawnFailure_WorktreeStillReturnedNoRollback
// is the regression guard against over-eager compensation: an agent-spawn
// failure is non-fatal, and must NOT roll back an already-created worktree.
func TestCreateWorktreeFromIssue_AgentSpawnFailure_WorktreeStillReturnedNoRollback(t *testing.T) {
	issues := &fakeIssueSourceClient{issue: newHappyIssue()}
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-fix", HeadSHA: "sha1"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-fix"}}
	createWT := NewCreateWorktree(resolver, projects, local, relay)
	agents := &fakeAgentSpawner{err: errors.New("dev server not connected")}
	uc := NewCreateWorktreeFromIssue(issues, createWT, agents, projects)

	result, err := uc.Execute(context.Background(), CreateWorktreeFromIssueInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main",
		IssueRef: domain.IssueRef{Provider: "github", Repo: "owner/repo", Number: 42},
	})
	if err != nil {
		t.Fatalf("expected agent spawn failure to be non-fatal, got error: %v", err)
	}
	if result.Worktree.WorktreeID != "wt-1" {
		t.Fatalf("expected the worktree to still be returned, got %+v", result.Worktree)
	}
	if result.AgentStartError == "" {
		t.Error("expected AgentStartError to be populated")
	}
	if result.AgentSessionID != "" {
		t.Errorf("expected no session id on spawn failure, got %q", result.AgentSessionID)
	}
	if local.calledRemoveWorktree {
		t.Error("expected NO rollback of the worktree on a non-fatal agent spawn failure")
	}
}

func TestCreateWorktreeFromIssue_SkipAgentStart_AgentSpawnerNeverCalled(t *testing.T) {
	issues := &fakeIssueSourceClient{issue: newHappyIssue()}
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-fix", HeadSHA: "sha1"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-fix"}}
	createWT := NewCreateWorktree(resolver, projects, local, relay)
	agents := &fakeAgentSpawner{sessionID: "should-not-happen"}
	uc := NewCreateWorktreeFromIssue(issues, createWT, agents, projects)

	result, err := uc.Execute(context.Background(), CreateWorktreeFromIssueInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main",
		IssueRef:       domain.IssueRef{Provider: "github", Repo: "owner/repo", Number: 42},
		SkipAgentStart: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agents.calls != 0 {
		t.Fatalf("expected SpawnAndInject to never be called when skip_agent_start is set, got %d calls", agents.calls)
	}
	if result.AgentSessionID != "" {
		t.Errorf("expected no session id when agent start was skipped, got %q", result.AgentSessionID)
	}
}

// TestCreateWorktreeFromIssue_SkipStatusUpdate_LineageIssueFieldsEmpty is
// BR-PI-06's regression guard: the opt-out must reach project-service's
// persisted row via CreateWorktree's own Lineage param, not just this
// saga's own in-memory state.
func TestCreateWorktreeFromIssue_SkipStatusUpdate_LineageIssueFieldsEmpty(t *testing.T) {
	issues := &fakeIssueSourceClient{issue: newHappyIssue()}
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-fix", HeadSHA: "sha1"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-fix"}, issueStatusSyncEnabled: true}
	createWT := NewCreateWorktree(resolver, projects, local, relay)
	agents := &fakeAgentSpawner{}
	uc := NewCreateWorktreeFromIssue(issues, createWT, agents, projects)

	result, err := uc.Execute(context.Background(), CreateWorktreeFromIssueInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main",
		IssueRef:         domain.IssueRef{Provider: "github", Repo: "owner/repo", Number: 42},
		SkipStatusUpdate: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects.gotRecordCreatedLineage.LinkedIssueProvider != "" || projects.gotRecordCreatedLineage.LinkedIssueRef != "" {
		t.Fatalf("expected empty linked-issue lineage reaching CreateWorktree when skip_status_update is set, got %+v", projects.gotRecordCreatedLineage)
	}
	if result.StatusUpdateEnqueued {
		t.Error("expected StatusUpdateEnqueued=false when skip_status_update is set")
	}
}

func TestCreateWorktreeFromIssue_IssueStatusSyncDisabled_LineageIssueFieldsEmpty(t *testing.T) {
	issues := &fakeIssueSourceClient{issue: newHappyIssue()}
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-fix", HeadSHA: "sha1"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{recordCreatedResult: domain.WorktreeRecord{ID: "wt-1", Path: "/repo-fix"}, issueStatusSyncEnabled: false}
	createWT := NewCreateWorktree(resolver, projects, local, relay)
	agents := &fakeAgentSpawner{}
	uc := NewCreateWorktreeFromIssue(issues, createWT, agents, projects)

	result, err := uc.Execute(context.Background(), CreateWorktreeFromIssueInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main",
		IssueRef: domain.IssueRef{Provider: "github", Repo: "owner/repo", Number: 42},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects.gotRecordCreatedLineage.LinkedIssueProvider != "" || projects.gotRecordCreatedLineage.LinkedIssueRef != "" {
		t.Fatalf("expected empty linked-issue lineage reaching CreateWorktree when project-level sync is disabled, got %+v", projects.gotRecordCreatedLineage)
	}
	if result.StatusUpdateEnqueued {
		t.Error("expected StatusUpdateEnqueued=false when project-level sync is disabled")
	}
}

// TestCreateWorktreeFromIssue_AgentAndStatusFailuresNeverFailTheSaga is the
// named invariant test the task spec calls for: combining a non-fatal
// agent-spawn failure with disabled status sync must still return a
// successful saga result (err == nil), worktree intact.
func TestCreateWorktreeFromIssue_AgentAndStatusFailuresNeverFailTheSaga(t *testing.T) {
	issues := &fakeIssueSourceClient{issue: newHappyIssue()}
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{createWorktreeResult: domain.WorktreeCreateResult{Path: "/repo-fix", HeadSHA: "sha1"}}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{
		recordCreatedResult:       domain.WorktreeRecord{ID: "wt-1", Path: "/repo-fix"},
		issueStatusSyncEnabled:    false,
		issueStatusSyncEnabledErr: errors.New("project-service unreachable"),
	}
	createWT := NewCreateWorktree(resolver, projects, local, relay)
	agents := &fakeAgentSpawner{err: errors.New("agent spawn boom")}
	uc := NewCreateWorktreeFromIssue(issues, createWT, agents, projects)

	result, err := uc.Execute(context.Background(), CreateWorktreeFromIssueInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main",
		IssueRef: domain.IssueRef{Provider: "github", Repo: "owner/repo", Number: 42},
	})
	if err != nil {
		t.Fatalf("expected agent-spawn and status-sync failures to never fail the saga, got: %v", err)
	}
	if result.Worktree.WorktreeID != "wt-1" {
		t.Fatalf("expected the worktree to still be returned, got %+v", result.Worktree)
	}
	if result.AgentStartError == "" {
		t.Error("expected AgentStartError to be populated")
	}
	if local.calledRemoveWorktree {
		t.Error("expected no compensation for either non-fatal failure")
	}
}
