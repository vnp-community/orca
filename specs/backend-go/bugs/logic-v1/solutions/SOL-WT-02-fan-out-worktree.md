# SOL-WT-02: Fan-out saga at `api-gateway` composing existing per-worktree/per-agent RPCs

**Resolves:** [BUG-WT-02](../BUG-WT-02-fan-out-not-implemented.md)
**Service:** `api-gateway` (new usecase, new `wscompat` channel) — calls into `git-gateway-service`, `project-service`, `infra-fleet-service`; no changes to those services' proto surfaces
**Affected files (proposed):**
- `backend-go/services/api-gateway/internal/usecase/fan_out_create_worktrees.go` (new)
- `backend-go/services/api-gateway/internal/usecase/fan_out_create_worktrees_test.go` (new)
- `backend-go/services/api-gateway/internal/usecase/ports.go` — `WorktreeCreator`, `AgentSpawner`, `PromptInjector` ports
- `backend-go/services/api-gateway/internal/adapter/grpc/` (or a new `adapter/fanout/`) — port implementations wrapping `gitgatewayv1.GitGatewayServiceClient`, `projectv1.ProjectServiceClient`, `infrafleetv1.InfraFleetServiceClient`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go` — new `worktree.fanOut` channel
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### Where the saga lives: `api-gateway`, not a new service and not inside `git-gateway-service`/`infra-fleet-service`

BUG-WT-02 correctly identifies that no service currently coordinates
"create N worktrees, spawn N agents, inject N prompts" as one flow. The TDD
docs don't specify this flow either — this is a genuine, flagged extension
— but they do specify every building block the flow needs, and they specify
which service is allowed to call which:

- `02-microservices-decomposition.md`'s dependency graph already has
  `gw --> proj`, `gw --> infra`, `gw --> git` (`:134-136`) — `api-gateway`
  is the *only* node in that graph with a direct edge to all three services
  this saga touches. Placing the saga anywhere else (e.g. inside
  `git-gateway-service`) would require a new edge like `git --> infra`'s
  *agent-spawn* use (today's only `git --> infra` edge is connection
  resolution, per `git-gateway-service.md` §7) or a new `git --> proj`
  write path beyond bookkeeping — both larger, undocumented boundary
  changes. Doing it at the edge needs zero new dependency edges.
- `08-inter-service-communication.md`'s API Gateway responsibilities list
  already includes "response aggregation" and managing "real-time surfaces
  ... agent status" (`:47-67`) — this solution extends that from
  *read*-aggregation (the existing precedent,
  `channels_worktree.go`'s `worktree.detectedList`, merging two services'
  read data via `errgroup`) to a *write*-orchestration case. **This is the
  one genuine extension beyond what's explicitly documented** — the TDD
  only describes read-model aggregation living at the edge
  (`05-data-architecture.md`'s "Read models... across service boundaries"
  section, `:114-124`), not multi-step write sagas. Flagged here rather
  than papered over, matching this task's instruction. The alternative —
  giving `api-gateway`'s thin `internal/usecase/` layer (today: identity
  validation, rate limiting, WS bridging — `bridge_ws_session.go`,
  `rate_limit.go`, `validate_identity.go`) its first real multi-service
  business saga — is judged the lesser deviation, since every other
  placement requires an undocumented new cross-service dependency edge.
- **Per-item building blocks are already fully specified, not extensions**:
  - Worktree creation: `git-gateway-service.CreateWorktree`
    ([SOL-WT-01](./SOL-WT-01-tao-worktree.md)'s validated version), already real.
  - "Starting an agent" = spawning a PTY running the agent's CLI command,
    per `business-capabilities.md`'s own framing of `project.agentSpawn`
    relaying to `agent.exec` and the "terminal-agent-prompt injection...
    reaches into the PTY layer" note (`business-capabilities.md:51-52,167`).
    `infra-fleet-service.md`'s `SpawnTerminalSession` RPC (`:124`,
    already real: `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go`)
    accepts a `shell` field ("agent applies its own default if empty",
    `infrafleet.proto:300`) — setting it to the resolved agent-type's
    launch command *is* "spawning an agent" in this architecture, with no
    new RPC needed.
  - Resolving which host to spawn on: `project-service.GetProjectContext`
    (`project-service.md` §2's own prescribed two-step pattern: "resolve
    context here, then call the execution-owning service" — `:49-53`) plus
    `infra-fleet-service.ResolveConnection`/`EstablishConnection`, both real.
  - Prompt injection: `infra-fleet-service.AttachPty`'s bidirectional
    stream, `PtyInput{data}` frame (`infrafleet.proto:344-352`), already
    real and already the mechanism `api-gateway`'s WS bridge uses for
    ordinary terminal keystrokes (`bridge_ws_session.go`) — this solution
    reuses it for one write instead of many.

  So the "genuine extension" here is narrow: *the coordination itself*, not
  any of the four RPCs it calls.

### Critical correctness note: this is NOT `errgroup.WithContext`'s cancel-on-first-error pattern

`worktree.detectedList`'s existing `errgroup` use
(`channels_worktree.go:209-220`) is a read-aggregation where failing fast on
any error is correct — a partial read is useless. BR-WT-08 requires the
**opposite**: one item's failure must never affect the others. Using
`errgroup.WithContext` here would cancel every sibling worktree's context
the instant one fails, directly violating BR-WT-08. This solution uses a
plain `sync.WaitGroup` with per-index result capture instead — called out
explicitly since it's an easy mistake to copy the wrong existing precedent.

---

## Design — `usecase` layer

```go
// internal/usecase/fan_out_create_worktrees.go
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
	Prompt                                    string
	N                                          int
	AgentType                                  string
}

