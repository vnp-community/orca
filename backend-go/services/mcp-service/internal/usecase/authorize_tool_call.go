package usecase

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
)

// Neutral, agent-facing texts. They never reveal policy ids or hard-deny
// membership; details go to the audit trail only.
var neutralMessages = map[string]string{
	domain.ReasonRateLimited:   "Rate limit exceeded; retry shortly.",
	domain.ReasonLoopSlowDown:  "Repeated identical call; slow down.",
	domain.ReasonLoopBlocked:   "Repeated identical call; blocked for a few minutes.",
	domain.ReasonEgressSecret:  "This call was blocked because its arguments contain credentials.",
	domain.ReasonArgsTooLarge:  "The arguments are too large to be reviewed.",
	domain.ReasonApprovalFlood: "Too many approval requests are already pending.",
	domain.ReasonInvalidArgs:   "The tool arguments are invalid.",
	domain.ReasonKillSwitch:    "MCP access is temporarily disabled by an administrator.",
	"policy_unavailable":       "Orca cannot authorize this action right now.",
}

const defaultDenyMessage = "This action isn't permitted by your organization's policy."

func neutralMessage(reason string) string {
	if m, ok := neutralMessages[reason]; ok {
		return m
	}
	return defaultDenyMessage
}

type AuthorizeInput struct {
	Tool domain.ToolRef
	Ctx  domain.CallContext
	Args []byte
}

type AuthorizeOutput struct {
	Outcome, Source, ApprovalID, CallID, Message, ParamsHash, Prompt string
	Reasons                                                          []string
	ApprovalExpiresAt                                                *time.Time
	ElicitationEligible                                              bool
}

// AuthorizeToolCall is the full gate. Order is invariant: kill state, policy,
// argument checks, approval, rate limit + journal.
type AuthorizeToolCall struct {
	core      *GovernanceCore
	approvals ApprovalRepository
	calls     ToolCallRepository
	red       domain.Redactor
	clock     Clock

	mu         sync.Mutex
	suppressed map[string]suppressState
}

type suppressState struct {
	last  time.Time
	count int
}

func NewAuthorizeToolCall(core *GovernanceCore, approvals ApprovalRepository, calls ToolCallRepository, red domain.Redactor, clock Clock) *AuthorizeToolCall {
	return &AuthorizeToolCall{core: core, approvals: approvals, calls: calls, red: red, clock: clock, suppressed: map[string]suppressState{}}
}

func (uc *AuthorizeToolCall) deny(ctx context.Context, base domain.ToolCall, source, reason string, reasons []string, record bool) AuthorizeOutput {
	if len(reasons) == 0 {
		reasons = []string{reason}
	}
	if record {
		base.Decision, base.ReasonCode, base.State = domain.CallDeny, reason, domain.CallStateDone
		if uc.shouldRecord(base, reason) {
			base.SuppressedCount = uc.takeSuppressed(base)
			// A failed journal write must not turn a deny into anything else.
			_ = uc.calls.RecordFinalCall(ctx, base, uc.clock.Now())
		}
	}
	return AuthorizeOutput{Outcome: domain.DecisionDeny, Source: source, Reasons: reasons, Message: neutralMessage(reason), ParamsHash: base.ParamsHash}
}

func suppressKey(c domain.ToolCall) string {
	return c.TenantID + "|" + c.UserID + "|" + c.ClientID + "|" + c.ToolName
}

// shouldRecord throttles audit rows for flood-type denies to one per 10s per
// (user, client, tool); the skipped ones are counted into the next row.
func (uc *AuthorizeToolCall) shouldRecord(c domain.ToolCall, reason string) bool {
	if reason != domain.ReasonRateLimited && reason != domain.ReasonLoopSlowDown && reason != domain.ReasonLoopBlocked {
		return true
	}
	uc.mu.Lock()
	defer uc.mu.Unlock()
	k := suppressKey(c)
	st := uc.suppressed[k]
	now := uc.clock.Now()
	if !st.last.IsZero() && now.Sub(st.last) < 10*time.Second {
		st.count++
		uc.suppressed[k] = st
		return false
	}
	return true
}

func (uc *AuthorizeToolCall) takeSuppressed(c domain.ToolCall) int {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	k := suppressKey(c)
	st := uc.suppressed[k]
	n := st.count
	uc.suppressed[k] = suppressState{last: uc.clock.Now()}
	if len(uc.suppressed) > 10000 { // bound memory under key churn
		uc.suppressed = map[string]suppressState{k: uc.suppressed[k]}
	}
	return n
}

