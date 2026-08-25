# TASK-166: Tests for `ssh.*` usecases and `wscompat` channels

**From Solution:** SOL-024
**Priority:** P1
**Service:** `infra-fleet-service` + `api-gateway`
**File:** `services/infra-fleet-service/internal/usecase/{list_ssh_targets,get_ssh_state,establish_connection}_test.go` (new), `services/api-gateway/internal/adapter/wscompat/channels_test.go`
**Depends on:** TASK-162, TASK-163, TASK-164, TASK-165
**Status:** `[ ]` TODO

---

## Changes to make

### `internal/usecase/list_ssh_targets_test.go`

Fake `SshTargetRepository`, tenant-scoping assertion (a target from
another tenant never appears in the result):

```go
package usecase_test

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

func TestListSshTargets_ScopesByTenant(t *testing.T) {
	repo := &fakeSshTargetRepository{
		targets: map[string][]domain.SshTarget{
			"t1": {{ID: "s1", TenantID: "t1", Host: "h1"}},
			"t2": {{ID: "s2", TenantID: "t2", Host: "h2"}},
		},
	}
	uc := usecase.NewListSshTargets(repo)

	got, err := uc.Execute(tenant.WithTenantID(context.Background(), "t1"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "s1" {
		t.Errorf("expected only t1's target, got %+v", got)
	}
}
```

### `internal/usecase/get_ssh_state_test.go`

Three cases, table-driven, fake repos, no real network:

```go
func TestGetSshState_ThreeCases(t *testing.T) {
	cases := []struct {
		name          string
		devServerFound bool
		connFound      bool
		wantConnected  bool
	}{
		{name: "no dev server bound", devServerFound: false, wantConnected: false},
		{name: "dev server bound, no active connection", devServerFound: true, connFound: false, wantConnected: false},
		{name: "dev server bound, active connection", devServerFound: true, connFound: true, wantConnected: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			devServers := &fakeDevServerRepository{found: tc.devServerFound}
			conns := &fakeConnectionRepository{found: tc.connFound}
			uc := usecase.NewGetSshState(&fakeSshTargetRepository{}, devServers, conns)

			got, err := uc.Execute(tenant.WithTenantID(context.Background(), "t1"), usecase.SshStateInput{SshTargetID: "s1"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Connected != tc.wantConnected {
				t.Errorf("got Connected=%v, want %v", got.Connected, tc.wantConnected)
			}
		})
	}
}
```

### `internal/usecase/establish_connection_test.go`

Fake `DevServerAgentClient.Health` returning `true`/`false`/error,
asserting `Status: "established"` only on the `true` path and
`INFRA_SSH_CONNECT_FAILED` on the other two; assert
`devServers.Register` is called with a `relay-ssh`-mode `DevServer` when
no existing binding is found:

```go
func TestEstablishConnection_HealthGatesResult(t *testing.T) {
	t.Run("healthy agent establishes connection", func(t *testing.T) {
		sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
		devServers := &fakeDevServerRepository{found: false}
		conns := &fakeConnectionRepository{}
		agent := &fakeDevServerAgentClient{healthy: true}
		uc := usecase.NewEstablishConnection(sshTargets, devServers, conns, agent)

		conn, err := uc.Execute(tenant.WithTenantID(context.Background(), "t1"), usecase.EstablishConnectionInput{SshTargetID: "s1"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if conn.Status != "established" {
			t.Errorf("got status %q, want established", conn.Status)
		}
		if !devServers.registerCalled || devServers.lastRegistered.Mode != domain.ConnectionModeRelaySSH {
			t.Error("expected a relay-ssh-mode DevServer to be registered")
		}
	})

	t.Run("unreachable agent fails", func(t *testing.T) {
		sshTargets := &fakeSshTargetRepository{single: domain.SshTarget{ID: "s1", TenantID: "t1", Host: "h1"}}
		devServers := &fakeDevServerRepository{found: false}
		conns := &fakeConnectionRepository{}
		agent := &fakeDevServerAgentClient{healthy: false}
		uc := usecase.NewEstablishConnection(sshTargets, devServers, conns, agent)

		_, err := uc.Execute(tenant.WithTenantID(context.Background(), "t1"), usecase.EstablishConnectionInput{SshTargetID: "s1"})
		if err == nil {
			t.Fatal("expected error when agent is unreachable")
		}
	})
}
```

Add the necessary fields (`registerCalled`, `lastRegistered`, `single`,
`healthy`) to whatever fakes already exist in this package's test files —
extend, don't duplicate.

### `services/api-gateway/internal/adapter/wscompat/channels_test.go`

One test per new channel (`ssh.listTargets`, `ssh.getUserAccount`,
`ssh.getState`, `ssh.connect`), fake `InfraFleetServiceClient`.
`ssh.getUserAccount` specifically asserts it calls `ListSshTargets` (not
a second, dedicated RPC) and filters client-side by `sshTargetId`.

### Contract test (optional but recommended)

`POST /v1/infra/ssh-targets` (existing REST create) then `ssh.listTargets`
over WS returns the same target — round-trip guard against the REST and
WS surfaces drifting. Skip if no existing test harness spins up both
transports together; note as a follow-up if so.

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/infra-fleet-service/internal/usecase/... -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestRegisterSshChannels -v
```
