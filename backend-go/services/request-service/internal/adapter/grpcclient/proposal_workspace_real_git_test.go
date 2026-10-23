package grpcclient

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// These tests run ProposalWorkspace and GitBaseRefChecker against a REAL git repository. The git-gateway process itself is not
// started (its executor is an internal package of another module); realGitGateway re-implements the few calls the adapters
// make with the same git commands and the same error codes as git-gateway's local executor and use cases, so everything on
// request-service's side of the gRPC boundary is real: the commands, the refs, the worktrees on disk, the SHAs.

type realGitGateway struct {
	gitgatewayv1.GitGatewayServiceClient
	t        *testing.T
	repoPath string
	mu       sync.Mutex
	byKey    map[string]*gitgatewayv1.CreateWorktreeResponse
	// projects receives a worktree record for every worktree made, as git-gateway's saga writes one to project-service.
	projects *fakeProjects
}

func (g *realGitGateway) git(args ...string) (string, error) {
	g.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", g.repoPath}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// BaseRefDefault is localgit's: the remote's HEAD branch, which is not the branch checked out in the clone.
func (g *realGitGateway) BaseRefDefault(context.Context, *gitgatewayv1.BaseRefDefaultRequest, ...grpc.CallOption) (*gitgatewayv1.BaseRefDefaultResponse, error) {
	out, err := g.git("symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return nil, status.Error(codes.Internal, "GITGATEWAY_BASE_REF_DEFAULT_FAILED: "+err.Error())
	}
	return &gitgatewayv1.BaseRefDefaultResponse{Ref: out[strings.LastIndex(out, "/")+1:]}, nil
}

// CreateWorktree is git-gateway's: worktree add at repoPath-<branch>, -b unless the branch exists locally, and a git
// "invalid reference" answer becomes WORKTREE_BASE_REF_NOT_FOUND with the local branches listed.
func (g *realGitGateway) CreateWorktree(_ context.Context, in *gitgatewayv1.CreateWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if prev, ok := g.byKey[in.GetIdempotencyKey()]; ok && in.IdempotencyKey != nil {
		return prev, nil
	}
	target := g.repoPath + "-" + strings.ReplaceAll(in.GetBranch(), "/", "-")
	if _, err := os.Stat(target); err == nil { // git-gateway's duplicate-path pre-check
		return nil, status.Error(codes.AlreadyExists, "WORKTREE_PATH_EXISTS: path already exists")
	}
	args := []string{"worktree", "add", target, "-b", in.GetBranch(), in.GetBaseRef()}
	if _, err := g.git("show-ref", "--verify", "--quiet", "refs/heads/"+in.GetBranch()); err == nil {
		args = []string{"worktree", "add", target, in.GetBranch()}
	}
	if out, err := g.git(args...); err != nil {
		if strings.Contains(out, "invalid reference") || strings.Contains(out, "not a valid object name") || strings.Contains(out, "unknown revision") {
			names, _ := g.git("for-each-ref", "--format=%(refname:short)", "refs/heads/")
			return nil, status.Error(codes.NotFound, fmt.Sprintf("WORKTREE_BASE_REF_NOT_FOUND: branch %q not found; available: %s", in.GetBaseRef(), strings.ReplaceAll(names, "\n", ", ")))
		}
		return nil, status.Error(codes.Internal, "WORKTREE_CREATE_FAILED: "+err.Error())
	}
	head, _ := exec.Command("git", "-C", target, "rev-parse", "HEAD").Output()
	resp := &gitgatewayv1.CreateWorktreeResponse{WorktreeId: "wt-" + filepath.Base(target), Path: target, HeadSha: strings.TrimSpace(string(head))}
	if g.byKey == nil {
		g.byKey = map[string]*gitgatewayv1.CreateWorktreeResponse{}
	}
	g.byKey[in.GetIdempotencyKey()] = resp
	if g.projects != nil {
		g.projects.worktrees = append(g.projects.worktrees, &projectv1.Worktree{Id: resp.WorktreeId, Path: target, Branch: in.GetBranch(),
			RepoId: in.GetRepoId(), Active: true})
	}
	return resp, nil
}

// SearchRefs is localgit's: short ref names containing the query.
func (g *realGitGateway) SearchRefs(_ context.Context, in *gitgatewayv1.SearchRefsRequest, _ ...grpc.CallOption) (*gitgatewayv1.SearchRefsResponse, error) {
	out, err := g.git("for-each-ref", "--format=%(refname:short)")
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	var refs []string
	for _, r := range strings.Split(out, "\n") {
		if r != "" && strings.Contains(r, in.GetQuery()) {
			refs = append(refs, r)
		}
	}
	return &gitgatewayv1.SearchRefsResponse{Refs: refs}, nil
}

func (g *realGitGateway) GetStatus(_ context.Context, in *gitgatewayv1.GetStatusRequest, _ ...grpc.CallOption) (*gitgatewayv1.GetStatusResponse, error) {
	path := g.repoPath + "-" + strings.TrimPrefix(in.GetWorktreeId(), "wt-"+filepath.Base(g.repoPath)+"-")
	for _, resp := range g.byKey {
		if resp.GetWorktreeId() == in.GetWorktreeId() {
			path = resp.GetPath()
		}
	}
	if _, err := os.Stat(path); err != nil {
		return nil, status.Error(codes.NotFound, "worktree gone")
	}
	return &gitgatewayv1.GetStatusResponse{}, nil
}

// PrefetchCreateBase is localgit.FetchAndResolveRef plus the use case's classification.
func (g *realGitGateway) PrefetchCreateBase(_ context.Context, in *gitgatewayv1.PrefetchCreateBaseRequest, _ ...grpc.CallOption) (*gitgatewayv1.PrefetchCreateBaseResponse, error) {
	ref := in.GetBaseRef()
	out, fetchErr := g.git("fetch", "origin", ref)
	if fetchErr == nil {
		sha, err := g.git("rev-parse", "FETCH_HEAD^{commit}")
		return &gitgatewayv1.PrefetchCreateBaseResponse{ResolvedSha: sha}, mapErr(err)
	}
	if sha, err := g.git("rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
		return &gitgatewayv1.PrefetchCreateBaseResponse{ResolvedSha: sha}, nil
	}
	if strings.Contains(out, "couldn't find remote ref") || strings.Contains(out, "'origin' does not appear") {
		names, _ := g.git("for-each-ref", "--format=%(refname:short)", "refs/heads/")
		return nil, status.Error(codes.NotFound, fmt.Sprintf("WORKTREE_BASE_REF_NOT_FOUND: branch %q not found; available: %s", ref, strings.ReplaceAll(names, "\n", ", ")))
	}
	return nil, status.Error(codes.Unavailable, "WORKTREE_PREFETCH_UNAVAILABLE: "+strings.TrimSpace(out))
}

func mapErr(err error) error {
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	return nil
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@example.com", "-c", "user.name=T"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// realRepo builds the situation the feature exists for: main and release/x with different commits, a clone whose checkout is
// on a third branch (so "the current HEAD" is neither of them), release/y known only on the remote.
type realRepo struct {
	clone                                       string
	mainSHA, releaseXSHA, releaseYSHA, thirdSHA string
	gw                                          *realGitGateway
	projects                                    *fakeProjects
}

func newRealRepo(t *testing.T) *realRepo {
	t.Helper()
	root := t.TempDir()
	seed := filepath.Join(root, "seed")
	if err := os.MkdirAll(seed, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, seed, "init", "-b", "main")
	commit := func(dir, file string) string {
		if err := os.WriteFile(filepath.Join(dir, file), []byte(file+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, dir, "add", file)
		runGit(t, dir, "commit", "-m", "add "+file)
		return runGit(t, dir, "rev-parse", "HEAD")
	}
	r := &realRepo{}
	r.mainSHA = commit(seed, "main.txt")
	runGit(t, seed, "checkout", "-b", "release/x")
	r.releaseXSHA = commit(seed, "release-x.txt")
	runGit(t, seed, "checkout", "-b", "release/y", "main")
	r.releaseYSHA = commit(seed, "release-y.txt")
	runGit(t, seed, "checkout", "main")
	origin := filepath.Join(root, "origin.git")
	runGit(t, root, "clone", "--bare", seed, origin)
	r.clone = filepath.Join(root, "clone")
	runGit(t, root, "clone", origin, r.clone)
	runGit(t, r.clone, "branch", "release/x", "origin/release/x") // release/y stays remote-only
	runGit(t, r.clone, "checkout", "-b", "third", "main")
	r.thirdSHA = commit(r.clone, "third.txt")
	if r.thirdSHA == r.mainSHA || r.mainSHA == r.releaseXSHA || r.releaseXSHA == r.releaseYSHA {
		t.Fatal("setup: the branches must differ")
	}
	r.projects = &fakeProjects{repos: []*projectv1.Repo{{Id: "repo-1"}}}
	r.gw = &realGitGateway{t: t, repoPath: r.clone, projects: r.projects}
	return r
}

func (r *realRepo) workspace() *ProposalWorkspace {
	return NewProposalWorkspace(r.gw, r.projects)
}

func headOf(t *testing.T, ws usecase.Workspace) string {
	t.Helper()
	return runGit(t, ws.Path, "rev-parse", "HEAD")
}

func TestRealGit_EnsureWithReleaseBranch_CutsTheWorktreeFromThatBranch(t *testing.T) {
	r := newRealRepo(t)
	in := ensureIn()
	in.BaseRef = "release/x"
	ws, err := r.workspace().Ensure(ctxWithIdentity(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := headOf(t, ws); got != r.releaseXSHA {
		t.Fatalf("HEAD = %s, want release/x = %s (main %s, third %s)", got, r.releaseXSHA, r.mainSHA, r.thirdSHA)
	}
	if ws.Branch != "request/42-proposal" || runGit(t, ws.Path, "rev-parse", "--abbrev-ref", "HEAD") != "request/42-proposal" {
		t.Fatalf("the proposal branch name does not change: %+v", ws)
	}
}

func TestRealGit_EnsureWithoutRef_CutsFromTheRepositoryDefaultNotFromTheCheckedOutBranch(t *testing.T) {
	r := newRealRepo(t)
	ws, err := r.workspace().Ensure(ctxWithIdentity(), ensureIn())
	if err != nil {
		t.Fatal(err)
	}
	got := headOf(t, ws)
	if got != r.mainSHA || got == r.thirdSHA {
		t.Fatalf("HEAD = %s, want main = %s and not the checked-out third = %s", got, r.mainSHA, r.thirdSHA)
	}
}

func TestRealGit_EnsureWithABranchKnownOnlyOnTheRemote(t *testing.T) {
	r := newRealRepo(t)
	in := ensureIn()
	in.BaseRef = "release/y"
	ws, err := r.workspace().Ensure(ctxWithIdentity(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := headOf(t, ws); got != r.releaseYSHA || ws.BaseRef != "refs/remotes/origin/release/y" {
		t.Fatalf("HEAD = %s want %s, base %q", got, r.releaseYSHA, ws.BaseRef)
	}
	// Passing the bare name makes git create a local branch "release/y" and check it out instead of the proposal branch.
	if branch := runGit(t, ws.Path, "rev-parse", "--abbrev-ref", "HEAD"); branch != "request/42-proposal" {
		t.Fatalf("the worktree must be on the proposal branch, it is on %q", branch)
	}
	if local := runGit(t, r.clone, "branch", "--list", "release/y"); local != "" {
		t.Fatalf("no local branch may appear for the person's ref: %q", local)
	}
}

// With only the local branch deleted, the remote-tracking branch still resolves, so the worktree is made from it.
func TestRealGit_LocalBranchDeletedButRemoteTrackingKept_StillResolves(t *testing.T) {
	r := newRealRepo(t)
	runGit(t, r.clone, "branch", "-D", "release/x")
	in := ensureIn()
	in.BaseRef = "release/x"
	ws, err := r.workspace().Ensure(ctxWithIdentity(), in)
	if err != nil || headOf(t, ws) != r.releaseXSHA {
		t.Fatalf("%+v %v", ws, err)
	}
}

func TestRealGit_RecreateAfterTheWorktreeWasRemoved_KeepsTheBranchAndItsBase(t *testing.T) {
	r := newRealRepo(t)
	w := r.workspace()
	in := ensureIn()
	in.BaseRef = "release/x"
	first, err := w.Ensure(ctxWithIdentity(), in)
	if err != nil {
		t.Fatal(err)
	}
	runGit(t, r.clone, "worktree", "remove", "--force", first.Path)
	second, err := w.Ensure(ctxWithIdentity(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got := headOf(t, second); got != r.releaseXSHA || second.Branch != first.Branch {
		t.Fatalf("recreated HEAD = %s, want %s; branch %q", got, r.releaseXSHA, second.Branch)
	}
}

func TestRealGit_BaseRefDeletedAfterItWasChosen_TypedErrorAndNoWorktree(t *testing.T) {
	r := newRealRepo(t)
	runGit(t, r.clone, "branch", "-D", "release/x")
	runGit(t, r.clone, "branch", "-dr", "origin/release/x") // gone locally and as a remote-tracking branch
	before := runGit(t, r.clone, "worktree", "list", "--porcelain")
	in := ensureIn()
	in.BaseRef = "release/x"
	_, err := r.workspace().Ensure(ctxWithIdentity(), in)
	if !domain.IsBaseRefNotFound(err) {
		t.Fatalf("want REQUEST_BASE_REF_NOT_FOUND, got %v", err)
	}
	details, _ := domain.BaseRefDetails(err)
	if details["ref"] != "release/x" || !strings.Contains(details["suggestions"], "main") {
		t.Fatalf("details = %v", details)
	}
	if after := runGit(t, r.clone, "worktree", "list", "--porcelain"); after != before {
		t.Fatalf("a failed creation must leave no worktree:\n%s\nvs\n%s", before, after)
	}
	if _, serr := os.Stat(r.clone + "-request-42-proposal"); !os.IsNotExist(serr) {
		t.Fatalf("no directory may be left behind: %v", serr)
	}
}

func TestRealGit_Checker_AgainstTheRealRepository(t *testing.T) {
	r := newRealRepo(t)
	c := NewGitBaseRefChecker(r.gw, &fakeProjects{repos: []*projectv1.Repo{{Id: "repo-1"}}})
	for _, ref := range []string{"main", "release/x", "release/y"} {
		if got, err := c.Check(ctxWithIdentity(), "proj", ref); err != nil || got.Status != usecase.BaseRefFound {
			t.Errorf("%s: %+v %v", ref, got, err)
		}
	}
	got, err := c.Check(ctxWithIdentity(), "proj", "release/none")
	if err != nil || got.Status != usecase.BaseRefNotFound || len(got.Suggestions) == 0 || len(got.Suggestions) > domain.MaxBaseRefSuggestions {
		t.Fatalf("a branch nobody has: %+v %v", got, err)
	}
	runGit(t, r.clone, "remote", "set-url", "origin", filepath.Join(filepath.Dir(r.clone), "gone.git"))
	if got, err := c.Check(ctxWithIdentity(), "proj", "release/none"); err != nil || got.Status != usecase.BaseRefUnreachable {
		t.Fatalf("a remote that cannot be read says nothing about the ref: %+v %v", got, err)
	}
	if got, err := c.Check(ctxWithIdentity(), "proj", "release/x"); err != nil || got.Status != usecase.BaseRefFound {
		t.Fatalf("a local branch is found even when the remote is down: %+v %v", got, err)
	}
}

// A database reset starts Request numbers at 1 again, while the repository still holds request/42-proposal from an older Request
// (its worktree and branch, at an old commit). The new Request must not fail and must not build on the old commits.
func TestRealGit_StaleProposalWorktreeAndBranchOfAnotherRequest_UsesTheRequestsOwnBranch(t *testing.T) {
	r := newRealRepo(t)
	runGit(t, r.clone, "worktree", "add", r.clone+"-request-42-proposal", "-b", "request/42-proposal", "third")
	in := ensureIn()
	ws, err := r.workspace().Ensure(ctxWithIdentity(), in)
	if err != nil {
		t.Fatal(err)
	}
	own := domain.ProposalBranchFor(in.RequestNumber, in.RequestID)
	if ws.Branch != own || runGit(t, ws.Path, "rev-parse", "--abbrev-ref", "HEAD") != own {
		t.Fatalf("branch %q, want the Request's own %q", ws.Branch, own)
	}
	if got := headOf(t, ws); got != r.mainSHA {
		t.Fatalf("HEAD = %s, want the default branch %s, not the old Request's commit %s", got, r.mainSHA, r.thirdSHA)
	}
	again, err := r.workspace().Ensure(ctxWithIdentity(), in)
	if err != nil || again.Path != ws.Path || again.Branch != own {
		t.Fatalf("a second run must find the same worktree: %+v %v", again, err)
	}
}

// Only the branch is left (its worktree was pruned): git-gateway would check it out, so the new change would start on old commits.
func TestRealGit_StaleProposalBranchWithoutWorktree_UsesTheRequestsOwnBranch(t *testing.T) {
	r := newRealRepo(t)
	runGit(t, r.clone, "branch", "request/42-proposal", "third")
	in := ensureIn()
	ws, err := r.workspace().Ensure(ctxWithIdentity(), in)
	if err != nil {
		t.Fatal(err)
	}
	if ws.Branch != domain.ProposalBranchFor(in.RequestNumber, in.RequestID) || headOf(t, ws) != r.mainSHA {
		t.Fatalf("workspace %+v at %s, want the own branch at main %s", ws, headOf(t, ws), r.mainSHA)
	}
}
