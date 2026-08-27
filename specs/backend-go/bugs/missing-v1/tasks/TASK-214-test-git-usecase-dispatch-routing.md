# TASK-214: Usecase dispatch-routing tests for all new `git-gateway-service` usecases (Groups A-E)

**From Solution:** SOL-032 (Test plan — `usecase/*_test.go`)
**Priority:** P1
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/usecase/dispatch_test.go` (extend), plus new `*_test.go` files as needed
**Depends on:** TASK-207, TASK-208, TASK-209, TASK-210, TASK-211
**Status:** `[x]` DONE — Dispatch-routing tests added for every usecase this pass actually implemented (Stage/Unstage/History/CheckIgnored/ForkSync/UpstreamStatus/RemoteCommitURL/RemoteFileURL/GeneratePullRequestFields/DiscoverCommitMessageModels, plus the file-io usecases in `filesystem_dispatch_test.go`) in `dispatch_test.go`, following its existing `fakeGitExecutor`-per-package convention. Closed the 2 remaining gaps found this pass: `TestUnstage_MissingPaths_ReturnsError` (Stage had this validation test, Unstage didn't) and `TestRemoteFileURL_RoutesByConnectionState` (RemoteCommitURL had a routing test, RemoteFileURL only had its missing-path guard). Only covers what's implemented — TASK-207's 9 branch/ref methods and TASK-209/210's 6 BLOCKED methods have no usecases to test since they were never built. `go build`/`go vet`/`go test ./internal/usecase/...` clean (80 tests).

---

## Context

Per SOL-032's test plan and `dispatch_test.go`'s existing convention: one
table-driven-style test per new usecase, using a fake `ConnectionResolver`
(both connected/not-connected cases) and a fake `GitExecutor`, asserting
the branch selection (local vs. relay) and that the resolved `repoPath`
reaches the executor call. No real Postgres/gRPC, per
`03-clean-architecture-guidelines.md`'s usecase-test policy.

## Changes to make

**File:** `backend-go/services/git-gateway-service/internal/usecase/dispatch_test.go`

### Step 1: Extend `fakeGitExecutor` with the new methods

Add a `called<Method>`/`got*`/`<method>Err` field triple for every method
TASK-207/208/209/210 added to `GitExecutor`, following the existing
`calledCommit`/`commitErr` pattern exactly, and implement each new method
on `fakeGitExecutor`:

```go
type fakeGitExecutor struct {
	name string

	// ... existing fields (calledGetStatus, calledCommit, etc.) unchanged ...

	calledCheckout          bool
	calledListLocalBranches bool
	calledFastForward       bool
	calledRebaseFromBase    bool
	calledAbortRebase       bool
	calledAbortMerge        bool
	calledConflictOperation bool
	calledDiscard           bool
	calledBulkDiscard       bool
	calledStage             bool
	calledUnstage           bool
	calledHistory           bool
	calledCommitCompare     bool
	calledBranchCompare     bool
	calledCommitDiff        bool
	calledBranchDiff        bool
	calledSubmoduleStatus   bool
	calledCheckIgnored      bool
	calledForkSync          bool
	calledUpstreamStatus    bool
	calledFetch             bool
	calledRemoteCommitURL   bool
	calledRemoteFileURL     bool

	checkoutErr error
	// ... one <method>Err field per new method, same pattern ...
}

func (f *fakeGitExecutor) Checkout(ctx context.Context, repoPath, ref string, create bool) (domain.CheckoutResult, error) {
	f.calledCheckout = true
	f.gotRepoPath = repoPath
	if f.checkoutErr != nil {
		return domain.CheckoutResult{}, f.checkoutErr
	}
	return domain.CheckoutResult{Success: true, Branch: ref}, nil
}

