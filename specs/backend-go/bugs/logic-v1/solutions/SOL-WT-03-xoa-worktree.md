# SOL-WT-03: Pre-delete safety checks + agent-kill step for worktree removal

**Resolves:** [BUG-WT-03](../BUG-WT-03-xoa-worktree-partial.md)
**Service:** `git-gateway-service` (new RPC + `RemoveWorktree` changes) — calls `infra-fleet-service` over the already-existing dependency edge
**Affected files (proposed):**
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — new `CheckWorktreeDeleteSafety` RPC; `RemoveWorktreeRequest` gains `stop_agents`; `RemoveWorktree` returns a real message instead of `Empty`
- `backend-go/services/git-gateway-service/internal/usecase/check_worktree_delete_safety.go` (new)
- `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go` — safety-check + kill-agent steps
- `backend-go/services/git-gateway-service/internal/usecase/ports.go` — `TerminalSessionLister` (new port, infra-fleet-service client)
- `backend-go/services/git-gateway-service/internal/adapter/infraclient/` (new/extended) — implements `TerminalSessionLister` against `infrafleetv1.InfraFleetServiceClient`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go` — new `worktree.checkDeleteSafety` channel, `worktree.rm` passes `stopAgents`
- `backend-go/services/git-gateway-service/internal/usecase/remove_worktree_test.go`, `check_worktree_delete_safety_test.go`
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

`git-gateway-service.md` §7 already establishes the dependency edge this
solution needs: "**Calls `infra-fleet-service`** — resolves the owning
host's `connectionId`... and (when present) performs the actual relay
dispatch" (`:246-251`). Today that edge is used only for `ConnectionResolver`
and relay dispatch. This solution adds one more call on the *same* edge
(`infra-fleet-service.ListTerminalSessions`, already a real RPC per
`infra-fleet-service.md:132`) rather than opening a new dependency —
consistent with `02-microservices-decomposition.md`'s dependency graph,
which lists `git --> infra` once, not once per RPC it happens to call.

The 4 spec-listed safety checks split cleanly along what already exists vs.
what doesn't:

- **Uncommitted/untracked changes** (spec step 2a/2b): fully answerable
  today via `GitExecutor.GetStatus`, already required on every executor
  (`ports.go:62`) and already returning per-file `state` including
  `"untracked"` (`gitgateway.proto`'s `FileStatus`, `:130-133`). No new
  capability — just a usecase that calls it *before* attempting removal
  instead of relying on git's own dirty-worktree refusal (today's only
  enforcement, and one `force=true` fully bypasses, per the bug's own
  finding).
- **Agent-running check** (spec step 2c): the closest primitive is
  `infra-fleet-service.ListTerminalSessions(connection_id)`
  (`infra-fleet-service.md:132`, real, returns `TerminalSession{pty_id,
  connection_id, cwd, ...}`). This system models "an agent" as a CLI
  process running inside a PTY (§ the same framing
  [SOL-WT-02](./SOL-WT-02-fan-out-worktree.md) uses for spawning one) — so
  "is an agent running in this worktree" is answerable by filtering
  `ListTerminalSessions`' response to sessions whose `cwd` is the worktree's
  path (or a subpath of it). **Flagged limitation**: this is a heuristic,
  not a precise "agent" identity — a session could be an ordinary shell, not
  an AI-CLI process. No backend-go concept of "this specific PTY is running
  an agent, not a shell" exists anywhere in the reviewed TDD docs; a precise
  fix needs `SpawnTerminalSession`-time tagging (e.g. an `is_agent` flag
  threaded through from [SOL-WT-02](./SOL-WT-02-fan-out-worktree.md)'s
  `SpawnAgentTerminal`), which this solution does not add — out of scope,
  documented as a known imprecision rather than silently claimed as solved.
- **Directory-lock/other-process check** ([A3]): **no backend primitive
  exists anywhere in this system for this.** It would require a host-side
  `lsof`/`fuser`-equivalent scan, which is not in the Dev Server Agent's
  `fs.*` surface (per `BUG-009`'s own finding, cited in SOL-009: the agent
  implements `stat/readDir/readFile/writeFile/mkdir/rmdir/glob/grep` only).
  **This is a genuine architecture gap, not an implementation task** — per
  this task's own instructions, stated plainly rather than forcing a fake
  solution: closing it needs a new Dev Server Agent capability (a process
  scan RPC) before any Go-side usecase can call it. Out of scope for this
  fix; recommend filing as a separate agent-side capability request if
  product still wants it.

`RemoveWorktreeResponse` today is `google.protobuf.Empty`
(`gitgateway.proto`'s `RemoveWorktree` RPC), which is why the bug correctly
notes there's no data shape for a client to build [A1]/[A2]'s recovery
dialogs from. Rather than stuff structured detail into the removal call
itself, this solution follows the spec's own flow shape (safety checks run
*before* the confirm dialog is shown, at spec step 2 — separate from the
actual delete at step 5): a dedicated `CheckWorktreeDeleteSafety` read RPC,
called by the client before rendering the dialog, mirroring how
`worktree.detectedList` is already a separate read call from `worktree.rm`.

---

## Design — proto additions

```protobuf
message CheckWorktreeDeleteSafetyRequest { string worktree_id = 1; }
message CheckWorktreeDeleteSafetyResponse {
  int32 uncommitted_files = 1;   // modified + added + deleted + conflicted, per FileStatus.state
  int32 untracked_files = 2;
  bool agent_running = 3;        // heuristic — see "Design rationale" limitation above
  repeated string active_pty_ids = 4;
  bool safe_to_delete = 5;       // true iff all counts are zero and !agent_running
}