type FanOutItemResult struct {
	Index                            int
	WorktreeID, Path, HeadSHA        string
	PtyID, ConnectionID              string
	Status                           string // "ready" | "failed"
	Error                            string
}

// WorktreeCreator wraps git-gateway-service's already-real CreateWorktree RPC.
type WorktreeCreator interface {
	CreateWorktree(ctx context.Context, projectID, repoID, branch, baseRef string) (worktreeID, path, headSHA string, err error)
}

// AgentSpawner composes project-service.GetProjectContext + infra-fleet-service's
// ResolveConnection/SpawnTerminalSession — "starting an agent" per this
// file's Design rationale section.
type AgentSpawner interface {
	SpawnAgentTerminal(ctx context.Context, projectID, worktreePath, agentType string) (ptyID, connectionID string, err error)
}

// PromptInjector wraps infra-fleet-service's AttachPty stream — opens it,
// sends AttachToSession{pty_id} then PtyInput{data: prompt}, closes.
type PromptInjector interface {
	InjectPrompt(ctx context.Context, connectionID, ptyID, prompt string) error
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
			// BR-WT-08: this goroutine's ctx is the SAME parent ctx for all
			// items (deliberately not per-item errgroup.WithContext
			// cancellation) — one item's error below only ever writes to
			// results[idx], never cancels a sibling's in-flight call.
			results[idx] = uc.runOne(ctx, in, idx)
		}(i)
	}
	wg.Wait()
	return results, nil
}

