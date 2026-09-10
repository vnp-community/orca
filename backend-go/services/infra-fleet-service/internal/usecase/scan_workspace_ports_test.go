package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// fakeDevServerAgentClient is an in-memory DevServerAgentClient — stands in
// for adapter/devserveragent's real stub in these usecase-layer tests, which
// only exercise dispatch logic, not the wire protocol. Also backs the
// Terminal/PTY usecase tests (spawn_terminal_session_test.go,
// attach_pty_test.go, wait_terminal_session_test.go, etc.) — guarded by mu
// since AttachPty drives calls from its own goroutine (see attach_pty.go's
// run method).
type fakeDevServerAgentClient struct {
	mu sync.Mutex

	execResult map[string]any
	execErr    error
	execCalls  []string // methods called with, for assertions
	// execParams records the params map passed on each Exec call, in the
	// same order as execCalls — TASK-BE-EVM-001's recipeId/runtimeId
	// assertions need to inspect what was actually sent, not just the
	// method name.
	execParams []map[string]any

	// execCalled/lastMethod are a simpler single-call view of execCalls,
	// used by kill_workspace_port_test.go/establish_connection_test.go.
	execCalled bool
	lastMethod string

	// healthy/healthErr drive Health's fake answer — used by
	// establish_connection_test.go.
	healthy   bool
	healthErr error
	// isConnected drives IsConnected's fake answer.
	isConnected    bool
	spawnPtyResult SpawnPtyResult
	spawnPtyErr    error
	spawnPtyCalls  []SpawnPtyInput

	writePtyErr   error
	writePtyCalls [][]byte

	resizePtyErr   error
	resizePtyCalls []resizePtyCall

	killPtyErr   error
	killPtyCalls []string

	sendSignalErr   error
	sendSignalCalls []string // "ptyID:signal", for assertions

	// streamPtyEvents, if non-nil, is returned as-is from StreamPty — the
	// test owns writing to (and closing) it. streamPtyUnsubscribed records
	// whether the returned unsubscribe func was called.
	streamPtyEvents       chan PtyEvent
	streamPtyErr          error
	streamPtyUnsubscribed bool

	// streamScreencastEvents/Err/Unsubscribed mirror streamPtyEvents's
	// convention exactly, for StreamScreencast.
	streamScreencastEvents       chan ScreencastEvent
	streamScreencastErr          error
	streamScreencastUnsubscribed bool
	streamScreencastCalls        []ScreencastParams

	agentStatusResult AgentStatusResult
	agentStatusErr    error

	inspectResult InspectProcessResult
	inspectErr    error

	// lastHandshakeInfo/lastHandshakeOK drive LastHandshakeInfo's fake
	// answer — used by establish_connection_test.go.
	lastHandshakeInfo HandshakeInfo
	lastHandshakeOK   bool

	// cancelReconnectCalls records every CancelReconnect(devServerID) call —
	// used by teardown_connection_test.go.
	cancelReconnectCalls []string

	// --- Agent sessions (TASK-AG-01..05) ---
	spawnAgentResult SpawnAgentResult
	spawnAgentErr    error
	spawnAgentCalls  []SpawnAgentInput

	killAgentErr   error
	killAgentCalls []string // "ptyID:signal", for assertions

	sendAgentInputErr   error
	sendAgentInputCalls []string // "ptyID:data", for assertions

	streamAgentHooksEvents chan AgentHookEvent
	streamAgentHooksErr    error

	// execStreamFrames/execStreamErr drive ExecStream's fake answer
	// (TASK-PW-03-08) — same "test owns writing to (and closing) it" shape
	// as streamPtyEvents above. execStreamUnsubscribed records whether the
	// returned unsubscribe func was called.
	execStreamFrames       chan map[string]any
	execStreamErr          error
	execStreamCalls        []string // methods called with, for assertions
	execStreamUnsubscribed bool

	// streamVmProvisionEvents/Err/Unsubscribed/Calls mirror
	// streamScreencastEvents's convention exactly, for StreamVmProvision
	// (TASK-BE-EVM-003).
	streamVmProvisionEvents       chan VmProvisionEvent
	streamVmProvisionErr          error
	streamVmProvisionUnsubscribed bool
	streamVmProvisionCalls        []VmProvisionParams

	// dialHiddenSshTarget* drive DialHiddenSshTarget's fake answer —
	// TASK-BE-EVM-014's AgentOutboundSshProvisioner tests.
	dialHiddenSshTargetResult      string
	dialHiddenSshTargetFingerprint string
	dialHiddenSshTargetErr         error
	dialHiddenSshTargetCalls       []domain.EphemeralVmSshTarget

	// readCredentialFile* drive ReadCredentialFile's fake answer —
	// TASK-BE-EVM-017's BackendRelaySshProvisioner tests.
	readCredentialFileResult string
	readCredentialFileErr    error
	readCredentialFileCalls  []readCredentialFileCall

	// streamFileChanges* mirror streamScreencastEvents's convention exactly,
	// for StreamFileChanges (BACKLOG-003).
	streamFileChangesEvents       chan FileChangeEvent
	streamFileChangesErr          error
	streamFileChangesUnsubscribed bool
	streamFileChangesCalls        []string // path, per call

	// streamExecOutputEvents/Err/Unsubscribed mirror streamPtyEvents's
	// convention exactly, for StreamExecOutput (TASK-AG-FLOWTASK-002).
	streamExecOutputEvents       chan ExecOutputEvent
	streamExecOutputErr          error
	streamExecOutputUnsubscribed bool
	streamExecOutputCalls        []string // stepIDs, for assertions
}

