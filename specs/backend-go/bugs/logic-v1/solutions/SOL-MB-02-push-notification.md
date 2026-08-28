# SOL-MB-02: Wire agent-lifecycle/rate-limit events, implement `deliver_push.go`, add offline buffering and per-event-type preferences

**Resolves:** [BUG-MB-02](../BUG-MB-02-push-notification-partial.md)
**Service:** `notification-service` (push delivery, buffering, preferences)
+ `infra-fleet-service` (agent-lifecycle event publishing, `ReadyForInput`
heuristic fix) + `ai-provider-service` (rate-limit event publishing)
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/usecase/attach_pty.go` (extend — quiescence tracking + lifecycle event emission)
- `backend-go/services/infra-fleet-service/internal/usecase/get_terminal_agent_status.go` (extend — real `ReadyForInput`)
- `backend-go/services/infra-fleet-service/internal/adapter/eventbus/publisher.go` (new subjects)
- `backend-go/services/ai-provider-service/internal/usecase/*.go` (extend — rate-limit detection + outbox event)
- `backend-go/services/notification-service/internal/domain/notification_event.go` (extend `subjectRules`)
- `backend-go/services/notification-service/internal/usecase/deliver_push.go` (new — the currently-missing usecase)
- `backend-go/services/notification-service/internal/usecase/notification_preferences.go` (new)
- `backend-go/services/notification-service/internal/adapter/postgres/` (new tables: `buffered_notifications`, `notification_preferences`)
- `backend-go/services/notification-service/internal/adapter/external/webpush/`, `internal/adapter/external/apns/`, `internal/adapter/external/fcm/` (new)
- `backend-go/services/notification-service/internal/adapter/grpc-client/authclient/` (new — calls `auth-service.ResolveDeviceSharedSecret`, SOL-MB-01)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### Three independent gaps, three different owning services

BUG-MB-02 is not one gap — it is the four trigger events never being
published (owned by `infra-fleet-service`/`ai-provider-service`, the
services that observe agent/provider state), plus mobile push delivery
never being implemented (owned by `notification-service`, per
`notification-service.md` §6's `deliver_push.go` already being named in the
package layout as a planned file — `notification-service.md:218`), plus two
business rules (BR-MB-07 offline buffering, BR-MB-08 per-event-type
settings) whose data this service's own doc explicitly says doesn't exist.
Each is designed separately below against its owning service's TDD.

### Trigger events: `infra-fleet-service` already sees the two hard signals

"Agent completed" and "agent error" are not new information
`infra-fleet-service` has to invent — `usecase.PtyServerMessage.Exited`
(seen in `AttachPty`'s existing outbound loop,
`backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go:562-567`)
already tells this service exactly when a PTY process exits and with what
code. This solution's addition is publishing that fact as a domain event
(exit code 0 → completed, nonzero → error), not detecting anything new.

"Agent waiting for input" is the genuinely hard one, and this service's own
code already documents why: `AgentStatus`'s doc comment
(`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:182-196`)
states `ReadyForInput` is "STILL set equal to `AgentRunning`" and "genuinely
can't be improved without an agent-side change beyond this pass's scope" —
the agent's `pty.listProcesses` surface reports only `{id,cwd,title,pid}`,
no busy/idle signal. **This solution does not claim to close that gap
fully** — it proposes the specific in-process improvement the existing
comment itself gestures at ("a raw-output-quiescence timer... not just
wiring"), using data `infra-fleet-service` already has without any `agent/`
change: `AttachPty`'s `Execute` already relays every `PtyServerMessage.Output`
chunk (`server.go:476-510`); tracking the last-output timestamp per
`ptyId` in that same loop and treating "no output for N seconds while
`AgentRunning`" as `ReadyForInput` is strictly better than the current
identity mapping, using only data already flowing through this process. A
fully accurate signal (an explicit prompt-detection marker) needs a new
`agent/` RPC — **flagged explicitly as out of backend-go's reach alone**,
per the task's instruction to call this out.

### Rate-limit events: `ai-provider-service` is the natural publisher, not designed here in full

Neither `ai-provider-service.md` nor this bug's own investigation names an
existing rate-limit-classification code path in enough detail to cite
line-by-line — the bug report's grep (`BUG-MB-02:28`) confirms zero existing
publish call sites. This solution specifies the **event contract**
(`orca.aiprovider.account.rate_limited`, payload shape below) that
`notification-service`'s consumer side needs, and the integration point
(wherever `ai-provider-service` already classifies a provider response as
rate-limited, e.g. HTTP 429 handling in its provider-call path) — the exact
call site is `ai-provider-service`'s own scope to locate and is not
re-derived here since it is outside this bug's and this doc set's assigned
TDD reading list.

### `deliver_push.go`: a genuine new external-integration surface, not just wiring

`notification-service.md` §6 already names `deliver_push.go` and
`adapter/external/webpush/` in its planned package layout
(`notification-service.md:218,227`) and §9 states delivery is
"VAPID-signed Web Push" through `credential-broker-service`'s Transit
signing (`notification-service.md:249-254,278-293`) — so the *shape* of
this work is scoped by the TDD. What the TDD does **not** resolve, and this
solution must, is a real imprecision: §1's diagram
(`notification-service.md:81-84`) labels the push path uniformly
`"VAPID-signed Web Push" -> APNs/FCM`, but native **APNs** and **FCM** each
require their *own* provider-specific auth distinct from VAPID — APNs a
provider JWT signed with an ES256 `.p8` key, FCM an OAuth2 service-account
token — not the Web Push protocol's VAPID JWT at all. VAPID only applies to
the `web` channel (browser Push API endpoints). This solution resolves the
imprecision by treating `ios`/`android` as their own credential class,
mediated through `credential-broker-service` the same way (never a raw
signing key in this service, consistent with §9's headline property) but
via **two additional** Transit-backed credentials (an APNs signing key, an
FCM service-account key), not a reuse of the VAPID key for a purpose it
cannot serve. **Real APNs/FCM delivery is a new external integration this
solution scopes but does not consider "just wiring"** — it requires
provisioning real Apple/Google push credentials in Vault, which is an
operational/product prerequisite outside backend-go code, called out
explicitly per the task's instruction.

### BR-MB-08 (per-event-type preferences) directly contradicts `notification-service.md` §2's stated non-goal — flagged, not silently overridden

`notification-service.md` §2 states: "**Explicitly out of scope**:
per-user notification preferences (which event types a user wants, per
channel). Nothing in the TS surface models this as first-class data today,
so no table is invented for it here" (`notification-service.md:48-50`).
BL-MB-02's BR-MB-08 requires exactly this: "Notification settings có thể
cấu hình per-event-type." This is a real conflict between the TDD's stated
scope and the business rule this bug must close — **this solution adds the
table §2 says doesn't exist**, and flags this explicitly as an amendment
`notification-service.md` §2 should reconcile (the TDD's stated reason —
"nothing in TS models this" — was true for the *legacy* surface but is no
longer true once BL-MB-02 is treated as in-scope business logic), rather
than silently building it as if §2 had never said otherwise.

---

## Design — `infra-fleet-service`: publish agent-lifecycle events + fix `ReadyForInput`

```go
// internal/usecase/attach_pty.go (extended)
// ptyLiveState is a per-pod, in-memory map[ptyID]*ptyLiveState — same
// per-pod-ownership caveat infra-fleet-service.md §8 already flags for
// AttachPty's live connection pooling ("a given connectionId's live
// transport lives on exactly one pod at a time"); this quiescence state
// inherits that constraint rather than introducing a new one. A
// GetTerminalAgentStatus call landing on a DIFFERENT pod than the one
// running the live AttachPty stream for that ptyId sees no quiescence data
// and falls back to today's AgentRunning-equals-ReadyForInput behavior —
// an honest degrade, not a wrong answer.
type ptyLiveState struct {
    lastOutputAt time.Time
    agentRunning bool
}

const readyForInputQuiescence = 3 * time.Second // heuristic threshold; tunable, not a business rule

// In AttachPty.run's existing outbound relay loop, on every non-empty
// Output chunk:
func (a *AttachPty) onOutputChunk(ptyID string, chunk []byte) {
    a.liveStates.Store(ptyID, &ptyLiveState{lastOutputAt: time.Now(), agentRunning: true})
}

// On Exited:
func (a *AttachPty) onExited(ctx context.Context, ptyID string, exitCode int32) {
    a.liveStates.Delete(ptyID)
    eventType := "orca.infra.terminal_session.agent_completed"
    if exitCode != 0 {
        eventType = "orca.infra.terminal_session.agent_error"
    }
    a.outbox.Enqueue(ctx, domain.OutboxEvent{
        ID: uuid.NewString(), Subject: eventType, OccurredAt: time.Now(),
        PayloadJSON: mustJSON(agentLifecyclePayload{PtyID: ptyID, ExitCode: exitCode}),
    }) // same outbox mechanism infra-fleet-service.md §7 already uses for connection.established/.lost
}
```

```go
// internal/usecase/get_terminal_agent_status.go (extended)
func (uc *GetTerminalAgentStatus) Execute(ctx context.Context, ptyID string) (AgentStatusResult, error) {
    // ... unchanged resolveTerminalSession + uc.agent.AgentStatus(...) call ...
    if result.AgentRunning {
        if live, ok := uc.liveStates.Load(ptyID); ok {
            wasReady := result.ReadyForInput
            result.ReadyForInput = time.Since(live.(*ptyLiveState).lastOutputAt) > readyForInputQuiescence
            if result.ReadyForInput && !wasReady {
                // Debounced transition — emit "agent waiting" exactly once
                // per idle period, not on every poll.
                uc.publishAgentWaiting(ctx, ptyID)
            }
        }
        // No live state on this pod (cross-pod caveat above): falls
        // through to AgentStatus's own ReadyForInput=AgentRunning value,
        // unchanged from today.
    }
    return result, nil
}
```

Event payload shape (all three new `infra.*` subjects share it):

```go
type agentLifecyclePayload struct {
    PtyID      string   `json:"pty_id"`
    ConnectionID string `json:"connection_id"`
    AgentKind  string   `json:"agent_kind"`
    ExitCode   *int32   `json:"exit_code,omitempty"` // set only for agent_completed/agent_error
    UserIDs    []string `json:"user_ids"`             // resolved from the session's owning tenant/user — see wiring note below
}
```

**Resolving `user_ids` for the payload**: `TerminalSession` (domain) carries
`TenantID` but no user identity today
(`backend-go/services/infra-fleet-service/internal/domain/terminal_session.go:12-25`)
— PTY sessions aren't currently user-scoped beyond tenant. This solution
threads the creating user's ID through `SpawnTerminalSession`'s existing
input (already available from the WS-bridge's resolved identity per
`api-gateway.md` §8's identity-propagation) into `terminal_sessions`, a
schema extension (`created_by_user_id` column) flagged here as required for
this bug's payload to carry a real recipient, not fabricated.

## Design — `ai-provider-service`: rate-limit event contract

```go
// Published from wherever ai-provider-service classifies a provider
// response as rate-limited (existing error-mapping path, not re-designed
// here — see rationale above).
type rateLimitPayload struct {
    AccountID string  `json:"account_id"`
    Provider  string  `json:"provider"`
    UserID    string  `json:"user_id"`
    ResetAt   *int64  `json:"reset_at_unix_ms,omitempty"` // from the provider's Retry-After if present
}
// Subject: orca.aiprovider.account.rate_limited
```

## Design — `notification-service`: subject rules extension

```go
// internal/domain/notification_event.go — subjectRules gains four entries
"orca.infra.terminal_session.agent_completed": {
    Type: "agent_completed", Title: "✅ Agent xong", Body: "{agent} đã hoàn thành task.", // BR text per BL-MB-02's trigger table
    Severity: SeverityInfo, Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
},
"orca.infra.terminal_session.agent_error": {
    Type: "agent_error", Title: "❌ Agent lỗi", Severity: SeverityWarning,
    Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
},
"orca.infra.terminal_session.agent_waiting": {
    Type: "agent_waiting", Title: "⏸ Agent chờ input", Severity: SeverityInfo,
    Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
},
"orca.aiprovider.account.rate_limited": {
    Type: "rate_limited", Title: "⚠️ Rate limit", Severity: SeverityWarning,
    Channels: []DeliveryChannel{ChannelDeliveryWS, ChannelDeliveryPush},
},
```

`consumer.go`'s subject list (`internal/adapter/eventbus/consumer.go:45-50`)
gains these four subjects alongside the existing six.

## Design — `deliver_push.go` (the currently-missing usecase)

```go
// internal/usecase/deliver_push.go
type DeliverPush struct {
    subscriptions PushSubscriptionRepository
    devices       DeviceSecretResolver   // adapter/grpc-client/authclient — SOL-MB-01's ResolveDeviceSharedSecret
    sealer        E2ESealer              // NaCl secretbox/box, encrypts the payload with the device's shared secret
    vapidSigner   VaultSigner            // existing signer field, wired but unused today (server.go:30-34) — this usecase is what finally calls it, web channel only
    apns          APNsClient             // adapter/external/apns — own credential, NOT VAPID (see rationale)
    fcm           FCMClient              // adapter/external/fcm  — own credential, NOT VAPID
    buffer        BufferedNotificationRepository // BR-MB-07
    preferences   NotificationPreferenceRepository // BR-MB-08
}

func (uc *DeliverPush) Execute(ctx context.Context, event domain.NotificationEvent) error {
    if !hasChannel(event.Channels, domain.ChannelDeliveryPush) {
        return nil
    }
    for _, userID := range event.RecipientUserIDs {
        subs, err := uc.subscriptions.ListByUser(ctx, event.TenantID, userID)
        for _, sub := range subs {
            if sub.Status != domain.SubscriptionActive {
                continue
            }
            allowed, err := uc.preferences.IsEnabled(ctx, event.TenantID, userID, event.Type, sub.Channel) // BR-MB-08
            if err != nil || !allowed {
                continue
            }
            if err := uc.deliverOne(ctx, event, sub); err != nil {
                uc.buffer.Enqueue(ctx, event.TenantID, userID, sub.ID, event, maxBufferedPerUser) // BR-MB-07: caps at 50, evicts oldest on overflow
            }
        }
    }
    return nil
}

func (uc *DeliverPush) deliverOne(ctx context.Context, event domain.NotificationEvent, sub domain.PushSubscription) error {
    plaintext := framePayloadJSON(event) // same shape StreamNotifications already sends over WS
    // BR-MB-05: encrypt before it ever crosses the network — with the
    // BL-MB-01 shared secret, not VAPID (VAPID authenticates the sender to
    // the push service; it does not encrypt the payload body for BR-MB-05's
    // purpose here).
    deviceID, err := uc.subscriptions.DeviceIDFor(ctx, sub.ID) // a push subscription registered from a paired mobile device carries the pairing's device_id
    if err != nil {
        return err // web-channel subscriptions with no paired device: skip E2E-encrypt, deliver via standard Web Push encryption (RFC 8291) only — not a mobile-companion flow
    }
    secret, err := uc.devices.ResolveSharedSecret(ctx, deviceID) // SOL-MB-01's internal-only RPC; secret held only for this call's duration
    ciphertext, nonce, err := uc.sealer.Seal(plaintext, secret)
    switch sub.Channel {
    case domain.ChannelWeb:
        jwt, err := uc.vapidSigner.Sign(ctx, vapidClaims(sub.Endpoint)) // credential-broker-service Transit sign — the signer field this usecase finally uses
        return uc.webpush.Send(ctx, sub.Endpoint, sub.P256dhKey, sub.AuthKey, ciphertext, nonce, jwt)
    case domain.ChannelIOS:
        return uc.apns.Send(ctx, sub.Endpoint, ciphertext, nonce) // APNs provider JWT signed via its own Vault Transit key, mediated the same way
    case domain.ChannelAndroid:
        return uc.fcm.Send(ctx, sub.Endpoint, ciphertext, nonce) // FCM OAuth2 service-account token, mediated the same way
    default:
        return domain.ErrUnsupportedChannel
    }
}
```

## Design — data model additions (`notification` schema)

```sql
-- BR-MB-07: offline buffering, max 50 per (tenant,user,subscription).
CREATE TABLE notification.buffered_notifications (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL,
    user_id        UUID NOT NULL,
    subscription_id UUID NOT NULL REFERENCES notification.push_subscriptions(id) ON DELETE CASCADE,
    notification_event_json JSONB NOT NULL,
    buffered_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at   TIMESTAMPTZ
);
CREATE INDEX idx_buffered_notifications_pending ON notification.buffered_notifications(subscription_id, buffered_at)
    WHERE delivered_at IS NULL;
-- Eviction: enqueue path deletes the oldest row for this subscription_id
-- once count > 50, inside the same insert transaction — no separate
-- reaper needed for the cap itself (only for very old delivered rows).

-- BR-MB-08: per-event-type settings. Flagged addition beyond
-- notification-service.md §2's stated non-goal — see rationale above.
CREATE TABLE notification.notification_preferences (
    tenant_id   UUID NOT NULL,
    user_id     UUID NOT NULL,
    event_type  TEXT NOT NULL,   -- domain.NotificationEvent.Type values, e.g. "agent_completed"
    channel     TEXT NOT NULL CHECK (channel IN ('ws','web','ios','android')),
    enabled     BOOLEAN NOT NULL DEFAULT true,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id, event_type, channel)
);
-- Absence of a row == enabled (default-on), so this table only needs to
-- carry explicit opt-outs, not a full cross-product seed per user.
```

On `StreamNotifications` reconnect (WS side), the gRPC handler additionally
drains `buffered_notifications` for the subscribing user before entering
its live `Subscribe` loop — same delivery frame shape, `delivered_at` set
on send.

## Test plan

- `attach_pty_test.go` — quiescence: a fake PTY stream with output then
  N+1s of silence flips `ReadyForInput` true exactly once; output resuming
  resets it; `Exited{0}` → `agent_completed`, `Exited{1}` → `agent_error`
  published (assert outbox fake received exactly one enqueue with the
  right subject).
- `get_terminal_agent_status_test.go` — no live-state entry (cross-pod
  case) falls back to today's `ReadyForInput == AgentRunning`, unchanged
  behavior — regression guard the fix must not break the existing
  best-effort fallback.
- `notification_event_test.go` — four new subjects each translate via
  `TranslateEvent`, `defaultRule` no longer needed for them.
- `deliver_push_test.go`:
  - Web channel signs via `vapidSigner` (assert called), iOS/Android do
    not (assert `vapidSigner.Sign` NOT called for those channels —
    regression guard against the VAPID/APNs conflation this solution
    resolves).
  - Every channel's outbound call receives ciphertext, never the plaintext
    `NotificationEvent` body (BR-MB-05) — assert on the fake
    APNs/FCM/webpush client's captured argument.
  - Delivery failure → `buffer.Enqueue` called; success → not called.
  - A disabled preference row → `deliverOne` never called for that
    subscription (BR-MB-08).
- `buffered_notifications` repository test — 51st enqueue for one
  subscription evicts the oldest (BR-MB-07's cap), never grows unbounded.
- `StreamNotifications` reconnect integration test — buffered rows for the
  reconnecting user are delivered before/alongside the live stream, each
  marked `delivered_at`.
- `adapter/nacl` E2E-seal round trip — ciphertext produced here decrypts
  correctly with the shared secret SOL-MB-01's pairing flow derives
  (cross-solution consistency check).

## References

- `specs/backend-go/bugs/logic-v1/BUG-MB-02-push-notification-partial.md` — problem statement and line-cited findings
- `docs/logic/mobile-companion/BL-MB-02-push-notification.md` — trigger table, BR-MB-05..08
- `specs/backend-go/tdd/services/notification-service.md:11-30` (§1 two delivery mechanisms), `:81-84` (diagram, the VAPID/APNs imprecision this solution resolves), `:216-227` (§6 package layout naming `deliver_push.go`/`webpush/` as planned), `:248-254,278-306` (§7/§9 mobile-push-is-separate-path, Vault-mediated signing pattern), `:48-50` (§2's preferences non-goal this solution amends)
- `specs/backend-go/tdd/services/infra-fleet-service.md:60-76` (§2 coordination-only boundary — PTY bytes DO pass through this service's process even though it doesn't "own" them, which is what makes the quiescence timer possible), `:446-483` (§8 per-pod connection-ownership caveat this solution's cross-pod fallback inherits)
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:30-45` (event conventions, outbox pattern, idempotent consumers)
- `specs/backend-go/bugs/logic-v1/BUG-MB-01-pair-device-not-implemented.md` and `SOL-MB-01-pair-device.md` — the shared-secret source this solution's E2E encryption depends on
- `backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:182-196` — `ReadyForInput` hard-coded, with the agent-side-change caveat this solution respects
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go:476-567` — `AttachPty`'s existing output/exit relay loop this solution taps into
- `backend-go/services/notification-service/internal/adapter/grpc/server.go:30-34` — `signer` wired but unused, the field this solution's `deliverOne` finally calls
