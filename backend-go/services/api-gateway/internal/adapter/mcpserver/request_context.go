package mcpserver

import "context"

// ElicitDecision is a user's answer to an MCP elicitation/create form.
type ElicitDecision struct {
	Action  string // accept | decline | cancel
	Approve bool   // the form's boolean, only meaningful for accept
}

// Elicitor asks the connected client's user a yes/no question. It is present
// in a tool call's context ONLY when the client declared the elicitation
// capability. Elicitation is advisory for anything but write_reversible tools
// (mcp-service enforces that); it travels through the possibly manipulated
// client, so it must never be the only approval path for riskier tools.
//
// Limitation: the client's answer is a separate POST that must reach the
// replica that sent the question (the SDK keeps outgoing calls per connection).
// Behind a load balancer without session affinity the question can time out;
// the out-of-band approval still works, so nothing is lost but convenience.
type Elicitor func(ctx context.Context, message string) (ElicitDecision, error)

type elicitorKey struct{}

func WithElicitor(ctx context.Context, e Elicitor) context.Context {
	return context.WithValue(ctx, elicitorKey{}, e)
}

func ElicitorFromContext(ctx context.Context) (Elicitor, bool) {
	e, ok := ctx.Value(elicitorKey{}).(Elicitor)
	return e, ok && e != nil
}

type progressKey struct{}

// ProgressFunc reports tool progress to the client; throttled to 5/s per token.
type ProgressFunc func(progress, total float64, message string)

func WithProgress(ctx context.Context, f ProgressFunc) context.Context {
	return context.WithValue(ctx, progressKey{}, f)
}

// ReportProgress is a no-op unless the client sent a progressToken.
func ReportProgress(ctx context.Context, progress, total float64, message string) {
	if f, ok := ctx.Value(progressKey{}).(ProgressFunc); ok && f != nil {
		f(progress, total, message)
	}
}
