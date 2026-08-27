# TASK-MB-02-05: Wire 4 new subjects into `notification-service`'s `subjectRules` + consumer bindings

**From Solution:** SOL-MB-02
**Priority:** P1
**Service:** `notification-service`
**File:** `backend-go/services/notification-service/internal/domain/notification_event.go`, `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go`
**Depends on:** TASK-MB-02-01, TASK-MB-02-02, TASK-MB-02-04
**Status:** `[x]` DONE — added the 4 `subjectRules` entries + 4 `Subjects` consumer bindings (StreamName `INFRA`/`AIPROVIDER`, matching infra-fleet-service's/ai-provider-service's real `EnsureStream` calls verbatim); `TestTranslateEvent_MobilePushSubjectsMapToRules` covers all 4 subjects; `go build`/`go vet`/`go test` clean.

---

## Context

`subjectRules` (`notification_event.go:97`) is "illustrative, not
exhaustive" by design — adding a subject needs no schema change, only a
new map entry plus a consumer binding. This task adds the four subjects
TASK-MB-02-01/02/04 now actually publish.

## Changes to make

In `internal/domain/notification_event.go`, add to `subjectRules`:

```go
"orca.infra.terminal_session.agent_completed": {
	Type: "agent_completed", Title: "✅ Agent xong", Body: "{agent} đã hoàn thành task.",
	Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
},
"orca.infra.terminal_session.agent_error": {
	Type: "agent_error", Title: "❌ Agent lỗi",
	Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
},
"orca.infra.terminal_session.agent_waiting": {
	Type: "agent_waiting", Title: "⏸ Agent chờ input",
	Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
},
"orca.aiprovider.account.rate_limited": {
	Type: "rate_limited", Title: "⚠️ Rate limit",
	Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
},
```

(Match the actual `subjectRule` struct field names/shape already used by
the existing 6 entries in this map — read the struct definition before
adding these literals verbatim.)

In `internal/adapter/eventbus/consumer.go`, add to `Subjects`:

```go
{StreamName: "INFRA", Subject: "orca.infra.terminal_session.agent_completed"},
{StreamName: "INFRA", Subject: "orca.infra.terminal_session.agent_error"},
{StreamName: "INFRA", Subject: "orca.infra.terminal_session.agent_waiting"},
{StreamName: "AIPROVIDER", Subject: "orca.aiprovider.account.rate_limited"},
```

`StreamName` must match whatever `infra-fleet-service`/`ai-provider-service`
name their own JetStream stream via `EnsureStream` in TASK-MB-02-01/04's
publisher — confirm the exact stream name each service's `cmd/server/main.go`
passes to `EnsureStream` before hardcoding `"INFRA"`/`"AIPROVIDER"` here;
use the same string.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/notification-service/... && go vet ./services/notification-service/...
go test ./services/notification-service/internal/domain/... -run TranslateEvent
```

Test: each of the 4 new subjects translates via `TranslateEvent` to a
`NotificationEvent` with the right `Type`/`Severity`/`Channels` — no
fallback to `defaultRule` needed for them.
