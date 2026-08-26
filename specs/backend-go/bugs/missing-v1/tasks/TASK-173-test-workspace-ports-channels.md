# TASK-173: Tests for `workspacePorts.scan`/`workspacePorts.kill`

**From Solution:** SOL-027
**Priority:** P1
**Service:** `infra-fleet-service` + `api-gateway`
**File:** `services/infra-fleet-service/internal/usecase/kill_workspace_port_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-169, TASK-170, TASK-171, TASK-172
**Status:** `[ ]` TODO

---

## Changes to make

### New file `services/infra-fleet-service/internal/usecase/kill_workspace_port_test.go`

Mirror `scan_workspace_ports_test.go`'s exact table shape:

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

func TestKillWorkspacePort_ResolveThenDispatch(t *testing.T) {
	t.Run("no connectionId: honest not-implemented, no agent call", func(t *testing.T) {
		resolver := &fakeConnectionResolver{}
		agent := &fakeDevServerAgentClient{}
		uc := usecase.NewKillWorkspacePort(resolver, agent)

		ok, reason, err := uc.Execute(tenant.WithTenantID(context.Background(), "t1"), usecase.KillWorkspacePortInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || reason == "" {
			t.Errorf("expected ok=false with a reason, got ok=%v reason=%q", ok, reason)
		}
		if agent.execCalled {
			t.Error("expected no agent call when connectionId is empty")
		}
	})

	t.Run("connectionId resolves but not connected: same as no connectionId, no agent call", func(t *testing.T) {
		resolver := &fakeConnectionResolver{connected: false}
		agent := &fakeDevServerAgentClient{}
		uc := usecase.NewKillWorkspacePort(resolver, agent)

		ok, _, err := uc.Execute(tenant.WithTenantID(context.Background(), "t1"), usecase.KillWorkspacePortInput{ConnectionID: "c1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Error("expected ok=false when not connected")
		}
		if agent.execCalled {
			t.Error("expected no agent call when not connected")
		}
	})

	t.Run("connectionId resolves and connected: relays to agent ports.kill", func(t *testing.T) {
		resolver := &fakeConnectionResolver{connected: true}
		agent := &fakeDevServerAgentClient{execResult: map[string]any{"ok": true}}
		uc := usecase.NewKillWorkspacePort(resolver, agent)

		ok, _, err := uc.Execute(tenant.WithTenantID(context.Background(), "t1"), usecase.KillWorkspacePortInput{
			ConnectionID: "c1", WorktreeID: "wt1", PID: 123, Port: 8080,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Error("expected ok=true")
		}
		if agent.lastMethod != "ports.kill" {
			t.Errorf("expected agent method ports.kill, got %q", agent.lastMethod)
		}
	})

	t.Run("agent exec error is propagated, never swallowed into ok:false", func(t *testing.T) {
		resolver := &fakeConnectionResolver{connected: true}
		agent := &fakeDevServerAgentClient{execErr: errFakeAgentExec}
		uc := usecase.NewKillWorkspacePort(resolver, agent)

		_, _, err := uc.Execute(tenant.WithTenantID(context.Background(), "t1"), usecase.KillWorkspacePortInput{ConnectionID: "c1"})
		if err == nil {
			t.Fatal("expected error to propagate, not be swallowed into ok:false")
		}
	})
}
```

Extend whatever `fakeConnectionResolver`/`fakeDevServerAgentClient`
already exist in this package's `scan_workspace_ports_test.go` (add
`execCalled`/`lastMethod`/`execResult`/`execErr` tracking fields there if
missing) rather than declaring new fakes. Declare a package-level
`errFakeAgentExec = errors.New("fake agent exec failure")` if this
package doesn't already have an equivalent sentinel.

### `services/api-gateway/internal/adapter/wscompat/channels_test.go`

One test per channel:

- `workspacePorts.scan` calls `ScanWorkspacePorts` on the fake
  `InfraFleetServiceClient` with the decoded args and reshapes the
  response via `toWorkspacePortScanResult`.
- `workspacePorts.kill` calls `KillWorkspacePort` and passes through
  `{ok, reason}` verbatim on failure, `{ok:true}` on success.

### Regression guard

A test asserting `workspacePorts.scan`/`workspacePorts.kill` are both
present in the registry (no `notImplementedHandler` fallthrough) — same
style as BUG-002's sibling reports.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/infra-fleet-service/internal/usecase/... -run TestKillWorkspacePort -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestRegisterWorkspacePortsChannels -v
```
