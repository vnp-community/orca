// ── workflow.* (workflow-service) ────────────────────────────────────────
package wscompat

import (
	"context"
	"encoding/json"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

// registerWorkflowChannels wires workflow.execute/.cancel/.template.create/
// .template.update/.getExecution/.pause/.resume/.template.list/
// .template.resolve/.hasActiveExecutions/.executeAdHocStep against
// workflow-service's gRPC client — all bind tenant_id via gRPC metadata
// (AttachIdentity), matching this file's devServer.*/fleet.* precedent
// (CreateTemplateRequest.tenant_id is also set explicitly from Identity for
// consistency, since the REST handler at /v1/workflows/templates does the
// same).
//
// CR-PW-005: this registers the remaining 7 of 11 WorkflowServiceClient RPCs
// that already existed in the proto/gRPC server but had no wscompat channel
// — Web-mode clients calling e.g. workflow.getExecution or
// workflow.template.list (CR-PW-003's ExecutionMonitor/WorkflowMonitor) 404'd
// with "unknown channel" until now, even though Electron/local mode already
// worked (it talks to the legacy TS workflow-rpc-handler directly). No proto
// change here — pure wiring, same pattern as the 4 pre-existing channels.
//
// Known pre-existing issue, NOT introduced or fixed by this change (kept
// consistent with workflow.execute/.cancel/.template.create/.template.update
// above, which already do this): every handler below returns the raw
// *workflowv1.X proto message, whose encoding/json struct tags are
// snake_case (`json:"template_id,omitempty"` — protoc-gen-go's camelCase
// `json=templateId` tag is protojson-only, and this envelope serializes
// `Result any` via plain encoding/json). This likely already ships
// snake_case keys to the frontend's camelCase-typed WorkflowExecution/
// WorkflowTemplate for the 4 pre-existing channels too. Flagged here (see
// channels_tenant_project.go and channels_dev_server_access_control.go for
// the same finding, already flagged and fixed in those newer files via an
// explicit camelCase view struct) but deliberately NOT fixed in this pass —
// fixing it would mean changing the 4 already-shipped channels' response
// shape too, which is out of CR-PW-005's scope (pure wiring of missing
// channels). See CR-PW-005's doc, "Không thuộc phạm vi CR này".
func registerWorkflowChannels(r *Registry, client workflowv1.WorkflowServiceClient) {
	r.Register("workflow.execute", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type executeArgs struct {
			TemplateID  string `json:"templateId"`
			ProjectID   string `json:"projectId"`
			RootTraceID string `json:"rootTraceId"`
			RequestID   string `json:"requestId"`
		}
		in, err := decodeArg[executeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.Execute(ctx, &workflowv1.ExecuteRequest{
			TemplateId: in.TemplateID, ProjectId: in.ProjectID,
			RootTraceId: in.RootTraceID, RequestId: in.RequestID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetExecution(), nil
	})

	r.Register("workflow.cancel", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type cancelArgs struct {
			ExecutionID string `json:"executionId"`
		}
		in, err := decodeArg[cancelArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.CancelExecution(ctx, &workflowv1.CancelExecutionRequest{Id: in.ExecutionID})
		if err != nil {
			return nil, err
		}
		return resp.GetExecution(), nil
	})

	r.Register("workflow.template.create", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type createArgs struct {
			Name             string `json:"name"`
			DAGJSON          string `json:"dagJson"`
			Scope            string `json:"scope"`
			ParentTemplateID string `json:"parentTemplateId"`
		}
		in, err := decodeArg[createArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.CreateTemplate(ctx, &workflowv1.CreateTemplateRequest{
			TenantId: id.TenantID, Name: in.Name, DagJson: in.DAGJSON,
			Scope: in.Scope, ParentTemplateId: in.ParentTemplateID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetTemplate(), nil
	})

	r.Register("workflow.template.update", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type updateArgs struct {
			ID               string `json:"id"`
			Name             string `json:"name"`
			DAGJSON          string `json:"dagJson"`
			Scope            string `json:"scope"`
			ParentTemplateID string `json:"parentTemplateId"`
			ExpectedVersion  int32  `json:"expectedVersion"`
		}
		in, err := decodeArg[updateArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.UpdateTemplate(ctx, &workflowv1.UpdateTemplateRequest{
			Id: in.ID, Name: in.Name, DagJson: in.DAGJSON, Scope: in.Scope,
			ParentTemplateId: in.ParentTemplateID, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetTemplate(), nil
	})

	r.Register("workflow.getExecution", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type getExecutionArgs struct {
			ExecutionID string `json:"executionId"`
		}
		in, err := decodeArg[getExecutionArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetExecution(ctx, &workflowv1.GetExecutionRequest{Id: in.ExecutionID})
		if err != nil {
			return nil, err
		}
		return resp.GetExecution(), nil
	})

	r.Register("workflow.pause", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type pauseArgs struct {
			ExecutionID string `json:"executionId"`
		}
		in, err := decodeArg[pauseArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.PauseExecution(ctx, &workflowv1.PauseExecutionRequest{Id: in.ExecutionID})
		if err != nil {
			return nil, err
		}
		return resp.GetExecution(), nil
	})

	r.Register("workflow.resume", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resumeArgs struct {
			ExecutionID string `json:"executionId"`
		}
		in, err := decodeArg[resumeArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ResumeExecution(ctx, &workflowv1.ResumeExecutionRequest{Id: in.ExecutionID})
		if err != nil {
			return nil, err
		}
		return resp.GetExecution(), nil
	})

	r.Register("workflow.template.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			Scope     string `json:"scope"`
			PageToken string `json:"pageToken"`
			PageSize  int32  `json:"pageSize"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ListTemplates(ctx, &workflowv1.ListTemplatesRequest{
			Scope: in.Scope, PageToken: in.PageToken, PageSize: in.PageSize,
		})
		if err != nil {
			return nil, err
		}
		// List-shaped channels return [] not null when empty (established
		// convention — see TASK-BE-008's devServerGroup.list precedent).
		templates := resp.GetTemplates()
		if templates == nil {
			templates = []*workflowv1.WorkflowTemplate{}
		}
		return map[string]any{"templates": templates, "nextPageToken": resp.GetNextPageToken()}, nil
	})

	r.Register("workflow.template.resolve", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type resolveArgs struct {
			TemplateID string `json:"templateId"`
		}
		in, err := decodeArg[resolveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ResolveTemplate(ctx, &workflowv1.ResolveTemplateRequest{TemplateId: in.TemplateID})
		if err != nil {
			return nil, err
		}
		chain := resp.GetChain()
		if chain == nil {
			chain = []*workflowv1.WorkflowTemplate{}
		}
		return map[string]any{"template": resp.GetTemplate(), "chain": chain}, nil
	})

	r.Register("workflow.hasActiveExecutions", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type hasActiveArgs struct {
			ProjectID string `json:"projectId"`
		}
		in, err := decodeArg[hasActiveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.HasActiveExecutions(ctx, &workflowv1.HasActiveExecutionsRequest{ProjectId: in.ProjectID})
		if err != nil {
			return nil, err
		}
		return map[string]any{"hasActive": resp.GetHasActive()}, nil
	})

	// workflow.executeAdHocStep: TenantId always comes from Identity, never
	// args, matching workflow.template.create's precedent above (a
	// malicious/buggy frontend payload must not be able to run a step under
	// a different tenant). stepType uses the same parseStepType helper
	// task.*/automation.* channels already use for this enum
	// (channels_automation_task.go) — kept in sync there, not duplicated.
	r.Register("workflow.executeAdHocStep", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type executeAdHocStepArgs struct {
			StepType       string `json:"stepType"`
			StepConfigJSON string `json:"stepConfigJson"`
			RequestID      string `json:"requestId"`
		}
		in, err := decodeArg[executeAdHocStepArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.ExecuteAdHocStep(ctx, &workflowv1.ExecuteAdHocStepRequest{
			TenantId: id.TenantID, StepType: parseStepType(in.StepType),
			StepConfigJson: in.StepConfigJSON, RequestId: in.RequestID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetResult(), nil
	})
}
