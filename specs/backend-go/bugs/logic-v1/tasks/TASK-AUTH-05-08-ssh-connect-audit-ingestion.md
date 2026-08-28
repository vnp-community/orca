# TASK-AUTH-05-08: `infra-fleet-service` publishes `ssh.connect`; `auth-service` ingests it via NATS consumer

**From Solution:** SOL-AUTH-05
**Priority:** P2
**Service:** `infra-fleet-service` (outbox publish) + `auth-service` (NATS consumer)
**File:** `backend-go/services/infra-fleet-service/internal/usecase/` (connection-establish usecase, existing file — add outbox publish), `backend-go/services/auth-service/internal/adapter/natsconsumer/audit_ingest.go` (new)
**Depends on:** TASK-AUTH-05-02
**Status:** `[x]` DONE — infra-fleet-service's `EstablishConnection` now enqueues an `orca.infrafleet.ssh.connected` outbox event (`internal/adapter/postgres.Repository.CreateConnectionWithOutbox`, same-transaction pattern bootstrapped fresh from usage-service's precedent; migration `0007_outbox`) published by a new `common/outbox.Relay` wired in `cmd/server/main.go`. auth-service durably consumes it via a new `internal/adapter/natsconsumer.AuditIngestConsumer` (the first real caller of `commoneventbus.Consumer.Subscribe`, durable consumer name `auth-service-ssh-connect-audit`) that calls a new `usecase.HandleSSHConnectedEvent`, appending an `action="ssh.connect"`, `target_type="ssh_host"` audit entry. Both services gained `NATSURL` config and graceful NATS-unavailable degradation matching usage-service/notification-service. Verified: `go build`/`go vet`/`go test` clean for `infra-fleet-service`, `auth-service`, `common`; `go test ./services/auth-service/internal/adapter/natsconsumer/... -run TestAuditIngestConsumer -v` passes (well-formed event → 1 Append call with the expected action/target_type; malformed payload dropped without error/panic); infra-fleet-service unit tests cover outbox-publish-attempted-after-success, missing-actor-is-not-fatal, and repository-failure-propagates; a `-tags=integration` Postgres round-trip test (`TestRepository_Outbox_EnqueueFetchMarkPublished`) confirms the enqueue/fetch/mark-published cycle against a real database (this test, like the pre-existing `TestRepository_ResolveConnection_FoundAndNotFound` in the same file, is occasionally flaky on the very first container in a run due to `common/testutil.StartPostgres`'s readiness probe only waiting for the port to listen, not for Postgres to accept queries — a pre-existing environmental issue, not a regression from this change).

---

## Context

`auth-service.md` already frames the audit log as "own + ingested from other services' outbox streams," and `07-security-architecture.md` states every service emits structured audit events via the outbox pattern for security-relevant actions in its own domain. SSH connection lifecycle is owned by `infra-fleet-service`, not `auth-service` — a `grep -rn "ssh.connect"` across the codebase returns zero matches today, confirming no connection-establish usecase emits this event at all. This task is the mechanical realization of a sentence the TDD already states but doesn't spell out: publish from `infra-fleet-service`'s outbox, consume in `auth-service`.

## Changes to make

In `infra-fleet-service`'s connection-establish usecase (the file that currently opens/records an SSH connection — locate it under `backend-go/services/infra-fleet-service/internal/usecase/`), add an outbox-insert call after a connection is successfully established, using the same outbox-insert pattern this service's other domain events already use:

```go
// After the connection is successfully established, publish to the
// outbox — auth-service's audit_ingest consumer picks this up and appends
// an ssh.connect audit entry. Mirrors this service's existing outbox
// pattern for other domain events; not a new mechanism.
if err := uc.outbox.Publish(ctx, "ssh.connect", sshConnectedEvent{
	ActorUserID:  actorUserID,
	TenantID:     tenantID,
	ConnectionID: connectionID,
	Host:         host,
	OccurredAt:   now,
}); err != nil {
	// best-effort — a failed outbox publish must not fail the connection
	// itself, matching every other best-effort audit-adjacent write in
	// this codebase
	log.Warn("failed to publish ssh.connect outbox event", "err", err)
}
```

Adjust field/type names to match this usecase's actual existing variables (`actorUserID`, `tenantID`, `connectionID`, `host`, `now`) and this service's actual outbox publish method signature — do not invent a new outbox mechanism if `infra-fleet-service` already has one for other events.

Create `backend-go/services/auth-service/internal/adapter/natsconsumer/audit_ingest.go`:

```go
package natsconsumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"

	"github.com/stablyai/orca-go/services/auth-service/internal/domain"
	"github.com/stablyai/orca-go/services/auth-service/internal/usecase"
)

type sshConnectedEvent struct {
	ActorUserID string    `json:"actor_user_id"`
	TenantID    string    `json:"tenant_id"`
	ConnectionID string   `json:"connection_id"`
	Host        string    `json:"host"`
	OccurredAt  time.Time `json:"occurred_at"`
}

// AuditIngestConsumer subscribes to other services' outbox-published
// security-relevant events and appends them to auth-service's own audit
// log — per auth-service.md's "own + ingested from other services' outbox
// streams" framing. Trust boundary: the NATS subject is only publishable
// by services inside the mesh (mTLS + default-deny NetworkPolicy), so this
// consumer does not re-authenticate the event's claimed actor beyond what
// infra-fleet-service already validated when the connection was
// established.
type AuditIngestConsumer struct {
	audit usecase.AuditRepository
}

func NewAuditIngestConsumer(audit usecase.AuditRepository) *AuditIngestConsumer {
	return &AuditIngestConsumer{audit: audit}
}

func (c *AuditIngestConsumer) handleSSHConnected(msg *nats.Msg) {
	var evt sshConnectedEvent
	if err := json.Unmarshal(msg.Data, &evt); err != nil {
		return // malformed event — logged, not fatal to the consumer loop
	}
	entry, err := domain.NewAuditEntry(uuid.NewString(), evt.TenantID, evt.ActorUserID,
		"ssh.connect", "ssh_host", evt.Host,
		map[string]any{"connectionId": evt.ConnectionID}, "", evt.OccurredAt)
	if err != nil {
		return
	}
	_ = c.audit.Append(context.Background(), entry)
}
```

Wire `AuditIngestConsumer` to subscribe to `infra-fleet-service`'s `ssh.connect` NATS subject in `auth-service`'s `cmd/server/main.go`, matching whatever pattern this codebase already uses to wire other NATS subscriptions (check `internal/adapter/natsconsumer/` for an existing sibling consumer to match the subscribe-call shape, subject naming convention, and durable-consumer/queue-group configuration exactly).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... ./services/auth-service/...
go test ./services/auth-service/internal/adapter/natsconsumer/... -run TestAuditIngestConsumer -v
```

Expected: a well-formed `ssh.connect` NATS message produces exactly one `Append` call with `action: "ssh.connect"`, `target_type: "ssh_host"`; a malformed message is dropped without panicking or blocking the consumer loop; `infra-fleet-service`'s connection-establish usecase test confirms the outbox publish is attempted after a successful connection, and that a publish failure does not fail the connection itself.
