package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"regexp"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// McpAuditQuerier is the part of auth-service's client used for audit reads
// (satisfied by authv1.AuthServiceClient).
type McpAuditQuerier interface {
	QueryAuditLog(ctx context.Context, in *authv1.QueryAuditLogRequest, opts ...grpc.CallOption) (*authv1.QueryAuditLogResponse, error)
}

// McpAuditEntry mirrors CONTRACT section 1.
type McpAuditEntry struct {
	ID          string `json:"id"`
	At          string `json:"at"`
	ActorType   string `json:"actorType"`
	UserID      string `json:"userId"`
	UserName    string `json:"userName,omitempty"`
	ClientName  string `json:"clientName"`
	SessionID   string `json:"sessionId"`
	Tool        string `json:"tool"`
	Risk        string `json:"risk"`
	Decision    string `json:"decision"`
	ArgsSummary string `json:"argsSummary"`
	Result      any    `json:"result"`
	DurationMs  *int64 `json:"durationMs,omitempty"`
	Approver    string `json:"approver,omitempty"`
	TraceID     string `json:"traceId,omitempty"`
}

var (
	auditDecisions = map[string]bool{"allow": true, "deny": true, "approved": true, "denied": true, "expired": true}
	auditRisks     = map[string]bool{"read": true, "write_reversible": true, "exec": true, "destructive": true, "admin": true}
	uuidPattern    = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func registerMcpAuditChannel(r *Registry, d McpChannelDeps) {
	r.Register("mcp.admin.audit.query", mcpHandler(d, true, mcpAuditQuery(d)))
}

func mcpAuditQuery(d McpChannelDeps) ChannelHandler {
	return func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		var in struct {
			From     string `json:"from"`
			To       string `json:"to"`
			UserID   string `json:"userId"`
			Tool     string `json:"tool"`
			Decision string `json:"decision"`
			Cursor   string `json:"cursor"`
			Limit    int    `json:"limit"`
		}
		if len(args) > 0 {
			if err := json.Unmarshal(args[0], &in); err != nil {
				return nil, errors.New("MCP_INVALID_ARGUMENT: argument is not a valid object")
			}
		}
		req := &authv1.QueryAuditLogRequest{
			TenantId: id.TenantID, ActorType: "agent", Action: "mcp.tool_call", TargetId: in.Tool, PageToken: in.Cursor,
			Order: authv1.AuditOrder_AUDIT_ORDER_TIME_DESC,
			Since: timestamppb.New(time.Unix(0, 0)),
		}
		var from, to time.Time
		var err error
		if in.From != "" {
			if from, err = time.Parse(time.RFC3339, in.From); err != nil {
				return nil, errors.New("MCP_INVALID_ARGUMENT: from must be RFC 3339")
			}
			req.Since = timestamppb.New(from)
		}
		if in.To != "" {
			if to, err = time.Parse(time.RFC3339, in.To); err != nil {
				return nil, errors.New("MCP_INVALID_ARGUMENT: to must be RFC 3339")
			}
			req.To = timestamppb.New(to)
		}
		if !from.IsZero() && !to.IsZero() && from.After(to) {
			return nil, errors.New("MCP_INVALID_ARGUMENT: from is after to")
		}
		if in.UserID != "" {
			if !uuidPattern.MatchString(in.UserID) {
				return nil, errors.New("MCP_INVALID_ARGUMENT: userId must be a UUID")
			}
			req.ActorId = in.UserID
		}
		if in.Decision != "" {
			if !auditDecisions[in.Decision] {
				return nil, errors.New("MCP_INVALID_ARGUMENT: unknown decision")
			}
			req.MetadataFilters = []*authv1.AuditMetadataFilter{{Key: "decision", Value: in.Decision}}
		}
		req.PageSize = 50
		if in.Limit > 0 {
			req.PageSize = int32(min(in.Limit, 200))
		}
		if d.Audit == nil {
			return nil, errors.New("MCP_UNAVAILABLE: audit log is not configured")
		}
		ctx, cancel := mcpCtx(ctx, id)
		defer cancel()
		resp, err := d.Audit.QueryAuditLog(ctx, req)
		if err != nil {
			return nil, err
		}
		out := struct {
			Entries    []McpAuditEntry `json:"entries"`
			NextCursor string          `json:"nextCursor,omitempty"`
		}{Entries: make([]McpAuditEntry, 0, len(resp.GetEntries())), NextCursor: resp.GetNextPageToken()}
		for _, e := range resp.GetEntries() {
			entry, ok := toMcpAuditEntry(e)
			if !ok {
				slog.WarnContext(ctx, "mcp audit row skipped: missing or unknown fields", slog.String("audit_id", e.GetId()))
				continue
			}
			out.Entries = append(out.Entries, entry)
		}
		return out, nil
	}
}

// toMcpAuditEntry maps an auth-service row; rows that do not carry the
// fields the contract requires are dropped (and logged), never half-rendered.
func toMcpAuditEntry(e *authv1.AuditEntry) (McpAuditEntry, bool) {
	var meta map[string]any
	if json.Unmarshal([]byte(e.GetMetadataJson()), &meta) != nil || meta == nil {
		return McpAuditEntry{}, false
	}
	str := func(k string) string { s, _ := meta[k].(string); return s }
	decision, risk := str("decision"), str("risk")
	if !auditDecisions[decision] || !auditRisks[risk] || e.GetTargetId() == "" || e.GetActorId() == "" {
		return McpAuditEntry{}, false
	}
	out := McpAuditEntry{
		ID: e.GetId(), At: tsString(e.GetOccurredAt()), ActorType: "agent", UserID: e.GetActorId(), ClientName: str("client_name"),
		SessionID: str("mcp_session_id"), Tool: e.GetTargetId(), Risk: risk, Decision: decision, ArgsSummary: str("args_summary"),
		Approver: str("approver"), TraceID: str("trace_id"),
	}
	if r := str("result"); r == "ok" || r == "error" {
		out.Result = r
	}
	if ms, ok := meta["duration_ms"].(float64); ok && out.Result != nil {
		v := int64(ms)
		out.DurationMs = &v
	}
	return out, true
}
