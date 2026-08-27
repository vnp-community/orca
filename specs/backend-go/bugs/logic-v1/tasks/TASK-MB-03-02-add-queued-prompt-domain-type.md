# TASK-MB-03-02: Add `QueuedPrompt` domain type (BR-MB-11 length validation)

**From Solution:** SOL-MB-03
**Priority:** P0
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/domain/queued_prompt.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BR-MB-11 (max 10,000 chars) must be an invariant of construction, matching
this codebase's existing "invariant lives in the constructor" convention
(e.g. `domain.Worktree`, SOL-MB-01's `domain.PairedDevice`).

## Changes to make

```go
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
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... && go vet ./services/infra-fleet-service/...
go test ./services/infra-fleet-service/internal/domain/... -run NewQueuedPrompt
```

Test: rejects empty and >10,000-char prompts; accepts exactly 10,000 chars.
