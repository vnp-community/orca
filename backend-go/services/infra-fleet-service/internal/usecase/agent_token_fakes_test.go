package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeAgentTokenRepository is an in-memory usecase.AgentTokenRepository.
type fakeAgentTokenRepository struct {
	byDevServer map[string][]domain.AgentToken // devServerID -> active tokens, newest last
	byHash      map[string]domain.AgentToken

	countActiveErr error
	insertErr      error
	listActiveErr  error
	findByHashErr  error
	activeForErr   error
	touchErr       error
	revokeErr      error

	inserted []domain.AgentToken
	revoked  []string // ids passed to Revoke
	touched  []string // ids passed to TouchLastUsed
}

func newFakeAgentTokenRepository() *fakeAgentTokenRepository {
	return &fakeAgentTokenRepository{
		byDevServer: make(map[string][]domain.AgentToken),
		byHash:      make(map[string]domain.AgentToken),
	}
}

func (f *fakeAgentTokenRepository) CountActive(ctx context.Context, tenantID, devServerID string) (int, error) {
	if f.countActiveErr != nil {
		return 0, f.countActiveErr
	}
	return len(f.byDevServer[devServerID]), nil
}

func (f *fakeAgentTokenRepository) Insert(ctx context.Context, t domain.AgentToken) error {
	if f.insertErr != nil {
		return f.insertErr
	}
	f.inserted = append(f.inserted, t)
	f.byDevServer[t.DevServerID] = append(f.byDevServer[t.DevServerID], t)
	if t.TokenHash != "" {
		f.byHash[t.TokenHash] = t
	}
	return nil
}

func (f *fakeAgentTokenRepository) ListActive(ctx context.Context, tenantID, devServerID string) ([]domain.AgentToken, error) {
	if f.listActiveErr != nil {
		return nil, f.listActiveErr
	}
	return f.byDevServer[devServerID], nil
}

func (f *fakeAgentTokenRepository) FindActiveByHash(ctx context.Context, hash string) (domain.AgentToken, bool, error) {
	if f.findByHashErr != nil {
		return domain.AgentToken{}, false, f.findByHashErr
	}
	t, ok := f.byHash[hash]
	if !ok || !t.Active() {
		return domain.AgentToken{}, false, nil
	}
	return t, true, nil
}

func (f *fakeAgentTokenRepository) ActiveForDevServer(ctx context.Context, tenantID, devServerID string) (domain.AgentToken, bool, error) {
	if f.activeForErr != nil {
		return domain.AgentToken{}, false, f.activeForErr
	}
	tokens := f.byDevServer[devServerID]
	if len(tokens) == 0 {
		return domain.AgentToken{}, false, nil
	}
	return tokens[len(tokens)-1], true, nil
}

func (f *fakeAgentTokenRepository) TouchLastUsed(ctx context.Context, id string) error {
	f.touched = append(f.touched, id)
	return f.touchErr
}

func (f *fakeAgentTokenRepository) Revoke(ctx context.Context, tenantID, id string) (domain.AgentToken, error) {
	if f.revokeErr != nil {
		return domain.AgentToken{}, f.revokeErr
	}
	f.revoked = append(f.revoked, id)
	for devServerID, tokens := range f.byDevServer {
		for i, t := range tokens {
			if t.ID == id {
				now := t.CreatedAt
				t.RevokedAt = &now
				f.byDevServer[devServerID][i] = t
				return t, nil
			}
		}
	}
	return domain.AgentToken{ID: id}, nil
}

// fakeLiveSessionCloser is an in-memory usecase.LiveSessionCloser.
type fakeLiveSessionCloser struct {
	closeErr    error
	closed      int
	calls       int
	lastDevID   string
	lastTokenID string
}

func (f *fakeLiveSessionCloser) CloseSessionsForDevServerToken(ctx context.Context, devServerID, tokenID string) (int, error) {
	f.calls++
	f.lastDevID = devServerID
	f.lastTokenID = tokenID
	if f.closeErr != nil {
		return 0, f.closeErr
	}
	return f.closed, nil
}

// fakeCredentialBrokerClient is an in-memory usecase.CredentialBrokerClient.
type fakeCredentialBrokerClient struct {
	writeErr   error
	resolveErr error

	writeCalls    int
	lastEnvelope  []byte
	lastTenantID  string
	lastOwnerID   string
	writtenRefID  string
	resolveValues map[string][]byte
}

func (f *fakeCredentialBrokerClient) WriteCredential(ctx context.Context, tenantID, ownerID string, envelope []byte) (CredentialRef, error) {
	f.writeCalls++
	f.lastTenantID = tenantID
	f.lastOwnerID = ownerID
	f.lastEnvelope = envelope
	if f.writeErr != nil {
		return CredentialRef{}, f.writeErr
	}
	refID := f.writtenRefID
	if refID == "" {
		refID = "cred-ref-1"
	}
	return CredentialRef{ID: refID}, nil
}

func (f *fakeCredentialBrokerClient) ResolveCredential(ctx context.Context, credentialRefID string) ([]byte, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}
	return f.resolveValues[credentialRefID], nil
}
