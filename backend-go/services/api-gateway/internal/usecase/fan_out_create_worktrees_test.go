package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stablyai/orca-go/common/apperrors"
)

// fakeWorktreeCreator/fakeAgentSpawner/fakePromptInjector are in-memory
// fakes guarded by a mutex — Execute runs items concurrently (BR-WT-08), so
// every fake here must be safe for concurrent use.
type fakeWorktreeCreator struct {
	mu    sync.Mutex
	calls []struct{ projectID, repoID, branch, baseRef string }

	// failBranch, if non-empty, makes CreateWorktree fail only when called
	// with this exact branch name — lets a test target one specific index.
	failBranch string
}

func (f *fakeWorktreeCreator) CreateWorktree(ctx context.Context, projectID, repoID, branch, baseRef string) (string, string, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, struct{ projectID, repoID, branch, baseRef string }{projectID, repoID, branch, baseRef})
	if f.failBranch != "" && branch == f.failBranch {
		return "", "", "", errors.New("create worktree failed")
	}
	return "wt-" + branch, "/repo-" + branch, "sha-" + branch, nil
}

func (f *fakeWorktreeCreator) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type agentSpawnerCall struct {
	projectID, worktreePath, agentType string
	seq                                int64
}

type fakeAgentSpawner struct {
	mu       sync.Mutex
	calls    []agentSpawnerCall
	seq      atomic.Int64
	failPath string // fail SpawnAgentTerminal when worktreePath matches this
}

func (f *fakeAgentSpawner) SpawnAgentTerminal(ctx context.Context, projectID, worktreePath, agentType string) (string, string, error) {
	n := f.seq.Add(1)
	f.mu.Lock()
	f.calls = append(f.calls, agentSpawnerCall{projectID, worktreePath, agentType, n})
	f.mu.Unlock()
	if f.failPath != "" && worktreePath == f.failPath {
		return "", "", errors.New("spawn agent failed")
	}
	return "pty-" + worktreePath, "conn-" + worktreePath, nil
}

func (f *fakeAgentSpawner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type promptInjectorCall struct {
	connectionID, ptyID, prompt string
	seq                         int64
}

type fakePromptInjector struct {
	mu    sync.Mutex
	calls []promptInjectorCall
	seq   *atomic.Int64 // shared sequence counter with fakeAgentSpawner, for ordering assertions
}

func (f *fakePromptInjector) InjectPrompt(ctx context.Context, connectionID, ptyID, prompt string) error {
	var n int64
	if f.seq != nil {
		n = f.seq.Add(1)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, promptInjectorCall{connectionID, ptyID, prompt, n})
	return nil
}

func (f *fakePromptInjector) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakePromptInjector) callsFor(ptyID string) []promptInjectorCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []promptInjectorCall
	for _, c := range f.calls {
		if c.ptyID == ptyID {
			out = append(out, c)
		}
	}
	return out
}

func TestFanOutCreateWorktrees_RejectsN0AndN11_NoCallsMade(t *testing.T) {
	for _, n := range []int{0, 11} {
		worktrees := &fakeWorktreeCreator{}
		agents := &fakeAgentSpawner{}
		prompts := &fakePromptInjector{}
		uc := NewFanOutCreateWorktrees(worktrees, agents, prompts)

		_, err := uc.Execute(context.Background(), FanOutCreateWorktreesInput{
			ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main", BranchPrefix: "feat", Prompt: "do it", N: n, AgentType: "claude",
		})
		if err == nil {
			t.Fatalf("N=%d: expected an error", n)
		}
		var ae *apperrors.AppError
		if !errors.As(err, &ae) || ae.Code != "FANOUT_N_OUT_OF_RANGE" {
			t.Fatalf("N=%d: expected FANOUT_N_OUT_OF_RANGE, got %v", n, err)
		}
		if worktrees.callCount() != 0 {
			t.Errorf("N=%d: expected zero CreateWorktree calls, got %d", n, worktrees.callCount())
		}
	}
}

