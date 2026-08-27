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
