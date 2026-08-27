package usecase

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReapExpiredSessions_ComputesCutoffAndReturnsCount(t *testing.T) {
	sessions := newFakeSessionRepository()
	now := time.Now()
	clock := &fakeClock{now: now}
	retention := 7 * 24 * time.Hour

	uc := NewReapExpiredSessions(sessions, clock, retention)
	n, err := uc.Execute(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 rows removed from an empty repo, got %d", n)
	}

	if len(sessions.deleteExpiredCutoffs) != 1 {
		t.Fatalf("expected exactly one DeleteExpiredBefore call, got %d", len(sessions.deleteExpiredCutoffs))
	}
	wantCutoff := now.Add(-retention)
	if !sessions.deleteExpiredCutoffs[0].Equal(wantCutoff) {
		t.Errorf("expected cutoff = now - retention = %v, got %v", wantCutoff, sessions.deleteExpiredCutoffs[0])
	}
}

func TestReapExpiredSessions_PropagatesRepositoryError(t *testing.T) {
	sessions := newFakeSessionRepository()
	sessions.deleteExpiredErr = errors.New("boom")
	clock := &fakeClock{now: time.Now()}

	uc := NewReapExpiredSessions(sessions, clock, 7*24*time.Hour)
	_, err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("expected a repo error to propagate")
	}
}
