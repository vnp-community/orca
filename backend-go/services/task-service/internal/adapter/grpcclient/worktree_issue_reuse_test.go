package grpcclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type fakeTaskSourceReader struct {
	src domain.TaskSource
	ok  bool
	err error
}

func (f fakeTaskSourceReader) GetSource(context.Context, string, string) (domain.TaskSource, bool, error) {
	return f.src, f.ok, f.err
}

// projectWithWorktrees adds ListWorktrees to the shared project fake.
type projectWithWorktrees struct {
	*fakeProjectServiceClient
	worktrees []*projectv1.Worktree
	err       error
	called    bool
}

func (f *projectWithWorktrees) ListWorktrees(_ context.Context, _ *projectv1.ListWorktreesRequest, _ ...grpc.CallOption) (*projectv1.ListWorktreesResponse, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return &projectv1.ListWorktreesResponse{Worktrees: f.worktrees}, nil
}

func strp(s string) *string { return &s }

func jiraSource(ref string) fakeTaskSourceReader {
	return fakeTaskSourceReader{ok: true, src: domain.TaskSource{Provider: domain.SourceProviderJira, Ref: ref}}
}

func newIssueReuseProvisioner(sources TaskSourceReader, wts []*projectv1.Worktree, listErr error) (*WorktreeProvisioner, *fakeGitGatewayCreateWorktreeClient, *projectWithWorktrees) {
	git := &fakeGitGatewayCreateWorktreeClient{createWorktreeResp: &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-new", Path: "/srv/wt-new"}}
	projects := &projectWithWorktrees{
		fakeProjectServiceClient: &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "repo-1", ProjectId: "proj-1"}}}},
		worktrees:                wts, err: listErr,
	}
	return NewWorktreeProvisioner(git, projects).WithTaskSources(sources), git, projects
}

func TestWorktreeProvisioner_AdoptsWorktreeLinkedToTaskIssue(t *testing.T) {
	p, git, _ := newIssueReuseProvisioner(jiraSource("ENG-1"), []*projectv1.Worktree{
		{Id: "wt-other", Path: "/p/other", LinkedIssueProvider: strp("jira"), LinkedIssueRef: strp("ENG-2"), Status: "active"},
		{Id: "wt-eng1", Path: "/p/eng1", LinkedIssueProvider: strp("jira"), LinkedIssueRef: strp("ENG-1"), Status: "active"},
	}, nil)

	id, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "wt-eng1" || path != "/p/eng1" {
		t.Errorf("want the ENG-1 worktree adopted, got id=%q path=%q", id, path)
	}
	if git.createWorktreeCalled {
		t.Error("must not create a second worktree for the same issue")
	}
}

func TestWorktreeProvisioner_IssueReuse_SkipsOwnedByOtherTaskAndInactive(t *testing.T) {
	p, git, _ := newIssueReuseProvisioner(jiraSource("ENG-1"), []*projectv1.Worktree{
		{Id: "wt-owned", Path: "/p/owned", LinkedIssueProvider: strp("jira"), LinkedIssueRef: strp("ENG-1"), TaskId: strp("task-9"), Status: "active"},
		{Id: "wt-done", Path: "/p/done", LinkedIssueProvider: strp("jira"), LinkedIssueRef: strp("ENG-1"), Status: "completed"},
	}, nil)

	id, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "wt-new" || !git.createWorktreeCalled {
		t.Errorf("want a fresh worktree when candidates are owned/inactive, got id=%q created=%v", id, git.createWorktreeCalled)
	}
}

func TestWorktreeProvisioner_IssueReuse_LookupFailureFallsBackToCreate(t *testing.T) {
	p, git, _ := newIssueReuseProvisioner(jiraSource("ENG-1"), nil, errors.New("project-service down"))
	id, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"})
	if err != nil || id != "wt-new" || !git.createWorktreeCalled {
		t.Fatalf("want fall back to create, got id=%q err=%v created=%v", id, err, git.createWorktreeCalled)
	}
}

func TestWorktreeProvisioner_NoSource_DoesNotListWorktrees(t *testing.T) {
	p, git, projects := newIssueReuseProvisioner(fakeTaskSourceReader{}, nil, nil)
	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if projects.called || !git.createWorktreeCalled {
		t.Errorf("task without a source must keep the old path: listed=%v created=%v", projects.called, git.createWorktreeCalled)
	}
}

func TestWorktreeProvisioner_ExistingTaskWorktreeWinsOverIssueLookup(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{}
	projects := &projectWithWorktrees{fakeProjectServiceClient: &fakeProjectServiceClient{getWorktreeResp: &projectv1.Worktree{Id: "wt-own", Path: "/p/own"}}}
	p := NewWorktreeProvisioner(git, projects).WithTaskSources(jiraSource("ENG-1"))

	id, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", WorktreeID: "wt-own"})
	if err != nil || id != "wt-own" || projects.called {
		t.Fatalf("task.WorktreeID must win: id=%q err=%v listed=%v", id, err, projects.called)
	}
}

func TestWorktreeProvisioner_CreateCarriesIssueLinkAndTaskID(t *testing.T) {
	p, git, _ := newIssueReuseProvisioner(jiraSource("ENG-1"), nil, nil)
	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	req := git.gotCreateWorktree
	if req.GetLinkedIssueProvider() != "jira" || req.GetLinkedIssueRef() != "ENG-1" {
		t.Errorf("want the task's issue recorded on the new worktree, got %q/%q", req.GetLinkedIssueProvider(), req.GetLinkedIssueRef())
	}
	if req.GetTaskId() != "task-1" {
		t.Errorf("existing task_id behavior must be kept, got %q", req.GetTaskId())
	}
}

