package agentrelay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ExecPrompt runs agent.execPrompt on the target's agent and returns the parsed result. The per-line output
// notifications of the run are not touched here (see the package doc).
func (r *Relayer) ExecPrompt(ctx context.Context, t Target, p ExecParams) (ExecResult, error) {
	if err := p.validate(); err != nil {
		return ExecResult{}, err
	}
	cctx, cancel, err := r.prepare(ctx, r.execDeadline(p))
	if err != nil {
		return ExecResult{}, err
	}
	defer cancel()
	raw, err := json.Marshal(p)
	if err != nil {
		return ExecResult{}, fmt.Errorf("agentrelay: marshal %s params: %w", MethodExecPrompt, err)
	}
	resultJSON, err := r.call(cctx, t, MethodExecPrompt, raw)
	if err != nil {
		return ExecResult{}, classifyExecError(err)
	}
	return parseExecResult(resultJSON)
}

// execDeadline is the run's own timeout plus slack, so the agent can report a timeout before the RPC deadline hides it.
// Zero means the run has no timeout of ours and the caller's context decides.
func (r *Relayer) execDeadline(p ExecParams) time.Duration {
	if !r.derivesDeadline || p.TimeoutMS <= 0 {
		return 0
	}
	return time.Duration(p.TimeoutMS)*time.Millisecond + r.slack
}
