package domain

import (
	"testing"
	"time"
)

func TestNewUsageSession_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		tenantID string
		userID   string
		provider Provider
		input    int64
		endedAt  time.Time
		wantErr  error
	}{
		{"valid", "t1", "u1", ProviderClaude, 100, now.Add(time.Minute), nil},
		{"empty tenant", "", "u1", ProviderClaude, 100, now.Add(time.Minute), ErrEmptyTenant},
		{"empty user", "t1", "", ProviderClaude, 100, now.Add(time.Minute), ErrEmptyTenant},
		{"invalid provider", "t1", "u1", Provider("bogus"), 100, now.Add(time.Minute), ErrInvalidProvider},
		{"negative tokens", "t1", "u1", ProviderClaude, -1, now.Add(time.Minute), ErrNegativeTokens},
		{"end before start", "t1", "u1", ProviderClaude, 100, now.Add(-time.Minute), ErrEndBeforeStart},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewUsageSession("s1", tt.tenantID, tt.userID, tt.provider, "wt1",
				tt.input, 50, 0, 0, 0.01, now, tt.endedAt, "req-1")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestDailyUsageRollup_ApplySession(t *testing.T) {
	now := time.Now()
	session, err := NewUsageSession("s1", "t1", "u1", ProviderClaude, "wt1", 100, 50, 0, 0, 0.02, now, now, "req-1")
	if err != nil {
		t.Fatalf("unexpected error building session: %v", err)
	}

	rollup := DailyUsageRollup{TenantID: "t1", UserID: "u1", Provider: ProviderClaude, Day: DayKey(now)}
	rollup = rollup.ApplySession(session)
	rollup = rollup.ApplySession(session)

	if rollup.SessionCount != 2 {
		t.Errorf("expected SessionCount=2, got %d", rollup.SessionCount)
	}
	if rollup.TotalInputTokens != 200 {
		t.Errorf("expected TotalInputTokens=200, got %d", rollup.TotalInputTokens)
	}
	if rollup.TotalCostUSD != 0.04 {
		t.Errorf("expected TotalCostUSD=0.04, got %v", rollup.TotalCostUSD)
	}
}

func TestDayKey_TruncatesToUTCMidnight(t *testing.T) {
	t1 := time.Date(2026, 8, 17, 23, 59, 59, 0, time.UTC)
	got := DayKey(t1)
	want := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("DayKey(%v) = %v, want %v", t1, got, want)
	}
}