message RemoveWorktreeRequest {
  string worktree_id = 1;
  bool   force = 2;        // unchanged: maps to `git worktree remove --force`
  bool   stop_agents = 3;  // NEW — spec's "Stop & Delete" choice: kill active PTY
                            // sessions found in this worktree before removing it
}
message RemoveWorktreeResponse {  // NEW — replaces google.protobuf.Empty
  int32 uncommitted_files_discarded = 1; // echoes the safety-check counts that were overridden by force, for the UI's post-delete confirmation toast
  repeated string stopped_pty_ids = 2;
}
```

`RemoveWorktree` RPC signature changes from `returns
(google.protobuf.Empty)` to `returns (RemoveWorktreeResponse)` — a breaking
wire change, but `RemoveWorktree` currently has exactly one caller
(`wscompat`'s `worktree.rm`, `channels_worktree.go:76-90`), so the blast
radius is one call site, updated in the same change.

---

## Design — usecase

```go
// internal/usecase/check_worktree_delete_safety.go
type CheckWorktreeDeleteSafety struct {
	resolver  ConnectionResolver
	local     GitExecutor
	relay     GitExecutor
	terminals TerminalSessionLister
}

func (uc *CheckWorktreeDeleteSafety) Execute(ctx context.Context, worktreeID string) (domain.DeleteSafetyReport, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, worktreeID)
	if err != nil {
		return domain.DeleteSafetyReport{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}
	status, err := executor.GetStatus(ctx, repoPath)
	if err != nil {
		return domain.DeleteSafetyReport{}, apperrors.New(apperrors.KindInternal, "WORKTREE_STATUS_FAILED", "failed to check worktree status", err)
	}

	report := domain.DeleteSafetyReport{}
	for _, f := range status.Files {
		if f.State == "untracked" {
			report.UntrackedFiles++
		} else {
			report.UncommittedFiles++ // modified/added/deleted/conflicted
		}
	}

	conn, err := uc.resolver.ResolveConnection(ctx, worktreeID) // reuse the same resolution CreateWorktree/RemoveWorktree already do
	if err == nil && conn.Connected {
		sessions, listErr := uc.terminals.ListSessions(ctx, conn.ConnectionID)
		if listErr == nil {
			for _, s := range sessions {
				if strings.HasPrefix(s.Cwd, repoPath) {
					report.ActivePtyIDs = append(report.ActivePtyIDs, s.PtyID)
				}
			}
		}
	}
	report.AgentRunning = len(report.ActivePtyIDs) > 0
	report.SafeToDelete = report.UncommittedFiles == 0 && report.UntrackedFiles == 0 && !report.AgentRunning
	return report, nil
}
```

```go
// internal/usecase/remove_worktree.go — extended
func (uc *RemoveWorktree) Execute(ctx context.Context, in RemoveWorktreeInput) (domain.RemoveWorktreeResult, error) {
	executor, repoPath, err := dispatchExecutor(ctx, uc.resolver, uc.local, uc.relay, in.WorktreeID)
	if err != nil {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_RESOLVE_FAILED", "failed to resolve host", err)
	}

	// BR-WT-09/BR-WT-10 — re-run the same checks CheckWorktreeDeleteSafety
	// exposes, as a server-side guard against a client that skips the
	// pre-check call (or races a change between check and confirm).
	status, err := executor.GetStatus(ctx, repoPath)
	if err == nil {
		dirty := 0
		for _, f := range status.Files {
			dirty++
		}
		if dirty > 0 && !in.Force {
			return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_HAS_UNCOMMITTED_CHANGES",
				fmt.Sprintf("%d files uncommitted", dirty), nil)
		}
	}

	var stoppedPtyIDs []string
	if conn, cErr := uc.resolver.ResolveConnection(ctx, in.WorktreeID); cErr == nil && conn.Connected {
		if sessions, lErr := uc.terminals.ListSessions(ctx, conn.ConnectionID); lErr == nil {
			var active []string
			for _, s := range sessions {
				if strings.HasPrefix(s.Cwd, repoPath) {
					active = append(active, s.PtyID)
				}
			}
			if len(active) > 0 && !in.StopAgents {
				return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindFailedPrecondition, "WORKTREE_AGENT_RUNNING",
					fmt.Sprintf("%d active session(s) in this worktree", len(active)), nil)
			}
			for _, ptyID := range active {
				if err := uc.terminals.Kill(ctx, ptyID); err != nil {
					// Best-effort — a kill failure should not block the
					// delete the user explicitly confirmed; the orphaned
					// PTY self-heals when its process exits against a
					// now-removed cwd, same "safe terminal state" posture
					// this package's doc comment already applies to
					// bookkeeping staleness.
					continue
				}
				stoppedPtyIDs = append(stoppedPtyIDs, ptyID)
			}
		}
	}

	if err := executor.RemoveWorktree(ctx, repoPath, in.Force); err != nil {
		return domain.RemoveWorktreeResult{}, apperrors.New(apperrors.KindInternal, "WORKTREE_REMOVE_FAILED", "git worktree remove failed", err)
	}
	if err := uc.projects.RecordWorktreeRemoved(ctx, in.WorktreeID); err != nil {
		return domain.RemoveWorktreeResult{StoppedPtyIDs: stoppedPtyIDs}, apperrors.New(apperrors.KindInternal, "WORKTREE_BOOKKEEPING_STALE", "worktree removed but bookkeeping update failed; will self-heal via worktree.detectedList", err)
	}
	return domain.RemoveWorktreeResult{StoppedPtyIDs: stoppedPtyIDs}, nil
}
```

`TerminalSessionLister` (new port, `ports.go`):

```go
type TerminalSessionLister interface {
	ListSessions(ctx context.Context, connectionID string) ([]domain.TerminalSessionRef, error)
	Kill(ctx context.Context, ptyID string) error
}
```

Implemented against `infrafleetv1.InfraFleetServiceClient.ListTerminalSessions`/
`KillTerminalSession` — both already real RPCs
(`infra-fleet-service.md:131-132`).

---

## Design — wiring (`wscompat`)

```go
r.Register("worktree.checkDeleteSafety", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type checkArgs struct{ WorktreeID string `json:"worktreeId"` }
	in, err := decodeArg[checkArgs](args, 0)
	if err != nil {
		return nil, err
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	resp, err := gitClient.CheckWorktreeDeleteSafety(ctx, &gitgatewayv1.CheckWorktreeDeleteSafetyRequest{WorktreeId: in.WorktreeID})
	if err != nil {
		return nil, err
	}
	return resp, nil
})

