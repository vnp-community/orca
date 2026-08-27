# SOL-MB-03: Add a mobile-authenticated dispatch channel with idle-gating, queueing, validation, and overwrite confirmation

**Resolves:** [BUG-MB-03](../BUG-MB-03-remote-dispatch-not-implemented.md)
**Service:** `infra-fleet-service` (business rules — gating/queueing/validation)
+ `api-gateway` (mobile-reachable `wscompat` channel, E2E decrypt)
**Affected files (proposed):**
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (new RPCs: `DispatchPrompt`, `GetQueuedPrompt`)
- `backend-go/services/infra-fleet-service/internal/domain/queued_prompt.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/dispatch_prompt.go` (new)
- `backend-go/services/infra-fleet-service/internal/usecase/attach_pty.go` (extend — drain queue on ready transition, reuses SOL-MB-02's quiescence signal)
- `backend-go/services/infra-fleet-service/internal/adapter/postgres/queued_prompt_repository.go` (new)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_mobile_dispatch.go` (new)
- `backend-go/services/api-gateway/internal/adapter/grpc-client/authclient/` (reuse — `ResolveDeviceSharedSecret`, SOL-MB-01)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### Reuse `terminal.send`'s PTY-write primitive; do not fork it

BUG-MB-03 correctly identifies `terminal.send`
(`backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:283-304`)
as "the only PTY-input-injection primitive in backend-go," writing
`in.Data` straight into the PTY stream with no gating. This solution does
**not** duplicate that write path — it adds a new `wscompat` channel
(`mobile.dispatch`) that, after decrypting and validating the mobile
payload, calls a **new** `infra-fleet-service` RPC (`DispatchPrompt`) which
itself performs the gate/queue decision and, only when clear to proceed,
performs the identical `PtyClientFrame_Input` write `terminal.send`
performs today. The business-rule logic (BR-MB-09..12) belongs in
`infra-fleet-service`, not `api-gateway`, per `api-gateway.md` §2's "pure
edge, zero business logic" boundary — an `if` deciding "is this agent ready
to receive input" is exactly the kind of decision that doc says does not
belong in the gateway.

### Why `infra-fleet-service`, not a new "dispatch" concept elsewhere

`infra-fleet-service.md` already owns "Terminal/PTY session **routing** —
which `ptyId` belongs to which connection, and dispatching spawn/write/resize/kill
calls to the right place" (`infra-fleet-service.md:28-30`) and already
exposes `RouteTerminalWrite`
(`infra-fleet-service.md:125-129`) as a coordination-only RPC for input
routing. `DispatchPrompt` is a business-rule-aware sibling of that existing
RPC group — it decides *whether and when* to route a write, using the
`GetTerminalAgentStatus`/`ReadyForInput` signal this service already
computes (extended by SOL-MB-02's quiescence fix) — not a capability that
belongs in a different service. This mirrors `ResolveConnection`'s status
as "the single call every... branch in the whole system reduces to"
(`infra-fleet-service.md:439-444`): `DispatchPrompt` is the same shape of
single decision point, scoped to one more concern (agent readiness) this
service already tracks.

### Mobile transport reuses `wscompat`'s existing Identity path, not a new WS server

`api-gateway.md` §9 already validates "short-lived RS256 JWT... for
mobile/CLI" the same way it validates a browser session cookie, producing
the same resolved `Identity` every `wscompat` channel handler receives
(`api-gateway.md:290-298`). SOL-MB-01's pairing flow issues exactly that
JWT. This solution therefore does **not** need a second WS listener or
protocol — a mobile client authenticated via its paired-device JWT can open
the same `/ws` connection every browser client does and call a
mobile-specific channel, gated by requiring the request body be a
`box`-sealed envelope keyed to that JWT's associated `device_id` (carried
in the JWT's claims, minted by `CompleteDevicePairing`/`IssueToken` in
SOL-MB-01). This is the minimal-new-surface option consistent with
`api-gateway.md` §6's `usecase/`-is-thin, `adapter/`-is-everything shape —
one more `wscompat` channel file, not a new bridging mechanism.

### `ReadyForInput` dependency on SOL-MB-02

BR-MB-09's idle/waiting gate needs a real `ReadyForInput` signal — today
hard-coded equal to `AgentRunning`
(`backend-go/services/infra-fleet-service/internal/adapter/devserveragent/methods.go:191,210`,
cited identically by both BUG-MB-02 and BUG-MB-03). This solution consumes
SOL-MB-02's quiescence-based improvement rather than re-solving it — a
genuine cross-bug dependency, called out explicitly rather than silently
assuming a better signal exists.

---

## Design — proto additions (`orca.infra.v1`)

```protobuf
service InfraFleetService {
  // ... existing RPCs unchanged, including RouteTerminalWrite ...

  // DispatchPrompt is the ONE decision point BR-MB-09/10/12 all reduce to:
  // gate on agent readiness, queue if running, require confirmation to
  // overwrite an existing queued prompt.
  rpc DispatchPrompt(DispatchPromptRequest) returns (DispatchPromptResponse);
  rpc GetQueuedPrompt(GetQueuedPromptRequest) returns (GetQueuedPromptResponse);
}

message DispatchPromptRequest {
  string pty_id = 1;
  string prompt = 2;          // already decrypted by the caller (api-gateway) before this RPC — see wiring
  bool   overwrite = 3;       // BR-MB-12: true only on a caller's explicit confirmation of a second dispatch
  string dispatched_by_device_id = 4; // audit/attribution — which paired mobile device sent this
}
message DispatchPromptResponse {
  enum Outcome {
    OUTCOME_UNSPECIFIED = 0;
    INJECTED_IMMEDIATELY = 1;  // BR-MB-09: agent was idle/waiting, written straight to the PTY
    QUEUED = 2;                // BR-MB-10: agent running, held for later
    REJECTED_NEEDS_CONFIRMATION = 3; // BR-MB-12: a prompt is already queued and overwrite=false
  }
  Outcome outcome = 1;
  string  existing_queued_prompt_preview = 2; // populated only for REJECTED_NEEDS_CONFIRMATION — first N chars, for the mobile UI's confirmation dialog
}

message GetQueuedPromptRequest { string pty_id = 1; }
message GetQueuedPromptResponse {
  bool   has_queued_prompt = 1;
  string prompt = 2;
  int64  queued_at_unix_ms = 3;
}
```

## Design — domain

```go
// internal/domain/queued_prompt.go
const MaxPromptLength = 10_000 // BR-MB-11

var (
    ErrPromptTooLong          = errors.New("domain: prompt exceeds 10,000 characters")
    ErrPromptEmpty            = errors.New("domain: prompt is empty")
    ErrQueuedPromptExists     = errors.New("domain: a prompt is already queued for this pty — confirmation required")
)

type QueuedPrompt struct {
    PtyID             string
    TenantID          string
    Prompt            string
    DispatchedByDeviceID string
    QueuedAt          time.Time
}

// NewQueuedPrompt enforces BR-MB-11 at construction — the same "invariant
// lives in the constructor" convention every other domain type in this
// codebase follows (e.g. Worktree, PairedDevice).
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

## Design — data model (`infra` schema — extends `infra-fleet-service.md` §5)

```sql
-- One queued prompt per pty_id at a time — BR-MB-12's "overwrite requires
-- confirmation" rule is enforced by this being a single row per pty_id,
-- not a list: a second INSERT without overwrite=true must fail/be
-- rejected by the usecase before ever reaching this table.
CREATE TABLE infra.queued_prompts (
    pty_id                   UUID PRIMARY KEY REFERENCES infra.terminal_sessions(pty_id) ON DELETE CASCADE,
    tenant_id                UUID NOT NULL,
    prompt                   TEXT NOT NULL,
    dispatched_by_device_id  UUID,     -- logical FK -> auth.paired_devices.id (SOL-MB-01); NULL if dispatched from a non-mobile authenticated session
    queued_at                TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Persisted (not in-memory) deliberately: a queued prompt must survive until
the agent becomes ready, which — per §8's own flagged constraint that a
`connectionId`'s live transport lives on exactly one pod — could outlast
the pod that received the original `DispatchPrompt` call, or even a pod
restart. Durable storage is the correct choice here in a way the per-pod
quiescence *state* (SOL-MB-02) is not, since a queued prompt is a real
pending business fact, not a cache.

## Design — usecase

```go
// internal/usecase/dispatch_prompt.go
type DispatchPrompt struct {
    sessions TerminalSessionRepository
    resolver ConnectionResolver
    agent    DevServerAgentClient
    queue    QueuedPromptRepository
    liveStates *ptyLiveStateRegistry // SOL-MB-02's quiescence tracker, same in-process registry
}

func (uc *DispatchPrompt) Execute(ctx context.Context, in DispatchPromptInput) (DispatchOutcome, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    prompt, err := domain.NewQueuedPrompt(in.PtyID, tenantID, in.Prompt, in.DeviceID, time.Now()) // BR-MB-11 validated here
    if err != nil {
        return DispatchOutcome{}, err
    }

    session, devServer, err := resolveTerminalSession(ctx, tenantID, in.PtyID, uc.sessions, uc.resolver)
    if err != nil {
        return DispatchOutcome{}, err
    }

    status, _ := uc.agent.AgentStatus(ctx, devServer, in.PtyID) // best-effort, same degrade-to-false convention as GetTerminalAgentStatus

    existing, hasExisting, err := uc.queue.Get(ctx, in.PtyID)
    if hasExisting && !in.Overwrite { // BR-MB-12
        return DispatchOutcome{Outcome: REJECTED_NEEDS_CONFIRMATION, ExistingPreview: preview(existing.Prompt, 200)}, nil
    }

    idle := !status.AgentRunning || status.ReadyForInput // BR-MB-09: "idle" (no agent) or "waiting" (ready) both qualify
    if idle {
        if err := uc.writeToPty(ctx, session, prompt.Prompt); err != nil {
            return DispatchOutcome{}, err
        }
        uc.queue.Delete(ctx, in.PtyID) // clears any stale queued entry now that we injected directly
        return DispatchOutcome{Outcome: INJECTED_IMMEDIATELY}, nil
    }

    // BR-MB-10: agent running — queue instead of dropping or rejecting.
    if err := uc.queue.Upsert(ctx, prompt); err != nil {
        return DispatchOutcome{}, err
    }
    return DispatchOutcome{Outcome: QUEUED}, nil
}

func (uc *DispatchPrompt) writeToPty(ctx context.Context, session domain.TerminalSession, prompt string) error {
    // Identical PtyClientFrame_Input write terminal.send performs today —
    // reused via the same DevServerAgentClient port, not duplicated logic.
    return uc.agent.WritePtyInput(ctx, session, []byte(prompt))
}
```

**Draining the queue.** `AttachPty`'s ready-transition hook (SOL-MB-02's
`if result.ReadyForInput && !wasReady` branch in `GetTerminalAgentStatus`,
mirrored in `AttachPty`'s own quiescence check) additionally checks
`uc.queue.Get(ctx, ptyID)` on the same transition and, if a queued prompt
exists, calls the same `writeToPty` path and deletes the queue row — this
is the mechanism that actually delivers a BR-MB-10-queued prompt once the
agent frees up, not a separate poller.

## Design — wiring (`wscompat`)

```go
// channels_mobile_dispatch.go
type mobileDispatchArgs struct {
    PtyID          string `json:"ptyId"`
    EncryptedBody  string `json:"encryptedBody"` // base64 NaCl box/secretbox ciphertext
    Nonce          string `json:"nonce"`
    Overwrite      bool   `json:"overwrite"`
}

func registerMobileDispatchChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient, devices authclient.DeviceSecretResolver) {
    r.Register("mobile.dispatch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        in, err := decodeArg[mobileDispatchArgs](args, 0)
        if err != nil {
            return nil, err
        }
        // id.DeviceID: threaded through from the JWT claims SOL-MB-01's
        // CompleteDevicePairing/IssueToken mints — mobile Identity carries
        // a device_id, a browser session's Identity does not (mobile.*
        // channels are unreachable from a plain browser session).
        if id.DeviceID == "" {
            return nil, errNotAMobileSession
        }
        secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID) // SOL-MB-01's internal RPC
        if err != nil {
            return nil, err
        }
        prompt, err := unseal(in.EncryptedBody, in.Nonce, secret) // BR-MB-05-equivalent decrypt step BUG-MB-03 flagged as entirely missing
        if err != nil {
            return nil, fmt.Errorf("wscompat: decrypting mobile dispatch payload: %w", err)
        }

        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.DispatchPrompt(ctx, &infrafleetv1.DispatchPromptRequest{
            PtyId: in.PtyID, Prompt: prompt, Overwrite: in.Overwrite, DispatchedByDeviceId: id.DeviceID,
        })
        if err != nil {
            return nil, err
        }
        return dispatchOutcomeView{
            Outcome: resp.GetOutcome().String(),
            ExistingQueuedPromptPreview: resp.GetExistingQueuedPromptPreview(),
        }, nil
    })
}
```

`registerMobileDispatchChannel` is called from a new `RegisterMobileChannels(r, ...)`
grouping in `cmd/server/main.go`'s composition root, parallel to
`RegisterRealChannels`/`RegisterPushChannels` — kept in its own file per
this codebase's established convention (`channels_push.go`'s own doc
comment: "Kept in this SEPARATE file... so this pass's edits never touch
the shared, high-churn `channels.go`").

## Test plan

- `queued_prompt_test.go` (domain) — `NewQueuedPrompt` rejects empty and
  >10,000-char prompts (BR-MB-11), accepts exactly 10,000.
- `dispatch_prompt_test.go`:
  - Agent not running (idle) → `INJECTED_IMMEDIATELY`, PTY write called,
    no queue row written (BR-MB-09).
  - Agent running, `ReadyForInput=false`, no existing queue entry →
    `QUEUED`, PTY write NOT called (assert fake `WritePtyInput` uncalled)
    (BR-MB-10).
  - Agent running, existing queued prompt, `overwrite=false` →
    `REJECTED_NEEDS_CONFIRMATION`, existing row unchanged, preview matches
    the existing prompt's first 200 chars (BR-MB-12).
  - Same scenario with `overwrite=true` → existing row replaced.
  - Agent transitions running→ready with a queued prompt present → queue
    drained via the `AttachPty` ready-transition hook, PTY write called
    exactly once, queue row deleted afterward (regression guard against a
    double-delivery race between the hook and a concurrent `DispatchPrompt`
    call — assert via a fake queue repository's atomic
    `GetAndDelete`-style call).
- `channels_mobile_dispatch_test.go`:
  - A session with no `DeviceID` (plain browser JWT) is rejected before
    any decrypt attempt (assert `ResolveSharedSecret` not called).
  - Malformed/garbage ciphertext → decode error, `DispatchPrompt` RPC never
    called (assert fake gRPC client received zero calls) — regression
    guard mirroring SOL-009's "known gap, don't attempt anyway" test
    pattern.
  - Valid encrypted payload round-trips to a `DispatchPromptRequest` with
    the correctly decrypted plaintext prompt.
- Integration: a full pairing (SOL-MB-01 fixture) → dispatch → PTY receives
  bytes end-to-end test, exercising the real NaCl seal/unseal boundary
  between the two solutions rather than only unit-level fakes.

## References

- `specs/backend-go/bugs/logic-v1/BUG-MB-03-remote-dispatch-not-implemented.md` — problem statement and line citations
- `docs/logic/mobile-companion/BL-MB-03-remote-dispatch.md` — flow, BR-MB-09..12
- `specs/backend-go/tdd/services/infra-fleet-service.md:14-37` (§1 ownership incl. "Terminal/PTY session routing"), `:117-137` (§3 `RouteTerminalWrite`/`ResolveConnection` precedent this RPC's shape follows), `:439-444` (single-decision-point framing `DispatchPrompt` mirrors), `:446-483` (§8 per-pod connection ownership caveat)
- `specs/backend-go/tdd/services/api-gateway.md:27-63` (§2 pure-edge/zero-business-logic boundary — why gating logic is NOT in `wscompat`), `:284-298` (§9 mobile JWT validation, identity propagation), `:138-166` (§6 package layout, thin `usecase/`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:283-304` — `terminal.send`, the PTY-write primitive this solution reuses rather than forks
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go:1-9` — separate-file convention this solution's new channel file follows
- `specs/backend-go/bugs/logic-v1/BUG-MB-01-pair-device-not-implemented.md`, `SOL-MB-01-pair-device.md` — the mobile JWT/`device_id`/shared-secret this solution's transport and decrypt step depend on
- `specs/backend-go/bugs/logic-v1/BUG-MB-02-push-notification-partial.md`, `SOL-MB-02-push-notification.md` — the `ReadyForInput` quiescence signal BR-MB-09's gate depends on
