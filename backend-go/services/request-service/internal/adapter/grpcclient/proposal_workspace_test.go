package grpcclient

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeProposalGit struct {
	gitgatewayv1.GitGatewayServiceClient
	created     []*gitgatewayv1.CreateWorktreeRequest
	baseRef     string
	statusErr   map[string]error // by worktree id
	files       []*gitgatewayv1.FileStatus
	discarded   []string
	discardOK   bool
	failedPaths []string
	staged      [][]string
	commits     []*gitgatewayv1.CommitRequest
	commitSha   string
	written     []*gitgatewayv1.WriteFileRequest
	read        *gitgatewayv1.ReadFileResponse
	readErr     error
	removed     []*gitgatewayv1.RemoveWorktreeRequest
	removeErr   error
	createErr   error
	nextID      int
	stat        *gitgatewayv1.StatFileResponse
	statErr     error
	dir         *gitgatewayv1.ReadDirResponse
	dirErr      error
	refs        []string // local refs SearchRefs knows
	refsErr     error
}

func (f *fakeProposalGit) SearchRefs(_ context.Context, in *gitgatewayv1.SearchRefsRequest, _ ...grpc.CallOption) (*gitgatewayv1.SearchRefsResponse, error) {
	if f.refsErr != nil {
		return nil, f.refsErr
	}
	var out []string
	for _, r := range f.refs {
		if strings.Contains(r, in.GetQuery()) {
			out = append(out, r)
		}
	}
	return &gitgatewayv1.SearchRefsResponse{Refs: out}, nil
}

func newFakeProposalGit() *fakeProposalGit {
	return &fakeProposalGit{baseRef: "main", discardOK: true, commitSha: "abc123", statusErr: map[string]error{}}
}

func (f *fakeProposalGit) BaseRefDefault(_ context.Context, in *gitgatewayv1.BaseRefDefaultRequest, _ ...grpc.CallOption) (*gitgatewayv1.BaseRefDefaultResponse, error) {
	if in.GetRepoId() == "" {
		return nil, status.Error(codes.InvalidArgument, "repo required")
	}
	return &gitgatewayv1.BaseRefDefaultResponse{Ref: f.baseRef}, nil
}

func (f *fakeProposalGit) CreateWorktree(_ context.Context, in *gitgatewayv1.CreateWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.created = append(f.created, in)
	f.nextID++
	// Same key, same worktree: the saga's dedupe.
	id := "wt-" + in.GetIdempotencyKey()
	return &gitgatewayv1.CreateWorktreeResponse{WorktreeId: id, Path: "/srv/repo-" + in.GetBranch()}, nil
}

func (f *fakeProposalGit) GetStatus(_ context.Context, in *gitgatewayv1.GetStatusRequest, _ ...grpc.CallOption) (*gitgatewayv1.GetStatusResponse, error) {
	if err := f.statusErr[in.GetWorktreeId()]; err != nil {
		return nil, err
	}
	return &gitgatewayv1.GetStatusResponse{Files: f.files}, nil
}

func (f *fakeProposalGit) BulkDiscard(_ context.Context, in *gitgatewayv1.BulkDiscardRequest, _ ...grpc.CallOption) (*gitgatewayv1.BulkDiscardResponse, error) {
	f.discarded = append(f.discarded, in.GetPaths()...)
	return &gitgatewayv1.BulkDiscardResponse{Success: f.discardOK, FailedPaths: f.failedPaths}, nil
}

func (f *fakeProposalGit) Stage(_ context.Context, in *gitgatewayv1.StageRequest, _ ...grpc.CallOption) (*gitgatewayv1.StageResponse, error) {
	f.staged = append(f.staged, in.GetPaths())
	return &gitgatewayv1.StageResponse{}, nil
}

func (f *fakeProposalGit) Commit(_ context.Context, in *gitgatewayv1.CommitRequest, _ ...grpc.CallOption) (*gitgatewayv1.CommitResponse, error) {
	f.commits = append(f.commits, in)
	return &gitgatewayv1.CommitResponse{CommitSha: f.commitSha}, nil
}

