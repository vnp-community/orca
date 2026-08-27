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
// .template.update against workflow-service's gRPC client — all four bind
// tenant_id via gRPC metadata (AttachIdentity), matching this file's
// devServer.*/fleet.* precedent (CreateTemplateRequest.tenant_id is also
// set explicitly from Identity for consistency, since the REST handler at
// /v1/workflows/templates does the same).
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
}
