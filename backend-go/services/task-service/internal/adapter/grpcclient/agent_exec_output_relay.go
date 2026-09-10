package grpcclient

import (
	"context"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/task-service/internal/usecase"
)

// AgentExecOutputRelay implements usecase.AgentExecOutputStreamer against
// infra-fleet-service's real StreamExecOutput RPC (TASK-AG-FLOWTASK-002) —
// SimpleExecutor's one caller. Kept as its own small adapter (not inlined
// into SimpleExecutor) so SimpleExecutor's own tests only need a trivial
// channel-returning fake of the narrow usecase.AgentExecOutputStreamer
// port, not a hand-rolled grpc.ServerStreamingClient — that complexity
// lives once, in this file's own test.
type AgentExecOutputRelay struct {
	relay infrafleetv1.InfraFleetServiceClient
}

func NewAgentExecOutputRelay(relay infrafleetv1.InfraFleetServiceClient) *AgentExecOutputRelay {
	return &AgentExecOutputRelay{relay: relay}
}

// StreamExecOutput opens the RPC and pumps stream.Recv() into a channel —
// see usecase.AgentExecOutputStreamer's doc comment for why a dial/stream
// failure here just closes an empty channel rather than returning an error:
// SimpleExecutor's own unary Relay('agent.execPrompt') call is the real
// completion signal, this is best-effort progress only.
func (r *AgentExecOutputRelay) StreamExecOutput(ctx context.Context, connectionID, stepID string) <-chan usecase.AgentExecOutputChunk {
	out := make(chan usecase.AgentExecOutputChunk, 64)

	stream, err := r.relay.StreamExecOutput(ctx, &infrafleetv1.StreamExecOutputRequest{
		ConnectionId: connectionID, StepId: stepID,
	})
	if err != nil {
		close(out)
		return out
	}

	go func() {
		defer close(out)
		for {
			ev, err := stream.Recv()
			if err != nil {
				return // io.EOF (server closed the stream — see AttachPty's own precedent) or a transport/context-cancel error, either way stop
			}
			select {
			case out <- usecase.AgentExecOutputChunk{Stream: ev.GetStream(), Data: ev.GetData()}:
			case <-ctx.Done():
				return
			}
		}
	}()

	return out
}
