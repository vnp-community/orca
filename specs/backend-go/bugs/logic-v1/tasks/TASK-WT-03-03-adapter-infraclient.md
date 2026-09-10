# TASK-WT-03-03: Implement `TerminalSessionLister` against `infrafleetv1.InfraFleetServiceClient`

**From Solution:** SOL-WT-03
**Priority:** P0
**Service:** `git-gateway-service`
**File:** `backend-go/services/git-gateway-service/internal/adapter/infraclient/terminal_sessions.go` (new)
**Depends on:** TASK-WT-03-02
**Status:** `[x]` DONE — Created infraclient/terminal_sessions.go implementing TerminalSessionLister against infrafleetv1 (field names confirmed against real proto); go build clean.

---

## Context

Implements the port from [TASK-WT-03-02](./TASK-WT-03-02-ports-terminal-session-lister.md) against the already-real `infra-fleet-service.ListTerminalSessions`/`KillTerminalSession` RPCs.

## Changes to make

Before implementing, confirm the exact `TerminalSession` field names:

```bash
cd /opt/repos/orca/backend-go
grep -n "message TerminalSession {" -A 10 proto/orca/infrafleet/v1/infrafleet.proto
```

Create `backend-go/services/git-gateway-service/internal/adapter/infraclient/terminal_sessions.go`:

```go
// Package infraclient implements git-gateway-service's outbound calls to
// infra-fleet-service beyond connection resolution/relay dispatch — see
// usecase.TerminalSessionLister's doc comment for the dependency-edge
// reasoning (SOL-WT-03).
package infraclient

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

type TerminalSessionLister struct {
	client infrafleetv1.InfraFleetServiceClient
}

func NewTerminalSessionLister(client infrafleetv1.InfraFleetServiceClient) *TerminalSessionLister {
	return &TerminalSessionLister{client: client}
}

func (t *TerminalSessionLister) ListSessions(ctx context.Context, connectionID string) ([]domain.TerminalSessionRef, error) {
	resp, err := t.client.ListTerminalSessions(ctx, &infrafleetv1.ListTerminalSessionsRequest{ConnectionId: connectionID})
	if err != nil {
		return nil, err
	}
	out := make([]domain.TerminalSessionRef, 0, len(resp.GetSessions()))
	for _, s := range resp.GetSessions() {
		out = append(out, domain.TerminalSessionRef{PtyID: s.GetPtyId(), Cwd: s.GetCwd()})
	}
	return out, nil
}

func (t *TerminalSessionLister) Kill(ctx context.Context, ptyID string) error {
	_, err := t.client.KillTerminalSession(ctx, &infrafleetv1.KillTerminalSessionRequest{PtyId: ptyID})
	return err
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/git-gateway-service/...
```

Expected: clean build; `*infraclient.TerminalSessionLister` satisfies `usecase.TerminalSessionLister`.
