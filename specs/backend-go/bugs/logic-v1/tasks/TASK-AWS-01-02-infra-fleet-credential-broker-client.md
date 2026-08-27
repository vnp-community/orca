# TASK-AWS-01-02: Add `infra-fleet-service`'s `CredentialBrokerClient` port + adapter + dial wiring

**From Solution:** SOL-AWS-01
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/ports.go`, `backend-go/services/infra-fleet-service/internal/adapter/grpcclient/credential_broker_client.go` (new), `backend-go/services/infra-fleet-service/internal/config/config.go`, `backend-go/services/infra-fleet-service/cmd/server/main.go`
**Depends on:** TASK-AWS-01-01
**Status:** `[ ]` TODO

---

## Context

`infra-fleet-service` does not dial `credential-broker-service` today.
Unlike `ai-provider-service`'s `CredentialBrokerClient` (which deliberately
never calls the plaintext-returning `ResolveCredential` RPC — see that
adapter's SECURITY-CRITICAL doc comment), `infra-fleet-service` legitimately
must: relay-websocket mode requires Orca to *present* the plaintext bearer
token outbound (SOL-AWS-01's core rationale — this is a different, real
exception, not a copy-paste of ai-provider-service's pattern). This client
also needs `WriteCredential`, for `CreateAgentToken`'s relay-websocket
branch (TASK-AWS-03-05).

## Changes to make

Append to `backend-go/services/infra-fleet-service/internal/usecase/ports.go`:

```go
// CredentialBrokerClient is infra-fleet-service's port to
// credential-broker-service — used ONLY for relay-websocket agent tokens
// (SOL-AWS-01). Unlike most services' identically-named ports, this one
// DOES call the plaintext-returning ResolveCredential RPC: relay-websocket
// mode requires Orca to present the token outbound as an Authorization
// header, not merely compare against a stored hash the way
// direct-websocket's TokenHash branch does. See adapter/grpcclient's doc
// comment for the full justification before touching this port.
type CredentialBrokerClient interface {
	// WriteCredential writes envelope (raw token bytes) under
	// (tenantID, ownerID=devServerID, CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN)
	// and returns a reference — the plaintext itself is never returned.
	WriteCredential(ctx context.Context, tenantID, ownerID string, envelope []byte) (CredentialRef, error)
	// ResolveCredential returns the plaintext bytes for credentialRefID —
	// called once per dial (never cached across process restarts), see
	// SOL-AWS-01's "resolve on every dial" guarantee.
	ResolveCredential(ctx context.Context, credentialRefID string) ([]byte, error)
}

// CredentialRef is what CredentialBrokerClient.WriteCredential returns —
// an opaque pointer, never the secret itself.
type CredentialRef struct {
	ID string
}
```

Create `backend-go/services/infra-fleet-service/internal/adapter/grpcclient/credential_broker_client.go`:

```go
// Package grpcclient implements infra-fleet-service's CredentialBrokerClient
// port (internal/usecase/ports.go) against a real credential-broker-service
// gRPC connection — see SOL-AWS-01.
//
// UNLIKE ai-provider-service's identically-named adapter, this client DOES
// call ResolveCredential (the plaintext-returning RPC) — relay-websocket
// mode requires infra-fleet-service to present the token outbound as an
// Authorization: Bearer header, a genuinely different case from every
// other service's credential usage in this codebase, where the secret is
// only ever compared against, never re-presented. Do not copy this
// pattern to another service's CredentialBrokerClient without the same
// justification.
package grpcclient

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	credentialbrokerv1 "github.com/stablyai/orca-go/proto/gen/go/orca/credentialbroker/v1"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// CredentialBrokerClient implements usecase.CredentialBrokerClient against
// a real credential-broker-service connection.
type CredentialBrokerClient struct {
	client credentialbrokerv1.CredentialBrokerServiceClient
}

// New wraps an already-dialed connection to credential-broker-service.
func New(conn grpc.ClientConnInterface) *CredentialBrokerClient {
	return &CredentialBrokerClient{client: credentialbrokerv1.NewCredentialBrokerServiceClient(conn)}
}

var _ usecase.CredentialBrokerClient = (*CredentialBrokerClient)(nil)

const credentialCategory = credentialbrokerv1.CredentialCategory_CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN

func (c *CredentialBrokerClient) WriteCredential(ctx context.Context, tenantID, ownerID string, envelope []byte) (usecase.CredentialRef, error) {
	resp, err := c.client.WriteCredential(ctx, &credentialbrokerv1.WriteCredentialRequest{
		TenantId: tenantID, OwnerId: ownerID, Category: credentialCategory, EncryptedEnvelope: envelope,
	})
	if err != nil {
		return usecase.CredentialRef{}, fmt.Errorf("grpcclient: credential-broker-service WriteCredential: %w", err)
	}
	return usecase.CredentialRef{ID: resp.GetMetadata().GetId()}, nil
}

func (c *CredentialBrokerClient) ResolveCredential(ctx context.Context, credentialRefID string) ([]byte, error) {
	resp, err := c.client.ResolveCredential(ctx, &credentialbrokerv1.ResolveCredentialRequest{CredentialId: credentialRefID})
	if err != nil {
		return nil, fmt.Errorf("grpcclient: credential-broker-service ResolveCredential: %w", err)
	}
	return resp.GetValue(), nil
}
```

In `backend-go/services/infra-fleet-service/internal/config/config.go`, add
the address field (mirrors `ai-provider-service`'s
`CredentialBrokerAddr`/`CREDENTIAL_BROKER_ADDR` convention):

```go
type Config struct {
	commonconfig.Base
	ServerDeployment bool
	// CredentialBrokerAddr is credential-broker-service's gRPC target —
	// dialed for relay-websocket agent token write/resolve (SOL-AWS-01).
	CredentialBrokerAddr string
}

func Load() (Config, error) {
	base, err := commonconfig.LoadBase("infra-fleet-service")
	if err != nil {
		return Config{}, err
	}
	return Config{
		Base:                  base,
		ServerDeployment:      os.Getenv("ORCA_SERVER_DEPLOYMENT") == "true",
		CredentialBrokerAddr:  commonconfig.StringEnv("CREDENTIAL_BROKER_ADDR", "credential-broker-service:9090"),
	}, nil
}
```

In `backend-go/services/infra-fleet-service/cmd/server/main.go`, dial the
connection and construct the client, following `ai-provider-service`'s
`cmd/server/main.go` dial pattern (`grpc.NewClient` +
`insecure.NewCredentials()` — internal mesh, mTLS terminated below this
layer per this codebase's existing convention) — add near the other setup,
before `agentClient := infradevserveragent.New(...)`:

```go
credentialBrokerConn, err := grpc.NewClient(cfg.CredentialBrokerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
if err != nil {
	return fmt.Errorf("dialing credential-broker-service at %s: %w", cfg.CredentialBrokerAddr, err)
}
defer credentialBrokerConn.Close()
credentialBrokerClient := infragrpcclient.New(credentialBrokerConn)
```

(add `"google.golang.org/grpc/credentials/insecure"` and
`infragrpcclient "github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/grpcclient"`
imports, matching this file's existing `infra*` import-aliasing
convention.)

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/...
go vet ./services/infra-fleet-service/...
```

Expected: clean build; `var _ usecase.CredentialBrokerClient = (*CredentialBrokerClient)(nil)`
compiles.