// runOne is also the retry entry point for [A1] — a caller can re-invoke it
// for a single failed index without re-running the other N-1 items.
func (uc *FanOutCreateWorktrees) runOne(ctx context.Context, in FanOutCreateWorktreesInput, idx int) FanOutItemResult {
	branch := fmt.Sprintf("%s-%d", in.BranchPrefix, idx+1)
	worktreeID, path, headSHA, err := uc.worktrees.CreateWorktree(ctx, in.ProjectID, in.RepoID, branch, in.BaseRef)
	if err != nil {
		return FanOutItemResult{Index: idx, Status: "failed", Error: err.Error()}
	}

	// BR-WT-07: prompt injection only happens after SpawnAgentTerminal has
	// returned successfully (the PTY exists) — sequential within this
	// goroutine, not raced against the spawn call.
	ptyID, connectionID, err := uc.agents.SpawnAgentTerminal(ctx, in.ProjectID, path, in.AgentType)
	if err != nil {
		return FanOutItemResult{Index: idx, WorktreeID: worktreeID, Path: path, HeadSHA: headSHA, Status: "failed", Error: err.Error()}
	}

	if err := uc.prompts.InjectPrompt(ctx, connectionID, ptyID, in.Prompt); err != nil {
		return FanOutItemResult{Index: idx, WorktreeID: worktreeID, Path: path, HeadSHA: headSHA, PtyID: ptyID, ConnectionID: connectionID, Status: "failed", Error: err.Error()}
	}

	return FanOutItemResult{Index: idx, WorktreeID: worktreeID, Path: path, HeadSHA: headSHA, PtyID: ptyID, ConnectionID: connectionID, Status: "ready"}
}
```

### Known limitation: "fully started" is approximated as "PTY spawned successfully"

BR-WT-07 says the prompt must be injected only after the agent is *fully
started* — not merely after the PTY process exists. Nothing in the current
TDD or agent protocol defines a "the CLI inside this PTY is ready for
input" signal (no ready-banner contract documented anywhere in
`infra-fleet-service.md` or the agent RPC catalog cited from
`git-gateway-service.md`). This solution uses "`SpawnTerminalSession`
returned successfully" as the readiness signal, which is a conservative
under-approximation, not the spec's literal intent. **Flagged as a known
gap**, not silently resolved: a real fix needs either a documented
per-agent-type ready-output pattern (read `PtyOutput` frames until a match,
capped by a timeout) or an explicit `agent.ready` signal from the Dev
Server Agent side — both out of scope for a backend-go-only change, since
the agent side of that contract doesn't exist today.

### `AgentSpawner`/`PromptInjector` implementation sketch

```go
// internal/adapter/fanout/agent_spawner.go
func (s *grpcAgentSpawner) SpawnAgentTerminal(ctx context.Context, projectID, worktreePath, agentType string) (string, string, error) {
	proj, err := s.projectClient.GetProjectContext(ctx, &projectv1.GetProjectContextRequest{ProjectId: projectID})
	if err != nil {
		return "", "", err
	}
	conn, err := s.infraClient.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: proj.GetDevServerId()})
	if err != nil {
		return "", "", err
	}
	resp, err := s.infraClient.SpawnTerminalSession(ctx, &infrafleetv1.SpawnTerminalSessionRequest{
		ConnectionId: conn.GetConnectionId(),
		Cwd:          worktreePath,
		Shell:        agentLaunchCommand(agentType), // e.g. "claude", "codex" — small lookup table, not part of this saga's logic
	})
	if err != nil {
		return "", "", err
	}
	return resp.GetSession().GetPtyId(), conn.GetConnectionId(), nil
}

