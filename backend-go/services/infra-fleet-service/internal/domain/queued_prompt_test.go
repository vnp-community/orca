package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNewQueuedPrompt_ValidatesInvariants(t *testing.T) {
	now := time.Now()

	t.Run("rejects empty prompt", func(t *testing.T) {
		_, err := NewQueuedPrompt("pty-1", "tenant-1", "", "device-1", now)
		if !errors.Is(err, ErrPromptEmpty) {
			t.Fatalf("expected ErrPromptEmpty, got %v", err)
		}
	})

	t.Run("rejects prompt over 10,000 chars", func(t *testing.T) {
		_, err := NewQueuedPrompt("pty-1", "tenant-1", strings.Repeat("a", MaxPromptLength+1), "device-1", now)
		if !errors.Is(err, ErrPromptTooLong) {
			t.Fatalf("expected ErrPromptTooLong, got %v", err)
		}
	})

	t.Run("accepts exactly 10,000 chars", func(t *testing.T) {
		prompt := strings.Repeat("a", MaxPromptLength)
		qp, err := NewQueuedPrompt("pty-1", "tenant-1", prompt, "device-1", now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qp.PtyID != "pty-1" || qp.TenantID != "tenant-1" || qp.Prompt != prompt || qp.DispatchedByDeviceID != "device-1" || !qp.QueuedAt.Equal(now) {
			t.Errorf("unexpected QueuedPrompt: %+v", qp)
		}
	})
}
