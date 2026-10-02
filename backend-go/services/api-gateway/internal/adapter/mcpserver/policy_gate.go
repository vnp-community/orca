package mcpserver

import (
	"context"
	"encoding/json"
	"time"
)

type ToolMeta struct {
	Name, Channel, Namespace, Risk, RequiredScope string
	OpenWorld, UntrustedOutput                    bool
} // Risk: read|write_reversible|exec|destructive|admin
type GateDecision struct {
	Outcome    string /* "allow"|"require_approval"|"deny" */
	Reasons    []string
	ApprovalID string
	Message    string
	// ElicitationEligible / ApprovalPrompt: mcp-service allows asking the
	// client user in-band (write_reversible only). The prompt is built by the
	// server from the verbatim argument preview, never by the model.
	ElicitationEligible bool
	ApprovalPrompt      string
}
type PolicyGate interface {
	Decide(ctx context.Context, p Principal, meta ToolMeta, args json.RawMessage) (GateDecision, error)
	AwaitApproval(ctx context.Context, p Principal, approvalID string) (approved bool, err error)
	Complete(ctx context.Context, p Principal, meta ToolMeta, result string /* "ok"|"error" */, dur time.Duration)
}

// ElicitationDecider is an OPTIONAL capability of a PolicyGate: record the
// answer of an in-band elicitation (decided_via='elicitation'). mcp-service
// re-checks the tool's risk and ignores an approve for anything but
// write_reversible; a decline/cancel is always honored.
type ElicitationDecider interface {
	DecideByElicitation(ctx context.Context, p Principal, approvalID string, approve bool) error
}
