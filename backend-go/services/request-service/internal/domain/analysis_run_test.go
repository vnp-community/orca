package domain

import (
	"strings"
	"testing"
	"time"
)

func TestAnalysisRun_Transitions(t *testing.T) {
	now := time.Now()
	leaseOwner := "worker-1"
	leaseExpires := now.Add(time.Minute)

	run := &AnalysisRun{
		Status:         RunStatusRunning,
		Attempt:        1,
		LeaseOwner:     &leaseOwner,
		LeaseExpiresAt: &leaseExpires,
	}

	if err := run.RecordAttempt(); err != nil {
		t.Errorf("RecordAttempt failed: %v", err)
	}
	if run.Attempt != 2 {
		t.Errorf("expected attempt=2, got %v", run.Attempt)
	}

	if err := run.RecordAttempt(); err != ErrTooManyAttempts {
		t.Errorf("expected ErrTooManyAttempts, got %v", err)
	}

	run.Succeed(now)
	if run.Status != RunStatusSucceeded {
		t.Errorf("expected succeeded, got %v", run.Status)
	}
	if run.FinishedAt == nil || !run.FinishedAt.Equal(now) {
		t.Errorf("expected finished_at %v, got %v", now, run.FinishedAt)
	}
	if run.LeaseOwner != nil || run.LeaseExpiresAt != nil {
		t.Errorf("expected leases to be cleared on succeed")
	}

	run.Fail("ERR_1", "msg", now)
	if run.Status != RunStatusFailed {
		t.Errorf("expected failed, got %v", run.Status)
	}
	if run.ErrorCode == nil || *run.ErrorCode != "ERR_1" {
		t.Errorf("expected ERR_1, got %v", run.ErrorCode)
	}
	if run.ErrorMessage == nil || *run.ErrorMessage != "msg" {
		t.Errorf("expected msg, got %v", run.ErrorMessage)
	}
	if run.LeaseOwner != nil || run.LeaseExpiresAt != nil {
		t.Errorf("expected leases to be cleared on fail")
	}
}

func TestTruncateRaw(t *testing.T) {
	s := "hello"
	if TruncateRaw(s) != s {
		t.Errorf("short string truncated")
	}

	long := strings.Repeat("a", 256*1024+10)
	trunc := TruncateRaw(long)
	if len(trunc) != 256*1024 {
		t.Errorf("expected 256KB, got %v", len(trunc))
	}

	longVi := strings.Repeat("ế", (256*1024/3)+10)
	truncVi := TruncateRaw(longVi)
	if len(truncVi) > 256*1024 {
		t.Errorf("expected <= 256KB, got %v", len(truncVi))
	}
	// "ế" is 3 bytes. 256*1024 = 262144 bytes. 262144 % 3 = 1 byte left.
	// We expect 262143 bytes long.
	if len(truncVi) != 262143 {
		t.Errorf("expected 262143, got %v", len(truncVi))
	}
}