type readCredentialFileCall struct {
	devServer domain.DevServer
	path      string
}

// LastHandshakeInfo implements usecase.DevServerAgentClient.LastHandshakeInfo.
func (f *fakeDevServerAgentClient) LastHandshakeInfo(devServerID string) (HandshakeInfo, bool) {
	return f.lastHandshakeInfo, f.lastHandshakeOK
}

type resizePtyCall struct {
	Cols, Rows int32
}

func (f *fakeDevServerAgentClient) Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error) {
	f.mu.Lock()
	f.execCalls = append(f.execCalls, method)
	f.execParams = append(f.execParams, params)
	f.execCalled = true
	f.lastMethod = method
	f.mu.Unlock()
	if f.execErr != nil {
		return nil, f.execErr
	}
	return f.execResult, nil
}

func (f *fakeDevServerAgentClient) Health(ctx context.Context, devServer domain.DevServer) (bool, error) {
	if f.healthErr != nil {
		return false, f.healthErr
	}
	return f.healthy, nil
}

func (f *fakeDevServerAgentClient) IsConnected(devServerID string) bool {
	return f.isConnected
}

func (f *fakeDevServerAgentClient) SpawnPty(ctx context.Context, devServer domain.DevServer, in SpawnPtyInput) (SpawnPtyResult, error) {
	f.mu.Lock()
	f.spawnPtyCalls = append(f.spawnPtyCalls, in)
	f.mu.Unlock()
	if f.spawnPtyErr != nil {
		return SpawnPtyResult{}, f.spawnPtyErr
	}
	return f.spawnPtyResult, nil
}

func (f *fakeDevServerAgentClient) WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	f.mu.Lock()
	f.writePtyCalls = append(f.writePtyCalls, data)
	f.mu.Unlock()
	return f.writePtyErr
}

func (f *fakeDevServerAgentClient) ResizePty(ctx context.Context, devServer domain.DevServer, ptyID string, cols, rows int32) error {
	f.mu.Lock()
	f.resizePtyCalls = append(f.resizePtyCalls, resizePtyCall{Cols: cols, Rows: rows})
	f.mu.Unlock()
	return f.resizePtyErr
}

func (f *fakeDevServerAgentClient) KillPty(ctx context.Context, devServer domain.DevServer, ptyID string, graceful bool) error {
	f.mu.Lock()
	f.killPtyCalls = append(f.killPtyCalls, ptyID)
	f.mu.Unlock()
	return f.killPtyErr
}

