// Package policy is the shared, embedded OPA evaluation code Epic E's
// consuming services (auth-service, task-service, annotation-service) all
// use to query the orca-authz Rego bundle in-process — no sidecar, no
// network hop, per
// specs/backend-go/architecture/07-security-architecture.md's requirement
// that every service's fine-grained authorization check run "embedded,
// in-process." Each service constructs its own Evaluator pointed at the
// same bundle path and queries its own rule (e.g.
// "data.orca.authz.admin.allow") — one bundle, many entry-point rules.
package policy

import (
	"context"
	"fmt"
	"sync"

	"github.com/open-policy-agent/opa/rego"
)

// Evaluator loads the orca-authz Rego bundle from bundlePath and evaluates
// named boolean queries against it.
type Evaluator struct {
	bundlePath string

	mu       sync.Mutex
	prepared map[string]rego.PreparedEvalQuery
}

// NewEvaluator points an Evaluator at a Rego bundle directory (or a single
// .rego file) on disk.
func NewEvaluator(bundlePath string) *Evaluator {
	return &Evaluator{bundlePath: bundlePath, prepared: map[string]rego.PreparedEvalQuery{}}
}

// Decision evaluates the fully-qualified rule path query (e.g.
// "data.orca.authz.admin.allow") against input and reports whether it
// resolved to exactly `true`. Undefined, false, a non-boolean result, or an
// evaluation error are all treated as deny — this package has no
// partial-allow concept; callers get a plain bool and, on error, must
// decide their own fail-closed behavior (every Epic E call site in this
// system fails closed on a non-nil error, matching the fail-closed policy
// this codebase's other credential/permission checks already use).
//
// The bundle is compiled once per distinct query string and the prepared
// query is cached for the Evaluator's lifetime — a bundle edit requires a
// service restart to take effect. No hot-reload watcher exists yet; a
// deliberate simplification for this first-cut pass, not an oversight.
func (e *Evaluator) Decision(ctx context.Context, query string, input any) (bool, error) {
	pq, err := e.preparedQuery(ctx, query)
	if err != nil {
		return false, err
	}
	rs, err := pq.Eval(ctx, rego.EvalInput(input))
	if err != nil {
		return false, fmt.Errorf("policy: evaluating query %q: %w", query, err)
	}
	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return false, nil // undefined result (no matching rule) — default deny
	}
	allowed, ok := rs[0].Expressions[0].Value.(bool)
	if !ok {
		return false, fmt.Errorf("policy: query %q returned non-boolean result %T", query, rs[0].Expressions[0].Value)
	}
	return allowed, nil
}

func (e *Evaluator) preparedQuery(ctx context.Context, query string) (rego.PreparedEvalQuery, error) {
	e.mu.Lock()
	if pq, ok := e.prepared[query]; ok {
		e.mu.Unlock()
		return pq, nil
	}
	e.mu.Unlock()

	pq, err := rego.New(
		rego.Query(query),
		rego.Load([]string{e.bundlePath}, nil),
	).PrepareForEval(ctx)
	if err != nil {
		return rego.PreparedEvalQuery{}, fmt.Errorf("policy: preparing query %q against bundle %s: %w", query, e.bundlePath, err)
	}

	e.mu.Lock()
	e.prepared[query] = pq
	e.mu.Unlock()
	return pq, nil
}
