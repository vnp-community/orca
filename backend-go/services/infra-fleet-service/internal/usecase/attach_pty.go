package usecase

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
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

// AttachPty drives one bidirectional AttachPty stream: consumes decoded
// client frames (adapter/grpc reads these off the actual grpc.ServerStream
// and pushes them onto the inbound channel it hands to Execute), resolves
// the attach frame's pty_id into a live dev server, subscribes to its
// pty.data/pty.exit notifications via DevServerAgentClient.StreamPty, and
// pumps input/resize frames to the agent while relaying output/exit events
// back out. See Execute's doc comment for the two-channel contract
// adapter/grpc's handler pumps against the real stream.
type AttachPty struct {
	sessions TerminalSessionRepository
	resolver ConnectionResolver
	agent    DevServerAgentClient
	limiter  *ConnectionStreamLimiter
}

func NewAttachPty(sessions TerminalSessionRepository, resolver ConnectionResolver, agent DevServerAgentClient, limiter *ConnectionStreamLimiter) *AttachPty {
	return &AttachPty{sessions: sessions, resolver: resolver, agent: agent, limiter: limiter}
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
				outbound <- PtyServerMessage{Exited: true, ExitCode: ev.ExitCode}
				return
			}
			outbound <- PtyServerMessage{Output: ev.Data}
		}
	}
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
