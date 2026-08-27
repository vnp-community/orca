package httpgateway

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	annotationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/annotation/v1"
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
func mountAnnotationRoutes(r chi.Router, client annotationv1.AnnotationServiceClient) {
	r.Route("/v1/annotations", func(sub chi.Router) {
		sub.Post("/", handleCreateAnnotation(client))
		sub.Get("/", handleListAnnotations(client))
		sub.Put("/{id}", handleUpdateAnnotation(client))
		sub.Patch("/{id}", handleUpdateAnnotation(client))
		sub.Delete("/{id}", handleDeleteAnnotation(client))
	})
}

// createAnnotationRequestBody is the REST request shape for POST
// /v1/annotations.
type createAnnotationRequestBody struct {
	Anchor    *annotationAnchorBody `json:"anchor"`
	Content   string                `json:"content"`
	RequestID string                `json:"request_id"`
}

type annotationAnchorBody struct {
	RepoID   string `json:"repo_id"`
	FilePath string `json:"file_path"`
	Line     int32  `json:"line"`
	Ref      string `json:"ref"`
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
				RepoId:   body.Anchor.RepoID,
				FilePath: body.Anchor.FilePath,
				Line:     body.Anchor.Line,
				Ref:      body.Anchor.Ref,
			},
			Content:   body.Content,
			RequestId: body.RequestID,
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

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListAnnotations(ctx, &annotationv1.ListAnnotationsRequest{
			RepoId:    q.Get("repo_id"),
			FilePath:  q.Get("file_path"),
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

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		_, err := client.DeleteAnnotation(ctx, &annotationv1.DeleteAnnotationRequest{Id: id})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