func (f *fakeDevServerAgentClient) SendSignal(ctx context.Context, devServer domain.DevServer, ptyID string, signal string) error {
	f.mu.Lock()
	f.sendSignalCalls = append(f.sendSignalCalls, ptyID+":"+signal)
	f.mu.Unlock()
	return f.sendSignalErr
}

func (f *fakeDevServerAgentClient) StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan PtyEvent, func(), error) {
	if f.streamPtyErr != nil {
		return nil, nil, f.streamPtyErr
	}
	events := f.streamPtyEvents
	if events == nil {
		events = make(chan PtyEvent)
	}
	unsubscribe := func() {
		f.mu.Lock()
		f.streamPtyUnsubscribed = true
		f.mu.Unlock()
	}
	return events, unsubscribe, nil
}

func (f *fakeDevServerAgentClient) StreamScreencast(ctx context.Context, devServer domain.DevServer, params ScreencastParams) (<-chan ScreencastEvent, func(), error) {
	f.mu.Lock()
	f.streamScreencastCalls = append(f.streamScreencastCalls, params)
	f.mu.Unlock()
	if f.streamScreencastErr != nil {
		return nil, nil, f.streamScreencastErr
	}
	events := f.streamScreencastEvents
	if events == nil {
		events = make(chan ScreencastEvent)
	}
	unsubscribe := func() {
		f.mu.Lock()
		f.streamScreencastUnsubscribed = true
		f.mu.Unlock()
	}
	return events, unsubscribe, nil
}

func (f *fakeDevServerAgentClient) StreamFileChanges(ctx context.Context, devServer domain.DevServer, path string) (<-chan FileChangeEvent, func(), error) {
	f.mu.Lock()
	f.streamFileChangesCalls = append(f.streamFileChangesCalls, path)
	f.mu.Unlock()
	if f.streamFileChangesErr != nil {
		return nil, nil, f.streamFileChangesErr
	}
	events := f.streamFileChangesEvents
	if events == nil {
		events = make(chan FileChangeEvent)
	}
	unsubscribe := func() {
		f.mu.Lock()
		f.streamFileChangesUnsubscribed = true
		f.mu.Unlock()
	}
	return events, unsubscribe, nil
}

func (f *fakeDevServerAgentClient) AgentStatus(ctx context.Context, devServer domain.DevServer, ptyID string) (AgentStatusResult, error) {
	if f.agentStatusErr != nil {
		return AgentStatusResult{}, f.agentStatusErr
	}
	return f.agentStatusResult, nil
}

func (f *fakeDevServerAgentClient) InspectProcess(ctx context.Context, devServer domain.DevServer, ptyID string) (InspectProcessResult, error) {
	if f.inspectErr != nil {
		return InspectProcessResult{}, f.inspectErr
	}
	return f.inspectResult, nil
}

// CancelReconnect implements usecase.DevServerAgentClient.CancelReconnect —
// used by teardown_connection_test.go.
func (f *fakeDevServerAgentClient) CancelReconnect(devServerID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelReconnectCalls = append(f.cancelReconnectCalls, devServerID)
}

func (f *fakeDevServerAgentClient) SpawnAgent(ctx context.Context, devServer domain.DevServer, in SpawnAgentInput) (SpawnAgentResult, error) {
	f.mu.Lock()
	f.spawnAgentCalls = append(f.spawnAgentCalls, in)
	f.mu.Unlock()
	if f.spawnAgentErr != nil {
		return SpawnAgentResult{}, f.spawnAgentErr
	}
	return f.spawnAgentResult, nil
}

func (f *fakeDevServerAgentClient) KillAgent(ctx context.Context, devServer domain.DevServer, ptyID, signal string) error {
	f.mu.Lock()
	f.killAgentCalls = append(f.killAgentCalls, ptyID+":"+signal)
	f.mu.Unlock()
	return f.killAgentErr
}

