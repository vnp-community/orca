# TASK-WF-02-02: Add `Target`/`ProviderPin` to step configs and resolver ports

**From Solution:** SOL-WF-02
**Priority:** P0
**Service:** `workflow-service`
**File:** `backend-go/services/workflow-service/internal/domain/step.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`AgentStepConfig`/`ShellStepConfig`/`NotificationStepConfig` only carry a
raw `ConnectionID` passthrough today — no server-resolution or
provider-pin concept exists. This task adds the `Target` string shape
(four accepted prefixes) and `ProviderPin`, plus the `ServerResolver`/
`ProviderResolver`/`EventPublisher` usecase ports that consume them.
`ConnectionID` is kept as a deprecated back-compat alias.

## Changes to make

In `backend-go/services/workflow-service/internal/domain/step.go`:

```go
// Target is a dispatch-target string in one of four shapes — the
// orchestrator resolves it to a concrete connectionId before relaying:
//   "connection:<id>"   — direct passthrough, today's ConnectionID shape (back-compat)
//   "project:<id>"      — resolve via project-service.GetProject().dev_server_id, then infra-fleet-service.ResolveConnection
//   "server:<id>"       — resolve via infra-fleet-service.ResolveConnection(dev_server_id=<id>) directly
//   "fleet:tag:<tag>"   — load-balance across infra-fleet-service's healthy dev servers carrying <tag>
// ConnectionID is a deprecated alias: when Target is empty and
// ConnectionID is set, it's treated as "connection:<ConnectionID>".
type AgentStepConfig struct {
    Target       string `json:"target,omitempty"`
    ConnectionID string `json:"connectionId,omitempty"` // deprecated, see Target's doc comment
    Prompt       string `json:"prompt"`
    WorktreePath string `json:"worktreePath,omitempty"`
    TrustPreset  string `json:"trustPreset,omitempty"`
    // Provider pins a specific ai-provider-service account, bypassing the
    // priority cascade — workflow-service.md §7: an explicit
    // step.config.provider.accountId pin (validated active) beats
    // ai-provider-service's priority-chain resolution.
    Provider *ProviderPin `json:"provider,omitempty"`
    Model    string       `json:"model,omitempty"` // pass-through param, not resolved server-side
}

type ProviderPin struct {
    AccountID string `json:"accountId"`
}

func (c AgentStepConfig) effectiveTarget() string {
    if c.Target != "" {
        return c.Target
    }
    if c.ConnectionID != "" {
        return "connection:" + c.ConnectionID
    }
    return ""
}
```

Give `ShellStepConfig`/`NotificationStepConfig` the identical
`Target`/`ConnectionID`/`effectiveTarget()` pair (no `Provider`/`Model` —
those are agent-specific).

In `internal/usecase/ports.go`, add:

```go
// ServerResolver turns a step's Target string into a connectionId ready
// for infra-fleet-service.Relay — see domain.AgentStepConfig.Target's doc
// comment for the four accepted shapes. An empty connectionId result
// means "execute locally," unchanged from today.
type ServerResolver interface {
    Resolve(ctx context.Context, tenantID, target string) (connectionID string, err error)
}

// ProviderResolver resolves which ai-provider-service account an agent
// step should use — workflow-service.md §7's priority note.
type ProviderResolver interface {
    Resolve(ctx context.Context, tenantID, userID, projectID string, pin *domain.ProviderPin) (accountID string, err error)
}

// EventPublisher fans a step/execution lifecycle event out to live
// StreamExecutionEvents subscribers.
type EventPublisher interface {
    Publish(ctx context.Context, event domain.ExecutionEvent) error
}
```

Add `domain.ExecutionEvent` (mirrors the proto `ExecutionEvent` shape
added in TASK-WF-02-01) to `step.go` or a new small file alongside it.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/...
go test ./services/workflow-service/internal/domain/... -run TestAgentStepConfig
```

Expected: build clean; a table test confirms `effectiveTarget()` returns
`Target` when set, falls back to `"connection:"+ConnectionID`, and
returns `""` when neither is set.
