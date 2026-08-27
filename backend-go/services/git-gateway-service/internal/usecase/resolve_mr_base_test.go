package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// Symmetrical to resolve_pr_base_test.go's cases, through
// SCMClient.GetMergeRequestBase/ResolveMrBase.Execute.

func TestResolveMrBase_HappyPath(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{fetchAndResolveRefSHA: "resolved-sha-2"}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scm := &fakeSCMClient{mrBaseBranch: "main"}
	uc := NewResolveMrBase(scm, resolver, projects, local, relay)

	got, err := uc.Execute(context.Background(), "repo-1", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Branch != "main" || got.SHA != "resolved-sha-2" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestResolveMrBase_FetchAndResolveRefFails_ReturnsUnresolvableAndZeroValue(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{fetchAndResolveRefErr: errors.New("base branch not found locally")}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scm := &fakeSCMClient{mrBaseBranch: "main", mrBaseSHA: "should-not-leak"}
	uc := NewResolveMrBase(scm, resolver, projects, local, relay)

	got, err := uc.Execute(context.Background(), "repo-1", 7)
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_BASE_REF_UNRESOLVABLE" {
		t.Fatalf("expected WORKTREE_BASE_REF_UNRESOLVABLE, got %v", err)
	}
	if got != (domain.ResolvedBase{}) {
		t.Errorf("expected zero-value ResolvedBase on error (no leaked SCM data), got %+v", got)
	}
}

func TestResolveMrBase_SCMLookupFails_FetchAndResolveRefNeverCalled(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false, RepoPath: "/repo"}}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scm := &fakeSCMClient{mrBaseErr: apperrors.New(apperrors.KindInternal, "WORKTREE_SCM_GET_MR_BASE_UNIMPLEMENTED", "scm-integration-service has no RPC yet", nil)}
	uc := NewResolveMrBase(scm, resolver, projects, local, relay)

	_, err := uc.Execute(context.Background(), "repo-1", 7)
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_MR_BASE_LOOKUP_FAILED" {
		t.Fatalf("expected WORKTREE_MR_BASE_LOOKUP_FAILED, got %v", err)
	}
	if local.calledFetchAndResolve || relay.calledFetchAndResolve {
		t.Error("expected FetchAndResolveRef NOT to be called when the SCM lookup itself fails")
	}
}
