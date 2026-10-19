package agentrelay

import (
	"context"
	"sync"
	"time"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// recordedCall is one RPC the fake saw, with the facts about its context the tests care about.
type recordedCall struct {
	viaDevServer bool
	id           string // connection id or dev server id
	method       string
	paramsJSON   string
	hasDeadline  bool
	timeLeft     time.Duration
	metadata     any
}

type metadataKey struct{}

// fakeClient records calls and answers with respond. It is safe for concurrent use.
type fakeClient struct {
	mu      sync.Mutex
	calls   []recordedCall
	respond func(ctx context.Context) (string, error)
}

func (f *fakeClient) record(ctx context.Context, c recordedCall) (*infrafleetv1.RelayResponse, error) {
	if dl, ok := ctx.Deadline(); ok {
		c.hasDeadline, c.timeLeft = true, time.Until(dl)
	}
	c.metadata = ctx.Value(metadataKey{})
	f.mu.Lock()
	f.calls = append(f.calls, c)
	f.mu.Unlock()
	out, err := f.respond(ctx)
	if err != nil {
		return nil, err
	}
	return &infrafleetv1.RelayResponse{ResultJson: out}, nil
}

func (f *fakeClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.record(ctx, recordedCall{id: in.GetConnectionId(), method: in.GetMethod(), paramsJSON: in.GetParamsJson()})
}

func (f *fakeClient) RelayByDevServer(ctx context.Context, in *infrafleetv1.RelayByDevServerRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.record(ctx, recordedCall{viaDevServer: true, id: in.GetDevServerId(), method: in.GetMethod(), paramsJSON: in.GetParamsJson()})
}

func (f *fakeClient) snapshot() []recordedCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]recordedCall(nil), f.calls...)
}

func answer(resultJSON string) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return resultJSON, nil }
}

func fail(err error) func(context.Context) (string, error) {
	return func(context.Context) (string, error) { return "", err }
}
