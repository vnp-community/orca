package usecase

import (
	"context"
	"fmt"
	"sync"

	"github.com/stablyai/orca-go/common/apperrors"
)

const maxFanOutN = 10 // BR-WT-05

type FanOutCreateWorktreesInput struct {
	ProjectID, RepoID, BaseRef, BranchPrefix string
	Prompt                                   string
	N                                        int
	AgentType                                string
}

type FanOutItemResult struct {
	Index                     int
	WorktreeID, Path, HeadSHA string
	PtyID, ConnectionID       string
	Status                    string // "ready" | "failed"
	Error                     string
}

type FanOutCreateWorktrees struct {
	worktrees WorktreeCreator
	agents    AgentSpawner
	prompts   PromptInjector
}

func NewFanOutCreateWorktrees(w WorktreeCreator, a AgentSpawner, p PromptInjector) *FanOutCreateWorktrees {
	return &FanOutCreateWorktrees{worktrees: w, agents: a, prompts: p}
}

func (uc *FanOutCreateWorktrees) Execute(ctx context.Context, in FanOutCreateWorktreesInput) ([]FanOutItemResult, error) {
	if in.N < 1 || in.N > maxFanOutN { // BR-WT-05
		return nil, apperrors.New(apperrors.KindInvalidArgument, "FANOUT_N_OUT_OF_RANGE", "n must be between 1 and 10", nil)
	}
	// BR-WT-06 is enforced by construction: every item below uses the same
	// in.BaseRef — there is no per-item override in this input shape.

	results := make([]FanOutItemResult, in.N)
	var wg sync.WaitGroup
	for i := 0; i < in.N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// BR-WT-08: this goroutine shares the SAME parent ctx across
			// every item — deliberately NOT errgroup.WithContext's
			// cancel-on-first-error, unlike worktree.detectedList's
			// read-aggregation precedent. One item's error below only ever
			// writes to results[idx], never cancels a sibling's in-flight
			// call.
			results[idx] = uc.runOne(ctx, in, idx)
		}(i)
	}
	wg.Wait()
	return results, nil
}

// RunOne is also [A1]'s retry entry point — exported so a caller can
// re-invoke it for a single failed index without re-running the other N-1
// items.
func (uc *FanOutCreateWorktrees) RunOne(ctx context.Context, in FanOutCreateWorktreesInput, idx int) FanOutItemResult {
	return uc.runOne(ctx, in, idx)
}

func (uc *FanOutCreateWorktrees) runOne(ctx context.Context, in FanOutCreateWorktreesInput, idx int) FanOutItemResult {
	branch := fmt.Sprintf("%s-%d", in.BranchPrefix, idx+1)
	worktreeID, path, headSHA, err := uc.worktrees.CreateWorktree(ctx, in.ProjectID, in.RepoID, branch, in.BaseRef)
	if err != nil {
		return FanOutItemResult{Index: idx, Status: "failed", Error: err.Error()}
	}

	// BR-WT-07: prompt injection only happens after SpawnAgentTerminal has
	// returned successfully — sequential within this goroutine, not raced
	// against the spawn call. See this file's "Known limitation" note below
	// for what "fully started" is approximated as.
	ptyID, connectionID, err := uc.agents.SpawnAgentTerminal(ctx, in.ProjectID, path, in.AgentType)
	if err != nil {
		return FanOutItemResult{Index: idx, WorktreeID: worktreeID, Path: path, HeadSHA: headSHA, Status: "failed", Error: err.Error()}
	}

	if err := uc.prompts.InjectPrompt(ctx, connectionID, ptyID, in.Prompt); err != nil {
		return FanOutItemResult{Index: idx, WorktreeID: worktreeID, Path: path, HeadSHA: headSHA, PtyID: ptyID, ConnectionID: connectionID, Status: "failed", Error: err.Error()}
	}

	return FanOutItemResult{Index: idx, WorktreeID: worktreeID, Path: path, HeadSHA: headSHA, PtyID: ptyID, ConnectionID: connectionID, Status: "ready"}
}

// Known limitation, carried forward from the SOL, not silently resolved:
// BR-WT-07 says the prompt must be injected only after the agent is fully
// started, not merely after the PTY process exists. No documented "CLI
// inside this PTY is ready for input" signal exists anywhere in
// infra-fleet-service.md or the agent RPC catalog. This implementation uses
// "SpawnTerminalSession returned successfully" as the readiness signal — a
// conservative under-approximation. A real fix needs either a per-agent-type
// ready-output pattern or an explicit agent.ready signal from the Dev Server
// Agent side, both out of scope here since the agent-side contract doesn't
// exist yet.
