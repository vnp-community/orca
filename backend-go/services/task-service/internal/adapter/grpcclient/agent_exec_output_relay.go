package grpcclient

import (
	"context"
	"errors"
	"io"

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
// failure here closes the channel (after reporting it to onFailure) rather
// than returning an error: SimpleExecutor's own unary Relay call is the real
// completion signal, this is best-effort progress only.
func (r *AgentExecOutputRelay) StreamExecOutput(ctx context.Context, target usecase.AgentExecOutputTarget, onFailure func(error)) <-chan usecase.AgentExecOutputChunk {
	out := make(chan usecase.AgentExecOutputChunk, 64)
	fail := func(err error) {
		if onFailure != nil {
			onFailure(err)
		}
	}

	// ctx is expected to already carry tenant on its OUTGOING metadata —
	// SimpleExecutor.Execute passes the same relayCtx it built via
	// withTenantMetadata for its own Relay call (see that file's doc
	// comment for why the plain incoming ctx alone isn't enough:
	// infra-fleet-service rejects any call with no tenant in outgoing
	// metadata). This adapter doesn't rebuild it itself since it has no
	// tenantID parameter to build it from.
	stream, err := r.relay.StreamExecOutput(ctx, &infrafleetv1.StreamExecOutputRequest{
		ConnectionId: target.ConnectionID, DevServerId: target.DevServerID, StepId: target.StepID,
	})
	if err != nil {
		fail(err)
		close(out)
		return out
	}

	go func() {
		defer close(out)
		for {
			ev, err := stream.Recv()
			if err != nil {
				// The server's own refusal (e.g. no live agent session) only surfaces here, on the first Recv.
				// io.EOF is the server closing the stream; a cancelled ctx is SimpleExecutor ending the run.
				if !errors.Is(err, io.EOF) && ctx.Err() == nil {
					fail(err)
				}
				return
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
