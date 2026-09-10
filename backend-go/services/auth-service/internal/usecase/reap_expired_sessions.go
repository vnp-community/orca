package usecase

import (
	"context"
	"time"
)

// ReapExpiredSessions is a background-job usecase — purges session rows
// expired/revoked more than retention ago. Not a correctness mechanism
// (domain.Session.IsValid already enforces expiry at read time); this only
// bounds table growth, per auth-service.md §8's reaper NFR.
type ReapExpiredSessions struct {
	sessions  SessionRepository
	clock     Clock
	retention time.Duration
}

func NewReapExpiredSessions(sessions SessionRepository, clock Clock, retention time.Duration) *ReapExpiredSessions {
	return &ReapExpiredSessions{sessions: sessions, clock: clock, retention: retention}
}

func (uc *ReapExpiredSessions) Execute(ctx context.Context) (int64, error) {
	cutoff := uc.clock.Now().Add(-uc.retention)
	return uc.sessions.DeleteExpiredBefore(ctx, cutoff)
}
