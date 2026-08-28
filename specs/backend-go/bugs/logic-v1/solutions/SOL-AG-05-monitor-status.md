# SOL-AG-05: Add an `AgentOutputClassifier` in `infra-fleet-service` — two parsing tracks (stream-json vs. OSC/pattern), persisted status, IPC push; mobile push deferred to BUG-MB-01/04

**Resolves:** [BUG-AG-05](../BUG-AG-05-monitor-status-partial.md)
**Service:** `infra-fleet-service` (extended) + `api-gateway` (wscompat push)
**Affected files (proposed):**
- `backend-go/services/infra-fleet-service/internal/usecase/agent_output_classifier.go` (new)
- `backend-go/services/infra-fleet-service/internal/domain/agent_status_pattern.go` (new — pure functions, no I/O)
- `backend-go/services/infra-fleet-service/internal/domain/osc133_scanner.go` (new — Go port of `agent/src/shared/terminal-osc133-command-finished.ts`)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go` (extend with `AgentStatusEventPublisher`)
- `backend-go/services/infra-fleet-service/internal/adapter/eventbus/agent_status_publisher.go` (new — NATS JetStream outbox, `orca.infra.agent.statusChanged`)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go` (extended — subscribes to the event and `PushEvent`s `agent.statusChanged` to the renderer, same pattern as `channels_push.go`)
**Status:** 📋 Proposed — not yet implemented. Mobile/TweetNaCl delivery (BR-AG-16/17) is **explicitly out of scope for this solution** — root-caused and owned by `BUG-MB-01`/`BUG-MB-04`, see below.

---

## Design rationale (grounded in TDD)

### Scope boundary: mobile push is a separate, already-tracked bug

