package httpgateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	workflowv1 "github.com/stablyai/orca-go/proto/gen/go/orca/workflow/v1"
)

// mountWorkflowRoutes wires the real REST->gRPC reverse-proxy path for
// workflow-service, following the same hand-written translation pattern as
// mountUsageRoutes (usage_routes.go) — see that function's doc comment for
// why this isn't grpc-gateway-generated.
//
// Per execution-plan.md §15, workflow-service's DAG wave-dispatch (Kahn's
// topological sort, bounded worker pool, real StepExecution persistence) is
// real now; only its boot-time execution-recovery scan remains
// unimplemented, which doesn't block any individual RPC — so this replaces
// the previous 501 stub.
func mountWorkflowRoutes(r chi.Router, client workflowv1.WorkflowServiceClient) {
	r.Route("/v1/workflows", func(sub chi.Router) {
		sub.Post("/templates", handleCreateTemplate(client))
		sub.Get("/templates", handleListTemplates(client))
		sub.Get("/templates/resolve", handleResolveTemplate(client))
		sub.Post("/executions", handleExecuteWorkflow(client))
		sub.Get("/executions/{id}", handleGetExecution(client))
		sub.Post("/executions/{id}/pause", handlePauseExecution(client))
		sub.Post("/executions/{id}/resume", handleResumeExecution(client))
		sub.Post("/executions/{id}/cancel", handleCancelExecution(client))
		sub.Post("/executions/{id}/steps/adhoc", handleExecuteAdHocStep(client))
		sub.Get("/{id}/active-executions", handleWorkflowHasActiveExecutions(client))
	})
}

// createTemplateRequestBody is the REST request shape for POST
// /v1/workflows/templates — tenant_id is deliberately absent: it comes from
// the validated Identity, never trusted from the request body, matching
// every existing handler in usage_routes.go and auth_routes.go.
type createTemplateRequestBody struct {
	Name             string `json:"name"`
	DagJSON          string `json:"dag_json"`
	Scope            string `json:"scope"`
	ParentTemplateID string `json:"parent_template_id"`
}

func handleCreateTemplate(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createTemplateRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateTemplate(ctx, &workflowv1.CreateTemplateRequest{
			TenantId:         identity.TenantID,
			Name:             body.Name,
			DagJson:          body.DagJSON,
			Scope:            body.Scope,
			ParentTemplateId: body.ParentTemplateID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetTemplate())
	}
}

func handleListTemplates(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		var pageSize int32
		if v := q.Get("page_size"); v != "" {
			n, err := strconv.ParseInt(v, 10, 32)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "page_size must be an integer")
				return
			}
			pageSize = int32(n)
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListTemplates(ctx, &workflowv1.ListTemplatesRequest{
			Scope:     q.Get("scope"),
			PageToken: q.Get("page_token"),
			PageSize:  pageSize,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func handleResolveTemplate(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		templateID := q.Get("template_id")
		if templateID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "template_id query param is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ResolveTemplate(ctx, &workflowv1.ResolveTemplateRequest{
			TemplateId: templateID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// executeWorkflowRequestBody is the REST request shape for POST
// /v1/workflows/executions.
type executeWorkflowRequestBody struct {
	TemplateID  string `json:"template_id"`
	ProjectID   string `json:"project_id"`
	RootTraceID string `json:"root_trace_id"`
	RequestID   string `json:"request_id"`
}

func handleExecuteWorkflow(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body executeWorkflowRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Execute(ctx, &workflowv1.ExecuteRequest{
			TemplateId:  body.TemplateID,
			ProjectId:   body.ProjectID,
			RootTraceId: body.RootTraceID,
			RequestId:   body.RequestID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetExecution())
	}
}

func handleGetExecution(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetExecution(ctx, &workflowv1.GetExecutionRequest{Id: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetExecution())
	}
}

func handlePauseExecution(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.PauseExecution(ctx, &workflowv1.PauseExecutionRequest{Id: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetExecution())
	}
}

func handleResumeExecution(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ResumeExecution(ctx, &workflowv1.ResumeExecutionRequest{Id: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetExecution())
	}
}

func handleCancelExecution(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CancelExecution(ctx, &workflowv1.CancelExecutionRequest{Id: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetExecution())
	}
}

// executeAdHocStepRequestBody is the REST request shape for POST
// /v1/workflows/executions/{id}/steps/adhoc — the path's {id} is unused by
// the RPC itself (ExecuteAdHocStep runs a single step outside of any
// execution) but kept for REST-ish nesting consistency; tenant_id is
// deliberately absent from the body, taken from the validated Identity.
type executeAdHocStepRequestBody struct {
	StepType       string `json:"step_type"`
	StepConfigJSON string `json:"step_config_json"`
	RequestID      string `json:"request_id"`
}

func handleExecuteAdHocStep(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body executeAdHocStepRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ExecuteAdHocStep(ctx, &workflowv1.ExecuteAdHocStepRequest{
			TenantId:       identity.TenantID,
			StepType:       parseStepType(body.StepType),
			StepConfigJson: body.StepConfigJSON,
			RequestId:      body.RequestID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetResult())
	}
}

// workflowHasActiveExecutionsResponseBody is the REST response shape for GET
// /v1/workflows/{id}/active-executions — the {id} here is a project id, per
// HasActiveExecutionsRequest's project_id field.
type workflowHasActiveExecutionsResponseBody struct {
	HasActive bool `json:"has_active"`
}

func handleWorkflowHasActiveExecutions(client workflowv1.WorkflowServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.HasActiveExecutions(ctx, &workflowv1.HasActiveExecutionsRequest{ProjectId: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, workflowHasActiveExecutionsResponseBody{HasActive: resp.GetHasActive()})
	}
}
