package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func TestResolvePrBase_HappyPath(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{fetchAndResolveRefSHA: "resolved-sha-1"}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scm := &fakeSCMClient{prBaseBranch: "main"}
	uc := NewResolvePrBase(scm, reachability, projects, local, relay)

	got, err := uc.Execute(context.Background(), "repo-1", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Branch != "main" || got.SHA != "resolved-sha-1" {
		t.Errorf("unexpected result: %+v", got)
	}
}

func TestResolvePrBase_FetchAndResolveRefFails_ReturnsUnresolvableAndZeroValue(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{fetchAndResolveRefErr: errors.New("base branch not found locally")}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scm := &fakeSCMClient{prBaseBranch: "main", prBaseSHA: "should-not-leak"}
	uc := NewResolvePrBase(scm, reachability, projects, local, relay)

	got, err := uc.Execute(context.Background(), "repo-1", 42)
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

func TestResolvePrBase_SCMLookupFails_FetchAndResolveRefNeverCalled(t *testing.T) {
	reachability := &fakeDevServerReachability{}
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	projects := &fakeProjectClient{}
	scm := &fakeSCMClient{prBaseErr: apperrors.New(apperrors.KindInternal, "WORKTREE_SCM_GET_PR_BASE_UNIMPLEMENTED", "scm-integration-service has no RPC yet", nil)}
	uc := NewResolvePrBase(scm, reachability, projects, local, relay)

	_, err := uc.Execute(context.Background(), "repo-1", 42)
	if err == nil {
		t.Fatal("expected an error")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Code != "WORKTREE_PR_BASE_LOOKUP_FAILED" {
		t.Fatalf("expected WORKTREE_PR_BASE_LOOKUP_FAILED, got %v", err)
	}
	if local.calledFetchAndResolve || relay.calledFetchAndResolve {
		t.Error("expected FetchAndResolveRef NOT to be called when the SCM lookup itself fails")
	}
}