BL-AG-05 step 6 ("WebSocket TweetNaCl E2E push → Mobile App") is the exact
capability `BUG-MB-04-mobile-status-not-implemented.md` describes as
missing, root-caused by `BUG-MB-01-pair-device-not-implemented.md` (no
paired mobile session/shared secret exists to push to at all). Building a
second, competing mobile-push design inside this solution would duplicate
that work and risk drifting from it. **This solution owns detection,
classification, persistence, and desktop/renderer IPC push (BR-AG-14/15)
only** — `agent:statusChanged` is published to the same NATS/outbox +
wscompat-push mechanism every other real-time surface in this system
already uses (`08-inter-service-communication.md`'s event conventions), and
`BUG-MB-04`'s solution is expected to add mobile as a **second subscriber**
of that same event once `BUG-MB-01`'s pairing/transport lands — not
something this solution needs to build to close BUG-AG-05.

### Two parsing tracks, not one — grounded in what `agent.spawn` actually produces

BL-AG-05's design assumes one classification pipeline: OSC 133 A/B/D +
text-pattern matching over raw PTY bytes. Reading `agent-spawner.ts`'s
`AGENT_SPECS` (the same table SOL-AG-01/03 already ground the spawn/resume
design in) shows this only holds for **some** spawns:

```
claude,   fresh spawn (no resumeId): ['--output-format', 'stream-json', '--verbose']
claude,   resume:                    ['--resume', resumeId]                — plain TUI, no stream-json
codex:                                []  (fresh) / ['--session-file', ...] (resume) — plain TUI either way
gemini:                               ['--stream'] (fresh) / ['--resume', resumeId] (resume)
opencode:                             []  (fresh) / ['--session', resumeId] (resume)
```

A **fresh** Claude Code spawn (the common case for BL-AG-01, since resume
is the less-frequent BL-AG-03 path) runs with `--output-format stream-json`
— its PTY output is **newline-delimited JSON events** (`type: "assistant" |
"user" | "result" | ...`), not raw ANSI/OSC terminal bytes. Pattern-matching
OSC sequences against structured JSON lines would not just miss signals, it
would actively misfire (JSON text can contain the literal substring
"waiting for input" inside a quoted string without meaning what the pattern
implies). This solution therefore classifies on **two tracks**, selected by
what `SpawnAgentInput` actually requested:

1. **`stream-json` track** (Claude Code fresh spawns only, today): parse
   each output line as one JSON event; map event `type`/tool-use/result
   shape directly to `AgentStatus` — e.g. an in-flight `assistant`
   message ⇒ `running`, a `result` event ⇒ `completed`/`error` depending on
   its own status field, an error-shaped tool result matching
   `RATE_LIMIT_PATTERNS` inside the event's text content ⇒
   `agent:rateLimited`. This is **more reliable** than pattern-matching,
   not a fallback — Claude Code's own event stream already tells Orca what
   it's doing.
2. **OSC 133 + text-pattern track** (every resumed session, every non-Claude
   agent, and Claude's own resume path): BL-AG-05's originally-specified
   pipeline. Ported from the algorithm `agent/`'s shared code already
   implements for shell-PTY command-boundary tracking
   (`agent/src/shared/terminal-osc133-command-finished.ts`) — a
   chunk-boundary-safe scanner (handles an OSC sequence split across two
   `pty.data` frames), reused as an **algorithm port**, not a shared
   dependency (Go and TS, no code-sharing mechanism between them; the
   scanning **logic** — track a carry buffer up to `len(OSC_133_PREFIX)-1`
   bytes across chunks — is what's reused). Text-pattern matching
   (`waiting for input`, `RATE_LIMIT_PATTERNS`, `task completed`) applies
   on this track only, on the assumption that plain-TUI CLI output is where
   BL-AG-05's original design intended it.

**This two-track split is a genuine design decision this solution is
adding, not something `infra-fleet-service.md` or the BL doc specifies** —
flagged explicitly since it's the one place this solution goes beyond
porting an already-real mechanism.

### Where this belongs and how it consumes existing infrastructure

`infra-fleet-service` already relays every PTY byte through
`StreamPty`/`AttachPty` (`ports.go:165-172`, `server.go:476-510`) — the
classifier subscribes to the **same** `<-chan PtyEvent` an `AgentSession`'s
`AttachPty` stream already produces, as a second consumer, not a new
transport. This matches §7's framing of `infra-fleet-service` as the
service every `connectionId`-bound PTY byte already routes through.

## Design — `domain/agent_status_pattern.go` (pure, track 2)

```go
// RateLimitPatterns mirrors BL-AG-04/05's RATE_LIMIT_PATTERNS 1:1 —
// deliberately the SAME pattern set BUG-AG-04's switch-account flow
// consumes (SOL-AG-04), not a duplicate.
var RateLimitPatterns = map[string]*regexp.Regexp{
	"claude":   regexp.MustCompile(`(?i)rate.?limit|quota.?exceed|too.?many.?request`),
	"codex":    regexp.MustCompile(`(?i)rate.?limit|429|quota`),
	"opencode": regexp.MustCompile(`(?i)rate.?limit|quota`),
}

var (
	waitingPattern   = regexp.MustCompile(`(?i)waiting for input`)
	completedPattern = regexp.MustCompile(`(?i)task completed`)
)

// ClassifyText applies track-2's text-pattern rules to one output chunk —
// pure function, no I/O, unit-testable without a fake PTY stream.
func ClassifyText(agentKind, chunk string) (status AgentStatus, rateLimited bool, ok bool) {
	if pat, found := RateLimitPatterns[agentKind]; found && pat.MatchString(chunk) {
		return "", true, true // rate-limit is a side-channel event, not a status value — see classifier below
	}
	if completedPattern.MatchString(chunk) {
		return AgentStatusCompleted, false, true
	}
	if waitingPattern.MatchString(chunk) {
		return AgentStatusWaiting, false, true
	}
	return "", false, false
}
```

## Design — `domain/osc133_scanner.go` (pure, track 2)

Ports `terminal-osc133-command-finished.ts`'s chunk-boundary-safe scan —
same carry-buffer strategy, same three markers:

```go
const oscPrefix = "\x1b]133;"

// Osc133Scanner is stateful ACROSS calls (carries a partial-prefix tail
// between chunks) — one instance per AgentSession, not a package-level
// function, matching the TS original's per-pane scanner instance.
type Osc133Scanner struct{ carry string }

// Feed processes one raw output chunk, returning every complete marker
// found (A=start, C=exec, D=finished, with D's best-effort exit code).
func (s *Osc133Scanner) Feed(chunk string) []Osc133Marker { /* port of terminal-osc133-command-finished.ts:41-90 */ }
```

## Design — `usecase/agent_output_classifier.go`

```go
type AgentOutputClassifier struct {
	sessions  AgentSessionRepository
	relay     DevServerAgentClient // StreamPty — same port SOL-AG-01/02 already use
	publisher AgentStatusEventPublisher
}

// Run subscribes to sessionID's PTY stream and classifies every chunk until
// the stream closes (agent.exited) or ctx is cancelled. One goroutine per
// live AgentSession, started by StartAgentSession/ResumeAgentSession right
// after Create (SOL-AG-01/03) — not by a separate RPC call.
func (c *AgentOutputClassifier) Run(ctx context.Context, tenantID string, session domain.AgentSession, devServer domain.DevServer) {
	events, unsubscribe, err := c.relay.StreamPty(ctx, devServer, session.PtyID)
	if err != nil {
		return // spawn-time failure already surfaced via StartAgentSession's own error path
	}
	defer unsubscribe()

	scanner := &domain.Osc133Scanner{}
	deadline := time.AfterFunc(30*time.Second, func() { c.onStartupTimeout(ctx, tenantID, session) }) // BR-AG-04
	defer deadline.Stop()

	for ev := range events {
		switch ev.Type {
		case PtyEventData:
			var status domain.AgentStatus
			var rateLimited bool
			if session.UsesStreamJSON() { // track 1 — see rationale
				status, rateLimited = classifyStreamJSONLine(ev.Data)
			} else { // track 2
				for _, marker := range scanner.Feed(string(ev.Data)) {
					status = domain.StatusFromOSC133(marker) // A->running, D(exit=0)->idle, D(exit!=0)->error
				}
				if s, rl, ok := domain.ClassifyText(session.ModelID, string(ev.Data)); ok {
					status, rateLimited = s, rl
				}
			}
			if rateLimited {
				c.publisher.PublishRateLimited(ctx, tenantID, session.ID) // BL-AG-04 step 1's agent:rateLimited
				continue
			}
			if status != "" && status != session.Status {
				deadline.Stop() // first real signal cancels the 30s startup timer regardless of which status it is
				_ = c.sessions.UpdateStatus(ctx, tenantID, session.ID, status, time.Now().UTC())
				c.publisher.PublishStatusChanged(ctx, tenantID, session.ID, status) // BR-AG-14: <500ms target — see NFR note below
				session.Status = status
			}
		case PtyEventExit:
			final := domain.AgentStatusStopped
			if ev.ExitCode != 0 {
				final = domain.AgentStatusError
			}
			_ = c.sessions.MarkStoppedWithStatus(ctx, tenantID, session.ID, final, time.Now().UTC())
			c.publisher.PublishStatusChanged(ctx, tenantID, session.ID, final)
			return
		}
	}
}

// onStartupTimeout — BR-AG-04/[A3]: no idle signal within 30s → kill + error,
// composing SOL-AG-02's KillAgentSession rather than calling KillAgent directly.
func (c *AgentOutputClassifier) onStartupTimeout(ctx context.Context, tenantID string, session domain.AgentSession) { /* ... */ }
```

`session.UsesStreamJSON()` is a small domain predicate on `AgentSession`
(`ModelID == "claude" && ResumeOfSessionID == ""` — mirrors `AGENT_SPECS`'s
own branch exactly, see rationale table) — kept as a named method so the
branch condition has one definition, not copy-pasted at each call site.

### NFR — BR-AG-14 (<500ms detect-to-update)

The classifier runs in-process against the same `StreamPty` channel
`AttachPty` already demuxes from (no extra network hop between "byte
arrives" and "status computed"); `PublishStatusChanged`'s cost is the
dominant factor. This solution routes it through the **outbox** pattern
(`05-data-architecture.md`'s default), which adds a polling-relay delay —
worth flagging as a latency risk against the 500ms target: if the outbox
relay's poll interval is on the order of hundreds of ms, it eats most of
the budget. **Recommend a direct in-process publish to `api-gateway`'s
push-bridge for `agent.statusChanged` specifically** (bypassing the outbox
for this one high-frequency, low-value-of-durability event — a status
update that's lost on crash is just re-derived from the next PTY chunk, not
a business fact that must never be lost, unlike `task.completed`), while
still using the outbox for `agent:rateLimited` (a real alerting event worth
at-least-once delivery). This is a deliberate deviation from
`08-inter-service-communication.md`'s "publishing always goes through the
outbox" rule, flagged for explicit sign-off rather than silently applied,
since that doc states it as a hard "never a direct publish call" rule.

## Design — wiring (`wscompat`)

`channels_agent.go` (SOL-AG-01/02/04's file) adds a push subscription
mirroring `channels_push.go`'s existing pattern
(`push_bridge.go:25`'s `PushEvent`) — `agent.statusChanged` and
`agent:rateLimited` become `PushEvent{Channel: "agent.statusChanged", ...}`
frames to the renderer, the same IPC path `terminal.output` already uses
(`channels_terminal.go:259-266`), satisfying BR-AG-14's "renderer, not
mobile" half of BL-AG-05's step 5.

## Test plan

- `domain/agent_status_pattern_test.go` — pure: each `RATE_LIMIT_PATTERNS`
  regex against known claude/codex/opencode sample strings; `waiting for
  input`/`task completed` matches; a JSON string literally containing
  "waiting for input" does NOT match when routed through track 1 (asserted
  at the classifier level, see below — the domain function itself has no
  track awareness).
- `domain/osc133_scanner_test.go` — port the exact split-chunk test cases
  from `terminal-osc133-command-finished.test.ts` (BEL- and ST-terminated,
  split across chunk boundaries) so the Go port's behavior is verified
  against the same fixtures the TS original uses, not independently
  invented ones.
- `usecase/agent_output_classifier_test.go` — fake `DevServerAgentClient.StreamPty`
  feeding synthetic `PtyEvent`s:
  - a Claude fresh-spawn session (`UsesStreamJSON()==true`) fed OSC bytes
    that would trigger track 2 → asserts **no** status change (track 1 is
    exclusive when active, regression guard against double-classification).
  - a resumed/non-Claude session fed a rate-limit string → `PublishRateLimited`
    called, `UpdateStatus` **not** called (rate limit is not itself a
    persisted `AgentStatus` value).
  - no status signal within 30s (synthetic timer) → `KillAgentSession`
    invoked, session marked `error`.
  - `PtyEventExit` with `exitCode=0`/`!=0` → `stopped`/`error` respectively.

## References

- `specs/backend-go/bugs/logic-v1/BUG-AG-05-monitor-status-partial.md`
- `docs/logic/agent-orchestration/BL-AG-05-monitor-status.md` — status table, BR-AG-14/15/16/17, stream diagram
- `specs/backend-go/bugs/logic-v1/BUG-MB-04-mobile-status-not-implemented.md`, `BUG-MB-01-pair-device-not-implemented.md` — mobile push's actual owner, cited to justify this solution's scope boundary
- `specs/backend-go/tdd/architecture/08-inter-service-communication.md:30-45` (event conventions, the outbox-always rule this solution proposes one deliberate exception to)
- `specs/backend-go/tdd/architecture/05-data-architecture.md:82-98` (transactional outbox pattern)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:165-186` (`StreamPty`, `AgentStatus` — the existing best-effort heuristic this solution's real classifier supersedes for agent sessions specifically; `terminal.agentStatus`'s process-title heuristic remains correct for plain shells)
- `backend-go/services/infra-fleet-service/internal/adapter/grpc/server.go:476-510` (`AttachPty` — the existing consumer of the same `PtyEvent` channel this solution adds a second consumer to)
- `backend-go/services/api-gateway/internal/adapter/wscompat/channels_terminal.go:259-266` (`PushEvent` pattern this solution's renderer push follows)
- `agent/src/relay/agent-spawner.ts:130-195` (`AGENT_SPECS` — source of the two-track split, `--output-format stream-json` only on Claude fresh spawns)
- `agent/src/shared/terminal-osc133-command-finished.ts` (the chunk-boundary-safe OSC scanner this solution ports to Go) and its test file, cited for the fixture set to reuse
- `specs/backend-go/bugs/logic-v1/solutions/SOL-AG-01-khoi-dong-agent.md`, `SOL-AG-02-dung-agent.md`, `SOL-AG-04-switch-account.md` — the sibling solutions this one's classifier is invoked from / feeds into
