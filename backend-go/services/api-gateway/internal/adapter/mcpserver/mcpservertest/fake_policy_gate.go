package mcpservertest

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcpserver"
)

// FakeGate is a scriptable mcpserver.PolicyGate. Zero value allows everything.
type FakeGate struct {
	mu sync.Mutex
	// Decisions maps tool name -> decision; Default applies otherwise
	// (empty Outcome = allow).
	Decisions map[string]mcpserver.GateDecision
	Default   mcpserver.GateDecision
	DecideErr error
	// Approved / ApproveErr script AwaitApproval.
	Approved   bool
	ApproveErr error
	// Effective scripts ToolPolicyView when non-nil.
	Effective func(meta mcpserver.ToolMeta) mcpserver.EffectiveDecision

	Decided   []mcpserver.ToolMeta
	Completed []Completion
}

type Completion struct {
	Meta   mcpserver.ToolMeta
	Result string
}

func (g *FakeGate) Decide(_ context.Context, _ mcpserver.Principal, meta mcpserver.ToolMeta, _ json.RawMessage) (mcpserver.GateDecision, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Decided = append(g.Decided, meta)
	if g.DecideErr != nil {
		return mcpserver.GateDecision{}, g.DecideErr
	}
	if d, ok := g.Decisions[meta.Name]; ok {
		return d, nil
	}
	if g.Default.Outcome == "" {
		return mcpserver.GateDecision{Outcome: mcpserver.OutcomeAllow}, nil
	}
	return g.Default, nil
}

func (g *FakeGate) AwaitApproval(context.Context, mcpserver.Principal, string) (bool, error) {
	return g.Approved, g.ApproveErr
}

func (g *FakeGate) Complete(_ context.Context, _ mcpserver.Principal, meta mcpserver.ToolMeta, result string, _ time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Completed = append(g.Completed, Completion{meta, result})
}

// EffectiveDecision makes FakeGate a ToolPolicyView only through FakeViewGate.
type FakeViewGate struct{ *FakeGate }

func (g FakeViewGate) EffectiveDecision(_ context.Context, _ string, meta mcpserver.ToolMeta) (mcpserver.EffectiveDecision, error) {
	if g.Effective != nil {
		return g.Effective(meta), nil
	}
	return mcpserver.EffectiveDecision{Decision: mcpserver.OutcomeAllow, Source: "default"}, nil
}
