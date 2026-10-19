# agentrelay

One shared client for the two dev-server agent RPCs that run the AI CLI, relayed through infra-fleet-service:

- `agent.execPrompt` (`Relayer.ExecPrompt`): an agent run in a workspace.
- `ai.complete` (`Relayer.Complete`): a text completion. It never carries an API key.

```go
relay := agentrelay.New(infraClient, agentrelay.WithMetadata(withTenantMetadata))

params := agentrelay.ReadOnly(agentrelay.Run{StepID: id, Prompt: p, WorktreePath: repo, Env: env, TimeoutMS: 600_000})
res, err := relay.ExecPrompt(ctx, agentrelay.Target{DevServerID: ds, ConnectionID: conn}, params)
switch {
case errors.Is(err, agentrelay.ErrReadonlyUnsupported): // agent would have run in write mode
case errors.Is(err, agentrelay.ErrKnowledgeHubUnavailable): // retry without knowledgeRoot
case errors.Is(err, agentrelay.ErrTimeout): // context.DeadlineExceeded
}
```

## Builders

| Builder | Sends | Use |
| --- | --- | --- |
| `Plain(Run)` | caller's `TrustPreset`, no access options | workflow step, orchestration dispatch, task without a result contract |
| `ReadOnly(Run)` | `accessMode=readonly`, `repo_root`, no trust preset | analysis that must not change the repo |
| `WriteProfile(Run, profile)` | `accessMode=write`, `worktree`, `permissionProfile`, trust `default` | spec authoring in a worktree |
| `Contract(Run)` | result block, `reportChanges`, 1 MiB output, 25 min timeout | task runs that report back |

`Target`: `DevServerID` set means `RelayByDevServer`, otherwise `Relay` by `ConnectionID`.

## Options

`WithMetadata(fn)` attaches tenant/user to the outgoing context (each service passes its own). `WithTimeoutSlack(d)`
(default 30 s) is added to `TimeoutMS` for the RPC deadline so the agent reports its own timeout first;
`WithoutCallDeadline()` leaves the deadline to the caller's context.

## Rules this package keeps

- JSON names and `omitempty` match the agent and the structs it replaced; `testdata/wire/*.json` freezes the output of
  each flow and `TestWireEquivalence` compares the old struct, the new builders and the golden file.
  Refresh only on purpose: `go test ./agentrelay -run Wire -update`.
- No credential field in `CompleteParams`; a test fails if one appears.
- Output notifications (`agent.execPrompt.output`) are forwarded by infra-fleet-service, not handled here.
- Prompt and result text are never logged or put in error messages.