func TestWorktreeProvisioner_CreateWithoutSourceSendsNoIssueLink(t *testing.T) {
	p, git, _ := newIssueReuseProvisioner(fakeTaskSourceReader{}, nil, nil)
	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if git.gotCreateWorktree.LinkedIssueProvider != nil || git.gotCreateWorktree.LinkedIssueRef != nil {
		t.Error("a task without a source must not send an issue link")
	}
}

func TestWorktreeProvisioner_SourceLookupErrorStillCreatesWorktree(t *testing.T) {
	p, git, _ := newIssueReuseProvisioner(fakeTaskSourceReader{err: errors.New("table missing")}, nil, nil)
	id, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"})
	if err != nil || id != "wt-new" || git.gotCreateWorktree.LinkedIssueProvider != nil {
		t.Fatalf("lookup failure must degrade to a plain create: id=%q err=%v", id, err)
	}
}

func TestWorktreeProvisioner_RemovedWorktree_IsRecreatedNotFatal(t *testing.T) {
	git := &fakeGitGatewayCreateWorktreeClient{createWorktreeResp: &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-fresh", Path: "/srv/wt-fresh"}}
	projects := &projectWithWorktrees{fakeProjectServiceClient: &fakeProjectServiceClient{
		getWorktreeErr: status.Error(codes.NotFound, "worktree not found"),
		listReposResp:  &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "repo-1", ProjectId: "proj-1"}}},
	}}
	p := NewWorktreeProvisioner(git, projects)

	id, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", WorktreeID: "wt-gone"})
	if err != nil {
		t.Fatalf("a task whose worktree was cleaned up must get a new one, got %v", err)
	}
	if id != "wt-fresh" || path != "/srv/wt-fresh" || !git.createWorktreeCalled {
		t.Errorf("want a freshly created worktree, got id=%q path=%q created=%v", id, path, git.createWorktreeCalled)
	}
}

// Only "does not exist" may recreate: an unreachable or forbidden project-service
// says nothing about the worktree, and guessing would reintroduce BUG-028.
func TestWorktreeProvisioner_OtherLookupErrors_StillFailClosed(t *testing.T) {
	for name, lookupErr := range map[string]error{
		"unavailable":       status.Error(codes.Unavailable, "project-service down"),
		"permission denied": status.Error(codes.PermissionDenied, "no access"),
		"plain error":       errors.New("boom"),
	} {
		t.Run(name, func(t *testing.T) {
			git := &fakeGitGatewayCreateWorktreeClient{}
			projects := &projectWithWorktrees{fakeProjectServiceClient: &fakeProjectServiceClient{getWorktreeErr: lookupErr}}
			p := NewWorktreeProvisioner(git, projects)

			if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", WorktreeID: "wt-1"}); err == nil {
				t.Fatal("expected an error")
			}
			if git.createWorktreeCalled {
				t.Error("must not create a worktree when the lookup failed for any reason other than not found")
			}
		})
	}
}

// ctxCapturingGit records the context CreateWorktree was called with.
type ctxCapturingGit struct {
	*fakeGitGatewayCreateWorktreeClient
	gotCtx context.Context
}

func (g *ctxCapturingGit) CreateWorktree(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest, opts ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	g.gotCtx = ctx
	return g.fakeGitGatewayCreateWorktreeClient.CreateWorktree(ctx, in, opts...)
}

func TestWorktreeProvisioner_CreateForwardsTheActingUser(t *testing.T) {
	git := &ctxCapturingGit{fakeGitGatewayCreateWorktreeClient: &fakeGitGatewayCreateWorktreeClient{
		createWorktreeResp: &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-new", Path: "/srv/wt-new"}}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "repo-1", ProjectId: "proj-1"}}}}
	p := NewWorktreeProvisioner(git, projects)

	ctx := tenant.WithUserID(ctxWithTenant(t), "user-7")
	if _, _, err := p.EnsureWorktree(ctx, "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err != nil {
		t.Fatal(err)
	}
	md, _ := metadata.FromOutgoingContext(git.gotCtx)
	if got := md.Get(grpcmw.MetadataUserID); len(got) != 1 || got[0] != "user-7" {
		t.Errorf("the creating user must reach git-gateway, got %v", got)
	}
	if got := md.Get(grpcmw.MetadataTenantID); len(got) != 1 {
		t.Errorf("tenant must still be forwarded, got %v", got)
	}
}

func TestWorktreeProvisioner_CreateWithoutUserSendsNoUserHeader(t *testing.T) {
	git := &ctxCapturingGit{fakeGitGatewayCreateWorktreeClient: &fakeGitGatewayCreateWorktreeClient{
		createWorktreeResp: &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-new", Path: "/srv/wt-new"}}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "repo-1", ProjectId: "proj-1"}}}}
	p := NewWorktreeProvisioner(git, projects)

	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}); err != nil {
		t.Fatal(err)
	}
	md, _ := metadata.FromOutgoingContext(git.gotCtx)
	if got := md.Get(grpcmw.MetadataUserID); len(got) != 0 {
		t.Errorf("no user in context must not invent one, got %v", got)
	}
}
