package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeTerminalScrollbackSnapshotRepository is an in-memory
// TerminalScrollbackSnapshotRepository shared by every scrollback-snapshot
// usecase test (Save/Get/Delete/Expire) in this package.
type fakeTerminalScrollbackSnapshotRepository struct {
	mu sync.Mutex

	byKey map[string]domain.TerminalScrollbackSnapshot // key: tenantID+"/"+worktreeID+"/"+paneKey

	sumBytesByWorktree map[string]int64 // key: tenantID+"/"+worktreeID, excludePaneKey ignored by the fake unless sumErr is set

	upsertErr  error
	getErr     error
	sumErr     error
	deleteErr  error
	expireErr  error
	expireN    int
	upsertCalls []domain.TerminalScrollbackSnapshot
	deleteByWorktreeCalls []struct{ tenantID, worktreeID string }
	deleteExpiredCalls    []time.Time
}

func scrollbackKey(tenantID, worktreeID, paneKey string) string {
	return tenantID + "/" + worktreeID + "/" + paneKey
}

func (f *fakeTerminalScrollbackSnapshotRepository) Upsert(ctx context.Context, snap domain.TerminalScrollbackSnapshot) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertCalls = append(f.upsertCalls, snap)
	if f.upsertErr != nil {
		return f.upsertErr
	}
	if f.byKey == nil {
		f.byKey = make(map[string]domain.TerminalScrollbackSnapshot)
	}
	f.byKey[scrollbackKey(snap.TenantID, snap.WorktreeID, snap.PaneKey)] = snap
	return nil
}

func (f *fakeTerminalScrollbackSnapshotRepository) Get(ctx context.Context, tenantID, worktreeID, paneKey string) (bool, domain.TerminalScrollbackSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return false, domain.TerminalScrollbackSnapshot{}, f.getErr
	}
	snap, ok := f.byKey[scrollbackKey(tenantID, worktreeID, paneKey)]
	return ok, snap, nil
}

func (f *fakeTerminalScrollbackSnapshotRepository) SumUncompressedBytes(ctx context.Context, tenantID, worktreeID, excludePaneKey string) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sumErr != nil {
		return 0, f.sumErr
	}
	if f.sumBytesByWorktree != nil {
		if v, ok := f.sumBytesByWorktree[tenantID+"/"+worktreeID]; ok {
			return v, nil
		}
	}
	var total int64
	for _, snap := range f.byKey {
		if snap.TenantID == tenantID && snap.WorktreeID == worktreeID && snap.PaneKey != excludePaneKey {
			total += snap.UncompressedBytes
		}
	}
	return total, nil
}

func (f *fakeTerminalScrollbackSnapshotRepository) DeleteByWorktree(ctx context.Context, tenantID, worktreeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteByWorktreeCalls = append(f.deleteByWorktreeCalls, struct{ tenantID, worktreeID string }{tenantID, worktreeID})
	if f.deleteErr != nil {
		return f.deleteErr
	}
	for k, snap := range f.byKey {
		if snap.TenantID == tenantID && snap.WorktreeID == worktreeID {
			delete(f.byKey, k)
		}
	}
	return nil
}

func (f *fakeTerminalScrollbackSnapshotRepository) DeleteExpired(ctx context.Context, olderThan time.Time) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteExpiredCalls = append(f.deleteExpiredCalls, olderThan)
	if f.expireErr != nil {
		return 0, f.expireErr
	}
	if f.expireN > 0 || f.byKey == nil {
		return f.expireN, nil
	}
	var count int
	for k, snap := range f.byKey {
		if snap.UpdatedAt.Before(olderThan) {
			delete(f.byKey, k)
			count++
		}
	}
	return count, nil
}

// fakeClock is a deterministic Clock for scrollback-snapshot usecase tests.
type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }
