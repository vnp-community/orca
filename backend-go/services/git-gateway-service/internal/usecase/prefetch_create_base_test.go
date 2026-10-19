package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

func prefetchKit(local *fakeGitExecutor) *PrefetchCreateBase {
	return NewPrefetchCreateBase(&fakeDevServerReachability{allowLocal: true}, &fakeProjectClient{}, local, &fakeGitExecutor{name: "relay"})
}

func appErr(t *testing.T, err error) *apperrors.AppError {
	t.Helper()
	var ae *apperrors.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("want an AppError, got %v", err)
	}
	return ae
}

func TestPrefetchCreateBase_ResolvedRef_ReturnsSHA(t *testing.T) {
	local := &fakeGitExecutor{fetchAndResolveRefSHA: "abc123"}
	sha, err := prefetchKit(local).Execute(context.Background(), "repo-1", "release/x")
	if err != nil || sha != "abc123" {
		t.Fatalf("sha=%q err=%v", sha, err)
	}
}

func TestPrefetchCreateBase_MissingRef_IsNotFoundWithAvailableRefs(t *testing.T) {
	for name, gitErr := range map[string]error{
		"missing remote ref": errors.New("git fetch origin nope: exit status 128: fatal: couldn't find remote ref nope"),
		"relay exit":         errors.New("relay: git fetch exited with 128: fatal: couldn't find remote ref nope"),
		"invalid refspec":    errors.New("fatal: invalid refspec 'nope'"),
		"no origin at all":   errors.New("fatal: 'origin' does not appear to be a git repository\nfatal: Could not read from remote repository."),
	} {
		t.Run(name, func(t *testing.T) {
			local := &fakeGitExecutor{
				fetchAndResolveRefErr: gitErr,
				branchInfos:           []domain.BranchInfo{{Name: "main"}, {Name: "develop"}, {Name: "feature/nope-2"}},
			}
			_, err := prefetchKit(local).Execute(context.Background(), "repo-1", "nope")
			ae := appErr(t, err)
			if ae.Code != "WORKTREE_BASE_REF_NOT_FOUND" || ae.Kind != apperrors.KindNotFound {
				t.Fatalf("got %s %v", ae.Code, ae.Kind)
			}
			if !strings.Contains(ae.Message, "available: feature/nope-2, main, develop") {
				t.Errorf("the branches that contain the name must come first: %q", ae.Message)
			}
		})
	}
}

func TestPrefetchCreateBase_MissingRef_ListFailureStillNotFound(t *testing.T) {
	local := &fakeGitExecutor{fetchAndResolveRefErr: errors.New("fatal: couldn't find remote ref x"), listLocalBranchesErr: errors.New("boom")}
	_, err := prefetchKit(local).Execute(context.Background(), "repo-1", "x")
	ae := appErr(t, err)
	if ae.Code != "WORKTREE_BASE_REF_NOT_FOUND" || strings.Contains(ae.Message, "available") {
		t.Fatalf("%s %q", ae.Code, ae.Message)
	}
}

func TestPrefetchCreateBase_AvailableListIsBounded(t *testing.T) {
	var infos []domain.BranchInfo
	for i := 0; i < 500; i++ {
		infos = append(infos, domain.BranchInfo{Name: fmt.Sprintf("branch-%03d", i)})
	}
	local := &fakeGitExecutor{fetchAndResolveRefErr: errors.New("fatal: couldn't find remote ref x"), branchInfos: infos}
	_, err := prefetchKit(local).Execute(context.Background(), "repo-1", "x")
	if got := strings.Count(appErr(t, err).Message, "branch-"); got != maxAvailableRefsListed {
		t.Fatalf("%d refs listed, want %d", got, maxAvailableRefsListed)
	}
}