func (f *fakeProposalGit) WriteFile(_ context.Context, in *gitgatewayv1.WriteFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.WriteFileResponse, error) {
	f.written = append(f.written, in)
	return &gitgatewayv1.WriteFileResponse{BytesWritten: int64(len(in.GetContent()))}, nil
}

func (f *fakeProposalGit) ReadFile(_ context.Context, _ *gitgatewayv1.ReadFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.ReadFileResponse, error) {
	return f.read, f.readErr
}

func (f *fakeProposalGit) StatFile(_ context.Context, _ *gitgatewayv1.StatFileRequest, _ ...grpc.CallOption) (*gitgatewayv1.StatFileResponse, error) {
	return f.stat, f.statErr
}

func (f *fakeProposalGit) ReadDir(_ context.Context, _ *gitgatewayv1.ReadDirRequest, _ ...grpc.CallOption) (*gitgatewayv1.ReadDirResponse, error) {
	return f.dir, f.dirErr
}

func (f *fakeProposalGit) RemoveWorktree(_ context.Context, in *gitgatewayv1.RemoveWorktreeRequest, _ ...grpc.CallOption) (*gitgatewayv1.RemoveWorktreeResponse, error) {
	f.removed = append(f.removed, in)
	return &gitgatewayv1.RemoveWorktreeResponse{}, f.removeErr
}

func wsKit(repos ...*projectv1.Repo) (*ProposalWorkspace, *fakeProposalGit) {
	g := newFakeProposalGit()
	w := NewProposalWorkspace(g, &fakeProjects{repos: repos})
	w.now = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	return w, g
}

var oneRepo = &projectv1.Repo{Id: "repo-1"}

func ensureIn() usecase.EnsureWorkspaceInput {
	return usecase.EnsureWorkspaceInput{ProjectID: "proj", RequestID: "req-uuid", RequestNumber: 42, ChangeID: "add-ai"}
}

func TestProposalWorkspace_EnsureCreatesTheProposalBranchOnTheDefaultRepo(t *testing.T) {
	w, g := wsKit(oneRepo, &projectv1.Repo{Id: "repo-2"})
	ws, err := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil {
		t.Fatal(err)
	}
	c := g.created[0]
	if c.GetBranch() != "request/42-proposal" || c.GetRepoId() != "repo-1" || c.GetBaseRef() != "main" || c.GetProjectId() != "proj" {
		t.Fatalf("create = %+v", c)
	}
	if c.GetIdempotencyKey() != "request-proposal:req-uuid" || c.LinkedIssueProvider != nil || c.TaskId != nil {
		t.Fatalf("keyed per request and not linked to an issue: %+v", c)
	}
	if ws.Branch != "request/42-proposal" || ws.BaseRef != "main" || ws.WorktreeID == "" || ws.Path == "" {
		t.Fatalf("ws = %+v", ws)
	}
}

func TestProposalWorkspace_EnsureIsIdempotent(t *testing.T) {
	w, g := wsKit(oneRepo)
	a, err := w.Ensure(ctxWithIdentity(), ensureIn())
	b, err2 := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil || err2 != nil || a.WorktreeID != b.WorktreeID {
		t.Fatalf("a=%+v b=%+v %v %v", a, b, err, err2)
	}
	if len(g.created) != 2 || g.created[0].GetIdempotencyKey() != g.created[1].GetIdempotencyKey() {
		t.Fatalf("both calls must carry the same key so the gateway dedupes: %+v", g.created)
	}
}