func (uc *AuthorizeToolCall) Execute(ctx context.Context, in AuthorizeInput) (AuthorizeOutput, error) {
	id, err := userCaller(ctx)
	if err != nil {
		return AuthorizeOutput{}, err
	}
	now := uc.clock.Now()
	decs, snap, killEntry, killed := uc.core.evalBatch(ctx, id, []domain.ToolRef{in.Tool}, in.Ctx, false)
	d := decs[0]
	cc := in.Ctx
	base := domain.ToolCall{
		ID: uuid.NewString(), TenantID: id.TenantID, UserID: id.UserID, ClientID: cc.ClientID,
		ClientName: domain.SanitizeClientName(cc.ClientName), SessionID: cc.MCPSessionID, RootSessionID: cc.MCPRoot,
		ToolName: in.Tool.Name, Channel: in.Tool.Channel, Risk: in.Tool.Risk, RiskClass: domain.RiskClass(in.Tool.Risk),
		ReadUntrusted: in.Tool.ReadUntrusted, StartedAt: now,
	}
	if killed {
		_ = killEntry
		return uc.deny(ctx, base, "kill_switch", domain.ReasonKillSwitch, []string{"kill_switch_active"}, false), nil
	}
	hash, herr := domain.ParamsHash(id.TenantID, id.UserID, cc.ClientID, in.Tool.Channel, in.Args)
	if herr != nil {
		return uc.deny(ctx, base, "arguments", domain.ReasonInvalidArgs, nil, true), nil
	}
	base.ParamsHash = hash
	preview, redacted, perr := domain.ArgsPreview(in.Args, uc.red)
	if perr != nil {
		return uc.deny(ctx, base, "arguments", domain.ReasonInvalidArgs, nil, true), nil
	}
	base.ArgsSummary = domain.SummarizeArgs(preview, uc.red)

	if d.Decision == domain.DecisionDeny {
		reason := "policy_denied"
		if len(d.Reasons) > 0 {
			reason = d.Reasons[0]
		}
		return uc.deny(ctx, base, d.Source, reason, d.Reasons, true), nil
	}
	if in.Tool.OpenWorld {
		if err := domain.ValidateEgressArgs(in.Args, uc.red); err != nil {
			return uc.deny(ctx, base, "egress_guard", domain.ReasonEgressSecret, nil, true), nil
		}
	}
	limits := uc.core.cfg.Rate

	if d.Decision == domain.DecisionRequireApproval {
		if len(in.Args) > domain.MaxArgsPreviewBytes {
			return uc.deny(ctx, base, "arguments", domain.ReasonArgsTooLarge, nil, true), nil
		}
		for attempt := 0; attempt < 2; attempt++ {
			call := base
			call.Decision, call.State = domain.CallApproved, domain.CallStateStarted
			res, err := uc.calls.AdmitToolCall(ctx, AdmitRequest{Call: call, ConsumeApprovalHash: hash, Limits: limits, Now: now})
			if err != nil {
				return AuthorizeOutput{}, wrapRepoErr(err, "failed to admit tool call")
			}
			if res.Admitted {
				return AuthorizeOutput{Outcome: domain.DecisionAllow, Source: d.Source, Reasons: d.Reasons, CallID: call.ID, ParamsHash: hash}, nil
			}
			if !res.ApprovalMissing {
				return uc.deny(ctx, base, "rate_limit", res.DenyReason, nil, true), nil
			}
			a := domain.Approval{
				ID: uuid.NewString(), TenantID: id.TenantID, UserID: id.UserID, ClientID: cc.ClientID, ClientName: base.ClientName,
				SessionID: cc.MCPSessionID, ToolName: in.Tool.Name, ToolTitle: titleOr(in.Tool), Channel: in.Tool.Channel, Risk: in.Tool.Risk,
				ParamsHash: hash, ArgsPreview: preview, ArgsRedacted: redacted, Status: domain.ApprovalPending,
				CreatedAt: now, ExpiresAt: now.Add(time.Duration(snap.Settings.ApprovalTTLSeconds) * time.Second), Reasons: d.Reasons,
			}
			got, _, err := uc.approvals.FindOrCreatePendingApproval(ctx, a, ApprovalLimits{
				MaxPendingPerClient: uc.core.cfg.MaxPendingPerClient, MaxCreatedPerHour: uc.core.cfg.MaxApprovalsPerHour}, now)
			if errors.Is(err, ErrApprovalFlood) {
				return uc.deny(ctx, base, "approval_limit", domain.ReasonApprovalFlood, nil, true), nil
			}
			if err != nil {
				return AuthorizeOutput{}, wrapRepoErr(err, "failed to create approval")
			}
			if got.Status == domain.ApprovalApproved { // approved between the two statements: consume it
				continue
			}
			exp := got.ExpiresAt
			out := AuthorizeOutput{
				Outcome: domain.DecisionRequireApproval, Source: d.Source, Reasons: d.Reasons, ApprovalID: got.ID,
				ParamsHash: hash, ApprovalExpiresAt: &exp,
				Message: "This action needs your approval in Orca. Approve it, then retry the same call.",
			}
			out.ElicitationEligible = domain.ElicitationEligibleRisks[domain.Risk(in.Tool.Risk)] && !containsString(d.Reasons, "open_world_after_untrusted_read")
			if out.ElicitationEligible {
				out.Prompt = base.ClientName + " wants to use " + titleOr(in.Tool) + " (" + in.Tool.Risk + ") with:\n" + preview + "\nApprove?"
			}
			return out, nil
		}
		return AuthorizeOutput{}, domain.ErrInternal("approval state kept changing", nil)
	}

	call := base
	call.Decision, call.State = domain.CallAllow, domain.CallStateStarted
	res, err := uc.calls.AdmitToolCall(ctx, AdmitRequest{Call: call, Limits: limits, Now: now})
	if err != nil {
		return AuthorizeOutput{}, wrapRepoErr(err, "failed to admit tool call")
	}
	if !res.Admitted {
		return uc.deny(ctx, base, "rate_limit", res.DenyReason, nil, true), nil
	}
	return AuthorizeOutput{Outcome: domain.DecisionAllow, Source: d.Source, Reasons: d.Reasons, CallID: call.ID, ParamsHash: hash}, nil
}

func titleOr(t domain.ToolRef) string {
	if strings.TrimSpace(t.Title) != "" {
		return t.Title
	}
	return t.Name
}

func containsString(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
