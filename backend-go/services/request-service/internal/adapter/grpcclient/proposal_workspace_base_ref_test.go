package grpcclient

import (
	"context"
	"fmt"
	"strings"
	"testing"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// refGit is the fake gateway with a repository behind it: a start point resolves only when git would resolve it. Refs are given
// as short names; remote holds the branches the clone knows only as origin/<name>.
type refGit struct {
	*fakeProposalGit
	local, remote, tags []string
	attempts            []string
}

func hasRef(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (g *refGit) resolves(ref string) bool {
	switch {
	case strings.HasPrefix(ref, "refs/heads/"):
		return hasRef(g.local, strings.TrimPrefix(ref, "refs/heads/"))
	case strings.HasPrefix(ref, "refs/remotes/origin/"):
		return hasRef(g.remote, strings.TrimPrefix(ref, "refs/remotes/origin/"))
	case strings.HasPrefix(ref, "refs/tags/"):
		return hasRef(g.tags, strings.TrimPrefix(ref, "refs/tags/"))
	case strings.HasPrefix(ref, "refs/"):
		return false
	}
	return hasRef(g.local, ref) // the default branch, passed bare as before
}

func (g *refGit) CreateWorktree(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest, opts ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	g.attempts = append(g.attempts, in.GetBaseRef())
	if g.resolves(in.GetBaseRef()) {
		return g.fakeProposalGit.CreateWorktree(ctx, in, opts...)
	}
	return nil, status.Error(codes.NotFound, fmt.Sprintf("%s: branch %q not found; available: %s", codeGatewayBaseRefNotFound, in.GetBaseRef(), strings.Join(g.local, ", ")))
}

func refKit(local ...string) (*ProposalWorkspace, *refGit) {
	g := &refGit{fakeProposalGit: newFakeProposalGit(), local: local}
	return NewProposalWorkspace(g, &fakeProjects{repos: []*projectv1.Repo{oneRepo}}), g
}

func TestEnsure_UsesRequestBaseRef(t *testing.T) {
	w, g := refKit("main", "release/x")
	in := ensureIn()
	in.BaseRef = "release/x"
	ws, err := w.Ensure(ctxWithIdentity(), in)
	if err != nil || ws.BaseRef != "refs/heads/release/x" || g.created[0].GetBaseRef() != "refs/heads/release/x" {
		t.Fatalf("a chosen branch is passed fully qualified: ws=%+v created=%+v err=%v", ws, g.created, err)
	}
	if g.created[0].GetBranch() != "request/42-proposal" || g.created[0].GetIdempotencyKey() != "request-proposal:req-uuid" {
		t.Fatalf("the branch name and the idempotency key do not change with the base: %+v", g.created[0])
	}
}

func TestEnsure_EmptyUsesDefault(t *testing.T) {
	w, g := refKit("main", "release/x")
	ws, err := w.Ensure(ctxWithIdentity(), ensureIn())
	if err != nil || ws.BaseRef != "main" || g.created[0].GetBaseRef() != "main" {
		t.Fatalf("ws=%+v created=%+v err=%v", ws, g.created, err)
	}
	blank := ensureIn()
	blank.BaseRef = "  "
	if ws, err := w.Ensure(ctxWithIdentity(), blank); err != nil || ws.BaseRef != "main" {
		t.Fatalf("a blank ref is the default: %+v %v", ws, err)
	}
}

func TestEnsure_ChosenRefNeverAsksForTheDefault(t *testing.T) {
	g := &countingDefaultGit{refGit: &refGit{fakeProposalGit: newFakeProposalGit(), local: []string{"release/x"}}}
	w := NewProposalWorkspace(g, &fakeProjects{repos: []*projectv1.Repo{oneRepo}})
	in := ensureIn()
	in.BaseRef = "release/x"
	if _, err := w.Ensure(ctxWithIdentity(), in); err != nil || g.defaultCalls != 0 {
		t.Fatalf("defaultCalls=%d err=%v", g.defaultCalls, err)
	}
}

type countingDefaultGit struct {
	*refGit
	defaultCalls int
}

func (g *countingDefaultGit) BaseRefDefault(ctx context.Context, in *gitgatewayv1.BaseRefDefaultRequest, opts ...grpc.CallOption) (*gitgatewayv1.BaseRefDefaultResponse, error) {
	g.defaultCalls++
	return g.refGit.BaseRefDefault(ctx, in, opts...)
}

func TestEnsure_RecreateAfterRemovalKeepsBaseRef(t *testing.T) {
	w, g := refKit("main", "release/x")
	g.statusErr["wt-request-proposal:req-uuid"] = status.Error(codes.NotFound, "worktree gone")
	in := ensureIn()
	in.BaseRef = "release/x"
	ws, err := w.Ensure(ctxWithIdentity(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(g.created) != 2 || g.created[1].GetBaseRef() != "refs/heads/release/x" || ws.BaseRef != "refs/heads/release/x" || g.created[1].GetBranch() != g.created[0].GetBranch() {
		t.Fatalf("the recreated worktree starts from the same base on the same branch: %+v", g.created)
	}
}

func TestEnsure_BaseRefGone_TypedError(t *testing.T) {
	w, g := refKit("main", "release/1.4", "develop", "a", "b", "c", "d")
	in := ensureIn()
	in.BaseRef = "release/gone"
	_, err := w.Ensure(ctxWithIdentity(), in)
	if !domain.IsBaseRefNotFound(err) {
		t.Fatalf("want REQUEST_BASE_REF_NOT_FOUND, got %v", err)
	}
	details, _ := domain.BaseRefDetails(err)
	if details["ref"] != "release/gone" {
		t.Fatalf("details = %v", details)
	}
	if got := strings.Split(details["suggestions"], ","); len(got) != domain.MaxBaseRefSuggestions {
		t.Fatalf("at most %d suggestions: %v", domain.MaxBaseRefSuggestions, got)
	}
	if len(g.created) != 0 {
		t.Fatal("nothing may be created from a ref that does not exist")
	}
}

func TestEnsure_BaseRefGone_SuggestionsPutSimilarNamesFirst(t *testing.T) {
	w, _ := refKit("a", "b", "c", "d", "e", "f", "release/1.4")
	in := ensureIn()
	in.BaseRef = "release"
	_, err := w.Ensure(ctxWithIdentity(), in)
	if details, _ := domain.BaseRefDetails(err); !strings.HasPrefix(details["suggestions"], "release/1.4") {
		t.Fatalf("a name containing the asked one comes first: %q", details["suggestions"])
	}
}

// A clone knows branches other than the default only as origin/<name>. The bare name must never reach `git worktree add`: git
// would create a local branch of that name instead of the proposal branch.
func TestEnsure_ChosenRefKnownOnlyAsRemoteTrackingBranch_IsPassedAsTheRemoteRef(t *testing.T) {
	w, g := refKit("main")
	g.remote = []string{"release/x"}
	in := ensureIn()
	in.BaseRef = "release/x"
	ws, err := w.Ensure(ctxWithIdentity(), in)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(g.attempts, ",") != "refs/heads/release/x,refs/remotes/origin/release/x" || ws.BaseRef != "refs/remotes/origin/release/x" {
		t.Fatalf("attempts=%v ws=%+v", g.attempts, ws)
	}
	for _, a := range g.attempts {
		if a == "release/x" {
			t.Fatal("the bare name was sent to git-gateway")
		}
	}
}

func TestEnsure_ChosenTag(t *testing.T) {
	w, g := refKit("main")
	g.tags = []string{"v1.0"}
	in := ensureIn()
	in.BaseRef = "v1.0"
	if ws, err := w.Ensure(ctxWithIdentity(), in); err != nil || ws.BaseRef != "refs/tags/v1.0" {
		t.Fatalf("%+v %v attempts=%v", ws, err, g.attempts)
	}
}

func TestEnsure_ARefTypedAsOriginSlashName(t *testing.T) {
	w, g := refKit("main")
	g.remote = []string{"release/x"}
	in := ensureIn()
	in.BaseRef = "origin/release/x"
	if ws, err := w.Ensure(ctxWithIdentity(), in); err != nil || ws.BaseRef != "refs/remotes/origin/release/x" {
		t.Fatalf("%+v %v attempts=%v", ws, err, g.attempts)
	}
}

func TestEnsure_AFullyQualifiedRefIsUsedAsGiven(t *testing.T) {
	w, g := refKit("release/x")
	in := ensureIn()
	in.BaseRef = "refs/heads/release/x"
	if _, err := w.Ensure(ctxWithIdentity(), in); err != nil || strings.Join(g.attempts, ",") != "refs/heads/release/x" {
		t.Fatalf("%v attempts=%v", err, g.attempts)
	}
}

func TestEnsure_BaseRefGone_EveryKindOfRefIsTriedThenTheChosenNameIsReported(t *testing.T) {
	w, g := refKit("main")
	in := ensureIn()
	in.BaseRef = "release/gone"
	_, err := w.Ensure(ctxWithIdentity(), in)
	if details, _ := domain.BaseRefDetails(err); !domain.IsBaseRefNotFound(err) || details["ref"] != "release/gone" {
		t.Fatalf("%v %v", err, details)
	}
	if strings.Join(g.attempts, ",") != "refs/heads/release/gone,refs/remotes/origin/release/gone,refs/tags/release/gone" {
		t.Fatalf("attempts=%v", g.attempts)
	}
}

func TestEnsure_TheDefaultIsTakenAsIs(t *testing.T) {
	w, g := refKit()
	if _, err := w.Ensure(ctxWithIdentity(), ensureIn()); !domain.IsBaseRefNotFound(err) || strings.Join(g.attempts, ",") != "main" {
		t.Fatalf("the repository default is passed bare, once: %v %v", err, g.attempts)
	}
}

func TestEnsure_AnOutageWhileTryingASpellingIsNotAMissingRef(t *testing.T) {
	g := &flakyRetryGit{refGit: &refGit{fakeProposalGit: newFakeProposalGit(), local: []string{"main"}, remote: []string{"release/x"}}}
	w := NewProposalWorkspace(g, &fakeProjects{repos: []*projectv1.Repo{oneRepo}})
	in := ensureIn()
	in.BaseRef = "release/x"
	_, err := w.Ensure(ctxWithIdentity(), in)
	if domain.IsBaseRefNotFound(err) || err == nil || !strings.Contains(err.Error(), "REQUEST_ENGINE_NO_CONNECTION") {
		t.Fatalf("a gateway that goes away during the retry is an outage, not a missing ref: %v", err)
	}
}

type flakyRetryGit struct{ *refGit }

func (g *flakyRetryGit) CreateWorktree(ctx context.Context, in *gitgatewayv1.CreateWorktreeRequest, opts ...grpc.CallOption) (*gitgatewayv1.CreateWorktreeResponse, error) {
	if strings.HasPrefix(in.GetBaseRef(), "refs/remotes/") {
		return nil, status.Error(codes.Unavailable, "gateway down")
	}
	return g.refGit.CreateWorktree(ctx, in, opts...)
}

func TestEnsure_GitWordsInAGenericFailureAreAlsoAMissingRef(t *testing.T) {
	for _, msg := range []string{
		"WORKTREE_CREATE_FAILED: git worktree add failed: fatal: invalid reference: release/x",
		"WORKTREE_CREATE_FAILED: git worktree add failed: fatal: not a valid object name: 'release/x'",
		"WORKTREE_CREATE_FAILED: fatal: unknown revision or path not in the working tree",
	} {
		w, g := wsKit(oneRepo)
		g.createErr = status.Error(codes.Internal, msg)
		in := ensureIn()
		in.BaseRef = "refs/heads/release/x"
		if _, err := w.Ensure(ctxWithIdentity(), in); !domain.IsBaseRefNotFound(err) {
			t.Errorf("%q: %v", msg, err)
		}
	}
}

func TestEnsure_OtherNotFoundAndInternalFailuresAreNotAMissingRef(t *testing.T) {
	for name, err := range map[string]error{
		"repo not found": status.Error(codes.NotFound, "WORKTREE_REPO_NOT_FOUND: repo does not exist"),
		"bookkeeping":    status.Error(codes.Internal, "WORKTREE_BOOKKEEPING_FAILED: worktree created but bookkeeping failed; rolled back cleanly"),
	} {
		w, g := wsKit(oneRepo)
		g.createErr = err
		in := ensureIn()
		in.BaseRef = "release/x"
		if _, got := w.Ensure(ctxWithIdentity(), in); domain.IsBaseRefNotFound(got) || got == nil {
			t.Errorf("%s: %v", name, got)
		}
	}
}

func TestEnsure_ExistingWorktreeKeepsItsRecordedBase(t *testing.T) {
	g := newFakeProposalGit()
	g.createErr = status.Error(codes.AlreadyExists, "WORKTREE_PATH_EXISTS: path already exists")
	projects := &fakeProjects{repos: []*projectv1.Repo{oneRepo}, worktrees: []*projectv1.Worktree{
		{Id: "wt-live", Path: "/srv/wt", Branch: "request/42-proposal", RepoId: "repo-1", Active: true, BaseRef: strPtr("main")},
	}}
	w := NewProposalWorkspace(g, projects)
	in := ensureIn()
	in.BaseRef = "release/x"
	ws, err := w.Ensure(ctxWithIdentity(), in)
	if err != nil || ws.WorktreeID != "wt-live" || ws.BaseRef != "main" {
		t.Fatalf("a worktree that exists is not recut from the Request's current ref: %+v %v", ws, err)
	}
	projects.worktrees[0].BaseRef = nil
	if ws, err := w.Ensure(ctxWithIdentity(), in); err != nil || ws.BaseRef != "release/x" {
		t.Fatalf("an old record without a base falls back to the requested one: %+v %v", ws, err)
	}
}

func strPtr(s string) *string { return &s }
