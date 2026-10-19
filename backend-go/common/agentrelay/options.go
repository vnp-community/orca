package agentrelay

import (
	"context"
	"time"
)

// DefaultTimeoutSlack lets the agent report its own timeout before the RPC deadline hides it.
const DefaultTimeoutSlack = 30 * time.Second

// Option configures a Relayer at construction; a Relayer is immutable afterwards, so it is safe for concurrent use.
type Option func(*Relayer)

// WithMetadata attaches request-scoped data (tenant, acting user) to the outgoing context. Each service passes its own
// function, because what a downstream requires differs per service.
func WithMetadata(attach func(context.Context) (context.Context, error)) Option {
	return func(r *Relayer) {
		if attach != nil {
			r.attach = attach
		}
	}
}

// WithTimeoutSlack sets how much longer than ExecParams.TimeoutMS the RPC may run. A negative value is treated as zero.
func WithTimeoutSlack(d time.Duration) Option {
	return func(r *Relayer) { r.slack = max(d, 0) }
}

// WithoutCallDeadline stops ExecPrompt from deriving an RPC deadline from TimeoutMS, for callers whose own context
// already bounds the run (the agent still enforces TimeoutMS itself).
func WithoutCallDeadline() Option {
	return func(r *Relayer) { r.derivesDeadline = false }
}