func TestFanOutCreateWorktrees_AllNShareSameBaseRef(t *testing.T) {
	worktrees := &fakeWorktreeCreator{}
	agents := &fakeAgentSpawner{}
	prompts := &fakePromptInjector{}
	uc := NewFanOutCreateWorktrees(worktrees, agents, prompts)

	results, err := uc.Execute(context.Background(), FanOutCreateWorktreesInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main", BranchPrefix: "feat", Prompt: "do it", N: 5, AgentType: "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	worktrees.mu.Lock()
	defer worktrees.mu.Unlock()
	if len(worktrees.calls) != 5 {
		t.Fatalf("expected 5 CreateWorktree calls, got %d", len(worktrees.calls))
	}
	for _, c := range worktrees.calls {
		if c.baseRef != "main" {
			t.Errorf("expected every CreateWorktree call to use baseRef=main, got %q", c.baseRef)
		}
	}
}

// TestFanOutCreateWorktrees_OneItemFails_OthersStillComplete is the core
// regression guard against copying worktree.detectedList's
// errgroup.WithContext cancel-on-first-error pattern (BR-WT-08).
func TestFanOutCreateWorktrees_OneItemFails_OthersStillComplete(t *testing.T) {
	worktrees := &fakeWorktreeCreator{failBranch: "feat-3"} // index 2 (0-based) -> branch "feat-3"
	agents := &fakeAgentSpawner{}
	prompts := &fakePromptInjector{}
	uc := NewFanOutCreateWorktrees(worktrees, agents, prompts)

	results, err := uc.Execute(context.Background(), FanOutCreateWorktreesInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main", BranchPrefix: "feat", Prompt: "do it", N: 5, AgentType: "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	for _, idx := range []int{0, 1, 3, 4} {
		if results[idx].Status != "ready" {
			t.Errorf("index %d: expected status ready, got %q (error=%q)", idx, results[idx].Status, results[idx].Error)
		}
		if strings.Contains(strings.ToLower(results[idx].Error), "context canceled") {
			t.Errorf("index %d: result carries a context-cancellation error: %q — sibling failure must never cancel this item", idx, results[idx].Error)
		}
	}
	if results[2].Status != "failed" {
		t.Errorf("index 2: expected status failed, got %q", results[2].Status)
	}
}

// TestFanOutCreateWorktrees_PromptInjectedOnlyAfterSpawnSucceeds asserts
// BR-WT-07's ordering: InjectPrompt's call sequence number must be strictly
// greater than SpawnAgentTerminal's for the same item.
func TestFanOutCreateWorktrees_PromptInjectedOnlyAfterSpawnSucceeds(t *testing.T) {
	worktrees := &fakeWorktreeCreator{}
	agents := &fakeAgentSpawner{}
	prompts := &fakePromptInjector{seq: &agents.seq}
	uc := NewFanOutCreateWorktrees(worktrees, agents, prompts)

	results, err := uc.Execute(context.Background(), FanOutCreateWorktreesInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main", BranchPrefix: "feat", Prompt: "do it", N: 3, AgentType: "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, r := range results {
		if r.Status != "ready" {
			t.Fatalf("index %d: expected ready, got %q", r.Index, r.Status)
		}
		spawnCalls := agents.calls // spawner records its own seq at call time
		var spawnSeq int64 = -1
		for _, c := range spawnCalls {
			if c.worktreePath == r.Path {
				spawnSeq = c.seq
			}
		}
		if spawnSeq < 0 {
			t.Fatalf("index %d: no matching spawn call found for path %q", r.Index, r.Path)
		}
		injectCalls := prompts.callsFor(r.PtyID)
		if len(injectCalls) != 1 {
			t.Fatalf("index %d: expected exactly 1 InjectPrompt call for pty %q, got %d", r.Index, r.PtyID, len(injectCalls))
		}
		if injectCalls[0].seq <= spawnSeq {
			t.Errorf("index %d: expected InjectPrompt's seq (%d) to be strictly greater than SpawnAgentTerminal's (%d)", r.Index, injectCalls[0].seq, spawnSeq)
		}
	}
}

func TestFanOutCreateWorktrees_SpawnFails_PromptInjectorNeverCalled(t *testing.T) {
	worktrees := &fakeWorktreeCreator{}
	agents := &fakeAgentSpawner{failPath: "/repo-feat-2"} // index 1 -> branch feat-2 -> path /repo-feat-2
	prompts := &fakePromptInjector{}
	uc := NewFanOutCreateWorktrees(worktrees, agents, prompts)

	results, err := uc.Execute(context.Background(), FanOutCreateWorktreesInput{
		ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main", BranchPrefix: "feat", Prompt: "do it", N: 3, AgentType: "claude",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[1].Status != "failed" {
		t.Fatalf("index 1: expected failed, got %q", results[1].Status)
	}
	if prompts.callCount() != 2 {
		t.Errorf("expected InjectPrompt called for the 2 succeeding items only, got %d calls", prompts.callCount())
	}
}

func TestFanOutCreateWorktrees_RetrySingleIndex_ViaRunOne(t *testing.T) {
	worktrees := &fakeWorktreeCreator{failBranch: "feat-2"}
	agents := &fakeAgentSpawner{}
	prompts := &fakePromptInjector{}
	uc := NewFanOutCreateWorktrees(worktrees, agents, prompts)

	in := FanOutCreateWorktreesInput{ProjectID: "proj-1", RepoID: "repo-1", BaseRef: "main", BranchPrefix: "feat", Prompt: "do it", N: 3, AgentType: "claude"}
	results, err := uc.Execute(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[1].Status != "failed" {
		t.Fatalf("expected index 1 to have failed on first run")
	}
	countBefore0 := countCallsFor(worktrees, fmt.Sprintf("%s-%d", in.BranchPrefix, 1))
	countBefore2 := countCallsFor(worktrees, fmt.Sprintf("%s-%d", in.BranchPrefix, 3))

	// Fix the fake so a retry now succeeds, then retry only index 1.
	worktrees.mu.Lock()
	worktrees.failBranch = ""
	worktrees.mu.Unlock()
	retryResult := uc.RunOne(context.Background(), in, 1)
	if retryResult.Status != "ready" {
		t.Fatalf("expected retry to succeed, got %q (error=%q)", retryResult.Status, retryResult.Error)
	}

	if got := countCallsFor(worktrees, fmt.Sprintf("%s-%d", in.BranchPrefix, 1)); got != countBefore0 {
		t.Errorf("expected no additional calls for index 0's branch, got %d (was %d)", got, countBefore0)
	}
	if got := countCallsFor(worktrees, fmt.Sprintf("%s-%d", in.BranchPrefix, 3)); got != countBefore2 {
		t.Errorf("expected no additional calls for index 2's branch, got %d (was %d)", got, countBefore2)
	}
}

func countCallsFor(f *fakeWorktreeCreator, branch string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c.branch == branch {
			n++
		}
	}
	return n
}
