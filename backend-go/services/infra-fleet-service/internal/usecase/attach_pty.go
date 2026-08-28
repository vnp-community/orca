package usecase

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// PtyClientMessage is the usecase-layer decoding of one inbound
// PtyClientFrame — adapter/grpc's AttachPty handler translates the proto
// oneof into this shape before pushing it onto the inbound channel Execute
// consumes, keeping this package free of any proto import (see ports.go's
// package doc comment on why the wire-protocol client's own port lives
// here, same Dependency Inversion rule applies to this streaming shape).
// Exactly one field is set per message.
type PtyClientMessage struct {
	Attach *PtyAttachMessage
	Input  []byte
	Resize *PtyResizeMessage
}

// PtyAttachMessage is PtyClientMessage's first-frame-only binding: which
// pty_id this stream carries I/O for.
type PtyAttachMessage struct {
	PtyID string
}

// PtyResizeMessage is PtyClientMessage's in-stream resize variant (the
// low-latency alternative to the unary ResizeTerminalSession RPC).
type PtyResizeMessage struct {
	Cols int32
	Rows int32
}

// PtyServerMessage is the outbound counterpart AttachPty.Execute streams
// back — one of output bytes or an exit notification, mirroring PtyEvent
// but scoped to this one stream (no PtyID field needed: the whole stream is
// already bound to one pty).
type PtyServerMessage struct {
	Output   []byte
	Exited   bool
	ExitCode int32
}

// outboundQueueSize bounds the channel Execute writes to — generous enough
// that a burst of terminal output doesn't stall this usecase's processing
// loop waiting on adapter/grpc's stream.Send pump.
const outboundQueueSize = 64

// lastOutputBufferBytes is the in-memory ring buffer's cap — headroom above
// BR-MB-15's 500-char preview cap; truncation for the mobile contract
// happens at the read boundary (domain.TruncatedForMobile), not here, so
// this buffer stays free to hold more context for future non-mobile
// consumers.
const lastOutputBufferBytes = 2048

// ptyLiveState is one entry in the shared liveStates registry AttachPty
// writes and GetTerminalAgentStatus reads (TASK-MB-02-01/02) — per-pod,
// in-memory map[ptyID]*ptyLiveState. Inherits the same per-pod
// live-connection-ownership caveat this service's existing AttachPty
// pooling has (a connectionId's live transport lives on exactly one pod at
// a time): a GetTerminalAgentStatus call landing on a different pod sees no
// entry and falls back to the pre-quiescence behavior, an honest degrade.
type ptyLiveState struct {
	lastOutputAt time.Time
	agentRunning bool
	// readyNotified debounces agent_waiting — GetTerminalAgentStatus sets
	// this true on the poll that first observes quiescence, so a later poll
	// while still quiescent does not republish the same transition.
	readyNotified bool
	// lastOutput is a tail-truncated ring-buffer of recent PTY output
	// (TASK-MB-04-02), capped at lastOutputBufferBytes — read via
	// domain.TruncatedForMobile at the point of exposure (BR-MB-15).
	lastOutput []byte
}

// appendOutput grows buf by chunk, then tail-truncates to
// lastOutputBufferBytes — keeps the MOST RECENT bytes, not the oldest.
func appendOutput(buf []byte, chunk []byte) []byte {
	buf = append(buf, chunk...)
	if len(buf) > lastOutputBufferBytes {
		buf = buf[len(buf)-lastOutputBufferBytes:]
	}
	return buf
}

// LastOutputPreview reads ptyID's BR-MB-15-truncated output preview out of
// the shared liveStates registry — exported so adapter/grpc's
// ListTerminalSessions mapping can populate TerminalSession.LastOutputPreview
// without needing access to the unexported ptyLiveState type. Empty when no
// live entry exists (cross-pod case, or liveStates itself is nil), an
// honest absence, not an error.
func LastOutputPreview(liveStates *sync.Map, ptyID string) string {
	if liveStates == nil {
		return ""
	}
	v, ok := liveStates.Load(ptyID)
	if !ok {
		return ""
	}
	return domain.TruncatedForMobile(v.(*ptyLiveState).lastOutput)
}

// AttachPty drives one bidirectional AttachPty stream: consumes decoded
// client frames (adapter/grpc reads these off the actual grpc.ServerStream
// and pushes them onto the inbound channel it hands to Execute), resolves
// the attach frame's pty_id into a live dev server, subscribes to its
// pty.data/pty.exit notifications via DevServerAgentClient.StreamPty, and
// pumps input/resize frames to the agent while relaying output/exit events
// back out. See Execute's doc comment for the two-channel contract
// adapter/grpc's handler pumps against the real stream.
type AttachPty struct {
	sessions   TerminalSessionRepository
	resolver   ConnectionResolver
	agent      DevServerAgentClient
	limiter    *ConnectionStreamLimiter
	liveStates *sync.Map // map[string]*ptyLiveState — shared with GetTerminalAgentStatus, see ptyLiveState's doc comment
	events     LifecycleEventPublisher
}

func NewAttachPty(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient, limiter *ConnectionStreamLimiter, liveStates *sync.Map, events LifecycleEventPublisher) *AttachPty {
	return &AttachPty{sessions: sessions, resolver: resolver, agent: agent, limiter: limiter, liveStates: liveStates, events: events}
}