func TestProposalWorkspace_EnsureRecreatesAWorktreeThatWasRemoved(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.statusErr["wt-request-proposal:req-uuid"] = status.Error(codes.NotFound, "worktree gone")
	ws, err := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil {
		t.Fatal(err)
	}
	if len(g.created) != 2 || g.created[1].GetIdempotencyKey() == g.created[0].GetIdempotencyKey() || !strings.HasPrefix(g.created[1].GetIdempotencyKey(), "request-proposal:req-uuid:") {
		t.Fatalf("a removed worktree needs a fresh key: %+v", g.created)
	}
	if ws.WorktreeID != "wt-"+g.created[1].GetIdempotencyKey() {
		t.Fatalf("must return the recreated worktree: %+v", ws)
	}
}

// A worktree created before the gateway stored the idempotency key has no record to find by key: creating it again is refused
// because its path is taken, and the project's records say which worktree holds the branch.
func TestProposalWorkspace_EnsureFindsTheWorktreeAnOlderRecordHolds(t *testing.T) {
	g := newFakeProposalGit()
	g.createErr = status.Error(codes.AlreadyExists, `WORKTREE_PATH_EXISTS: path already exists; try "request-42-proposal"`)
	projects := &fakeProjects{repos: []*projectv1.Repo{oneRepo}, worktrees: []*projectv1.Worktree{
		{Id: "wt-other", Path: "/srv/other", Branch: "feature/x", RepoId: "repo-1", Active: true},
		{Id: "wt-removed", Path: "/srv/old", Branch: "request/42-proposal", RepoId: "repo-1", Active: false},
		{Id: "wt-live", Path: "/srv/repo-request-42-proposal", Branch: "request/42-proposal", RepoId: "repo-1", Active: true},
	}}
	w := NewProposalWorkspace(g, projects)
	ws, err := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil || ws.WorktreeID != "wt-live" || ws.Path != "/srv/repo-request-42-proposal" || ws.Branch != "request/42-proposal" {
		t.Fatalf("%+v %v", ws, err)
	}
	projects.worktrees = nil
	if _, err := w.Ensure(ctxWithIdentity(), ensureIn()); err == nil || !strings.Contains(err.Error(), "no record of this Request's branch") {
		t.Fatalf("a taken path with no record is a failure, not a guess: %v", err)
	}
}

func TestProposalWorkspace_EnsureRefusals(t *testing.T) {
	w, _ := wsKit(oneRepo)
	for name, in := range map[string]usecase.EnsureWorkspaceInput{
		"no project":    {RequestID: "r", RequestNumber: 1},
		"no request id": {ProjectID: "p", RequestNumber: 1},
		"no number":     {ProjectID: "p", RequestID: "r"},
	} {
		if _, err := w.Ensure(ctxWithIdentity(), in); err == nil || !strings.Contains(err.Error(), "REQUEST_ENGINE_REPO_NOT_RESOLVED") {
			t.Fatalf("%s: %v", name, err)
		}
	}
	noRepo, _ := wsKit()
	if _, err := noRepo.Ensure(ctxWithIdentity(), ensureIn()); err == nil || !strings.Contains(err.Error(), "REQUEST_ENGINE_REPO_NOT_RESOLVED") {
		t.Fatalf("a project without repos: %v", err)
	}
	if _, err := w.Ensure(context.Background(), ensureIn()); err == nil {
		t.Fatal("no tenant must fail")
	}
}

func TestProposalWorkspace_EnsureUsesTheRepoTheCallerNamed(t *testing.T) {
	w, g := wsKit(oneRepo)
	in := ensureIn()
	in.RepoID = "repo-9"
	if _, err := w.Ensure(ctxWithIdentity(), in); err != nil || g.created[0].GetRepoId() != "repo-9" {
		t.Fatalf("%v %+v", err, g.created)
	}
}

