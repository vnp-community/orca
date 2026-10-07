package websocket

import "context"

type UnaryRunner struct{}

func (r *UnaryRunner) Run(ctx context.Context, req interface{}) (*ResultEnvelope, error) {
	return &ResultEnvelope{}, nil
}
