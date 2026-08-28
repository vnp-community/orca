# BUG-MB-02: Push notification pipeline exists for generic domain events, but never for agent lifecycle/rate-limit events, never encrypted, and never actually delivered to a mobile device

**Business Logic:** [BL-MB-02](../../../../docs/logic/mobile-companion/BL-MB-02-push-notification.md) — Gửi Push Notification khi Agent có Sự kiện
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** High
**Symptom:** Carlos/Sam never get a mobile push when an agent completes, errors, waits for input, or hits a provider rate limit — none of those four events are ever published onto the bus notification-service listens to. Even for the domain events that ARE wired (task/workflow/automation completion), a subscribed mobile app registered via `ChannelIOS`/`ChannelAndroid` never actually receives anything: the code path that would send an APNs/FCM push is unimplemented. Only a live, foregrounded WebSocket subscriber gets the notification, and it always arrives as plaintext JSON, never E2E-encrypted.

---

## Spec summary

When an agent completes, errors, starts waiting for input, or a provider rate-limits, desktop must push an encrypted notification to mobile within 5 seconds if the mobile is online, or buffer it (max 50) for delivery on reconnect if offline. Notifications must be encrypted with the BL-MB-01 shared secret and be configurable per event type.

## What backend-go has

`notification-service` has a real, working pipeline for a *different* set of trigger events:

- `internal/adapter/eventbus/consumer.go:45-50` subscribes to exactly six subjects: `orca.task.task.completed`, `orca.workflow.execution.completed`/`.failed`, `orca.automation.run.completed`, `orca.credential.credential.rotated`, `orca.orchestration.decision_gate.opened`.
- `internal/usecase/handle_incoming_event.go:62-90` dedupes (`MarkProcessed`) and translates each delivered event via `domain.TranslateEvent` (`internal/domain/notification_event.go:141-176`), then calls `broadcaster.Broadcast`.
- `internal/adapter/broadcaster/broadcaster.go:91-104` fans the event out, in-process, to every `StreamNotifications` gRPC subscriber for that tenant+user.
- `internal/adapter/grpc/server.go:81-109` (`StreamNotifications`) is a real server-streaming RPC api-gateway opens one-per-connected-client.
- `api-gateway`'s `internal/adapter/wscompat/channels_push.go:44-67` (`registerNotificationStreamChannel`) and `push_bridge.go:38-52` (`pipePush`) genuinely deliver these frames over the legacy `/ws` protocol to a live browser/mobile WS client as a `{type:"push",...}` frame — this closes the gap `BUG-035` (missing-v1) previously documented for the transport primitive itself.
- Subscription lifecycle (register/unregister) is real: `internal/usecase/subscribe.go:40-62` and `internal/usecase/unregister_push_subscription.go`, backed by Postgres (`internal/adapter/postgres/repository.go`).

## What's missing

- **None of the spec's four trigger events are ever published.** `infra-fleet-service` (which owns PTY/agent execution — `internal/adapter/devserveragent/methods.go`) never calls `eventbus.Publish`/`PublishEvent` anywhere (`grep -rln "PublishEvent\|bus.Publish\|eventbus.Event{" backend-go/services/infra-fleet-service` returns nothing). "Agent waiting for input" isn't even detectable: `internal/adapter/devserveragent/methods.go:191,210` sets `ReadyForInput` literally equal to `AgentRunning` — the code comment admits it "genuinely can't be" distinguished today. `ai-provider-service` has no rate-limit event publishing either (same empty grep). So BL-MB-02's actual trigger table (agent completed/error/waiting, rate limit) has zero wiring end to end — the six wired subjects are unrelated generic job-completion events (tasks/workflows/automations/credentials/decision gates), not agent-turn or provider-rate-limit events.
- **No actual mobile push delivery.** `internal/adapter/grpc/server.go:30-34`: `signer is wired for the future DeliverPush usecase (mobile push delivery via APNs/FCM, ...) — not yet called from any RPC path in this scaffold.` There is no `deliver_push.go` usecase, no APNs/FCM client anywhere in the repo (`grep -rliE "webpush|apns|fcm|SendPush" backend-go` finds nothing but the doc-comment references above). `domain.NotificationEvent.Channels` tags every rule with `ChannelDeliveryPush` (`internal/domain/notification_event.go:100,104,108,112,119,123`) but nothing ever consumes that channel to actually send — only `ChannelDeliveryWS` (live broadcast) is implemented. A mobile app that is backgrounded/killed (the exact case push notifications exist for) gets nothing.
- **No encryption (BR-MB-05).** `toProtoFrame`/`framePayloadJSON` (`internal/adapter/grpc/server.go:111-117`) sends plaintext JSON; there is no shared-secret encryption step anywhere in the path, consistent with BL-MB-01 (pairing/shared-secret) not existing at all (see BUG-MB-01).
- **No offline buffering (BR-MB-07).** `broadcaster.go`'s doc comment (`internal/adapter/broadcaster/broadcaster.go:84-90`) is explicit: "no offline WS replay queue" — a recipient with no active subscription simply never receives the event, not buffered up to 50 for later delivery.
- **No per-event-type notification settings (BR-MB-08).** No preference table or filter exists; `notification_event.go:120-121`'s own comment for the credential-rotated rule admits "this scaffold has no preference filter at all yet."

## See also

- `specs/backend-go/bugs/missing-v1/BUG-035-ws-server-push-not-implemented.md` — documented the general server→client push transport gap; `channels_push.go`/`push_bridge.go` (found in this audit) show that gap is now closed for the `notifications.subscribe` channel specifically, so BUG-035 should be considered resolved for this call site even though it may still apply elsewhere.
- `specs/backend-go/bugs/missing-v1/BUG-003-web-push-endpoints-path-and-auth-mismatch.md` — documents the HTTP route/path/auth mismatches on the same `Subscribe`/`GetVapidPublicKey` surface this bug also touches; that bug is about wiring correctness, this one is about the business flow never firing for agent events and never completing actual device delivery.
- `specs/backend-go/bugs/logic-v1/BUG-MB-01-pair-device-not-implemented.md` — root cause of the missing encryption (no shared secret exists to encrypt with).

## References

- `backend-go/services/notification-service/internal/adapter/eventbus/consumer.go:45-50` — the six wired subjects (none are agent-lifecycle/rate-limit events)
- `backend-go/services/notification-service/internal/usecase/handle_incoming_event.go:62-90` — real dedup + translate + broadcast path
- `backend-go/services/notification-service/internal/domain/notification_event.go:100-132,141-176` — `TranslateEvent`, `subjectRules`, `ChannelDeliveryPush` tagging with no consumer
- `backend-go/services/notification-service/internal/adapter/broadcaster/broadcaster.go:84-104` — in-process-only fan-out, explicitly "no offline WS replay queue"
- `backend-go/services/notification-service/internal/adapter/grpc/server.go:30-34,81-117` — `signer` wired but unused; `StreamNotifications`; plaintext `toProtoFrame`
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go:44-67`, `push_bridge.go:38-52` — real WS push delivery to a live subscriber
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:191,210` — `ReadyForInput` hard-coded equal to `AgentRunning`, no real "waiting" detection
- `backend-go/services/notification-service/internal/domain/push_subscription.go:19-20` — `ChannelIOS`/`ChannelAndroid` enum values with no sender behind them
