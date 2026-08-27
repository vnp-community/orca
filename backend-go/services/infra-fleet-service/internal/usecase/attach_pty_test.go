package usecase

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeLifecycleEventPublisher is an in-memory LifecycleEventPublisher —
// records every PublishAgentLifecycle call for assertions, shared by
// attach_pty_test.go and get_terminal_agent_status_test.go.
type fakeLifecycleEventPublisher struct {
	mu    sync.Mutex
	calls []lifecycleEventCall
	err   error
}

type lifecycleEventCall struct {
	TenantID string
	Subject  string
	Payload  eventbus.AgentLifecyclePayload
}

func (f *fakeLifecycleEventPublisher) PublishAgentLifecycle(ctx context.Context, tenantID, subject string, payload eventbus.AgentLifecyclePayload) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, lifecycleEventCall{TenantID: tenantID, Subject: subject, Payload: payload})
	return f.err
}

func (f *fakeLifecycleEventPublisher) callsSnapshot() []lifecycleEventCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]lifecycleEventCall, len(f.calls))
	copy(out, f.calls)
	return out
}

func TestAttachPty_RequiresAttachFirstFrame(t *testing.T) {
	uc := NewAttachPty(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{}, NewConnectionStreamLimiter(0), &sync.Map{}, &fakeLifecycleEventPublisher{})

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Input: []byte("x")}
	_, errCh := uc.Execute(ctx, inbound)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected an error for a non-attach first frame")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh")
	}
}

func TestAttachPty_UnknownPtyID_ReturnsError(t *testing.T) {
	uc := NewAttachPty(&fakeTerminalSessionRepository{}, &fakeConnectionResolver{}, &fakeDevServerAgentClient{}, NewConnectionStreamLimiter(0), &sync.Map{}, &fakeLifecycleEventPublisher{})

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-unknown"}}
	_, errCh := uc.Execute(ctx, inbound)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a not-found error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh")
	}
}

// seedSession registers a connection-bound terminal session both in
// sessions and resolver — the common setup every terminal-usecase test
// (attach/wait/resize/kill/...) needs.
func seedSession(t *testing.T, sessions *fakeTerminalSessionRepository, resolver *fakeConnectionResolver, tenantID, ptyID, connectionID string) domain.DevServer {
	t.Helper()
	ds, err := domain.NewDevServer("ds1", tenantID, "10.0.0.5", domain.ConnectionModeRelayWebSocket, "")
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	if resolver.byConnectionID == nil {
		resolver.byConnectionID = map[string]domain.DevServer{}
	}
	resolver.byConnectionID[connectionID] = ds
	now := time.Now().UTC()
	if _, err := sessions.Create(context.Background(), domain.TerminalSession{PtyID: ptyID, TenantID: tenantID, ConnectionID: connectionID, CreatedAt: now, LastActiveAt: now}); err != nil {
		t.Fatalf("seeding session: %v", err)
	}
	return ds
}

func TestAttachPty_RelaysInputAndResizeToAgent(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	agent := &fakeDevServerAgentClient{streamPtyEvents: make(chan PtyEvent)}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	uc := NewAttachPty(sessions, resolver, agent, NewConnectionStreamLimiter(0), &sync.Map{}, &fakeLifecycleEventPublisher{})

	ctx, cancel := context.WithCancel(withTenant(context.Background(), "tenant-1"))
	defer cancel()
	inbound := make(chan PtyClientMessage, 4)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-1"}}
	inbound <- PtyClientMessage{Input: []byte("ls\n")}
	inbound <- PtyClientMessage{Resize: &PtyResizeMessage{Cols: 100, Rows: 40}}
	outbound, errCh := uc.Execute(ctx, inbound)

	deadline := time.After(2 * time.Second)
	for {
		agent.mu.Lock()
		gotWrite := len(agent.writePtyCalls) == 1
		gotResize := len(agent.resizePtyCalls) == 1
		agent.mu.Unlock()
		if gotWrite && gotResize {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for input/resize to reach the agent")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	select {
	case <-outbound:
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the stream to end after cancel")
	}
}

func TestAttachPty_RelaysAgentOutputAndExit(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	events := make(chan PtyEvent, 2)
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	liveStates := &sync.Map{}
	lifecycleEvents := &fakeLifecycleEventPublisher{}
	uc := NewAttachPty(sessions, resolver, agent, NewConnectionStreamLimiter(0), liveStates, lifecycleEvents)

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-1"}}
	outbound, errCh := uc.Execute(ctx, inbound)

	events <- PtyEvent{PtyID: "pty-1", Data: []byte("hello")}
	select {
	case msg := <-outbound:
		if string(msg.Output) != "hello" {
			t.Errorf("expected output %q, got %q", "hello", msg.Output)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for output event")
	}

	// An Output chunk must not publish any lifecycle event, and must
	// record a liveStates entry for the quiescence heuristic
	// (TASK-MB-02-01/02) to read.
	if calls := lifecycleEvents.callsSnapshot(); len(calls) != 0 {
		t.Errorf("expected an Output chunk to publish nothing, got %d calls: %+v", len(calls), calls)
	}
	if _, ok := liveStates.Load("pty-1"); !ok {
		t.Error("expected an Output chunk to store a liveStates entry for pty-1")
	}

	events <- PtyEvent{PtyID: "pty-1", Exited: true, ExitCode: 7}
	select {
	case msg, ok := <-outbound:
		if !ok {
			t.Fatal("expected an exited message before outbound closes")
		}
		if !msg.Exited || msg.ExitCode != 7 {
			t.Errorf("expected Exited=true ExitCode=7, got %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exit event")
	}

	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh to close after exit")
	}

	agent.mu.Lock()
	unsub := agent.streamPtyUnsubscribed
	agent.mu.Unlock()
	if !unsub {
		t.Error("expected StreamPty's unsubscribe to have been called")
	}

	// A non-zero exit code publishes exactly one agent_error event, and
	// clears the liveStates entry.
	calls := lifecycleEvents.callsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one lifecycle event published on exit, got %d: %+v", len(calls), calls)
	}
	if calls[0].Subject != eventbus.SubjectAgentError {
		t.Errorf("expected subject %q for a non-zero exit code, got %q", eventbus.SubjectAgentError, calls[0].Subject)
	}
	if calls[0].Payload.ExitCode == nil || *calls[0].Payload.ExitCode != 7 {
		t.Errorf("expected payload ExitCode=7, got %+v", calls[0].Payload.ExitCode)
	}
	if _, ok := liveStates.Load("pty-1"); ok {
		t.Error("expected the liveStates entry for pty-1 to be deleted on exit")
	}
}

// TestAttachPty_ExitCodeZero_PublishesAgentCompleted covers the other exit
// branch: ExitCode==0 publishes agent_completed, not agent_error.
func TestAttachPty_ExitCodeZero_PublishesAgentCompleted(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	events := make(chan PtyEvent, 1)
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	lifecycleEvents := &fakeLifecycleEventPublisher{}
	uc := NewAttachPty(sessions, resolver, agent, NewConnectionStreamLimiter(0), &sync.Map{}, lifecycleEvents)

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-1"}}
	outbound, errCh := uc.Execute(ctx, inbound)

	events <- PtyEvent{PtyID: "pty-1", Exited: true, ExitCode: 0}
	select {
	case <-outbound:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for exit event")
	}
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh to close after exit")
	}

	calls := lifecycleEvents.callsSnapshot()
	if len(calls) != 1 {
		t.Fatalf("expected exactly one lifecycle event published on exit, got %d: %+v", len(calls), calls)
	}
	if calls[0].Subject != eventbus.SubjectAgentCompleted {
		t.Errorf("expected subject %q for ExitCode=0, got %q", eventbus.SubjectAgentCompleted, calls[0].Subject)
	}
}

