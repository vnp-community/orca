# TASK-PW-04-04: `task-service` — `EnsureStream` + outbox `Relay` startup wiring

**From Solution:** SOL-PW-04
**Priority:** P0
**Service:** `task-service`
**File:** `backend-go/services/task-service/cmd/server/main.go`
**Depends on:** TASK-PW-04-02
**Status:** `[ ]` TODO

---

## Context

`common/eventbus`/`common/outbox` are real, generic, already-implemented
infra — this task is the startup wiring, copied from `usage-service`'s
already-working composition root
(`services/usage-service/cmd/server/main.go:100-109`), the direct
precedent for this exact pattern in this codebase.

## Changes to make

In `task-service/cmd/server/main.go`, alongside the existing
`repo`/usecase construction:

```go
var relay *outbox.Relay
pub, _, closeBus, err := eventbus.Connect(ctx, cfg.NATSURL)
if err != nil {
	logger.WarnContext(ctx, "eventbus unavailable, outbox events will queue until a future restart", slog.Any("error", err))
} else {
	defer func() { _ = closeBus() }()
	if err := pub.EnsureStream(ctx, "TASK", []string{"orca.task.>"}); err != nil {
		logger.WarnContext(ctx, "failed to ensure TASK stream", slog.Any("error", err))
	} else {
		relay = outbox.NewRelay(repo, pub, outbox.DefaultConfig, logger)
	}
}
if relay != nil {
	go relay.Run(ctx)
}
```

Match the exact graceful-degradation posture `usage-service`'s main.go
already uses (NATS unavailable at startup does not fail service startup —
outbox rows queue durably in Postgres and drain on a future restart once
NATS is reachable). Also match whatever shutdown-wait pattern
`usage-service`'s main.go uses for the relay goroutine (`services/usage-service/cmd/server/main.go`
around its "Wait for the outbox relay goroutine... to observe ctx" comment) —
verify that exact line before copying, since this task's own grep for it
only confirmed the comment's existence, not its full body.

`repo` here must satisfy `outbox.Store` (`FetchUnpublished`/
`MarkPublished`) — TASK-PW-04-02 already added both methods to
`task-service`'s `Repository`, so no adapter shim is needed.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/task-service/...
go run ./services/task-service/cmd/server &
sleep 2
# Confirm the TASK stream now exists in JetStream (requires a local NATS
# instance per this repo's dev-environment docs):
nats stream info TASK 2>&1 | head -20
kill %1
```

Expected: clean build; service starts without failing even if NATS is
down; when NATS is up, the `TASK` stream is created with subject filter
`orca.task.>`.