// worktree.rm — extended to pass the new field through
r.Register("worktree.rm", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
	type rmArgs struct {
		WorktreeID string `json:"worktreeId"`
		Force      bool   `json:"force"`
		StopAgents bool   `json:"stopAgents"` // NEW
	}
	// ...unchanged decode/identity...
	resp, err := gitClient.RemoveWorktree(ctx, &gitgatewayv1.RemoveWorktreeRequest{
		WorktreeId: in.WorktreeID, Force: in.Force, StopAgents: in.StopAgents,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil // was map[string]bool{"ok": true} — now the real response
})
```

---

## Test plan

- `usecase/check_worktree_delete_safety_test.go`:
  - `_CountsUncommittedAndUntrackedSeparately` (fake `GetStatus` with mixed `FileStatus.state`)
  - `_NoActiveConnection_AgentRunningFalse_NoTerminalCall`
  - `_ActiveSessionInWorktree_ReportsPtyID` (fake `ListSessions` with a matching-`cwd` and a non-matching-`cwd` session — only the matching one is reported)
  - `_SafeToDelete_TrueOnlyWhenAllCountsZero`
- `usecase/remove_worktree_test.go`:
  - `_UncommittedChanges_ForceFalse_RejectsBeforeGitCall` (assert `executor.RemoveWorktree` never called — regression guard against the current bypass)
  - `_UncommittedChanges_ForceTrue_ProceedsToGitCall`
  - `_AgentRunning_StopAgentsFalse_RejectsBeforeGitCall`
  - `_AgentRunning_StopAgentsTrue_KillsSessionsThenRemoves` (assert `Kill` called for each session, `RemoveWorktree` called after)
  - `_KillFails_StillProceedsWithRemoval_BestEffort`
  - Existing happy-path/bookkeeping-stale tests updated for the new response shape.
- `adapter/infraclient/` — contract test against a fake `infrafleetv1.InfraFleetServiceClient` for `ListSessions`/`Kill`.
- `wscompat/channels_worktree_test.go` — `worktree.checkDeleteSafety` channel test; `worktree.rm` test asserting `stopAgents` threads through to the gRPC request.

## References

- `specs/backend-go/bugs/logic-v1/BUG-WT-03-xoa-worktree-partial.md` — full gap list
- `specs/backend-go/tdd/services/git-gateway-service.md:243-256` (§7, existing `git --> infra` dependency edge this solution reuses)
- `specs/backend-go/tdd/architecture/02-microservices-decomposition.md:110-166` (dependency graph — no new edge required)
- `specs/backend-go/tdd/services/infra-fleet-service.md:124,131-132` (`SpawnTerminalSession`, `ListTerminalSessions`, `KillTerminalSession` RPC list)
- `backend-go/services/git-gateway-service/internal/usecase/remove_worktree.go:1-45` (current saga, no safety checks)
- `backend-go/services/git-gateway-service/internal/usecase/ports.go:61-62` (`GitExecutor.GetStatus`, already required)
- `backend-go/proto/orca/gitgateway/v1/gitgateway.proto` — `FileStatus`/`GetStatusResponse`, current `RemoveWorktreeRequest`/`RemoveWorktree` (`returns (google.protobuf.Empty)`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto:307-319` (`TerminalSession`, `ListTerminalSessionsRequest/Response`)
- `specs/backend-go/bugs/missing-v1/solutions/SOL-009-files-channels.md` (cited for the Dev Server Agent `fs.*` method-set finding this solution reuses to scope [A3] out)
- `docs/logic/worktree-management/BL-WT-03-xoa-worktree.md`