func (f *fakeDevServerAgentClient) SendAgentInput(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error {
	f.mu.Lock()
	f.sendAgentInputCalls = append(f.sendAgentInputCalls, ptyID+":"+string(data))
	f.mu.Unlock()
	return f.sendAgentInputErr
}

func (f *fakeDevServerAgentClient) StreamAgentHooks(ctx context.Context, devServer domain.DevServer) (<-chan AgentHookEvent, func(), error) {
	if f.streamAgentHooksErr != nil {
		return nil, nil, f.streamAgentHooksErr
	}
	events := f.streamAgentHooksEvents
	if events == nil {
		events = make(chan AgentHookEvent)
	}
	return events, func() {}, nil
}

func (f *fakeDevServerAgentClient) ExecStream(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (<-chan map[string]any, func(), error) {
	f.mu.Lock()
	f.execStreamCalls = append(f.execStreamCalls, method)
	f.mu.Unlock()
	if f.execStreamErr != nil {
		return nil, nil, f.execStreamErr
	}
	frames := f.execStreamFrames
	if frames == nil {
		frames = make(chan map[string]any)
	}
	unsubscribe := func() {
		f.mu.Lock()
		f.execStreamUnsubscribed = true
		f.mu.Unlock()
	}
	return frames, unsubscribe, nil
}

func (f *fakeDevServerAgentClient) StreamVmProvision(ctx context.Context, devServer domain.DevServer, params VmProvisionParams) (<-chan VmProvisionEvent, func(), error) {
	f.mu.Lock()
	f.streamVmProvisionCalls = append(f.streamVmProvisionCalls, params)
	f.mu.Unlock()
	if f.streamVmProvisionErr != nil {
		return nil, nil, f.streamVmProvisionErr
	}
	events := f.streamVmProvisionEvents
	if events == nil {
		events = make(chan VmProvisionEvent)
	}
	unsubscribe := func() {
		f.mu.Lock()
		f.streamVmProvisionUnsubscribed = true
		f.mu.Unlock()
	}
	return events, unsubscribe, nil
}

func (f *fakeDevServerAgentClient) StreamExecOutput(ctx context.Context, devServer domain.DevServer, stepID string) (<-chan ExecOutputEvent, func(), error) {
	f.mu.Lock()
	f.streamExecOutputCalls = append(f.streamExecOutputCalls, stepID)
	f.mu.Unlock()
	if f.streamExecOutputErr != nil {
		return nil, nil, f.streamExecOutputErr
	}
	events := f.streamExecOutputEvents
	if events == nil {
		events = make(chan ExecOutputEvent)
	}
	unsubscribe := func() {
		f.mu.Lock()
		f.streamExecOutputUnsubscribed = true
		f.mu.Unlock()
	}
	return events, unsubscribe, nil
}

func (f *fakeDevServerAgentClient) DialHiddenSshTarget(ctx context.Context, devServer domain.DevServer, runtimeID string, target domain.EphemeralVmSshTarget) (string, string, error) {
	f.mu.Lock()
	f.dialHiddenSshTargetCalls = append(f.dialHiddenSshTargetCalls, target)
	f.mu.Unlock()
	if f.dialHiddenSshTargetErr != nil {
		return "", "", f.dialHiddenSshTargetErr
	}
	hiddenTargetID := runtimeID
	if f.dialHiddenSshTargetResult != "" {
		hiddenTargetID = f.dialHiddenSshTargetResult
	}
	return hiddenTargetID, f.dialHiddenSshTargetFingerprint, nil
}

func (f *fakeDevServerAgentClient) ReadCredentialFile(ctx context.Context, devServer domain.DevServer, path string) (string, error) {
	f.mu.Lock()
	f.readCredentialFileCalls = append(f.readCredentialFileCalls, readCredentialFileCall{devServer: devServer, path: path})
	f.mu.Unlock()
	if f.readCredentialFileErr != nil {
		return "", f.readCredentialFileErr
	}
	return f.readCredentialFileResult, nil
}

func TestScanWorkspacePorts_RequiresTenantContext(t *testing.T) {
	uc := NewScanWorkspacePorts(&fakeConnectionResolver{}, &fakeDevServerAgentClient{})
	_, err := uc.Execute(context.Background(), ScanWorkspacePortsInput{})
	if err == nil {
		t.Fatal("expected an error when no tenant is in context")
	}
}

func TestScanWorkspacePorts_NoConnectionID_ReturnsEmptyWithoutRelaying(t *testing.T) {
	resolver := &fakeConnectionResolver{}
	agent := &fakeDevServerAgentClient{}
	uc := NewScanWorkspacePorts(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	ports, err := uc.Execute(ctx, ScanWorkspacePortsInput{WorktreeID: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 0 {
		t.Errorf("expected no ports for a connectionless worktree, got %v", ports)
	}
	if len(agent.execCalls) != 0 {
		t.Error("expected no relay to the agent when no connectionId is set")
	}
}

// This is the regression test for TS Gap 7: a bound connectionId must
// always relay, never silently short-circuit to an empty result.
func TestScanWorkspacePorts_ConnectionIDBound_AlwaysRelays(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{
		"ports": []any{
			map[string]any{"port": float64(3000), "host": "127.0.0.1", "pid": float64(1234), "processName": "node"},
			map[string]any{"port": float64(8080), "host": "0.0.0.0", "pid": float64(5678), "processName": "python"},
		},
		"platform": "linux",
	}}
	uc := NewScanWorkspacePorts(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	ports, err := uc.Execute(ctx, ScanWorkspacePortsInput{ConnectionID: "conn-1", WorktreeID: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(agent.execCalls) != 1 || agent.execCalls[0] != "ports.detect" {
		t.Fatalf("expected exactly one ports.detect relay call, got %v", agent.execCalls)
	}
	want := []DetectedPort{
		{Port: 3000, Host: "127.0.0.1", PID: 1234, ProcessName: "node"},
		{Port: 8080, Host: "0.0.0.0", PID: 5678, ProcessName: "python"},
	}
	if len(ports) != len(want) || ports[0] != want[0] || ports[1] != want[1] {
		t.Errorf("expected %+v, got %+v", want, ports)
	}
}

// A bound connectionId whose agent call fails must propagate the error, not
// swallow it into an empty result — the exact bug class TS Gap 7 describes.
func TestScanWorkspacePorts_ConnectionIDBound_AgentFailurePropagates(t *testing.T) {
	ds, err := domain.NewDevServer("ds1", "tenant-1", "10.0.0.5", domain.ConnectionModeRelaySSH, "ssht1", nil)
	if err != nil {
		t.Fatalf("building dev server: %v", err)
	}
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{"conn-1": ds}}
	agent := &fakeDevServerAgentClient{execErr: errors.New("devserveragent: not implemented")}
	uc := NewScanWorkspacePorts(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	_, err = uc.Execute(ctx, ScanWorkspacePortsInput{ConnectionID: "conn-1", WorktreeID: "wt-1"})
	if err == nil {
		t.Fatal("expected the agent's error to propagate, not be swallowed into an empty result")
	}
}

func TestScanWorkspacePorts_ConnectionIDNotResolved_ReturnsEmptyWithoutRelaying(t *testing.T) {
	resolver := &fakeConnectionResolver{byConnectionID: map[string]domain.DevServer{}}
	agent := &fakeDevServerAgentClient{}
	uc := NewScanWorkspacePorts(resolver, agent)

	ctx := withTenant(context.Background(), "tenant-1")
	ports, err := uc.Execute(ctx, ScanWorkspacePortsInput{ConnectionID: "unknown-conn", WorktreeID: "wt-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 0 {
		t.Errorf("expected no ports when the connectionId doesn't resolve, got %v", ports)
	}
	if len(agent.execCalls) != 0 {
		t.Error("expected no relay to the agent when the connectionId doesn't resolve to a live dev server")
	}
}
