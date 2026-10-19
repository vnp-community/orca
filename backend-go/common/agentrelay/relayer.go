package agentrelay

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// Client is the part of infra-fleet-service's gRPC client this package uses; infrafleetv1.InfraFleetServiceClient
// satisfies it.
type Client interface {
	Relay(ctx context.Context, in *infrafleetv1.RelayRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error)
	RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, opts ...grpc.CallOption) (*infrafleetv1.RelayResponse, error)
}

// Relayer sends agent RPCs through infra-fleet-service. It holds no mutable state, so one value serves concurrent
// callers.
type Relayer struct {
	client          Client
	attach          func(context.Context) (context.Context, error)
	slack           time.Duration
	derivesDeadline bool
}

// New wraps an infra-fleet client. Without WithMetadata the context is sent as is.
func New(client Client, opts ...Option) *Relayer {
	r := &Relayer{client: client, slack: DefaultTimeoutSlack, derivesDeadline: true}
	for _, o := range opts {
		o(r)
	}
	return r
}

// prepare attaches metadata, then bounds the call by timeout when it is positive. The returned cancel is never nil.
func (r *Relayer) prepare(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc, error) {
	if r.attach != nil {
		attached, err := r.attach(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("agentrelay: attach request metadata: %w", err)
		}
		ctx = attached
	}
	if timeout > 0 {
		cctx, cancel := context.WithTimeout(ctx, timeout)
		return cctx, cancel, nil
	}
	return ctx, func() {}, nil
}

// call sends one RPC and returns the relay's resultJson; the error is the raw transport error for the classifiers.
func (r *Relayer) call(ctx context.Context, t Target, method string, paramsJSON []byte) (string, error) {
	byConn, byDev := t.relayRequest(method, paramsJSON)
	var (
		resp *infrafleetv1.RelayResponse
		err  error
	)
	if byDev != nil {
		resp, err = r.client.RelayByDevServer(ctx, byDev)
	} else {
		resp, err = r.client.Relay(ctx, byConn)
	}
	if err != nil {
		return "", err
	}
	return resp.GetResultJson(), nil
}
