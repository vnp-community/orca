# SOL-MB-04: Add a mobile-facing worktree+agent status aggregation endpoint with output capture, truncation, and E2E-encrypted transport

**Resolves:** [BUG-MB-04](../BUG-MB-04-mobile-status-not-implemented.md)
**Service:** `project-service` (aggregation — owns `Worktree`, already calls
`infra-fleet-service`) + `infra-fleet-service` (extend `TerminalSession`
with a `lastOutput` capture) + `api-gateway` (mobile-reachable channel,
E2E encrypt)
**Affected files (proposed):**
- `backend-go/proto/orca/project/v1/project.proto` (new RPC: `GetMobileWorktreeStatus`)
- `backend-go/services/project-service/internal/usecase/get_mobile_worktree_status.go` (new)
- `backend-go/services/project-service/internal/usecase/ports.go` (extend — `TerminalStatusResolver`)
- `backend-go/services/project-service/internal/adapter/grpc-client/infrafleet/` (new — thin client for `ListTerminalSessions`/`GetTerminalAgentStatus`)
- `backend-go/proto/orca/infrafleet/v1/infrafleet.proto` (extend `TerminalSession`/`GetTerminalAgentStatusResponse` with a truncated `last_output` field)
- `backend-go/services/infra-fleet-service/internal/usecase/attach_pty.go` (extend — ring-buffer output capture, reuses SOL-MB-02's output relay hook)
- `backend-go/services/infra-fleet-service/internal/domain/terminal_session.go` (extend — `LastOutput` field)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_mobile_status.go` (new — request/response + push channel)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

### Aggregation belongs in `project-service`, not `api-gateway` or a new composed-read service

`api-gateway.md` §2 is explicit: "no cross-service response orchestration —
if a REST call needs data from two services, that composition belongs to
the calling client or **to a service exposing a composed read**, not to a
gateway orchestration layer" (`api-gateway.md:36-39`). BL-MB-04's response
shape (`{worktrees: [{id, name, agent, status, duration, lastOutput}]}`)
needs exactly two services' data: `Worktree` identity/name (`project-service`)
and PTY/agent runtime status (`infra-fleet-service`). `project-service`
is the correct home for the composed read because the dependency edge
already exists in the decomposition graph — `infra-fleet-service.md`'s own
dependency table lists `project-service` as a caller
(`infra-fleet-service.md:354-356`: "validates a `devServerId` exists before
committing a project↔dev-server binding") and its mermaid diagram shows
`proj[project-service] --> infra[infra-fleet-service]`
(`infra-fleet-service.md:394`). Extending that existing edge to also fetch
terminal-session status for the worktrees `project-service` already owns
adds no new edge to the dependency graph — the reverse direction
(`infra-fleet-service` calling `project-service` for worktree names) is
**not** an existing edge and would be, so this solution deliberately picks
the direction that is already there.

### `project-service` already resolves the exact host a worktree needs

`project-service.md` (cited via `infra-fleet-service.md:38-52` and this
bug's own investigation) already threads `Worktree.ProjectID -> Project.DevServerID`
and validates it against `infra-fleet-service`. This solution's aggregation
usecase reuses that same resolution path to find, for a worktree's project,
which `dev_server_id` (if any) it's bound to, then asks
`infra-fleet-service` for that dev server's live terminal sessions — no new
resolution logic invented, just a new consumer of an existing lookup.

### `lastOutput` capture is a genuine new capability, not a wiring gap — but reuses SOL-MB-02's existing hook

BUG-MB-04 correctly finds "no code path snapshots or truncates PTY output
for a status summary at all" — `TerminalSession`
(`backend-go/services/infra-fleet-service/internal/domain/terminal_session.go`)
carries no output field today. This solution does not design a new
output-observation mechanism: SOL-MB-02 already adds a per-`ptyId`
in-process hook on every `AttachPty` output chunk (for the quiescence
timer). This solution's addition to that same hook is trivial by
comparison — append the chunk to a small ring buffer, already touching the
exact same code path — rather than a second, independent tap into the PTY
byte stream. BR-MB-15's 500-char truncation happens at the read side
(`GetTerminalAgentStatus`/the new aggregation call), not the write side, so
the buffer itself can hold a little more than 500 chars (e.g. the last
2,000 bytes) without the truncation-vs-buffering concerns being coupled.

### The worktree↔PTY correlation key: `Worktree.Path` == `TerminalSession.Cwd`

Neither `Worktree` (`project-service`) nor `TerminalSession`
(`infra-fleet-service`) carries an explicit FK to the other today — the
only shared value is a directory path (`Worktree.Path`,
`backend-go/services/project-service/internal/domain/worktree.go:32`, vs.
`TerminalSession.Cwd`,
`backend-go/services/infra-fleet-service/internal/domain/terminal_session.go:16`).
This solution correlates on that shared path rather than inventing a new
FK column, since a PTY session's `cwd` is already set to the worktree's
path at spawn time (per `SpawnTerminalSession`'s existing input, which
`terminal.create` in `wscompat` already threads a `cwd` through). This is
flagged explicitly as a **string-equality correlation, not a foreign key**
— a genuine architectural gap (no `terminal_sessions.worktree_id` column
exists) this solution works around rather than closes; closing it properly
(adding `worktree_id` to `SpawnTerminalSession`'s input and
`terminal_sessions` schema) is a clean, small follow-up this solution
recommends but does not require to satisfy BL-MB-04's response shape.

### Live updates reuse `notification-service`'s existing WS-push mechanism

BR-MB-14 (live update while foregrounded) is the same "server pushes,
`api-gateway` bridges to the client's WS" shape `channels_push.go` already
implements for `notifications.subscribe`
(`backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go:44-67`).
This solution adds a second, parallel `wscompat` stream channel
(`mobile.statusSubscribe`) following that exact `RegisterStream` pattern
rather than inventing a new bridging primitive — `api-gateway.md` §3's WS
endpoint table already treats each real-time surface as "maps to exactly
one owning service" (`api-gateway.md:99-100`); this one maps to
`project-service` (the aggregation owner), pushed on a poll-and-diff
interval rather than a true event stream, since worktree/agent status
changes are not currently published as domain events themselves (only the
lifecycle *transitions* SOL-MB-02 adds are).

---

## Design — proto additions

```protobuf
// orca.project.v1 (project-service)
service ProjectService {
  // ... existing RPCs unchanged ...

  // GetMobileWorktreeStatus is the ONE composed-read call BL-MB-04 reduces
  // to — see rationale above for why this composition lives here.
  rpc GetMobileWorktreeStatus(GetMobileWorktreeStatusRequest) returns (GetMobileWorktreeStatusResponse);
}

message GetMobileWorktreeStatusRequest {} // tenant_id/user_id via metadata, per 08-inter-service-communication.md

message GetMobileWorktreeStatusResponse {
  repeated MobileWorktreeStatus worktrees = 1;
  int64 generated_at_unix_ms = 2; // backs BR-MB-16's client-side "last updated X ago" — the client caches this response and computes the delta itself
}

message MobileWorktreeStatus {
  string id = 1;              // Worktree.ID
  string name = 2;            // Worktree.Branch (matches BL-MB-04's example "fix-auth", "add-tests" — branch names, not raw paths)
  string agent = 3;           // AgentKind, e.g. "claude" | "codex" | "" if none running
  string status = 4;          // "completed" | "running" | "waiting" | "idle" | "unknown"
  int64  duration_ms = 5;     // now - TerminalSession.CreatedAt for a running session; 0/omitted if idle
  string last_output = 6;     // BR-MB-15: truncated to 500 chars server-side, never a raw dump
}
```

```protobuf
// orca.infrafleet.v1 — extend existing messages, additive only
message TerminalSession {
  // ... existing fields unchanged ...
  string last_output_preview = 10; // NOT full output — see truncation note below; next available field number
}
message GetTerminalAgentStatusResponse {
  // ... existing fields unchanged ...
  string last_output_preview = 4;
}
```

## Design — domain (`infra-fleet-service`)

```go
// internal/domain/terminal_session.go (extended)
const lastOutputBufferBytes = 2048 // headroom above BR-MB-15's 500-char truncation, truncation happens at the read boundary, not here

type TerminalSession struct {
    // ... existing fields unchanged ...
    LastOutput []byte // ring-buffer tail of recent PTY output; NOT persisted to Postgres — see storage note
}

func (t *TerminalSession) AppendOutput(chunk []byte) {
    t.LastOutput = append(t.LastOutput, chunk...)
    if len(t.LastOutput) > lastOutputBufferBytes {
        t.LastOutput = t.LastOutput[len(t.LastOutput)-lastOutputBufferBytes:]
    }
}

// TruncatedForMobile applies BR-MB-15's 500-char cap at the point of
// exposure, not storage — keeps the buffer's internal size independent of
// the mobile contract, so a future non-mobile consumer wanting more
// context isn't retroactively capped by this rule.
func (t TerminalSession) TruncatedForMobile() string {
    s := string(t.LastOutput)
    if len(s) <= 500 {
        return s
    }
    return s[len(s)-500:]
}
```

**Storage note**: `LastOutput` is deliberately **not** added to the
`terminal_sessions` Postgres table (`infra-fleet-service.md` §5) — it lives
only in the same per-pod in-memory registry SOL-MB-02's quiescence tracker
already introduces (`ptyLiveState`, extended with a `lastOutput []byte`
field). PTY output is high-volume, ephemeral, and already understood by
this service's design to not be durable state (`TerminalSession`'s own doc
comment: "Holds no PTY bytes" — `infra-fleet-service.md:164`); persisting a
rolling output buffer to Postgres would contradict that existing design
statement. The same cross-pod caveat SOL-MB-02 flags applies here
identically: a status request landing on a different pod than the live
`AttachPty` stream sees an empty `last_output` for that session, an honest
absence rather than stale or fabricated data.

## Design — usecase (`project-service`)

```go
// internal/usecase/get_mobile_worktree_status.go
type GetMobileWorktreeStatus struct {
    worktrees WorktreeRepository
    projects  ProjectRepository
    terminals TerminalStatusResolver // adapter/grpc-client/infrafleet
}

// TerminalStatusResolver is the one new port this usecase needs — kept
// narrow (just the two infra-fleet-service RPCs this call actually uses)
// per ports.go's existing one-narrow-interface-per-dependency convention.
type TerminalStatusResolver interface {
    ListSessionsForDevServer(ctx context.Context, devServerID string) ([]infrafleetv1.TerminalSession, error)
}

func (uc *GetMobileWorktreeStatus) Execute(ctx context.Context) (MobileStatusResult, error) {
    tenantID, err := tenant.RequireTenantID(ctx)
    worktrees, err := uc.worktrees.ListActive(ctx, tenantID) // existing repository method, active worktrees only

    // Group by ProjectID -> DevServerID so ListSessionsForDevServer is
    // called once per distinct dev server, not once per worktree.
    byDevServer := map[string][]domain.Worktree{}
    for _, wt := range worktrees {
        project, err := uc.projects.Get(ctx, tenantID, wt.ProjectID)
        if err != nil || project.DevServerID == "" {
            continue // no bound dev server: worktree has no runtime status to report, not an error
        }
        byDevServer[project.DevServerID] = append(byDevServer[project.DevServerID], wt)
    }

    out := make([]MobileWorktreeStatus, 0, len(worktrees))
    for devServerID, wts := range byDevServer {
        sessions, err := uc.terminals.ListSessionsForDevServer(ctx, devServerID)
        if err != nil {
            // A degraded dev server shouldn't fail the whole response —
            // its worktrees show status "unknown", same "honest absence"
            // convention InspectTerminalProcess/GetTerminalAgentStatus
            // already use elsewhere in this system.
            for _, wt := range wts {
                out = append(out, MobileWorktreeStatus{ID: wt.ID, Name: wt.Branch, Status: "unknown"})
            }
            continue
        }
        byPath := indexByCwd(sessions) // correlation key: TerminalSession.Cwd == Worktree.Path — see rationale
        for _, wt := range wts {
            session, ok := byPath[wt.Path]
            if !ok {
                out = append(out, MobileWorktreeStatus{ID: wt.ID, Name: wt.Branch, Status: "idle"})
                continue
            }
            out = append(out, MobileWorktreeStatus{
                ID: wt.ID, Name: wt.Branch,
                Agent:      session.AgentKind,
                Status:     statusFrom(session), // "running"/"waiting"/"completed" per AgentRunning+ReadyForInput+ClosedAt
                DurationMs: durationFrom(session),
                LastOutput: session.LastOutputPreview, // already truncated server-side by infra-fleet-service per BR-MB-15
            })
        }
    }
    return MobileStatusResult{Worktrees: out, GeneratedAt: time.Now()}, nil
}
```

## Design — wiring (`wscompat`)

```go
// channels_mobile_status.go
func registerMobileStatusChannels(r *Registry, client projectv1.ProjectServiceClient, devices authclient.DeviceSecretResolver) {
    // Request/response — BR-MB-16's pull-to-refresh.
    r.Register("mobile.status", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
        if id.DeviceID == "" {
            return nil, errNotAMobileSession // mobile.* channels unreachable from a plain browser session, mirrors mobile.dispatch (SOL-MB-03)
        }
        ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
        resp, err := client.GetMobileWorktreeStatus(ctx, &projectv1.GetMobileWorktreeStatusRequest{})
        if err != nil {
            return nil, err
        }
        secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID) // BR-MB-13: encrypt in transit
        if err != nil {
            return nil, err
        }
        return sealedEnvelope(toMobileStatusView(resp), secret), nil
    })

    // Stream — BR-MB-14's live update while foregrounded. Poll-and-diff,
    // not a true event stream (see rationale) — only sends a frame when
    // the computed view actually changed since the last poll, so an idle
    // desktop doesn't spam the mobile client every tick.
    r.RegisterStream("mobile.statusSubscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
        if id.DeviceID == "" {
            return nil, errNotAMobileSession
        }
        secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID)
        out := make(chan PushEvent)
        go pollAndDiffMobileStatus(ctx, client, id, secret, out) // ticker (e.g. 5s), diffs against last-sent snapshot, sealedEnvelope on change
        return out, nil
    })
}
```

BR-MB-14 ("live update only when app foreground") is enforced client-side —
the mobile app only holds the `mobile.statusSubscribe` connection open
while foregrounded, closing it on backgrounding (the same "if it drops,
`WSSession` is discarded — nothing to fail over" model `api-gateway.md` §4
already describes for every WS session). The server does no
foreground-detection of its own; it simply stops polling once the stream's
`ctx` is cancelled by the client disconnecting, per the stateless-by-design
principle `api-gateway.md` §2 already establishes.

## Test plan

- `terminal_session_test.go` — `AppendOutput` caps at 2048 bytes, keeps the
  *tail* (most recent output), not the head; `TruncatedForMobile` caps at
  exactly 500 chars, also tail-truncated (BR-MB-15).
- `get_mobile_worktree_status_test.go`:
  - Worktree with no bound dev server → `Status: ""`/omitted runtime
    fields, not an error, not skipped from the list entirely.
  - Worktree whose `Path` matches no live `TerminalSession.Cwd` →
    `Status: "idle"`.
  - `ListSessionsForDevServer` erroring for one dev server → that dev
    server's worktrees degrade to `"unknown"`, other dev servers' worktrees
    in the same response are unaffected (regression guard against one bad
    host failing the whole mobile status screen).
  - `ListSessionsForDevServer` called exactly once per distinct
    `dev_server_id`, not once per worktree, for N worktrees sharing a dev
    server (assert call count on the fake).
- `channels_mobile_status_test.go`:
  - Non-mobile `Identity` (no `DeviceID`) rejected before any RPC call
    (mirrors SOL-MB-03's equivalent test).
  - Response body is always the sealed/encrypted envelope, never the raw
    `MobileStatusResult` JSON (BR-MB-13) — assert on the channel's raw
    return value shape in the test, not just that encryption was "called."
  - `mobile.statusSubscribe`: two consecutive identical polls produce
    exactly one `PushEvent` (the first), not two — regression guard for
    the diff-before-send behavior.
- Integration: a worktree with an active `AttachPty` stream producing
  output → `mobile.status` response's `lastOutput` reflects the most
  recent bytes, truncated correctly, exercising the real path from
  SOL-MB-02's output hook through to this endpoint.

## References

- `specs/backend-go/bugs/logic-v1/BUG-MB-04-mobile-status-not-implemented.md` — problem statement and line citations
- `docs/logic/mobile-companion/BL-MB-04-mobile-status.md` — response shape, BR-MB-13..16
- `specs/backend-go/tdd/services/api-gateway.md:27-39` (§2 no-cross-service-orchestration rule — why aggregation lives in `project-service`), `:89-100` (§3 WS endpoint table, one-owning-service-per-stream), `:113-117` (§4 `WSSession` ephemeral/no-failover model BR-MB-14's foreground-only enforcement relies on)
- `specs/backend-go/tdd/services/infra-fleet-service.md:14-37` (§1 ownership), `:161-165` (§4 `TerminalSession` "Holds no PTY bytes" — the existing design statement this solution's in-memory-only storage choice respects), `:354-360,392-404` (§7 existing `project-service --> infra-fleet-service` dependency edge this solution reuses), `:446-483` (§8 per-pod caveat)
- `specs/backend-go/bugs/logic-v1/BUG-MB-01-pair-device-not-implemented.md`, `SOL-MB-01-pair-device.md` — `device_id`/shared-secret this solution's transport encryption depends on
- `specs/backend-go/bugs/logic-v1/BUG-MB-02-push-notification-partial.md`, `SOL-MB-02-push-notification.md` — the `AttachPty` output-relay hook this solution's `lastOutput` capture reuses
- `backend-go/services/project-service/internal/domain/worktree.go:28-50` — `Worktree` fields this solution's response maps from
- `backend-go/services/infra-fleet-service/internal/domain/terminal_session.go` — `TerminalSession` fields, the correlation gap (`Cwd` vs. no `worktree_id`) this solution flags
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_push.go:38-67` — `RegisterStream`/`notifications.subscribe` pattern this solution's `mobile.statusSubscribe` follows