// Execute starts a goroutine driving the stream and returns immediately with
// two channels: outbound (PtyServerMessage values for adapter/grpc to
// stream.Send) and errCh (receives at most one terminal error, then both
// channels close). inbound is owned by the caller — Execute only reads from
// it; the caller must close it (or let ctx cancellation end the read) once
// the underlying grpc.ServerStream.Recv() loop ends. tenantID is pulled from
// ctx internally (tenant.RequireTenantID), matching every other usecase in
// this package — the caller does not need to extract it itself.
func (uc *AttachPty) Execute(ctx context.Context, inbound <-chan PtyClientMessage) (<-chan PtyServerMessage, <-chan error) {
	outbound := make(chan PtyServerMessage, outboundQueueSize)
	errCh := make(chan error, 1)
	go uc.run(ctx, inbound, outbound, errCh)
	return outbound, errCh
}

func (uc *AttachPty) run(ctx context.Context, inbound <-chan PtyClientMessage, outbound chan<- PtyServerMessage, errCh chan<- error) {
	defer close(outbound)
	defer close(errCh)

	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		errCh <- apperrors.New(apperrors.KindUnauthenticated, "INFRA_NO_TENANT", "no tenant in request context", err)
		return
	}

	first, err := readFirstAttach(ctx, inbound)
	if err != nil {
		errCh <- err
		return
	}
	ptyID := first.PtyID

	session, devServer, err := resolveTerminalSession(ctx, tenantID, ptyID, uc.sessions, uc.resolver)
	if err != nil {
		errCh <- err
		return
	}

	release, err := uc.limiter.Acquire(session.ConnectionID)
	if err != nil {
		errCh <- apperrors.New(apperrors.KindFailedPrecondition, "INFRA_TERMINAL_STREAM_LIMIT", err.Error(), err)
		return
	}
	defer release()

	events, unsubscribe, err := uc.agent.StreamPty(ctx, devServer, ptyID)
	if err != nil {
		errCh <- apperrors.New(apperrors.KindInternal, "INFRA_AGENT_STREAM_PTY_FAILED", "failed to subscribe to pty output", err)
		return
	}
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-inbound:
			if !ok {
				return
			}
			if err := handleInboundPtyMessage(ctx, uc.agent, devServer, ptyID, msg); err != nil {
				errCh <- err
				return
			}
		case ev, ok := <-events:
			if !ok {
				return
			}
			if ev.Exited {
				uc.liveStates.Delete(ptyID)
				uc.publishExitEvent(ctx, tenantID, ptyID, session, ev.ExitCode)
				outbound <- PtyServerMessage{Exited: true, ExitCode: ev.ExitCode}
				return
			}
			if len(ev.Data) > 0 {
				prev, _ := uc.liveStates.Load(ptyID)
				var buf []byte
				if prev != nil {
					buf = prev.(*ptyLiveState).lastOutput
				}
				uc.liveStates.Store(ptyID, &ptyLiveState{lastOutputAt: time.Now(), agentRunning: true, lastOutput: appendOutput(buf, ev.Data)})
			}
			outbound <- PtyServerMessage{Output: ev.Data}
		}
	}
}

// publishExitEvent publishes agent_completed/agent_error for one pty exit —
// best-effort (logged, never fails the relay loop): a missed publish only
// means a mobile push notification is late/missing (TASK-MB-02-01).
func (uc *AttachPty) publishExitEvent(ctx context.Context, tenantID, ptyID string, session domain.TerminalSession, exitCode int32) {
	if uc.events == nil {
		return
	}
	subject := eventbus.SubjectAgentCompleted
	if exitCode != 0 {
		subject = eventbus.SubjectAgentError
	}
	if err := uc.events.PublishAgentLifecycle(ctx, tenantID, subject, eventbus.AgentLifecyclePayload{
		PtyID:        ptyID,
		ConnectionID: session.ConnectionID,
		ExitCode:     &exitCode,
		UserIDs:      userIDsFor(session),
	}); err != nil {
		slog.Default().WarnContext(ctx, "failed to publish agent lifecycle event", slog.Any("error", err), slog.String("subject", subject), slog.String("pty_id", ptyID))
	}
}

// userIDsFor returns the one known recipient for session's lifecycle
// events, or nil when no user identity was captured at spawn time (see
// domain.TerminalSession.CreatedByUserID's doc comment).
func userIDsFor(session domain.TerminalSession) []string {
	if session.CreatedByUserID == "" {
		return nil
	}
	return []string{session.CreatedByUserID}
}

// handleInboundPtyMessage dispatches one client-stream message (input or
// in-stream resize — a repeated Attach frame is ignored, not an error, since
// AttachPty's proto contract only documents the FIRST frame as attach).
func handleInboundPtyMessage(ctx context.Context, agent DevServerAgentClient, devServer domain.DevServer, ptyID string, msg PtyClientMessage) error {
	switch {
	case msg.Input != nil:
		if err := agent.WritePty(ctx, devServer, ptyID, msg.Input); err != nil {
			return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_WRITE_PTY_FAILED", "failed to write to pty", err)
		}
	case msg.Resize != nil:
		if err := agent.ResizePty(ctx, devServer, ptyID, msg.Resize.Cols, msg.Resize.Rows); err != nil {
			return apperrors.New(apperrors.KindInternal, "INFRA_AGENT_RESIZE_PTY_FAILED", "failed to resize pty", err)
		}
	}
	return nil
}

// readFirstAttach waits for the stream's mandatory first message (an attach
// frame naming ptyID) — any other first message, or the stream ending
// before one arrives, is a protocol error.
func readFirstAttach(ctx context.Context, inbound <-chan PtyClientMessage) (*PtyAttachMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-inbound:
		if !ok {
			return nil, errors.New("usecase: AttachPty stream closed before an attach frame arrived")
		}
		if msg.Attach == nil {
			return nil, errors.New("usecase: AttachPty stream must open with an attach frame naming pty_id")
		}
		if msg.Attach.PtyID == "" {
			return nil, errors.New("usecase: AttachPty attach frame's pty_id is empty")
		}
		return msg.Attach, nil
	}
}
