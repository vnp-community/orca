package usecase

import (
	"context"
	"sync"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeQueuedPromptRepository is an in-memory QueuedPromptRepository — shared
// by GetTerminalAgentStatus's queue-drain tests and DispatchPrompt/
// GetQueuedPrompt's own tests, same "test against fakes" convention as
// fakeTerminalSessionRepository (see attach_pty_test.go).
type fakeQueuedPromptRepository struct {
	mu sync.Mutex

	byPtyID map[string]domain.QueuedPrompt

	getErr          error
	upsertErr       error
	deleteErr       error
	getAndDeleteErr error

	upsertCalls       []domain.QueuedPrompt
	deleteCalls       []string
	getAndDeleteCalls []string
}

func (f *fakeQueuedPromptRepository) Get(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return domain.QueuedPrompt{}, false, f.getErr
	}
	p, ok := f.byPtyID[ptyID]
	return p, ok, nil
}

func (f *fakeQueuedPromptRepository) Upsert(ctx context.Context, p domain.QueuedPrompt) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertCalls = append(f.upsertCalls, p)
	if f.upsertErr != nil {
		return f.upsertErr
	}
	if f.byPtyID == nil {
		f.byPtyID = make(map[string]domain.QueuedPrompt)
	}
	f.byPtyID[p.PtyID] = p
	return nil
}

func (f *fakeQueuedPromptRepository) Delete(ctx context.Context, ptyID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteCalls = append(f.deleteCalls, ptyID)
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.byPtyID, ptyID)
	return nil
}

func (f *fakeQueuedPromptRepository) GetAndDelete(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getAndDeleteCalls = append(f.getAndDeleteCalls, ptyID)
	if f.getAndDeleteErr != nil {
		return domain.QueuedPrompt{}, false, f.getAndDeleteErr
	}
	p, ok := f.byPtyID[ptyID]
	if ok {
		delete(f.byPtyID, ptyID)
	}
	return p, ok, nil
}

func (f *fakeQueuedPromptRepository) upsertCallsSnapshot() []domain.QueuedPrompt {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]domain.QueuedPrompt, len(f.upsertCalls))
	copy(out, f.upsertCalls)
	return out
}