func TestProposalWorkspace_EnsureMapsGatewayErrors(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.createErr = status.Error(codes.Unavailable, "gateway down")
	if _, err := w.Ensure(ctxWithIdentity(), ensureIn()); err == nil || !strings.Contains(err.Error(), "REQUEST_ENGINE_NO_CONNECTION") {
		t.Fatalf("unavailable: %v", err)
	}
	g.createErr = status.Error(codes.NotFound, "repo")
	if _, err := w.Ensure(ctxWithIdentity(), ensureIn()); err == nil || !strings.Contains(err.Error(), "REQUEST_ENGINE_REPO_NOT_RESOLVED") {
		t.Fatalf("not found: %v", err)
	}
	g.createErr = errors.New("git exploded")
	if _, err := w.Ensure(ctxWithIdentity(), ensureIn()); err == nil || !strings.Contains(err.Error(), "REQUEST_OPENSPEC_AGENT_FAILED") {
		t.Fatalf("other: %v", err)
	}
}

var ws1 = usecase.Workspace{WorktreeID: "wt-1", Path: "/srv/wt", Branch: "request/42-proposal"}

func TestProposalWorkspace_ChangedPathsIncludeNewRenamedAndDeleted(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.files = []*gitgatewayv1.FileStatus{
		{Path: "openspec/changes/add-ai/tasks.md", State: "untracked"},
		{Path: "b.md", OldPath: "a.md", State: "renamed"},
		{Path: "gone.md", State: "deleted"},
	}
	got, err := w.ChangedPaths(ctxWithIdentity(), ws1)
	if err != nil || len(got) != 3 || got[0].State != "untracked" || got[1].OldPath != "a.md" || got[2].State != "deleted" {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestProposalWorkspace_DiscardAllResetsEverythingIncludingOldNames(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.files = []*gitgatewayv1.FileStatus{{Path: "new.md", State: "untracked"}, {Path: "b.md", OldPath: "a.md", State: "renamed"}}
	if err := w.DiscardAll(ctxWithIdentity(), ws1); err != nil {
		t.Fatal(err)
	}
	if strings.Join(g.discarded, ",") != "new.md,b.md,a.md" {
		t.Fatalf("discarded %v", g.discarded)
	}
	g.discarded = nil
	g.files = nil
	if err := w.DiscardAll(ctxWithIdentity(), ws1); err != nil || len(g.discarded) != 0 {
		t.Fatalf("a clean worktree needs no call: %v %v", err, g.discarded)
	}
}

func TestProposalWorkspace_DiscardAllFailsWhenAPathCannotBeDiscarded(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.files = []*gitgatewayv1.FileStatus{{Path: "x.md", State: "modified"}}
	g.discardOK, g.failedPaths = false, []string{"x.md"}
	if err := w.DiscardAll(ctxWithIdentity(), ws1); err == nil || !strings.Contains(err.Error(), "x.md") {
		t.Fatalf("got %v", err)
	}
}

func TestProposalWorkspace_CommitStagesAndCommitsOnlyTheGivenPaths(t *testing.T) {
	w, g := wsKit(oneRepo)
	sha, err := w.Commit(ctxWithIdentity(), ws1, "proposal", []string{"openspec/changes/add-ai/tasks.md", `openspec\changes\add-ai\design.md`})
	if err != nil || sha != "abc123" {
		t.Fatalf("%q %v", sha, err)
	}
	want := "openspec/changes/add-ai/tasks.md,openspec/changes/add-ai/design.md"
	if strings.Join(g.staged[0], ",") != want || strings.Join(g.commits[0].GetPaths(), ",") != want {
		t.Fatalf("staged %v commit %v", g.staged, g.commits[0].GetPaths())
	}
}

func TestProposalWorkspace_CommitRefusals(t *testing.T) {
	w, g := wsKit(oneRepo)
	if _, err := w.Commit(ctxWithIdentity(), ws1, "m", nil); err == nil {
		t.Fatal("an empty path list would commit everything staged")
	}
	if _, err := w.Commit(ctxWithIdentity(), ws1, "  ", []string{"a.md"}); err == nil {
		t.Fatal("an empty message must fail")
	}
	if _, err := w.Commit(ctxWithIdentity(), ws1, "m", []string{"ok.md", "../escape.md"}); err == nil {
		t.Fatal("a path outside the worktree must fail")
	}
	if len(g.staged) != 0 || len(g.commits) != 0 {
		t.Fatal("nothing may reach git when a refusal applies")
	}
}

func TestCleanRelPath(t *testing.T) {
	good := map[string]string{"a/b.md": "a/b.md", "./a//b.md": "a/b.md", `a\b.md`: "a/b.md", "openspec/../openspec/x": "openspec/x"}
	for in, want := range good {
		if got, err := cleanRelPath(in); err != nil || got != want {
			t.Fatalf("%q -> %q %v", in, got, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "../x", "a/../../x", "/etc/passwd", "a\x00b", strings.Repeat("a/", 300)} {
		if _, err := cleanRelPath(bad); err == nil {
			t.Fatalf("%q must be refused", bad)
		}
	}
}

func TestProposalWorkspace_WriteFileCreatesParentsAndRefusesEscapes(t *testing.T) {
	w, g := wsKit(oneRepo)
	if err := w.WriteFile(ctxWithIdentity(), ws1, "openspec/changes/x/tasks.md", []byte("hi")); err != nil {
		t.Fatal(err)
	}
	if r := g.written[0]; !r.GetCreateParents() || r.GetEncoding() != "utf8" || string(r.GetContent()) != "hi" || r.GetWorktreeId() != "wt-1" {
		t.Fatalf("%+v", r)
	}
	if err := w.WriteFile(ctxWithIdentity(), ws1, "../outside", []byte("x")); err == nil || len(g.written) != 1 {
		t.Fatalf("an escape must not be written: %v", err)
	}
}

func TestProposalWorkspace_ReadFileLimits(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.read = &gitgatewayv1.ReadFileResponse{Content: []byte("hello"), Encoding: "utf8"}
	if b, err := w.ReadFile(ctxWithIdentity(), ws1, "tasks.md", 100); err != nil || string(b) != "hello" {
		t.Fatalf("%q %v", b, err)
	}
	if _, err := w.ReadFile(ctxWithIdentity(), ws1, "tasks.md", 3); err == nil || !strings.Contains(err.Error(), "REQUEST_OPENSPEC_INVALID_OUTPUT") {
		t.Fatalf("over the caller's limit: %v", err)
	}
	g.read = &gitgatewayv1.ReadFileResponse{Content: make([]byte, usecase.MaxProposalReadBytes+1), Encoding: "utf8"}
	if _, err := w.ReadFile(ctxWithIdentity(), ws1, "tasks.md", 0); err == nil {
		t.Fatal("the hard cap applies when the caller names none")
	}
	if _, err := w.ReadFile(ctxWithIdentity(), ws1, "tasks.md", 10_000_000); err == nil {
		t.Fatal("the hard cap applies when the caller asks for more")
	}
	g.read = &gitgatewayv1.ReadFileResponse{Content: []byte("AAA="), Encoding: "base64"}
	if _, err := w.ReadFile(ctxWithIdentity(), ws1, "image.png", 100); err == nil {
		t.Fatal("a binary file is not a proposal document")
	}
	if _, err := w.ReadFile(ctxWithIdentity(), ws1, "/etc/passwd", 100); err == nil {
		t.Fatal("an absolute path must be refused")
	}
	g.readErr = status.Error(codes.NotFound, "no such file")
	if _, err := w.ReadFile(ctxWithIdentity(), ws1, "missing.md", 100); err == nil {
		t.Fatal("a missing file must be an error")
	}
}

func TestProposalWorkspace_RemoveKeepsTheBranchAndToleratesAGoneWorktree(t *testing.T) {
	w, g := wsKit(oneRepo)
	if err := w.Remove(ctxWithIdentity(), ws1); err != nil {
		t.Fatal(err)
	}
	if len(g.removed) != 1 || g.removed[0].GetWorktreeId() != "wt-1" || !g.removed[0].GetForce() || g.removed[0].GetAllowOpenPr() {
		t.Fatalf("%+v", g.removed)
	}
	g.removeErr = status.Error(codes.NotFound, "already gone")
	if err := w.Remove(ctxWithIdentity(), ws1); err != nil {
		t.Fatalf("removing twice must be fine: %v", err)
	}
	g.removeErr = status.Error(codes.Internal, "busy")
	if err := w.Remove(ctxWithIdentity(), ws1); err == nil {
		t.Fatal("a real failure must surface")
	}
}

func TestProposalWorkspace_StatAndListDir(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.stat = &gitgatewayv1.StatFileResponse{Exists: true, IsDirectory: true}
	if info, err := w.Stat(ctxWithIdentity(), ws1, "src"); err != nil || !info.Exists || !info.IsDir {
		t.Fatalf("%+v %v", info, err)
	}
	g.statErr = status.Error(codes.NotFound, "gone")
	if info, err := w.Stat(ctxWithIdentity(), ws1, "src/missing.go"); err != nil || info.Exists {
		t.Fatalf("a missing path is not an error: %+v %v", info, err)
	}
	g.statErr = status.Error(codes.Internal, "boom")
	if _, err := w.Stat(ctxWithIdentity(), ws1, "src"); err == nil {
		t.Fatal("a real failure must surface")
	}
	if _, err := w.Stat(ctxWithIdentity(), ws1, "../x"); err == nil {
		t.Fatal("an escape must be refused")
	}

	g.dir = &gitgatewayv1.ReadDirResponse{Entries: []*gitgatewayv1.DirEntry{{Name: "a", IsDirectory: true}, {Name: "b.md"}}}
	got, err := w.ListDir(ctxWithIdentity(), ws1, "openspec/changes/x/specs")
	if err != nil || len(got) != 2 || !got[0].IsDir || got[1].Name != "b.md" {
		t.Fatalf("%+v %v", got, err)
	}
	g.dirErr = status.Error(codes.NotFound, "no dir")
	if got, err := w.ListDir(ctxWithIdentity(), ws1, "nope"); err != nil || len(got) != 0 {
		t.Fatalf("a missing directory lists as empty: %+v %v", got, err)
	}
	g.dirErr = status.Error(codes.Unavailable, "down")
	if _, err := w.ListDir(ctxWithIdentity(), ws1, "x"); err == nil || !strings.Contains(err.Error(), "REQUEST_ENGINE_NO_CONNECTION") {
		t.Fatalf("%v", err)
	}
}

func TestProposalWorkspace_BranchLeftByAnotherRequestIsNotReused(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.refs = []string{"main", "request/42-proposal"} // no worktree record of this project holds it
	ws, err := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil {
		t.Fatal(err)
	}
	own := domain.ProposalBranchFor(42, "req-uuid")
	if ws.Branch != own || g.created[len(g.created)-1].GetBranch() != own {
		t.Fatalf("branch %q, want the Request's own %q", ws.Branch, own)
	}
}

func TestProposalWorkspace_BranchOfThisProjectKeepsItsName(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.refs = []string{"request/42-proposal"}
	w.projects.(*fakeProjects).worktrees = []*projectv1.Worktree{{Id: "wt-old", Branch: "request/42-proposal", RepoId: "repo-1"}}
	ws, err := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil || ws.Branch != "request/42-proposal" {
		t.Fatalf("ws %+v err %v", ws, err)
	}
}

func TestProposalWorkspace_OwnBranchFromAnEarlierRunWins(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.refs = []string{"request/42-proposal", domain.ProposalBranchFor(42, "req-uuid")}
	ws, err := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil || ws.Branch != domain.ProposalBranchFor(42, "req-uuid") {
		t.Fatalf("ws %+v err %v", ws, err)
	}
}

func TestProposalWorkspace_LookupFailureKeepsTheUsualName(t *testing.T) {
	w, g := wsKit(oneRepo)
	g.refsErr = errors.New("relay down")
	ws, err := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil || ws.Branch != "request/42-proposal" {
		t.Fatalf("ws %+v err %v", ws, err)
	}
}
