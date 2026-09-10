package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeAgentSessionRepository implements AgentSessionRepository for tests —
// mirrors fakeTerminalSessionRepository's shape (terminal_session_lookup_test.go).
type fakeAgentSessionRepository struct {
	mu sync.Mutex

	byID map[string]domain.AgentSession

	createErr error
	getErr    error

	createCalls               []domain.AgentSession
	updateStatusCalls         []domain.AgentStatus
	markStoppedCalls          []string
	markStoppedWithStatusCall []domain.AgentStatus
	updateProviderSessionCall *struct{ Key, ID string }
}

func (f *fakeAgentSessionRepository) Create(ctx context.Context, s domain.AgentSession) (domain.AgentSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls = append(f.createCalls, s)
	if f.createErr != nil {
		return domain.AgentSession{}, f.createErr
	}
	if f.byID == nil {
		f.byID = make(map[string]domain.AgentSession)
	}
	f.byID[s.ID] = s
	return s, nil
}

func (f *fakeAgentSessionRepository) Get(ctx context.Context, tenantID, sessionID string) (bool, domain.AgentSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return false, domain.AgentSession{}, f.getErr
	}
	s, ok := f.byID[sessionID]
	if !ok || s.TenantID != tenantID {
		return false, domain.AgentSession{}, nil
	}
	return true, s, nil
}

func (f *fakeAgentSessionRepository) GetByPtyID(ctx context.Context, tenantID, ptyID string) (bool, domain.AgentSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range f.byID {
		if s.TenantID == tenantID && s.PtyID == ptyID {
			return true, s, nil
		}
	}
	return false, domain.AgentSession{}, nil
}

func (f *fakeAgentSessionRepository) LatestForWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.AgentSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest domain.AgentSession
	found := false
	for _, s := range f.byID {
		if s.TenantID != tenantID || s.WorktreeID != worktreeID {
			continue
		}
		if !found || s.StartedAt.After(latest.StartedAt) {
			latest = s
			found = true
		}
	}
	return found, latest, nil
}

func (f *fakeAgentSessionRepository) MostRecentActiveForWorktree(ctx context.Context, tenantID, worktreeID string) (bool, domain.AgentSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest domain.AgentSession
	found := false
	for _, s := range f.byID {
		if s.TenantID != tenantID || s.WorktreeID != worktreeID {
			continue
		}
		switch s.Status {
		case domain.AgentStatusSpawning, domain.AgentStatusRunning, domain.AgentStatusIdle, domain.AgentStatusWaiting:
		default:
			continue
		}
		if !found || s.StartedAt.After(latest.StartedAt) {
			latest = s
			found = true
		}
	}
	return found, latest, nil
}

func (f *fakeAgentSessionRepository) UpdateStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateStatusCalls = append(f.updateStatusCalls, status)
	if s, ok := f.byID[sessionID]; ok {
		s.Status = status
		s.LastActiveAt = now
		f.byID[sessionID] = s
	}
	return nil
}

func (f *fakeAgentSessionRepository) MarkStopped(ctx context.Context, tenantID, sessionID string, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markStoppedCalls = append(f.markStoppedCalls, sessionID)
	if s, ok := f.byID[sessionID]; ok {
		s.Status = domain.AgentStatusStopped
		s.StoppedAt = &now
		f.byID[sessionID] = s
	}
	return nil
}

func (f *fakeAgentSessionRepository) MarkStoppedWithStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.markStoppedWithStatusCall = append(f.markStoppedWithStatusCall, status)
	if s, ok := f.byID[sessionID]; ok {
		s.Status = status
		s.StoppedAt = &now
		f.byID[sessionID] = s
	}
	return nil
}

func (f *fakeAgentSessionRepository) UpdateProviderSession(ctx context.Context, tenantID, sessionID, providerSessionKey, providerSessionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updateProviderSessionCall = &struct{ Key, ID string }{Key: providerSessionKey, ID: providerSessionID}
	if s, ok := f.byID[sessionID]; ok {
		s.ResumeProviderSessionKey = providerSessionKey
		s.ResumeProviderSessionID = providerSessionID
		f.byID[sessionID] = s
	}
	return nil
}

// fakeAIProviderResolverClient implements AIProviderResolverClient for tests.
type fakeAIProviderResolverClient struct {
	providerType, accountID, status string
	err                             error

	calls []struct{ UserID, ProjectID, ExcludeAccountID string }
}

func (f *fakeAIProviderResolverClient) ResolveProvider(ctx context.Context, tenantID, userID, projectID, excludeAccountID string) (string, string, string, error) {
	f.calls = append(f.calls, struct{ UserID, ProjectID, ExcludeAccountID string }{userID, projectID, excludeAccountID})
	if f.err != nil {
		return "", "", "", f.err
	}
	return f.providerType, f.accountID, f.status, nil
}
