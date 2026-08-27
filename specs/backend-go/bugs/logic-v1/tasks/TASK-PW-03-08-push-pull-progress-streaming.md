# TASK-PW-03-08: Push/pull progress streaming — `PushStream`/`PullStream` (needs `infra-fleet-service` coordination)

**From Solution:** SOL-PW-03
**Priority:** P2 — genuine scope addition beyond `git-gateway-service`'s own boundary; coordinate with `infra-fleet-service`'s owner before starting
**Service:** `git-gateway-service`, `infra-fleet-service`, `api-gateway`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/relay.go`, `backend-go/services/git-gateway-service/internal/usecase/push_stream.go`, `backend-go/services/git-gateway-service/internal/usecase/pull_stream.go`
**Depends on:** TASK-PW-03-01, TASK-PW-03-02, TASK-PW-03-03
**Status:** [x] DONE — infra-fleet-service RelayStream RPC (proto+session.go multi-frame demux+ExecStream) + git-gateway-service StreamingGitExecutor (localgit os/exec pipe, grpcclient relay, PushStream/PullStream usecases+grpc handlers) + api-gateway git.push.progress/git.pull.progress wscompat channels, all building and tested; relay-ssh short-circuits before any executor call (verified by test).

---

## Context

The agent's `git.execStream` (Part A only) already streams `stdout`/
`stderr` line-by-line for `push`/`pull`/`fetch`, but
`infra-fleet-service`'s `Relay` RPC (`internal/usecase/relay.go`) is unary
request/response only — `uc.agent.Exec(ctx, devServer, method, params)`
returns one `map[string]any`, not a stream. **Before starting this task,
verify `relay.go`'s current signature still matches this description** —
confirm it has not already grown a streaming variant since SOL-PW-03 was
written.

This task genuinely spans three services. Split it into three PRs/commits
in this order, not one:

1. **`infra-fleet-service`**: add a new server-streaming `RelayStream` RPC
   (mirrors `Relay`'s shape but returns `stream map[string]any` or a typed
   frame message) that relays to the agent's `git.execStream`. This is new
   scope beyond `infra-fleet-service`'s current RPC surface — coordinate
   with that service's owner; do not add it silently as a side effect of a
   `git-gateway-service` PR.
2. **`git-gateway-service`**: `internal/usecase/push_stream.go`/
   `pull_stream.go` + a new `StreamingGitExecutor` port (`localgit`: pipes
   `os/exec`'s Stdout/Stderr line-by-line; `grpcclient`: relays to
   `RelayStream` from step 1) + `adapter/grpc`'s `PushStream`/`PullStream`
   server methods forwarding via `stream.Send`.
3. **`api-gateway`**: wscompat needs a subscription-style channel for
   this — verify against `registerWorkflowChannels`'s existing streaming
   pattern (if `workflow.execute`'s `StreamExecutionEvents` has a wscompat
   equivalent) before inventing a new mechanism. Forward `GitProgressEvent`
   frames as incremental WS frames under a new event name (e.g.
   `git.push.progress`), terminated by a final frame carrying the
   unary-equivalent `{success, hadConflicts}` once `GitProgressEvent.is_final`
   is seen.

## Changes to make

```go
// internal/usecase/push_stream.go
type PushStream struct {
	resolver ConnectionResolver
	local    StreamingGitExecutor
	relay    StreamingGitExecutor
}

func (uc *PushStream) Execute(ctx context.Context, in PushInput, sink func(domain.GitProgressLine) error) error {
	conn, err := uc.resolver.ResolveConnection(ctx, in.WorktreeID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "GITGATEWAY_RESOLVE_FAILED", "failed to resolve worktree's owning host", err)
	}
	if conn.Connected && conn.Mode == infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH {
		return apperrors.New(apperrors.KindFailedPrecondition, "GITGATEWAY_PUSH_STREAM_UNSUPPORTED_SSH_RELAY", "push progress streaming is not supported over an SSH-relay connection; retry against the unary Push RPC", domain.ErrGitOpUnsupportedOverSSHRelay)
	}
	executor := uc.local
	if conn.Connected {
		executor = uc.relay
	}
	return executor.PushStream(ctx, conn.RepoPath, in.Remote, in.Branch, sink)
}
```

`PullStream` follows the identical shape against `Pull`'s inputs.

Over a `relay-ssh` connection, push/pull progress streaming remains
unsupported after this task (Part B has no `execStream` equivalent) — the
unary `Push`/`Pull` RPCs remain the only path for that connection mode.
This is a documented degradation, not a silent capability loss; the
frontend is expected to retry against the unary RPC on
`ErrGitOpUnsupportedOverSSHRelay`.

## Test plan

`push_stream_test.go`/`pull_stream_test.go` — fake `StreamingGitExecutor`
emitting a sequence of lines then a final frame; assert the usecase
forwards each line to `sink` in order and returns after the final frame;
`relay-ssh` mode returns the typed error before ever constructing a
`StreamingGitExecutor` call.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... ./services/git-gateway-service/... ./services/api-gateway/...
go test ./services/git-gateway-service/internal/usecase/... -run 'TestPushStream|TestPullStream' -v
```

Expected: clean build across all three services; streaming usecase tests
pass; relay-ssh mode short-circuits before any `StreamingGitExecutor`
call (assert fake records zero calls, matching TASK-PW-03-06's pattern).
