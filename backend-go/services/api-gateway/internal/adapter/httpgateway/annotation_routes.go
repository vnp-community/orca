package httpgateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// mountAnnotationRoutes wires the real REST->gRPC reverse-proxy path for
// annotation-service, following the same hand-written translation pattern
// as mountUsageRoutes (usage_routes.go) — see that file's doc comment for
// why this isn't grpc-gateway-generated yet.
//
// annotation-service's RPCs don't accept tenant_id/author_id on the wire
// (CreateAnnotationRequest, ListAnnotationsRequest, etc. have no such
// fields) — the server derives them from the caller identity carried on
// the gRPC context, which is exactly what gatewaygrpc.AttachIdentity
// stamps on ctx below. So there's no field to scrub from the REST body
// here; the identity simply never enters the request message at all.
func mountAnnotationRoutes(r chi.Router, client annotationv1.AnnotationServiceClient, gitClient gitgatewayv1.GitGatewayServiceClient) {
	r.Route("/v1/annotations", func(sub chi.Router) {
		sub.Post("/", handleCreateAnnotation(client))
		sub.Get("/", handleListAnnotations(client))
		sub.Put("/{id}", handleUpdateAnnotation(client))
		sub.Patch("/{id}", handleUpdateAnnotation(client))
		sub.Delete("/{id}", handleDeleteAnnotation(client))
		sub.Post("/mark-sent", handleMarkAnnotationsSent(client))        // from TASK-CR-02-07
		sub.Post("/send-to-agent", handleSendToAgent(client, gitClient)) // from TASK-CR-03-05
	})
}

// createAnnotationRequestBody is the REST request shape for POST
// /v1/annotations.
type createAnnotationRequestBody struct {
	Anchor       *annotationAnchorBody `json:"anchor"`
	Content      string                `json:"content"`
	RequestID    string                `json:"request_id"`
	OriginalCode string                `json:"original_code"` // NEW
}

type annotationAnchorBody struct {
	RepoID     string `json:"repo_id"`
	WorktreeID string `json:"worktree_id"` // NEW
	FilePath   string `json:"file_path"`
	Line       int32  `json:"line"`
	EndLine    int32  `json:"end_line"` // NEW
	Side       int32  `json:"side"`     // NEW
	Ref        string `json:"ref"`
}

func handleCreateAnnotation(client annotationv1.AnnotationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createAnnotationRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.Anchor == nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "anchor is required")
			return
		}

		req := &annotationv1.CreateAnnotationRequest{
			Anchor: &annotationv1.Anchor{
				RepoId:     body.Anchor.RepoID,
				WorktreeId: body.Anchor.WorktreeID,
				FilePath:   body.Anchor.FilePath,
				Line:       body.Anchor.Line,
				EndLine:    body.Anchor.EndLine,
				Side:       annotationv1.Side(body.Anchor.Side),
				Ref:        body.Anchor.Ref,
			},
			Content:      body.Content,
			RequestId:    body.RequestID,
			OriginalCode: body.OriginalCode,
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateAnnotation(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetAnnotation())
	}
}

func handleListAnnotations(client annotationv1.AnnotationServiceClient) http.HandlerFunc {
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

		req := &annotationv1.ListAnnotationsRequest{
			RepoId:     q.Get("repo_id"),
			FilePath:   q.Get("file_path"),
			PageToken:  q.Get("page_token"),
			PageSize:   pageSize,
			WorktreeId: q.Get("worktree_id"), // NEW
		}
		if v := q.Get("sent_to_agent"); v != "" { // NEW
			sentToAgent := v == "true"
			req.SentToAgent = &sentToAgent
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListAnnotations(ctx, req)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// updateAnnotationRequestBody is the REST request shape for PUT/PATCH
// /v1/annotations/{id}. There's no author_id here either — UpdateAnnotation
// is enforced author-only (or admin) server-side via OPA, keyed off the
// identity on the gRPC context, not anything in this body.
type updateAnnotationRequestBody struct {
	Content  string `json:"content"`
	Resolved bool   `json:"resolved"`
}

func handleUpdateAnnotation(client annotationv1.AnnotationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")

		var body updateAnnotationRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.UpdateAnnotation(ctx, &annotationv1.UpdateAnnotationRequest{
			Id:       id,
			Content:  body.Content,
			Resolved: body.Resolved,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetAnnotation())
	}
}

func handleDeleteAnnotation(client annotationv1.AnnotationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		id := chi.URLParam(r, "id")
		confirmed := r.URL.Query().Get("confirmed") == "true" // NEW — BR-CR-08

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.DeleteAnnotation(ctx, &annotationv1.DeleteAnnotationRequest{Id: id, Confirmed: confirmed})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// markAnnotationsSentRequestBody is the REST request shape for POST
// /v1/annotations/mark-sent.
type markAnnotationsSentRequestBody struct {
	IDs []string `json:"ids"`
}

func handleMarkAnnotationsSent(client annotationv1.AnnotationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body markAnnotationsSentRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.MarkAnnotationsSent(ctx, &annotationv1.MarkAnnotationsSentRequest{Ids: body.IDs})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// sendToAgentRequestBody is the REST request shape for POST
// /v1/annotations/send-to-agent.
type sendToAgentRequestBody struct {
	WorktreeID   string `json:"worktree_id"`
	PtyID        string `json:"pty_id"`
	WorktreeName string `json:"worktree_name"`
}

// handleSendToAgent is annotation.sendToAgent's REST mirror. It calls the
// same transport-agnostic wscompat.SendReviewFeedbackToAgent the WS channel
// calls (channels_annotation_send.go), so the two transports' composition
// logic can't drift apart — see TASK-CR-03-05. See that function's doc
// comment for a caveat specific to this transport: PTY delivery needs a
// per-WebSocket-connection stream registry that a REST request's ctx never
// carries, so this route currently always fails at the delivery step with a
// clear error rather than silently doing nothing.
func handleSendToAgent(client annotationv1.AnnotationServiceClient, gitClient gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body sendToAgentRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.WorktreeID == "" || body.PtyID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "worktree_id and pty_id are required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		result, err := wscompat.SendReviewFeedbackToAgent(ctx, client, gitClient, body.WorktreeID, body.PtyID, body.WorktreeName)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}
