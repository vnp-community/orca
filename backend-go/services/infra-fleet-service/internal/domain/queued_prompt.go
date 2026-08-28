package domain

import (
	"errors"
	"time"
)

const MaxPromptLength = 10_000 // BR-MB-11

var (
	ErrPromptTooLong      = errors.New("domain: prompt exceeds 10,000 characters")
	ErrPromptEmpty        = errors.New("domain: prompt is empty")
	ErrQueuedPromptExists = errors.New("domain: a prompt is already queued for this pty — confirmation required")
)

// QueuedPrompt is a mobile-dispatched prompt held for delivery once the
// terminal's agent becomes ready for input (BR-MB-10) — see SOL-MB-03.
type QueuedPrompt struct {
	PtyID                string
	TenantID             string
	Prompt               string
	DispatchedByDeviceID string
	QueuedAt             time.Time
}

// NewQueuedPrompt enforces BR-MB-11 at construction.
func NewQueuedPrompt(ptyID, tenantID, prompt, deviceID string, now time.Time) (QueuedPrompt, error) {
	if prompt == "" {
		return QueuedPrompt{}, ErrPromptEmpty
	}
	if len(prompt) > MaxPromptLength {
		return QueuedPrompt{}, ErrPromptTooLong
	}
	return QueuedPrompt{PtyID: ptyID, TenantID: tenantID, Prompt: prompt, DispatchedByDeviceID: deviceID, QueuedAt: now}, nil
}
