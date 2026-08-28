# TASK-PRF-03-08: Wire `orca.project.devserver.changed` into `notification-service`'s consumer + subject rules

**From Solution:** SOL-PRF-03
**Priority:** P2 — WS push works via `defaultRule` without this, but title/body are generic until it lands
**Service:** `notification-service`
**File:** `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go`
**Depends on:** TASK-PRF-03-03
**Status:** `[x]` DONE — PROJECT/orca.project.devserver.changed subject binding + subjectRules entry added; consumer_test.go regression guard passes

---

## Context

`RebindDevServer`'s new `orca.project.devserver.changed` event
(TASK-PRF-03-05) already matches `notification-service`'s generic
`EventPayload{UserIDs, Title, Body, DeepLink}` shape exactly
(`notification_event.go`'s `EventPayload`), so it would translate via
`defaultRule` even without a dedicated `subjectRules` entry — but nothing
subscribes to the `PROJECT` stream/subject at all yet, so the event never
reaches this service's consumer in the first place. This task is the one
required cross-service change for the WS push to actually fire, plus a
better title/body than the generic default.

## Changes to make

In `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go`,
extend the static `Subjects` list:

```go
var Subjects = []SubjectBinding{
	{StreamName: "TASK", Subject: "orca.task.task.completed"},
	{StreamName: "WORKFLOW", Subject: "orca.workflow.execution.completed"},
	{StreamName: "WORKFLOW", Subject: "orca.workflow.execution.failed"},
	{StreamName: "AUTOMATION", Subject: "orca.automation.run.completed"},
	{StreamName: "CREDENTIAL", Subject: "orca.credential.credential.rotated"},
	{StreamName: "ORCHESTRATION", Subject: "orca.orchestration.decision_gate.opened"},
	{StreamName: "PROJECT", Subject: "orca.project.devserver.changed"}, // NEW
}
```

In `backend-go/services/notification-service/internal/domain/notification_event.go`'s
`subjectRules` map, add an entry for a better title/body than `defaultRule`
would produce:

```go
"orca.project.devserver.changed": {
	Type: "project_devserver_changed", Title: "Dev server changed",
	Body: "This project's dev server binding was changed.",
	Severity: SeverityWarning, Channels: []DeliveryChannel{ChannelDeliveryWS},
},
```

Match the exact field names/types of `subjectRule`'s struct (`Severity`/
`Channels` shown above are this task's best guess from SOL-PRF-03's citation
— confirm against the real `subjectRule` struct definition in this file
before wiring, since its exact fields weren't independently re-verified for
this task).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/notification-service/...
go test ./services/notification-service/... -v
```

Add a `consumer_test.go` case asserting the new `{StreamName: "PROJECT", ...}`
entry is present in the static subscription list — a regression guard for
this cross-service wiring step, since a missing entry fails silently
(the event is simply never consumed, no error anywhere).
