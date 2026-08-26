# TASK-028: Implement `TestConnection` usecase (relay via infra-fleet-service)

**From Solution:** SOL-005 (Group D)
**Priority:** P1
**Service:** `ai-provider-service`
**File:** `internal/usecase/{ports.go,test_connection.go}` (new usecase), `internal/adapter/grpcclient/infrafleet_client.go` (new), `internal/adapter/grpc/server.go`, `internal/config/config.go`, `cmd/server/main.go`
**Depends on:** TASK-024, TASK-025, TASK-026
**Status:** `[x]` DONE — `test_connection.go` confirmed present and wired.

---

## Context

`credential-broker-service.md` §3 states plainly that for
`AI_PROVIDER_KEY`, `ResolveCredential` "Returns Metadata only... never
plaintext... Execution plane (Dev Server Agent) decrypts locally." So
`ai-provider-service` cannot get a usable key back from
`ResolveCredential` — `TestConnection` must relay to whichever dev server
already holds this account's pushed ciphertext, via `infra-fleet-service`'s
existing generic `Relay` RPC, exactly the way SOL-004 does for
`accounts.*`. `ai-provider-service` becomes a plain gRPC client of
`infra-fleet-service` — it never gains its own Dev Server Agent adapter
(per `ai-provider-service.md` §6, "no `adapter/vault/` package at all" —
same reasoning extends to not duplicating agent wire-protocol code here).

**Flagged, not a blocker for this task:** the relayed agent method
`ai.testProviderConnection` does not exist on the Dev Server Agent's
JSON-RPC dispatcher yet. Per `08-inter-service-communication.md`, `agent/`
changes are out of scope for `backend-go`. This task's plumbing is
correct and buildable on its own merits; it is inert (returns a "method
not found" error from the agent) until that companion `agent/` work ships.
No separate blocked/doc-only task is created for this — SOL-005 doesn't
flag it as a large ask the way SOL-006 flags browser-driving (TASK-036);
tracking it is folded into this task's Context section instead.

---

## Changes to make

### Step 1 — `usecase/ports.go`: new `InfraFleetClient` port

```go
// ConnectionTestResult is TestConnection's usecase-level result — mirrors
// TestConnectionResponse{success, message}. No field here can ever hold
// key material — the agent decrypts locally and returns only a
// success/failure verdict.
type ConnectionTestResult struct {
	Success bool
	Message string
}

// InfraFleetClient is the narrow port TestConnection uses to reach
// infra-fleet-service's Relay RPC — NOT to be confused with
// infra-fleet-service's own Relay RPC on the other side of this call.
// Implemented by internal/adapter/grpcclient against a real
// infra-fleet-service gRPC connection. ai-provider-service never gains its
// own Dev Server Agent adapter (§6) — this is the one indirect path to the
// execution plane, entirely mediated by infra-fleet-service.
type InfraFleetClient interface {
	// Relay resolves devServerID to its current active connectionId (via
	// infra-fleet-service's ResolveConnection with dev_server_id set — see
	// specs/backend-go/bugs/missing-v1/tasks/TASK-025-add-infrafleet-connection-resolve-by-devserver-worktree.md)
	// then calls infra-fleet-service's Relay RPC with method/params.
	// Returns the agent's decoded JSON-RPC result.
	Relay(ctx context.Context, devServerID, method string, params map[string]any) (map[string]any, error)
}
```

### Step 2 — `usecase/test_connection.go` (new)

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

type TestConnectionInput struct {
	AccountID string
}

// TestConnection relays a live, lightweight provider API call to whichever
// dev server holds this account's pushed ciphertext — see this file's
// package-level context in TASK-028 for why ResolveCredential cannot be
// used here. The plaintext key never crosses into this service's memory at
// any point.
type TestConnection struct {
	repo  ProviderAccountRepository
	infra InfraFleetClient
}

func NewTestConnection(repo ProviderAccountRepository, infra InfraFleetClient) *TestConnection {
	return &TestConnection{repo: repo, infra: infra}
}

func (uc *TestConnection) Execute(ctx context.Context, in TestConnectionInput) (ConnectionTestResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindUnauthenticated, "AIPROVIDER_NO_TENANT", "no tenant in request context", err)
	}
	account, err := uc.repo.Get(ctx, tenantID, in.AccountID)
	if err != nil {
		return ConnectionTestResult{}, err
	}
	if account.DevServerID == "" {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindFailedPrecondition, "AIPROVIDER_NO_DEV_SERVER", "account has no dev server bound yet — push a credential first", nil)
	}

	// Relays a new agent-side JSON-RPC method (ai.testProviderConnection) —
	// see this task's Context section: out of scope for backend-go, this
	// call is inert until the agent implements it.
	result, err := uc.infra.Relay(ctx, account.DevServerID, "ai.testProviderConnection", map[string]any{
		"credentialRef": account.CredentialRef,
		"providerType":  string(account.ProviderType),
	})
	if err != nil {
		return ConnectionTestResult{}, apperrors.New(apperrors.KindInternal, "AIPROVIDER_TEST_CONNECTION_FAILED", "failed to relay connection test to dev server agent", err)
	}
	return parseConnectionTestResult(result), nil
}

