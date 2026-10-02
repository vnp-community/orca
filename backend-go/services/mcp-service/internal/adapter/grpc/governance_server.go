package grpc

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/eventhub"
	"github.com/stablyai/orca-go/services/mcp-service/internal/adapter/wire"
	"github.com/stablyai/orca-go/services/mcp-service/internal/domain"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	mcpv1 "github.com/stablyai/orca-go/proto/gen/go/orca/mcp/v1"
)

// GovernanceUsecases bundles the BE-MCP-SOL-012/013 usecases.
type GovernanceUsecases struct {
	Evaluate     *usecase.EvaluateToolCall
	Filter       *usecase.FilterTools
	Explain      *usecase.ExplainPolicy
	Policies     *usecase.PolicyAdmin
	Settings     *usecase.SettingsAdmin
	Authorize    *usecase.AuthorizeToolCall
	Complete     *usecase.CompleteToolCall
	Wait         *usecase.WaitApproval
	ListApproval *usecase.ListApprovals
	Decide       *usecase.DecideApproval
	KillAdmin    *usecase.KillSwitchAdmin
	KillState    *usecase.GetKillState
	Hub          *eventhub.Hub
}

// GovernanceServer adds the governance RPCs on top of any McpServiceServer.
type GovernanceServer struct {
	mcpv1.McpServiceServer
	uc GovernanceUsecases
}

func WithGovernance(base mcpv1.McpServiceServer, uc GovernanceUsecases) *GovernanceServer {
	return &GovernanceServer{McpServiceServer: base, uc: uc}
}

func toolRef(t *mcpv1.ToolRef) domain.ToolRef {
	return domain.ToolRef{
		Name: t.GetName(), Channel: t.GetChannel(), Namespace: t.GetNamespace(), Risk: t.GetRisk(), RequiredScope: t.GetRequiredScope(),
		Title: t.GetTitle(), OpenWorld: t.GetOpenWorld(), SpawnsProcess: t.GetSpawnsProcess(), ReadUntrusted: t.GetReadUntrusted(),
	}
}

func callContext(c *mcpv1.CallContext) domain.CallContext {
	return domain.CallContext{
		ClientID: c.GetClientId(), ClientName: c.GetClientName(), TokenKind: c.GetTokenKind(), MCPSessionID: c.GetMcpSessionId(),
		MCPRoot: c.GetMcpRoot(), TokenID: c.GetTokenId(), GrantID: c.GetGrantId(), Scopes: c.GetScopes(), Depth: int(c.GetDepth()),
	}
}

func protoDecision(d domain.PolicyDecision) *mcpv1.PolicyDecision {
	return &mcpv1.PolicyDecision{Decision: d.Decision, Source: d.Source, Reasons: d.Reasons, PolicyEpoch: d.Epoch}
}

func (s *GovernanceServer) EvaluateToolCall(ctx context.Context, req *mcpv1.EvaluateToolCallRequest) (*mcpv1.PolicyDecision, error) {
	d, err := s.uc.Evaluate.Execute(ctx, toolRef(req.GetTool()), callContext(req.GetCtx()))
	if err != nil {
		return nil, toStatus(err)
	}
	return protoDecision(d), nil
}

func (s *GovernanceServer) FilterTools(ctx context.Context, req *mcpv1.FilterToolsRequest) (*mcpv1.FilterToolsResponse, error) {
	tools := make([]domain.ToolRef, 0, len(req.GetTools()))
	for _, t := range req.GetTools() {
		tools = append(tools, toolRef(t))
	}
	ds, epoch, err := s.uc.Filter.Execute(ctx, tools, callContext(req.GetCtx()))
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.FilterToolsResponse{PolicyEpoch: epoch, Decisions: make([]*mcpv1.ToolDecision, 0, len(ds))}
	for _, d := range ds {
		resp.Decisions = append(resp.Decisions, &mcpv1.ToolDecision{Name: d.Name, Decision: protoDecision(d.Decision)})
	}
	return resp, nil
}

