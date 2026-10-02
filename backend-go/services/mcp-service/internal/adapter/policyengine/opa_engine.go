// Package policyengine evaluates the orca.authz.mcp Rego package in-process.
package policyengine

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/stablyai/orca-go/common/policy"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
)

const (
	decisionQuery     = "data.orca.authz.mcp.decision"
	hardDeniedQuery   = "data.orca.authz.mcp.hard_denied"
	hardChannelsQuery = "data.orca.authz.mcp.hard_deny_channels"
	hardPrefixesQuery = "data.orca.authz.mcp.hard_deny_prefixes"
)

// evalTimeout bounds an in-process evaluation (override of the 5s default:
// local CPU work, and a pathological policy must not stall tool calls).
const evalTimeout = 250 * time.Millisecond

// valuer is the part of policy.Evaluator the engine needs (fakeable in tests).
type valuer interface {
	Value(ctx context.Context, query string, input any) (any, error)
	Warm(ctx context.Context, queries ...string) error
}

type OPAEngine struct {
	ev  valuer
	log *slog.Logger
}

func New(bundlePath string, log *slog.Logger) *OPAEngine {
	return NewWithEvaluator(policy.NewEvaluator(bundlePath), log)
}

func NewWithEvaluator(ev valuer, log *slog.Logger) *OPAEngine {
	if log == nil {
		log = slog.Default()
	}
	return &OPAEngine{ev: ev, log: log}
}

// Warm compiles every query at startup so a broken bundle stops the service
// from starting instead of denying every call at runtime.
func (e *OPAEngine) Warm(ctx context.Context) error {
	return e.ev.Warm(ctx, decisionQuery, hardDeniedQuery, hardChannelsQuery, hardPrefixesQuery)
}

// Evaluate fails closed: any error, undefined result or malformed output is a deny.
func (e *OPAEngine) Evaluate(ctx context.Context, in domain.PolicyInput) domain.PolicyDecision {
	ctx, cancel := context.WithTimeout(ctx, evalTimeout)
	defer cancel()
	v, err := e.ev.Value(ctx, decisionQuery, in.AsMap())
	if err != nil {
		e.log.ErrorContext(ctx, "policy evaluation failed", slog.Any("error", err))
		return domain.DenyUnavailable()
	}
	m, ok := v.(map[string]any)
	if !ok {
		return domain.DenyUnavailable()
	}
	return domain.ParseDecision(m)
}

func (e *OPAEngine) stringSet(ctx context.Context, query string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, evalTimeout)
	defer cancel()
	v, err := e.ev.Value(ctx, query, map[string]any{})
	if err != nil {
		return nil, err
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("policyengine: %s did not return a set", query)
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (e *OPAEngine) HardDenySets(ctx context.Context) ([]string, []string, error) {
	ch, err := e.stringSet(ctx, hardChannelsQuery)
	if err != nil {
		return nil, nil, err
	}
	pf, err := e.stringSet(ctx, hardPrefixesQuery)
	if err != nil {
		return nil, nil, err
	}
	return ch, pf, nil
}

func (e *OPAEngine) IsHardDenied(ctx context.Context, channel string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, evalTimeout)
	defer cancel()
	v, err := e.ev.Value(ctx, hardDeniedQuery, map[string]any{"tool": map[string]any{"channel": channel}})
	if err != nil {
		return false, err
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("policyengine: hard_denied returned %T", v)
	}
	return b, nil
}

var _ usecase.PolicyEngine = (*OPAEngine)(nil)
