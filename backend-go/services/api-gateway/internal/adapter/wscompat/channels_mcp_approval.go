package wscompat

import (
	"context"
	"encoding/json"
	"errors"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// McpApproval mirrors CONTRACT section 1.
type McpApproval struct {
	ID          string          `json:"id"`
	CreatedAt   string          `json:"createdAt"`
	ExpiresAt   string          `json:"expiresAt"`
	Status      string          `json:"status"`
	Tool        McpApprovalTool `json:"tool"`
	ClientName  string          `json:"clientName"`
	SessionID   string          `json:"sessionId"`
	ArgsPreview McpArgsPreview  `json:"argsPreview"`
	ParamsHash  string          `json:"paramsHash"`
	DecidedAt   string          `json:"decidedAt,omitempty"`
	DecidedVia  string          `json:"decidedVia,omitempty"`
	Reasons     []string        `json:"reasons,omitempty"`
}

type McpApprovalTool struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Risk  string `json:"risk"`
}

type McpArgsPreview struct {
	Text     string `json:"text"`
	Redacted bool   `json:"redacted"`
}

func toMcpApproval(a *mcpv1.Approval) McpApproval {
	return McpApproval{
		ID: a.GetId(), CreatedAt: tsString(a.GetCreatedAt()), ExpiresAt: tsString(a.GetExpiresAt()), Status: a.GetStatus(),
		Tool:       McpApprovalTool{Name: a.GetToolName(), Title: a.GetToolTitle(), Risk: a.GetRisk()},
		ClientName: a.GetClientName(), SessionID: a.GetSessionId(),
		ArgsPreview: McpArgsPreview{Text: a.GetArgsPreview(), Redacted: a.GetArgsRedacted()},
		ParamsHash:  a.GetParamsHash(), DecidedAt: tsString(a.GetDecidedAt()), DecidedVia: a.GetDecidedVia(), Reasons: a.GetReasons(),
	}
}

func registerMcpApprovalChannels(r *Registry, d McpChannelDeps) {
	r.Register("mcp.approval.list", mcpHandler(d, false, mcpApprovalList(d)))
	r.Register("mcp.approval.decide", mcpHandler(d, false, mcpApprovalDecide(d)))
}

func mcpApprovalList(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		var in struct {
			Status string `json:"status"`
			Cursor string `json:"cursor"`
			Limit  int    `json:"limit"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args[0], &in); err != nil {
				return nil, errors.New("MCP_INVALID_ARGUMENT: argument is not a valid object")
			}
		}
		if in.Status != "" && in.Status != "pending" && in.Status != "all" {
			return nil, errors.New("MCP_INVALID_ARGUMENT: status must be pending or all")
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := c.ListApprovals(ctx, &mcpv1.ListApprovalsRequest{Status: in.Status, Cursor: in.Cursor, Limit: int32(in.Limit)})
		if err != nil {
			return nil, err
		}
		out := struct {
			Approvals  []McpApproval `json:"approvals"`
			NextCursor string        `json:"nextCursor,omitempty"`
		}{Approvals: make([]McpApproval, 0, len(resp.GetApprovals())), NextCursor: resp.GetNextCursor()}
		for _, a := range resp.GetApprovals() {
			out.Approvals = append(out.Approvals, toMcpApproval(a))
		}
		return out, nil
	}
}

// mcpApprovalDecide is the ONLY path that can decide an approval, and it is
// reachable only with a cookie/paired-device session: MCP bearer tokens are
// refused by the WS verifier (aud), and mcp.* is hard-denied to agents. The
// owner check happens in mcp-service against the user in the session
// identity; `decidedVia` is derived from the session, never from parameters.
func mcpApprovalDecide(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := mcpArgs[struct {
			ApprovalID string `json:"approvalId"`
			Decision   string `json:"decision"`
			ParamsHash string `json:"paramsHash"`
			Note       string `json:"note"`
		}](args)
		if err != nil {
			return nil, err
		}
		if id.UserID == "" {
			return nil, errors.New("MCP_NOT_FOUND: not found")
		}
		via := "web"
		if id.DeviceID != "" {
			via = "mobile"
		}
		c, err := mcpClient(d)
		if err != nil {
			return nil, err
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		a, err := c.DecideApproval(ctx, &mcpv1.DecideApprovalRequest{
			ApprovalId: in.ApprovalID, Decision: in.Decision, ParamsHash: in.ParamsHash, Note: in.Note, Via: via,
		})
		if err != nil {
			return nil, err
		}
		return toMcpApproval(a), nil
	}
}
