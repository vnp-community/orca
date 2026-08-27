# TASK-AWS-03-05: Add `CreateAgentToken`/`ListAgentTokens`/`RevokeAgentToken` usecases

**From Solution:** SOL-AWS-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/usecase/create_agent_token.go`, `list_agent_tokens.go`, `revoke_agent_token.go` (new)
**Depends on:** TASK-AWS-03-03, TASK-AWS-03-04, TASK-AWS-01-01, TASK-AWS-01-02
**Status:** [x] DONE — `create_agent_token.go`/`list_agent_tokens.go`/`revoke_agent_token.go` created verbatim per spec; fakes + tests added (11th-token-rejected, direct-websocket hash-only, relay-websocket credential-ref-only, relay-ssh rejected, revoke closes session exactly once, error propagation); all green.

---

## Context

`CreateAgentToken`'s relay-websocket branch calls
`usecase.CredentialBrokerClient.WriteCredential` with the
`CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN` category — both added by
SOL-AWS-01 (TASK-AWS-01-01/02) — so this task depends on those landing
first even though the table/domain type belong to SOL-AWS-03.

## Changes to make

Create `backend-go/services/infra-fleet-service/internal/usecase/create_agent_token.go`:

```go
package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// CreateAgentToken mints a new persistent agent token for a DevServer
// (BL-AWS-03). The plaintext is generated here, in the usecase layer, and
// returned exactly once — domain.AgentToken never holds it, mirroring
// credential-broker-service's CredentialMetadata invariant.
type CreateAgentToken struct {
	repo             AgentTokenRepository
	devServers       DevServerRepository
	credentialBroker CredentialBrokerClient
}

func NewCreateAgentToken(repo AgentTokenRepository, devServers DevServerRepository, credentialBroker CredentialBrokerClient) *CreateAgentToken {
	return &CreateAgentToken{repo: repo, devServers: devServers, credentialBroker: credentialBroker}
}

// Execute returns the plaintext token (shown once) and the persisted
// domain.AgentToken (which never carries the plaintext). tenantID is
// pulled from ctx here (tenant.RequireTenantID), matching
// CreateSshTarget.Execute's existing convention in this same package —
// not accepted as a caller-supplied parameter.
func (uc *CreateAgentToken) Execute(ctx context.Context, devServerID, name string) (plaintext string, _ domain.AgentToken, _ error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return "", domain.AgentToken{}, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	if name == "" {
		return "", domain.AgentToken{}, domain.ErrEmptyAgentTokenName
	}
	n, err := uc.repo.CountActive(ctx, tenantID, devServerID)
	if err != nil {
		return "", domain.AgentToken{}, err
	}
	if n >= domain.MaxActiveAgentTokensPerDevServer {
		return "", domain.AgentToken{}, domain.ErrAgentTokenLimitReached
	}
	dev, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return "", domain.AgentToken{}, err
	}

	raw, err := generateHexToken(32)
	if err != nil {
		return "", domain.AgentToken{}, err
	}
	tok := domain.AgentToken{ID: uuid.NewString(), TenantID: tenantID, DevServerID: devServerID, Name: name, CreatedAt: time.Now()}

	switch dev.Mode {
	case domain.ConnectionModeDirectWebSocket:
		tok.TokenHash = sha256Hex(raw)
	case domain.ConnectionModeRelayWebSocket:
		// See SOL-AWS-01: write raw to credential-broker-service, keep only
		// the returned CredentialRef.ID here — the plaintext is never
		// written to this service's own database.
		ref, err := uc.credentialBroker.WriteCredential(ctx, tenantID, devServerID, []byte(raw))
		if err != nil {
			return "", domain.AgentToken{}, err
		}
		tok.CredentialRefID = ref.ID
	default:
		// relay-ssh has no agent-token concept — the SSH connection itself
		// is the trust boundary (infra-fleet-service.md §9).
		return "", domain.AgentToken{}, domain.ErrInvalidConnectionMode
	}

	if err := uc.repo.Insert(ctx, tok); err != nil {
		return "", domain.AgentToken{}, err
	}
	return raw, tok, nil // raw returned ONLY here — never stored, never logged
}

func generateHexToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
```

Create `backend-go/services/infra-fleet-service/internal/usecase/list_agent_tokens.go`:

```go
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// ListAgentTokens returns the active token summaries for a DevServer —
// never the plaintext or the hash/credential_ref_id.
type ListAgentTokens struct {
	repo AgentTokenRepository
}

func NewListAgentTokens(repo AgentTokenRepository) *ListAgentTokens {
	return &ListAgentTokens{repo: repo}
}

func (uc *ListAgentTokens) Execute(ctx context.Context, devServerID string) ([]AgentTokenSummary, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return nil, apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	tokens, err := uc.repo.ListActive(ctx, tenantID, devServerID)
	if err != nil {
		return nil, err
	}
	out := make([]AgentTokenSummary, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, AgentTokenSummary{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt})
	}
	return out, nil
}

// AgentTokenSummary is the never-plaintext, never-secret-ref view of an
// AgentToken this usecase and the gRPC layer both use.
type AgentTokenSummary struct {
	ID         string
	Name       string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}
```

Create `backend-go/services/infra-fleet-service/internal/usecase/revoke_agent_token.go`:

```go
package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
)

// RevokeAgentToken revokes a token and closes any live session
// authenticated with it — immediate-effect, no deploy required (see
// SOL-AWS-01's "resolve on every dial" guarantee for the relay-websocket
// case, and SOL-AWS-03's LiveSessionCloser for direct-websocket).
type RevokeAgentToken struct {
	repo     AgentTokenRepository
	sessions LiveSessionCloser
}

func NewRevokeAgentToken(repo AgentTokenRepository, sessions LiveSessionCloser) *RevokeAgentToken {
	return &RevokeAgentToken{repo: repo, sessions: sessions}
}

func (uc *RevokeAgentToken) Execute(ctx context.Context, devServerID, id string) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
	}
	tok, err := uc.repo.Revoke(ctx, tenantID, id)
	if err != nil {
		return err
	}
	_, err = uc.sessions.CloseSessionsForDevServerToken(ctx, devServerID, tok.ID)
	return err
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/internal/usecase/...
```

Expected: clean build once `CredentialBrokerClient` (TASK-AWS-01-02) and the
proto enum (TASK-AWS-01-01) exist. Then add
`usecase/create_agent_token_test.go`/`revoke_agent_token_test.go` per
SOL-AWS-03's test plan (11th token rejected with
`ErrAgentTokenLimitReached`; direct-websocket rows get `TokenHash` only;
relay-websocket rows call `WriteCredential` and store `CredentialRefID`
only; revoke calls `CloseSessionsForDevServerToken` exactly once).