func (s *GovernanceServer) ExplainPolicy(ctx context.Context, req *mcpv1.ExplainPolicyRequest) (*mcpv1.PolicyDecision, error) {
	d, err := s.uc.Explain.Execute(ctx, usecase.ExplainInput{
		Tool: toolRef(req.GetToolRef()), UserID: req.GetUserId(), UserRole: req.GetUserRole(), ClientID: req.GetClientId(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return protoDecision(d), nil
}

func toProtoPolicy(p domain.ToolPolicy) *mcpv1.ToolPolicy {
	return &mcpv1.ToolPolicy{
		Id: p.ID, Version: int32(p.Version), Decision: p.Decision, Note: p.Note, UpdatedBy: p.UpdatedBy,
		UpdatedAt: timestamppb.New(p.UpdatedAt),
		Match:     &mcpv1.ToolPolicyMatch{Tool: p.Match.Tool, Namespace: p.Match.Namespace, Risk: p.Match.Risk, ClientId: p.Match.ClientID, Roles: p.Match.Roles},
	}
}

func (s *GovernanceServer) ListToolPolicies(ctx context.Context, _ *emptypb.Empty) (*mcpv1.ListToolPoliciesResponse, error) {
	ps, err := s.uc.Policies.List(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.ListToolPoliciesResponse{Policies: make([]*mcpv1.ToolPolicy, 0, len(ps))}
	for _, p := range ps {
		resp.Policies = append(resp.Policies, toProtoPolicy(p))
	}
	return resp, nil
}

func (s *GovernanceServer) UpsertToolPolicy(ctx context.Context, req *mcpv1.UpsertToolPolicyRequest) (*mcpv1.ToolPolicy, error) {
	p := req.GetPolicy()
	m := p.GetMatch()
	touched := make([]domain.ToolRef, 0, len(req.GetTouchedTools()))
	for _, t := range req.GetTouchedTools() {
		touched = append(touched, toolRef(t))
	}
	out, err := s.uc.Policies.Upsert(ctx, usecase.UpsertInput{
		Policy: domain.ToolPolicy{
			ID: p.GetId(), Version: int(p.GetVersion()), Decision: p.GetDecision(), Note: p.GetNote(),
			Match: domain.ToolPolicyMatch{Tool: m.GetTool(), Namespace: m.GetNamespace(), Risk: m.GetRisk(), ClientID: m.GetClientId(), Roles: m.GetRoles()},
		},
		TouchedTools: touched,
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return toProtoPolicy(out), nil
}

func (s *GovernanceServer) DeleteToolPolicy(ctx context.Context, req *mcpv1.DeleteToolPolicyRequest) (*mcpv1.DeleteToolPolicyResponse, error) {
	if err := s.uc.Policies.Delete(ctx, req.GetPolicyId()); err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.DeleteToolPolicyResponse{}, nil
}

func toProtoSettings(st domain.TenantSettings) *mcpv1.TenantSettings {
	ks := &mcpv1.KillSwitch{Active: st.KillSwitch.Active, Reason: st.KillSwitch.Reason}
	if st.KillSwitch.At != nil {
		ks.At = timestamppb.New(*st.KillSwitch.At)
	}
	return &mcpv1.TenantSettings{
		Enabled: st.Enabled, DcrEnabled: st.DCREnabled, MaxTokenDays: int32(st.MaxTokenDays),
		ApprovalTtlSeconds: int32(st.ApprovalTTLSeconds), KillSwitch: ks,
	}
}

func (s *GovernanceServer) GetTenantSettings(ctx context.Context, _ *emptypb.Empty) (*mcpv1.TenantSettings, error) {
	st, err := s.uc.Settings.Get(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	return toProtoSettings(st), nil
}

func (s *GovernanceServer) UpdateTenantSettings(ctx context.Context, req *mcpv1.UpdateTenantSettingsRequest) (*mcpv1.TenantSettings, error) {
	patch := usecase.SettingsPatch{Enabled: req.Enabled, DCREnabled: req.DcrEnabled}
	if req.MaxTokenDays != nil {
		v := int(*req.MaxTokenDays)
		patch.MaxTokenDays = &v
	}
	if req.ApprovalTtlSeconds != nil {
		v := int(*req.ApprovalTtlSeconds)
		patch.ApprovalTTLSeconds = &v
	}
	st, err := s.uc.Settings.Set(ctx, patch)
	if err != nil {
		return nil, toStatus(err)
	}
	return toProtoSettings(st), nil
}

func (s *GovernanceServer) AuthorizeToolCall(ctx context.Context, req *mcpv1.AuthorizeToolCallRequest) (*mcpv1.AuthorizeToolCallResponse, error) {
	out, err := s.uc.Authorize.Execute(ctx, usecase.AuthorizeInput{Tool: toolRef(req.GetTool()), Ctx: callContext(req.GetCtx()), Args: req.GetArguments()})
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.AuthorizeToolCallResponse{
		Outcome: out.Outcome, Source: out.Source, Reasons: out.Reasons, ApprovalId: out.ApprovalID, CallId: out.CallID, Message: out.Message,
		ElicitationEligible: out.ElicitationEligible, ApprovalPrompt: out.Prompt, ParamsHash: out.ParamsHash,
	}
	if out.ApprovalExpiresAt != nil {
		resp.ApprovalExpiresAt = timestamppb.New(*out.ApprovalExpiresAt)
	}
	return resp, nil
}

func (s *GovernanceServer) CompleteToolCall(ctx context.Context, req *mcpv1.CompleteToolCallRequest) (*mcpv1.CompleteToolCallResponse, error) {
	if err := s.uc.Complete.Execute(ctx, usecase.CompleteInput{
		CallID: req.GetCallId(), Result: req.GetResult(), ReasonCode: req.GetReasonCode(), DurationMs: req.GetDurationMs(),
	}); err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.CompleteToolCallResponse{}, nil
}

func (s *GovernanceServer) WaitApproval(ctx context.Context, req *mcpv1.WaitApprovalRequest) (*mcpv1.WaitApprovalResponse, error) {
	st, err := s.uc.Wait.Execute(ctx, req.GetApprovalId(), time.Duration(req.GetMaxWaitMs())*time.Millisecond)
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.WaitApprovalResponse{Status: st}, nil
}

func (s *GovernanceServer) ListApprovals(ctx context.Context, req *mcpv1.ListApprovalsRequest) (*mcpv1.ListApprovalsResponse, error) {
	out, err := s.uc.ListApproval.Execute(ctx, usecase.ListApprovalsInput{
		PendingOnly: req.GetStatus() != "all", Cursor: req.GetCursor(), Limit: int(req.GetLimit()),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	now := time.Now().UTC()
	resp := &mcpv1.ListApprovalsResponse{NextCursor: out.NextCursor, Approvals: make([]*mcpv1.Approval, 0, len(out.Approvals))}
	for _, a := range out.Approvals {
		resp.Approvals = append(resp.Approvals, wire.Approval(a, now))
	}
	return resp, nil
}

func (s *GovernanceServer) DecideApproval(ctx context.Context, req *mcpv1.DecideApprovalRequest) (*mcpv1.Approval, error) {
	a, err := s.uc.Decide.Execute(ctx, usecase.DecideApprovalInput{
		ApprovalID: req.GetApprovalId(), Decision: req.GetDecision(), ParamsHash: req.GetParamsHash(), Note: req.GetNote(), Via: req.GetVia(),
	})
	if err != nil {
		return nil, toStatus(err)
	}
	return wire.Approval(a, time.Now().UTC()), nil
}

func (s *GovernanceServer) SetKillSwitch(ctx context.Context, req *mcpv1.SetKillSwitchRequest) (*emptypb.Empty, error) {
	if err := s.uc.KillAdmin.Set(ctx, usecase.SetKillSwitchInput{
		Scope: req.GetScope(), TargetID: req.GetTargetId(), Reason: req.GetReason(), Active: req.GetActive(),
	}); err != nil {
		return nil, toStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *GovernanceServer) ListKillSwitches(ctx context.Context, _ *emptypb.Empty) (*mcpv1.ListKillSwitchesResponse, error) {
	es, err := s.uc.KillAdmin.List(ctx)
	if err != nil {
		return nil, toStatus(err)
	}
	resp := &mcpv1.ListKillSwitchesResponse{Entries: make([]*mcpv1.KillSwitchEntry, 0, len(es))}
	for _, e := range es {
		resp.Entries = append(resp.Entries, &mcpv1.KillSwitchEntry{
			Scope: e.Scope, TargetId: e.TargetID, Active: e.Active, Reason: e.Reason, At: timestamppb.New(e.SetAt), SetBy: e.SetBy,
		})
	}
	return resp, nil
}

func (s *GovernanceServer) GetKillState(ctx context.Context, req *mcpv1.GetKillStateRequest) (*mcpv1.GetKillStateResponse, error) {
	e, blocked, err := s.uc.KillState.Execute(ctx, req.GetClientId(), req.GetGrantId(), req.GetSessionId())
	if err != nil {
		return nil, toStatus(err)
	}
	return &mcpv1.GetKillStateResponse{Active: blocked, Scope: e.Scope, Reason: e.Reason}, nil
}

// StreamEvents has no stream interceptor in this service, so it reads the
// identity metadata itself (same keys as grpcmw.TenantExtractionInterceptor).
func (s *GovernanceServer) StreamEvents(_ *emptypb.Empty, stream mcpv1.McpService_StreamEventsServer) error {
	ctx := stream.Context()
	md, _ := metadata.FromIncomingContext(ctx)
	first := func(k string) string {
		if v := md.Get(k); len(v) > 0 {
			return v[0]
		}
		return ""
	}
	tenantID, userID := first(grpcmw.MetadataTenantID), first(grpcmw.MetadataUserID)
	if tenantID == "" || userID == "" {
		return status.Error(codes.Unauthenticated, domain.CodeNoTenant+": no identity in request")
	}
	ch, cancel, err := s.uc.Hub.Subscribe(tenantID, userID)
	if err != nil {
		return toStatus(err)
	}
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-ch:
			if !ok {
				// Buffer overflow: end the stream so the client resyncs.
				return status.Error(codes.ResourceExhausted, domain.CodeUnavailable+": event stream overflow, reconnect")
			}
			if err := stream.Send(ev); err != nil {
				return err
			}
		}
	}
}
