package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// EvaluateToolCall is the side-effect-free policy decision (also used for explain).
type EvaluateToolCall struct{ core *GovernanceCore }

func NewEvaluateToolCall(core *GovernanceCore) *EvaluateToolCall {
	return &EvaluateToolCall{core: core}
}

func (uc *EvaluateToolCall) Execute(ctx context.Context, tool domain.ToolRef, cc domain.CallContext) (domain.PolicyDecision, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return domain.PolicyDecision{}, err
	}
	ds, _, _, _ := uc.core.evalBatch(ctx, id, []domain.ToolRef{tool}, cc, false)
	return ds[0], nil
}

// FilterTools evaluates N tools against one snapshot (no N DB round trips).
type FilterTools struct{ core *GovernanceCore }

func NewFilterTools(core *GovernanceCore) *FilterTools { return &FilterTools{core: core} }

type ToolDecision struct {
	Name     string
	Decision domain.PolicyDecision
}

func (uc *FilterTools) Execute(ctx context.Context, tools []domain.ToolRef, cc domain.CallContext) ([]ToolDecision, int64, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return nil, 0, err
	}
	ds, snap, _, _ := uc.core.evalBatch(ctx, id, tools, cc, false)
	out := make([]ToolDecision, len(tools))
	for i, t := range tools {
		out[i] = ToolDecision{Name: t.Name, Decision: ds[i]}
	}
	return out, snap.Epoch, nil
}

// ExplainPolicy answers "what would happen" for an admin. It reuses the same
// evaluation path (no second implementation), ignoring token scopes and taint.
type ExplainPolicy struct{ core *GovernanceCore }

func NewExplainPolicy(core *GovernanceCore) *ExplainPolicy { return &ExplainPolicy{core: core} }

type ExplainInput struct {
	Tool     domain.ToolRef
	UserID   string
	UserRole string
	ClientID string
}

func (uc *ExplainPolicy) Execute(ctx context.Context, in ExplainInput) (domain.PolicyDecision, error) {
	caller, err := userCaller(ctx)
	if err != nil {
		return domain.PolicyDecision{}, err
	}
	if err := caller.requireAdmin(); err != nil {
		return domain.PolicyDecision{}, err
	}
	if in.Tool.Name == "" || in.Tool.Channel == "" {
		return domain.PolicyDecision{}, domain.ErrNotFound()
	}
	target := caller
	roleUnresolved := false
	if in.UserID != "" {
		target.UserID = in.UserID
		target.Role = in.UserRole
		if target.Role == "" {
			target.Role = domain.RoleUser
			roleUnresolved = true
		}
	}
	// Explain asks about the tool, not about a particular token: grant the scope it needs.
	cc := domain.CallContext{ClientID: in.ClientID, TokenKind: "mcp_pat", Scopes: []string{in.Tool.RequiredScope}}
	if in.ClientID != "" {
		cc.TokenKind = ""
	}
	ds, _, _, _ := uc.core.evalBatch(ctx, target, []domain.ToolRef{in.Tool}, cc, true)
	d := ds[0]
	if roleUnresolved {
		d.Reasons = append(d.Reasons, "role_unresolved")
	}
	return d, nil
}