func TestPrefetchCreateBase_NetworkFailure_IsUnavailableNotNotFound(t *testing.T) {
	for name, gitErr := range map[string]error{
		"dns":     errors.New("fatal: unable to access 'https://x/': Could not resolve host: x"),
		"refused": errors.New("ssh: connect to host x port 22: Connection refused\nfatal: Could not read from remote repository."),
		"auth":    errors.New("git@x: Permission denied (publickey)."),
		"relay":   errors.New("grpcclient: relayByDevServer git.exec: rpc error: code = Unavailable desc = transport is closing"),
	} {
		t.Run(name, func(t *testing.T) {
			local := &fakeGitExecutor{fetchAndResolveRefErr: gitErr}
			_, err := prefetchKit(local).Execute(context.Background(), "repo-1", "release/x")
			ae := appErr(t, err)
			if ae.Code != "WORKTREE_PREFETCH_UNAVAILABLE" || ae.Kind != apperrors.KindUnavailable {
				t.Fatalf("got %s %v", ae.Code, ae.Kind)
			}
			if local.calledListLocalBranches {
				t.Error("a ref that could not be asked about is not listed against the local branches")
			}
		})
	}
}

func TestPrefetchCreateBase_DeadlineAndHostFailures(t *testing.T) {
	t.Run("deadline", func(t *testing.T) {
		local := &fakeGitExecutor{fetchAndResolveRefErr: fmt.Errorf("fetch: %w", context.DeadlineExceeded)}
		_, err := prefetchKit(local).Execute(context.Background(), "repo-1", "main")
		if ae := appErr(t, err); ae.Kind != apperrors.KindDeadlineExceeded || ae.Code != "WORKTREE_PREFETCH_TIMEOUT" {
			t.Fatalf("%s %v", ae.Code, ae.Kind)
		}
	})
	t.Run("dev server went away mid call", func(t *testing.T) {
		local := &fakeGitExecutor{fetchAndResolveRefErr: ErrDevServerUnreachable("ds-1")}
		_, err := prefetchKit(local).Execute(context.Background(), "repo-1", "main")
		if ae := appErr(t, err); ae.Kind != apperrors.KindUnavailable || ae.Code != CodeDevServerUnreachable {
			t.Fatalf("%s %v", ae.Code, ae.Kind)
		}
	})
	t.Run("dev server offline before the call", func(t *testing.T) {
		repo := domain.RepoInfo{DevServerID: "ds-1"}
		uc := NewPrefetchCreateBase(&fakeDevServerReachability{reachable: false}, &fakeProjectClient{getRepoResult: repo}, &fakeGitExecutor{}, &fakeGitExecutor{})
		_, err := uc.Execute(context.Background(), "repo-1", "main")
		if hostErr := HostDispatchError(err); hostErr == nil || hostErr.Kind != apperrors.KindUnavailable {
			t.Fatalf("want the host-routing error to be reachable in the chain: %v", err)
		}
	})
	t.Run("anything else stays internal", func(t *testing.T) {
		local := &fakeGitExecutor{fetchAndResolveRefErr: errors.New("fatal: object directory is corrupt")}
		_, err := prefetchKit(local).Execute(context.Background(), "repo-1", "main")
		if ae := appErr(t, err); ae.Kind != apperrors.KindInternal || ae.Code != "WORKTREE_PREFETCH_FAILED" {
			t.Fatalf("%s %v", ae.Code, ae.Kind)
		}
	})
}

func TestPrefetchCreateBase_RejectsEmptyAndOptionLikeRefs(t *testing.T) {
	for _, ref := range []string{"", "  ", "--upload-pack=x", "-f"} {
		local := &fakeGitExecutor{}
		_, err := prefetchKit(local).Execute(context.Background(), "repo-1", ref)
		if ae := appErr(t, err); ae.Kind != apperrors.KindInvalidArgument || ae.Code != "WORKTREE_BASE_REF_INVALID" {
			t.Fatalf("%q: %s %v", ref, ae.Code, ae.Kind)
		}
		if local.calledFetchAndResolve {
			t.Fatalf("%q must never reach git", ref)
		}
	}
}

func TestPrefetchCreateBase_UnknownRepo_IsNotFound(t *testing.T) {
	uc := NewPrefetchCreateBase(&fakeDevServerReachability{allowLocal: true}, &fakeProjectClient{getRepoErr: errors.New("nope")}, &fakeGitExecutor{}, &fakeGitExecutor{})
	_, err := uc.Execute(context.Background(), "repo-1", "main")
	if ae := appErr(t, err); ae.Code != "WORKTREE_REPO_NOT_FOUND" {
		t.Fatal(ae.Code)
	}
}
