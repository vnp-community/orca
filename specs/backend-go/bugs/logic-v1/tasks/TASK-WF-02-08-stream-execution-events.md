# TASK-WF-02-08: Implement `StreamExecutionEvents` live push (broker, gRPC, wscompat)

**From Solution:** SOL-WF-02
**Priority:** P2
**Service:** `workflow-service` + `api-gateway`
**File:** `backend-go/services/workflow-service/internal/adapter/eventstream/broker.go` (new)
**Depends on:** TASK-WF-02-01, TASK-WF-02-06
**Status:** `[ ]` TODO

---

## Context

BUG-WF-02 finds no live-execution-streaming capability — a client can
only poll. This adds an in-process pub/sub broker, the gRPC
server-streaming handler, `waveDispatcher` publish calls, and the
`api-gateway` WS bridge.

**Scaling caveat (flag, don't paper over):** this in-process design only
works if `api-gateway`'s WS bridge routes a given `execution_id`'s
`workflow.execution.subscribe` call to the `workflow-service` replica
actually running that execution's dispatch goroutine. Default
recommendation for this pass: rely on single-writer-per-execution
in-process behavior (already true today) rather than introducing NATS —
revisit only once horizontal scaling of `workflow-service` is
load-tested and found to need it.

## Changes to make

Create `backend-go/services/workflow-service/internal/adapter/eventstream/broker.go`:

```go
package eventstream

// Broker is an in-process pub/sub keyed by execution_id — a subscriber
// (the gRPC StreamExecutionEvents handler) registers a channel for one
// execution_id; Publish fans out to every registered channel for that id.
// Explicitly single-instance — see this task's scaling caveat.
type Broker struct {
    mu   sync.Mutex
    subs map[string][]chan domain.ExecutionEvent // keyed by execution_id
}

func New() *Broker { return &Broker{subs: make(map[string][]chan domain.ExecutionEvent)} }

func (b *Broker) Publish(ctx context.Context, event domain.ExecutionEvent) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    for _, ch := range b.subs[event.ExecutionID] {
        select {
        case ch <- event:
        default: // slow subscriber — drop rather than block the dispatcher
        }
    }
    return nil
}

func (b *Broker) Subscribe(executionID string) (ch chan domain.ExecutionEvent, unsubscribe func()) {
    ch = make(chan domain.ExecutionEvent, 16)
    b.mu.Lock()
    b.subs[executionID] = append(b.subs[executionID], ch)
    b.mu.Unlock()
    return ch, func() {
        b.mu.Lock()
        defer b.mu.Unlock()
        subs := b.subs[executionID]
        for i, c := range subs {
            if c == ch {
                b.subs[executionID] = append(subs[:i], subs[i+1:]...)
                close(ch)
                break
            }
        }
    }
}
```

Wire `waveDispatcher.dispatchStep` to publish a `step.output`/
`step.completed` event after each step's terminal result (in addition to
the existing `stepExecutions.UpdateStepExecution` persistence call — the
stream is best-effort, Postgres remains the source of truth).
`Execute.runToCompletion` publishes `execution.completed` after
persisting the final status.

Add the gRPC handler in `internal/adapter/grpc/server.go`:

```go
func (s *Server) StreamExecutionEvents(req *workflowv1.StreamExecutionEventsRequest, stream workflowv1.WorkflowService_StreamExecutionEventsServer) error {
    ch, unsubscribe := s.broker.Subscribe(req.GetExecutionId())
    defer unsubscribe()
    for {
        select {
        case <-stream.Context().Done():
            return nil
        case event, ok := <-ch:
            if !ok {
                return nil
            }
            if err := stream.Send(toProtoEvent(event)); err != nil {
                return err
            }
        }
    }
}
```

Add `registerWorkflowChannels` to
`backend-go/services/api-gateway/internal/adapter/wscompat/channels.go`
(match whatever server-streaming registration primitive `wscompat`
already uses for `terminal.*`'s `AttachPty`-backed channels — confirm the
exact shape before implementing, do not invent a new one):

```go
func registerWorkflowChannels(r *Registry, client workflowv1.WorkflowServiceClient) {
    r.RegisterStream("workflow.execution.subscribe", func(ctx context.Context, id Identity, args []json.RawMessage, push func(any)) error {
        in, err := decodeArg[struct{ ExecutionID string `json:"executionId"` }](args, 0)
        if err != nil {
            return err
        }
        stream, err := client.StreamExecutionEvents(ctx, &workflowv1.StreamExecutionEventsRequest{ExecutionId: in.ExecutionID})
        if err != nil {
            return err
        }
        for {
            event, err := stream.Recv()
            if err == io.EOF {
                return nil
            }
            if err != nil {
                return err
            }
            push(event)
        }
    })
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/workflow-service/... ./services/api-gateway/...
go test ./services/workflow-service/internal/adapter/eventstream/... -race
go test ./services/workflow-service/internal/adapter/grpc/... -run TestStreamExecutionEvents
```

Expected: a subscriber registered before `Publish` receives the event; a
subscriber that unsubscribes mid-stream doesn't leak a goroutine (run
with `-race`); `StreamExecutionEvents` for an unknown `execution_id`
returns immediately (empty stream, not a hang).