// parseConnectionTestResult maps the agent's generic map[string]any result
// onto ConnectionTestResult, defensively (the agent method doesn't exist
// yet, so this is best-effort against the documented future shape).
func parseConnectionTestResult(result map[string]any) ConnectionTestResult {
	out := ConnectionTestResult{}
	if v, ok := result["success"].(bool); ok {
		out.Success = v
	}
	if v, ok := result["message"].(string); ok {
		out.Message = v
	}
	return out
}
```

### Step 3 — `adapter/grpcclient/infrafleet_client.go` (new)

```go
// Package grpcclient (this file) implements ai-provider-service's
// InfraFleetClient port against a real infra-fleet-service gRPC
// connection. Two calls per Relay: resolve dev_server_id -> connectionId,
// then relay — see TASK-025 for the ResolveConnection addition this
// depends on.
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// InfraFleetClient implements usecase.InfraFleetClient against a real
// infra-fleet-service connection.
type InfraFleetClient struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewInfraFleetClient(conn grpc.ClientConnInterface) *InfraFleetClient {
	return &InfraFleetClient{client: infrafleetv1.NewInfraFleetServiceClient(conn)}
}

var _ usecase.InfraFleetClient = (*InfraFleetClient)(nil)

func (c *InfraFleetClient) Relay(ctx context.Context, devServerID, method string, params map[string]any) (map[string]any, error) {
	resolved, err := c.client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{DevServerId: devServerID})
	if err != nil {
		return nil, fmt.Errorf("infrafleet: resolving dev server %s: %w", devServerID, err)
	}
	if !resolved.GetConnected() {
		return nil, fmt.Errorf("infrafleet: dev server %s has no active connection", devServerID)
	}

	paramsJSON, err := marshalParams(params)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: resolved.GetDevServer().GetId(),
		Method:       method,
		ParamsJson:   paramsJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("infrafleet: relaying %s to dev server %s: %w", method, devServerID, err)
	}
	return unmarshalResult(resp.GetResultJson())
}
```

`marshalParams`/`unmarshalResult` are small `encoding/json` helpers
(`json.Marshal(params)` / `json.Unmarshal([]byte(s), &out)`) — add them as
unexported functions in the same file.

**Verify at implementation time**: confirm which field
`ResolveConnectionResponse` actually returns the resolved connection's
dispatch key on (`resolved.GetDevServer().GetId()` above assumes the
`DevServer.Id` doubles as the `Relay` RPC's expected `connection_id` — this
mirrors SOL-006's own flagged uncertainty on the identical question for
`browser.*`, see TASK-034). If `ResolveConnectionResponse` instead needs a
separate `connection_id` echoed back (not just the `DevServer` it
resolved), extend that response message additively in TASK-025 rather than
guessing here.

### Step 4 — `internal/config/config.go`: add `InfraFleetServiceAddr`

```go
type Config struct {
	commonconfig.Base
	CredentialBrokerAddr string
	// InfraFleetServiceAddr is infra-fleet-service's gRPC target — dialed
	// by TestConnection's InfraFleetClient (TASK-028).
	InfraFleetServiceAddr string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("ai-provider-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                  base,
		CredentialBrokerAddr:  commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
		InfraFleetServiceAddr: commonconfig.StringEnv("INFRA_FLEET_SERVICE_ADDR", "infra-fleet-service:9090"),
	}, nil
}
```

### Step 5 — `cmd/server/main.go`: dial infra-fleet-service, wire `TestConnection`

Add alongside the existing `brokerConn` dial:

```go
infraFleetConn, err := grpc.NewClient(cfg.InfraFleetServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
if err != nil {
	return fmt.Errorf("dialing infra-fleet-service at %s: %w", cfg.InfraFleetServiceAddr, err)
}
defer func() { _ = infraFleetConn.Close() }()
infraFleet := aiprovidergrpcclient.NewInfraFleetClient(infraFleetConn)
```

Add `testConnectionUC := usecase.NewTestConnection(repo, infraFleet)`
alongside the other usecase constructions, and pass it into
`aiprovidergrpc.New(...)` alongside the existing 4 (TASK-026 already added
3 more to this constructor — extend the same signature here rather than
conflicting with it; if implemented after TASK-026, the constructor should
already take 7 usecases, this task adds the 8th).

### Step 6 — `adapter/grpc/server.go`: wire `TestConnection` RPC

Add a `TestConnection` gRPC method translating `TestConnectionRequest` into
`TestConnectionInput`, calling the usecase, mapping the result onto
`TestConnectionResponse{Success, Message}` — same
`apperrors.ToGRPCStatus`-on-error shape as every other handler in this
file.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/ai-provider-service
go build ./... && go vet ./...
```

Expected: clean build. Usecase-level tests are added in TASK-030.
