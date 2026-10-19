package grpcclient

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func newBaseRefProvisioner() (*WorktreeProvisioner, *fakeGitGatewayCreateWorktreeClient, *fakeProjectServiceClient) {
	git := &fakeGitGatewayCreateWorktreeClient{createWorktreeResp: &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-new", Path: "/srv/wt-new"}}
	projects := &fakeProjectServiceClient{listReposResp: &projectv1.ListReposResponse{Repos: []*projectv1.Repo{{Id: "repo-1", ProjectId: "proj-1"}}}}
	return NewWorktreeProvisioner(git, projects), git, projects
}

func TestEnsureWorktree_UsesBaseRef(t *testing.T) {
	p, git, _ := newBaseRefProvisioner()
	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}, "release/x"); err != nil {
		t.Fatal(err)
	}
	if got := git.gotCreateWorktree.GetBaseRef(); got != "release/x" {
		t.Fatalf("CreateWorktree base_ref = %q, want release/x", got)
	}
	if git.baseRefDefaultCalls != 0 {
		t.Fatal("an explicit ref must not ask git-gateway for the default branch")
	}
	if got := git.gotCreateWorktree.GetBranch(); got != "task/task-1" {
		t.Fatalf("the task branch is unchanged: %q", got)
	}
}

func TestEnsureWorktree_EmptyBaseRefUsesRepoDefault(t *testing.T) {
	for name, task := range map[string]domain.Task{
		"task of a Request":   {ID: "task-1", ProjectID: "proj-1", RequestID: "req-1"},
		"task run on its own": {ID: "task-2", ProjectID: "proj-1"},
	} {
		t.Run(name, func(t *testing.T) {
			p, git, _ := newBaseRefProvisioner()
			git.baseRefDefaultResp = &gitgatewayv1.BaseRefDefaultResponse{Ref: "develop"}
			ctx := tenant.WithUserID(ctxWithTenant(t), "user-1")
			if _, _, err := p.EnsureWorktree(ctx, "tenant-1", task, ""); err != nil {
				t.Fatal(err)
			}
			if git.gotBaseRefDefault.GetRepoId() != "repo-1" {
				t.Fatalf("the default is asked for the task's repo: %q", git.gotBaseRefDefault.GetRepoId())
			}
			if got := git.gotCreateWorktree.GetBaseRef(); got != "develop" {
				t.Fatalf("CreateWorktree base_ref = %q, want the repo default develop", got)
			}
			md, _ := metadata.FromOutgoingContext(git.gotBaseRefDefaultCtx)
			if md.Get(grpcmw.MetadataTenantID)[0] != "tenant-1" || md.Get(grpcmw.MetadataUserID)[0] != "user-1" {
				t.Fatalf("the lookup runs as the tenant and the user: %v", md)
			}
		})
	}
}

func TestEnsureWorktree_DefaultLookupFails_ErrorNoHeadFallback(t *testing.T) {
	cases := map[string]*fakeGitGatewayCreateWorktreeClient{
		"rpc error": {baseRefDefaultErr: errors.New("git-gateway unavailable")},
		"empty ref": {baseRefDefaultResp: &gitgatewayv1.BaseRefDefaultResponse{}},
	}
	for name, git := range cases {
		t.Run(name, func(t *testing.T) {
			p, _, _ := newBaseRefProvisioner()
			p.git = git
			_, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}, "")
			if err == nil || !strings.Contains(err.Error(), "default branch") {
				t.Fatalf("a failed default lookup must be an explicit error, got %v", err)
			}
			if git.createWorktreeCalled {
				t.Fatal("no worktree may be created from HEAD when the default branch is unknown")
			}
			if name == "rpc error" && !errors.Is(err, git.baseRefDefaultErr) {
				t.Fatalf("the cause must be kept for the caller to log: %v", err)
			}
		})
	}
}

func TestEnsureWorktree_ReuseExistingIgnoresBaseRef(t *testing.T) {
	p, git, projects := newBaseRefProvisioner()
	projects.getWorktreeResp = &projectv1.Worktree{Id: "wt-existing", Path: "/srv/wt-existing"}
	id, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", WorktreeID: "wt-existing"}, "release/x")
	if err != nil || id != "wt-existing" || path != "/srv/wt-existing" {
		t.Fatalf("got %q %q %v", id, path, err)
	}
	if git.createWorktreeCalled || git.baseRefDefaultCalls != 0 {
		t.Fatal("a reused worktree is neither created nor does it look up a default")
	}
}

func TestEnsureWorktree_AdoptsIssueWorktree(t *testing.T) {
	p, git, _ := newIssueReuseProvisioner(jiraSource("ENG-1"), []*projectv1.Worktree{
		{Id: "wt-eng1", Path: "/p/eng1", LinkedIssueProvider: strp("jira"), LinkedIssueRef: strp("ENG-1"), Status: "active"},
	}, nil)
	id, path, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1"}, "release/x")
	if err != nil || id != "wt-eng1" || path != "/p/eng1" {
		t.Fatalf("got %q %q %v", id, path, err)
	}
	if git.createWorktreeCalled || git.baseRefDefaultCalls != 0 {
		t.Fatal("an adopted issue worktree keeps the branch it already has; the ref is not applied")
	}
}

func TestEnsureWorktree_GoneWorktreeIsRecreatedFromTheRef(t *testing.T) {
	p, git, projects := newBaseRefProvisioner()
	projects.getWorktreeErr = status.Error(codes.NotFound, "worktree not found")
	if _, _, err := p.EnsureWorktree(ctxWithTenant(t), "tenant-1", domain.Task{ID: "task-1", ProjectID: "proj-1", WorktreeID: "wt-gone"}, "release/x"); err != nil {
		t.Fatal(err)
	}
	if got := git.gotCreateWorktree.GetBaseRef(); got != "release/x" {
		t.Fatalf("a worktree that no longer exists is created again from the Request's ref, got %q", got)
	}
}