// TestAppendOutput_CapsAtBufferSize_TailTruncated: appendOutput caps at
// lastOutputBufferBytes (2048), keeping the MOST RECENT bytes, not the
// oldest.
func TestAppendOutput_CapsAtBufferSize_TailTruncated(t *testing.T) {
	old := make([]byte, 100)
	for i := range old {
		old[i] = 'o'
	}
	buf := appendOutput(nil, old)

	chunk := make([]byte, lastOutputBufferBytes)
	for i := range chunk {
		chunk[i] = 'n'
	}
	buf = appendOutput(buf, chunk)

	if len(buf) != lastOutputBufferBytes {
		t.Fatalf("expected len %d, got %d", lastOutputBufferBytes, len(buf))
	}
	for i, b := range buf {
		if b != 'n' {
			t.Fatalf("expected only the most recent bytes to survive, found stale byte %q at index %d", b, i)
		}
	}
}

// TestAppendOutput_BelowCap_AccumulatesWithoutTruncation: total bytes under
// the cap are kept in full, appended in order.
func TestAppendOutput_BelowCap_AccumulatesWithoutTruncation(t *testing.T) {
	buf := appendOutput(nil, []byte("hello "))
	buf = appendOutput(buf, []byte("world"))
	if string(buf) != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", string(buf))
	}
}

// TestAttachPty_AccumulatesOutputBuffer_AcrossMultipleChunks is
// TASK-MB-04-02's regression guard: successive Output chunks on the same
// pty must be concatenated into the liveStates entry's lastOutput buffer
// (via appendOutput), not overwritten by the latest chunk alone.
func TestAttachPty_AccumulatesOutputBuffer_AcrossMultipleChunks(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	events := make(chan PtyEvent, 2)
	agent := &fakeDevServerAgentClient{streamPtyEvents: events}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	liveStates := &sync.Map{}
	uc := NewAttachPty(sessions, resolver, agent, NewConnectionStreamLimiter(0), liveStates, &fakeLifecycleEventPublisher{})

	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-1"}}
	outbound, _ := uc.Execute(ctx, inbound)

	events <- PtyEvent{PtyID: "pty-1", Data: []byte("hello ")}
	<-outbound
	events <- PtyEvent{PtyID: "pty-1", Data: []byte("world")}
	<-outbound

	v, ok := liveStates.Load("pty-1")
	if !ok {
		t.Fatal("expected a liveStates entry for pty-1")
	}
	got := string(v.(*ptyLiveState).lastOutput)
	if got != "hello world" {
		t.Errorf("expected accumulated buffer %q, got %q", "hello world", got)
	}
}

func TestAttachPty_StreamLimitReached(t *testing.T) {
	sessions := &fakeTerminalSessionRepository{}
	resolver := &fakeConnectionResolver{}
	agent := &fakeDevServerAgentClient{streamPtyEvents: make(chan PtyEvent)}
	seedSession(t, sessions, resolver, "tenant-1", "pty-1", "conn-1")
	limiter := NewConnectionStreamLimiter(1)
	release, err := limiter.Acquire("conn-1")
	if err != nil {
		t.Fatalf("priming the limiter: %v", err)
	}
	defer release()

	uc := NewAttachPty(sessions, resolver, agent, limiter, &sync.Map{}, &fakeLifecycleEventPublisher{})
	ctx := withTenant(context.Background(), "tenant-1")
	inbound := make(chan PtyClientMessage, 1)
	inbound <- PtyClientMessage{Attach: &PtyAttachMessage{PtyID: "pty-1"}}
	_, errCh := uc.Execute(ctx, inbound)

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected a stream-limit error")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for errCh")
	}
}
