# TASK-AG-05-06: Start the classifier per session + wire the outbox relay + push `agent.statusChanged`/`agent:rateLimited` to the renderer

**From Solution:** SOL-AG-05
**Priority:** P0
**Service:** `infra-fleet-service` + `api-gateway`
**File:** `backend-go/services/infra-fleet-service/cmd/server/main.go`, `backend-go/services/infra-fleet-service/internal/usecase/start_agent_session.go`, `backend-go/services/api-gateway/internal/adapter/wscompat/channels_agent.go`
**Depends on:** TASK-AG-05-04, TASK-AG-05-05
**Status:** `[x]` DONE — `StartAgentSession` gained a nil-safe `classifier *AgentOutputClassifier` field launched post-persist (covers Resume transitively); main.go wires NATS connect + `infraeventbus.New` + `AgentOutputClassifier` + the `agentRateLimitedRelay` outbox relay goroutine, degrading gracefully (classifier stays nil) if NATS is unreachable at startup; api-gateway gained its first NATS `commoneventbus.Consumer` (also degrade-safe) and the `agent.subscribeStatus` push channel (`Registry.RegisterStream`) forwarding tenant-filtered `agent.statusChanged`/`agent:rateLimited`. `TestStartAgentSession_LaunchesClassifierAfterPersist` and `TestAgentSubscribeStatusChannel_NilBus_ReturnsClosedChannel` pass; a live NATS integration check (BR-AG-14 push-latency budget) is left for a real docker-compose run per this task's own note.

---

## Context

Three wiring pieces: (1) `AgentOutputClassifier.Run` starts as a goroutine right after `StartAgentSession`/`ResumeAgentSession` persist a new session, (2) the `agent:rateLimited` outbox relay starts in `main.go` (mirroring `usage-service`'s relay startup), (3) `api-gateway` subscribes to both NATS subjects and forwards them to the renderer over a new `agent.subscribeStatus` push-only wscompat channel, following `channels_terminal.go`'s note that a pure-subscribe channel uses `Registry.RegisterStream` (no ack), same as `notifications.subscribe`.

## Changes to make

### 1. Start the classifier from `StartAgentSession`

Extend `StartAgentSession` (TASK-AG-01-07) with the two new dependencies
and a post-persist launch:

```go
type StartAgentSession struct {
	resolver   ConnectionResolver
	agent      DevServerAgentClient
	sessions   AgentSessionRepository
	classifier *AgentOutputClassifier
	clock      func() time.Time
}

func NewStartAgentSession(resolver ConnectionResolver, agent DevServerAgentClient, sessions AgentSessionRepository, classifier *AgentOutputClassifier) *StartAgentSession {
	return &StartAgentSession{resolver: resolver, agent: agent, sessions: sessions, classifier: classifier, clock: func() time.Time { return time.Now().UTC() }}
}
```

At the end of `Execute`, right before `return session, nil`:

```go
	tenantIDForClassifier := tenantID // avoid capturing the outer var name ambiguity in the closure below
	go uc.classifier.Run(context.Background(), tenantIDForClassifier, session, devServer)

	return session, nil
```

Apply the identical addition to `ResumeAgentSession.Execute` (TASK-AG-03-04)
— it calls `uc.start.Execute` internally, so the classifier already starts
transitively; no separate change needed there, but double check no second
`Run` gets started for the same session if `ResumeAgentSession` ever calls
`Execute` more than once (it doesn't, today).

Update `cmd/server/main.go`'s `NewStartAgentSession(...)` call to pass the
new `classifierUC` argument (construct `classifierUC` before
`startAgentSessionUC`, since the latter now depends on it):

```go
agentStatusPublisher := infraeventbus.New(natsPublisher, agentRateLimitedOutboxStore)
killAgentSessionUC := usecase.NewKillAgentSession(agentSessionStore, repo, agentClient, nil)
classifierUC := usecase.NewAgentOutputClassifier(agentSessionStore, agentClient, agentStatusPublisher, killAgentSessionUC)
startAgentSessionUC := usecase.NewStartAgentSession(repo, agentClient, agentSessionStore, classifierUC)
```

(Reorder `main.go`'s existing usecase construction block so
`killAgentSessionUC`/`classifierUC` are built before `startAgentSessionUC`
— today's file builds `startAgentSessionUC` first, TASK-AG-01-07/TASK-AG-02-04;
this task changes that order.)

### 2. Start the outbox relay

```go
agentRateLimitedRelay := outbox.NewRelay(agentRateLimitedOutboxStore, natsPublisher, outbox.Config{PollInterval: 500 * time.Millisecond, BatchSize: 100})
go agentRateLimitedRelay.Run(ctx) // matches usage-service's relay-startup call site's shape
```

### 3. `api-gateway` push subscription

Add a NATS consumer + push channel in `channels_agent.go`:

```go
func registerAgentStatusSubscribeChannel(r *Registry, bus *commoneventbus.Consumer) {
	r.RegisterStream("agent.subscribeStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		events := make(chan PushEvent)
		go func() {
			defer close(events)
			_ = bus.SubscribeEphemeral(ctx, "INFRA", "orca.infra.agent.statusChanged", func(ctx context.Context, ev commoneventbus.Event) error {
				if ev.TenantID != id.TenantID {
					return nil // tenant isolation — see tenant-service's consumer.go for the established pattern
				}
				select {
				case events <- PushEvent{Channel: "agent.statusChanged", Args: []json.RawMessage{ev.Payload}}:
				case <-ctx.Done():
				}
				return nil
			})
		}()
		go func() {
			_ = bus.SubscribeEphemeral(ctx, "INFRA", "orca.infra.agent.rateLimited", func(ctx context.Context, ev commoneventbus.Event) error {
				if ev.TenantID != id.TenantID {
					return nil
				}
				select {
				case events <- PushEvent{Channel: "agent:rateLimited", Args: []json.RawMessage{ev.Payload}}:
				case <-ctx.Done():
				}
				return nil
			})
		}()
		return events, nil
	})
}
```

Add `registerAgentStatusSubscribeChannel(r, bus)` to `registerAgentChannels`
— check the exact `commoneventbus.Consumer` construction call site
`api-gateway`'s `main.go`/`server.go` already uses for any existing
NATS-backed push channel (if none exists yet, this is the first — mirror
`tenant-service`'s `Consumer.Run`'s `SubscribeEphemeral` call shape, but
invoked per-WebSocket-connection here rather than once at service startup,
since each connection needs its own tenant-filtered forwarding goroutine).

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/... ./services/api-gateway/...
go test ./services/infra-fleet-service/internal/usecase/... -run TestStartAgentSession -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestAgentSubscribeStatus -v
```

Integration check (manual or docker-compose): spawn an agent session,
observe `agent.statusChanged` push frames arrive at the renderer within the
BR-AG-14 budget as the classifier detects OSC 133;C/D transitions; trigger
a synthetic rate-limit string and confirm `agent:rateLimited` arrives via
the outbox path (may lag `statusChanged` by up to the relay's
`PollInterval`, which is expected and documented in TASK-AG-05-05).