// internal/adapter/fanout/prompt_injector.go
func (p *grpcPromptInjector) InjectPrompt(ctx context.Context, connectionID, ptyID, prompt string) error {
	stream, err := p.infraClient.AttachPty(ctx)
	if err != nil {
		return err
	}
	defer stream.CloseSend()
	if err := stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Attach{Attach: &infrafleetv1.AttachToSession{PtyId: ptyID}}}); err != nil {
		return err
	}
	return stream.Send(&infrafleetv1.PtyClientFrame{Frame: &infrafleetv1.PtyClientFrame_Input{Input: &infrafleetv1.PtyInput{Data: []byte(prompt + "\n")}}})
}
```

---

## Design — wiring (`wscompat`)

```go
// channels_worktree.go — new channel
r.Register("worktree.fanOut", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type fanOutArgs struct {
		ProjectID, RepoID, BaseRef, BranchPrefix, Prompt, AgentType string
		N                                                            int
	}
	in, err := decodeArg[fanOutArgs](args, 0)
	if err != nil {
		return nil, err
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	results, err := fanOutUseCase.Execute(ctx, usecase.FanOutCreateWorktreesInput{
		ProjectID: in.ProjectID, RepoID: in.RepoID, BaseRef: in.BaseRef,
		BranchPrefix: in.BranchPrefix, Prompt: in.Prompt, N: in.N, AgentType: in.AgentType,
	})
	if err != nil {
		return nil, err // BR-WT-05 violation surfaces here, before any item runs
	}
	return map[string]any{"items": results}, nil
})
```

`fanOutUseCase` is constructed once in `main.go` alongside the existing
`gitClient`/`projectClient`/`infraClient` dials — no new service connection,
just a new composition root wiring for a usecase that reuses the clients
`registerWorktreeChannels` already receives as parameters plus a new
`infraClient` parameter (not currently threaded into `channels_worktree.go`;
this is the one new plumbing addition).

`[A2]` (N > resources, soft warning) is deliberately **not** enforced
server-side beyond the hard `N ≤ 10` cap — the spec's own language ("cảnh
báo... gợi ý giảm N... người dùng xác nhận") describes a client-side
heuristic warning + confirm, not a backend rule with a numeric threshold.

---

## Test plan

- `usecase/fan_out_create_worktrees_test.go`:
  - `_RejectsN0AndN11_NoCallsMade` (BR-WT-05, boundary)
  - `_AllNShareSameBaseRef` (assert every `WorktreeCreator.CreateWorktree` call receives identical `baseRef` — BR-WT-06 by construction)
  - `_OneItemFails_OthersStillComplete` (fake `WorktreeCreator` fails only for index 2; assert results[0], results[1], results[3..] are all `"ready"` and no context-cancellation error appears — BR-WT-08, the core regression guard against the errgroup mistake)
  - `_PromptInjectedOnlyAfterSpawnSucceeds` (fake `AgentSpawner` records a call, `PromptInjector` fake asserts it's invoked strictly after — BR-WT-07 ordering)
  - `_SpawnFails_PromptInjectorNeverCalled` (assert `PromptInjector` fake records zero calls for that index)
  - `_RetrySingleIndex_ViaRunOne` (calling `runOne` directly for a previously-failed index doesn't touch the others)
- `adapter/fanout/` — contract-style tests against fake `projectv1`/`infrafleetv1` clients for `SpawnAgentTerminal`'s two-hop resolution and `InjectPrompt`'s frame sequence (`Attach` before `Input`).
- `wscompat/channels_worktree_test.go` — `worktree.fanOut` channel test with a fake use case, asserting the wire response shape (`items` array) and that a `FANOUT_N_OUT_OF_RANGE` error surfaces as the channel error, not a 200-with-empty-items.

## References

- `specs/backend-go/bugs/logic-v1/BUG-WT-02-fan-out-not-implemented.md` — full gap list
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166` (dependency graph — `gw` is the only node with edges to all three services this saga needs)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:47-67` (API Gateway responsibilities, "response aggregation", real-time agent-status surfaces)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:114-124` (read-model aggregation-at-the-edge precedent this solution extends to a write case), `:100-112` (synchronous saga pattern)
- `specs/backend-go/tdd/services/project-service.md:42-53` (§2, `GetProjectContext` + "resolve context here, then call the execution-owning service" two-step pattern)
- `specs/backend-go/tdd/services/git-gateway-service.md` — `CreateWorktree`, the per-item building block this saga calls unchanged
- `specs/backend/api/business-capabilities.md:51-52,167` (`project.agentSpawn` → `agent.exec`; terminal-agent-prompt injection reaching the PTY layer)
- `backend-go/services/infra-fleet-service/internal/usecase/spawn_terminal_session.go:1-95` (real `SpawnTerminalSession`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:59-65,297-306,344-362` (`AttachPty`, `SpawnTerminalSessionRequest.shell`, `PtyClientFrame`/`PtyInput`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:196-233` (`worktree.detectedList`'s `errgroup.WithContext` precedent — the pattern this solution deliberately does NOT copy, see "Critical correctness note" above)
- `backend-go/services/api-gateway/internal/usecase/` (existing thin usecase layer this solution extends with its first multi-service saga)
- `docs/logic/worktree-management/BL-WT-02-fan-out-worktree.md`
