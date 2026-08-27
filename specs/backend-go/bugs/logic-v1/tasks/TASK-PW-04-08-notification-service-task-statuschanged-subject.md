# TASK-PW-04-08: `notification-service` learns `orca.task.task.statuschanged`

**From Solution:** SOL-PW-04
**Priority:** P2
**Service:** `notification-service`
**File:** `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go`, `backend-go/services/notification-service/internal/domain/notification_event.go`
**Depends on:** TASK-PW-04-04
**Status:** `[x]` DONE — added TASK-stream binding + WS-only/SeverityInfo subjectRules entry for orca.task.task.statuschanged; TestTranslateEvent_TaskStatusChanged passes; existing subject tests unaffected

---

## Context

**Grounding correction versus SOL-PW-04's own prose**: SOL-PW-04's Design
section claims "the only change needed [in notification-service] is
`domain.TranslateEvent` learning the two new subjects." That is only
half true — verified by reading `consumer.go`'s `Subjects` table and
`notification_event.go`'s `subjectRules` map directly:

- `orca.workflow.execution.completed`/`.failed` **already have** both a
  `SubjectBinding` (`consumer.go:46-47`) and a `subjectRules` entry
  (`notification_event.go:102-109`) — TASK-PW-04-06 alone makes this
  consumer start receiving real data; **no notification-service code
  change is needed for the workflow subject**.
- `orca.task.task.completed` **already has** both a binding
  (`consumer.go:45`) and a rule — not present in this file's grep for
  it until TASK-PW-04-03 was written, so this needed checking, not
  assuming; TASK-PW-04-03's `UpdateTask` already publishes this subject
  on a transition into `done`, so this one also needs no
  notification-service change.
- `orca.task.task.statuschanged` (the general, every-transition subject
  TASK-PW-04-03 publishes for `api-gateway`'s bridge) has **neither** a
  binding nor a rule — this is this task's actual, narrower scope.

## Changes to make

In `internal/adapter/eventbus/consumer.go`'s `Subjects` table:

```go
var Subjects = []SubjectBinding{
	{StreamName: "TASK", Subject: "orca.task.task.completed"},
	{StreamName: "TASK", Subject: "orca.task.task.statuschanged"}, // added SOL-PW-04
	{StreamName: "WORKFLOW", Subject: "orca.workflow.execution.completed"},
	{StreamName: "WORKFLOW", Subject: "orca.workflow.execution.failed"},
	{StreamName: "AUTOMATION", Subject: "orca.automation.run.completed"},
	{StreamName: "CREDENTIAL", Subject: "orca.credential.credential.rotated"},
	{StreamName: "ORCHESTRATION", Subject: "orca.orchestration.decision_gate.opened"},
}
```

In `internal/domain/notification_event.go`'s `subjectRules` map, decide
deliberately whether `orca.task.task.statuschanged` should surface a
user-facing notification at all — an `open -> in_progress` transition
firing a toast on every single task dispatch is likely noise the
`.completed` subject (already covered) doesn't have. The recommended
default is to route it through WS-only, low-severity so it's available
to any future in-app UI that wants it without becoming a push
notification:

```go
"orca.task.task.statuschanged": {
	Type: "task_status_changed", Title: "Task updated", Body: "",
	Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS}, // WS-only — deliberately no push; see this task's Context note on notification noise
},
```

If product wants this subject to never surface a notification at all
(purely an `api-gateway` bridge signal, not a notification-service
concern), the alternative is to NOT add a `subjectRules` entry and
instead have `TASK-PW-04-03`'s `UpdateTask` skip enqueuing
`orca.task.task.statuschanged` onto the `TASK` stream's subject filter
notification-service's `EnsureStream`/binding would otherwise pick up —
but that would require `task-service`'s `EnsureStream` subject list and
notification-service's binding list to diverge per-subject, which this
codebase's `EnsureStream(ctx, "TASK", []string{"orca.task.>"})` wildcard
(TASK-PW-04-04) does not support cleanly. Flagging both options here
rather than silently picking one — confirm the desired UX with whoever
owns notification-service before merging either path.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/notification-service/...
go test ./services/notification-service/internal/domain/... -run TestTranslateEvent -v
```

Expected: clean build; a new `TestTranslateEvent_TaskStatusChanged` case
(mirroring the existing `TestTranslateEvent`-family tests in
`notification_event_test.go`) asserts the new subject maps to a WS-only,
`SeverityInfo` `NotificationEvent`; existing subject tests are
unaffected (regression guard — this is a pure addition to the map).