// ListLocalBranches, FastForward, RebaseFromBase, AbortRebase, AbortMerge,
// ConflictOperation, Discard, BulkDiscard, Stage, Unstage, History,
// CommitCompare, BranchCompare, CommitDiff, BranchDiff, SubmoduleStatus,
// CheckIgnored, ForkSync, UpstreamStatus, Fetch, RemoteCommitURL,
// RemoteFileURL follow the identical shape: set the calledX flag, record
// gotRepoPath, return a fixed non-zero success value or the configured
// error.
```

### Step 2: One routing test pair per new usecase

Add `Test<Name>_RoutesByConnectionState` (Connected=true → relay,
Connected=false → local — split into 2 tests or 1 table-driven test
matching this file's existing per-usecase style) plus a
`Test<Name>_MissingWorktreeID_ReturnsError` for every new usecase, mirroring
`TestCommit_RoutesByConnectionState`/`TestCommit_MissingMessage_ReturnsError`.
Representative example for `Checkout`:

```go
func TestCheckout_RoutesByConnectionState(t *testing.T) {
	local := &fakeGitExecutor{}
	relay := &fakeGitExecutor{}
	uc := NewCheckout(&fakeConnectionResolver{conn: ResolvedConnection{Connected: true, RepoPath: "/repo/wt1"}}, local, relay)

	got, err := uc.Execute(context.Background(), CheckoutInput{WorktreeID: "wt1", Ref: "feature/x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !relay.calledCheckout || local.calledCheckout {
		t.Error("expected Checkout to route to relay when Connected=true")
	}
	if relay.gotRepoPath != "/repo/wt1" {
		t.Errorf("expected resolved repo path to be passed through, got %q", relay.gotRepoPath)
	}
	if !got.Success {
		t.Errorf("unexpected checkout result: %+v", got)
	}
}

func TestCheckout_MissingRef_ReturnsError(t *testing.T) {
	uc := NewCheckout(&fakeConnectionResolver{}, &fakeGitExecutor{}, &fakeGitExecutor{})
	_, err := uc.Execute(context.Background(), CheckoutInput{WorktreeID: "wt1"})
	if err == nil {
		t.Fatal("expected error for missing ref")
	}
}
```

Repeat for every usecase added in TASK-207 through TASK-210 (22 usecases:
`Checkout`, `ListLocalBranches`, `FastForward`, `RebaseFromBase`,
`AbortRebase`, `AbortMerge`, `ConflictOperation`, `Discard`, `BulkDiscard`,
`Stage`, `Unstage`, `History`, `CommitCompare`, `BranchCompare`,
`CommitDiff`, `BranchDiff`, `SubmoduleStatus`, `CheckIgnored`, `ForkSync`,
`UpstreamStatus`, `Fetch`, `RemoteCommitURL`, `RemoteFileURL` — 23 total).

### Step 3: `GeneratePullRequestFields` tests (TASK-211) — new file `generate_pull_request_fields_test.go`

Follow `TestGenerateCommitMessage_*`'s exact 4-test shape
(`dispatch_test.go:247-317`) with `fakeAICompleter`:

```go
func TestGeneratePullRequestFields_Connected_RelaysDiffAndReturnsFields(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: true, ConnectionID: "conn-1", RepoPath: "/repo/wt1"}}
	getDiff := NewGetDiff(resolver, &fakeGitExecutor{}, &fakeGitExecutor{})
	completer := &fakeAICompleter{message: "Add widget\nImplements the widget feature."}
	uc := NewGeneratePullRequestFields(resolver, getDiff, completer)

	got, err := uc.Execute(context.Background(), GeneratePullRequestFieldsInput{WorktreeID: "wt1", BaseBranch: "main"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Title != "Add widget" {
		t.Errorf("unexpected title: %q", got.Title)
	}
	if got.Description != "Implements the widget feature." {
		t.Errorf("unexpected description: %q", got.Description)
	}
}

func TestGeneratePullRequestFields_NotConnected_ReturnsFailedPrecondition(t *testing.T) {
	resolver := &fakeConnectionResolver{conn: ResolvedConnection{Connected: false}}
	getDiff := NewGetDiff(resolver, &fakeGitExecutor{}, &fakeGitExecutor{})
	uc := NewGeneratePullRequestFields(resolver, getDiff, &fakeAICompleter{})

	_, err := uc.Execute(context.Background(), GeneratePullRequestFieldsInput{WorktreeID: "wt1"})
	if err == nil {
		t.Fatal("expected error when worktree has no relay connection")
	}
	var ae *apperrors.AppError
	if !errors.As(err, &ae) || ae.Kind != apperrors.KindFailedPrecondition {
		t.Fatalf("expected KindFailedPrecondition AppError, got %v", err)
	}
}
```

### Step 4: `DiscoverCommitMessageModels` tests — new file `discover_commit_message_models_test.go`

```go
package usecase

import (
	"context"
	"testing"
)

type fakeAIProviderResolver struct {
	providerType, accountID, status string
	err                              error
}

func (f *fakeAIProviderResolver) ResolveProvider(ctx context.Context, tenantID, userID string) (string, string, string, error) {
	return f.providerType, f.accountID, f.status, f.err
}

func TestDiscoverCommitMessageModels_ReturnsResolvedAccount(t *testing.T) {
	resolver := &fakeAIProviderResolver{providerType: "PROVIDER_TYPE_ANTHROPIC", accountID: "acct-1", status: "active"}
	uc := NewDiscoverCommitMessageModels(resolver)

	got, err := uc.Execute(context.Background(), DiscoverCommitMessageModelsInput{TenantID: "t1", UserID: "u1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].AccountID != "acct-1" {
		t.Errorf("unexpected models: %+v", got)
	}
}

func TestDiscoverCommitMessageModels_NoAccount_ReturnsEmpty(t *testing.T) {
	uc := NewDiscoverCommitMessageModels(&fakeAIProviderResolver{})
	got, err := uc.Execute(context.Background(), DiscoverCommitMessageModelsInput{TenantID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result when no account resolved, got %+v", got)
	}
}

func TestDiscoverCommitMessageModels_MissingTenantID_ReturnsError(t *testing.T) {
	uc := NewDiscoverCommitMessageModels(&fakeAIProviderResolver{})
	_, err := uc.Execute(context.Background(), DiscoverCommitMessageModelsInput{})
	if err == nil {
		t.Fatal("expected error for missing tenant_id")
	}
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go/services/git-gateway-service
go build ./... && go test ./internal/usecase/... -count=1 -v
```

Expected: every new usecase has at least one Connected-routes-to-relay
test, one NotConnected-routes-to-local test (where applicable), and one
required-field-validation test; all pass.
