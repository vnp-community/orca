package httpgateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	usagev1 "github.com/stablyai/orca-go/proto/gen/go/orca/usage/v1"
)

// mountUsageRoutes wires the ONE real end-to-end REST->gRPC reverse-proxy
// path in this scaffold — see README.md "what's really wired": these
// handlers call straight through to usage-service's real gRPC API. This is
// the reference pattern every other downstream service's routes
// (stub_routes.go) become once that service's gRPC contract stabilizes.
//
// Production wiring should replace this hand-written translation with a
// grpc-gateway-generated mux built from usage.proto's google.api.http
// annotations, per api-gateway.md §3 — this scaffold hand-writes the
// equivalent routes to demonstrate the pattern without that codegen step.
func mountUsageRoutes(r chi.Router, client usagev1.UsageServiceClient) {
	r.Route("/v1/usage", func(sub chi.Router) {
		sub.Get("/daily", handleGetDailyUsage(client))
		sub.Post("/sessions", handleRecordUsageSession(client))
		sub.Get("/sessions", handleListSessions(client))
	})
}

func handleGetDailyUsage(client usagev1.UsageServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		userID := q.Get("user_id")
		if userID == "" {
			userID = identity.UserID
		}

		day := time.Now().UTC()
		if v := q.Get("day"); v != "" {
			parsed, err := time.Parse("2006-01-02", v)
			if err != nil {
				writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "day must be formatted YYYY-MM-DD")
				return
			}
			day = parsed
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetDailyUsage(ctx, &usagev1.GetDailyUsageRequest{
			TenantId: identity.TenantID,
			UserId:   userID,
			Provider: parseProvider(q.Get("provider")),
			Day:      timestamppb.New(day),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetRollup())
	}
}

// recordUsageSessionRequestBody is the REST request shape for POST
// /v1/usage/sessions — tenant_id/user_id are deliberately absent: they come
// from the validated Identity, never trusted from the request body, per
// api-gateway.md §9.
type recordUsageSessionRequestBody struct {
	ID               string    `json:"id"`
	Provider         string    `json:"provider"`
	WorktreeID       string    `json:"worktree_id"`
	InputTokens      int64     `json:"input_tokens"`
	OutputTokens     int64     `json:"output_tokens"`
	CacheReadTokens  int64     `json:"cache_read_tokens"`
	CacheWriteTokens int64     `json:"cache_write_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
	RequestID        string    `json:"request_id"`
}

func handleRecordUsageSession(client usagev1.UsageServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body recordUsageSessionRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		session := &usagev1.UsageSession{
			Id:               body.ID,
			TenantId:         identity.TenantID,
			UserId:           identity.UserID,
			Provider:         parseProvider(body.Provider),
			WorktreeId:       body.WorktreeID,
			InputTokens:      body.InputTokens,
			OutputTokens:     body.OutputTokens,
			CacheReadTokens:  body.CacheReadTokens,
			CacheWriteTokens: body.CacheWriteTokens,
			CostUsd:          body.CostUSD,
			RequestId:        body.RequestID,
		}
		if !body.StartedAt.IsZero() {
			session.StartedAt = timestamppb.New(body.StartedAt)
		}
		if !body.EndedAt.IsZero() {
			session.EndedAt = timestamppb.New(body.EndedAt)
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.RecordUsageSession(ctx, &usagev1.RecordUsageSessionRequest{Session: session})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetSession())
	}
}

func handleListSessions(client usagev1.UsageServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		userID := q.Get("user_id")
		if userID == "" {
			userID = identity.UserID
		}

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
		resp, err := client.ListSessions(ctx, &usagev1.ListSessionsRequest{
			TenantId:  identity.TenantID,
			UserId:    userID,
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

func parseProvider(v string) usagev1.Provider {
	switch v {
	case "claude":
		return usagev1.Provider_PROVIDER_CLAUDE
	case "codex":
		return usagev1.Provider_PROVIDER_CODEX
	case "opencode":
		return usagev1.Provider_PROVIDER_OPENCODE
	default:
		return usagev1.Provider_PROVIDER_UNSPECIFIED
	}
}

// writeGRPCError maps a gRPC client error to this REST API's error shape —
// the hand-written equivalent of the status<->HTTP mapping a generated
// grpc-gateway mux would apply automatically (see mountUsageRoutes' doc
// comment).
func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeJSONError(w, http.StatusBadGateway, "UPSTREAM_ERROR", err.Error())
		return
	}
	writeJSONError(w, grpcCodeToHTTPStatus(st.Code()), st.Code().String(), st.Message())
}

func grpcCodeToHTTPStatus(c codes.Code) int {
	switch c {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.FailedPrecondition:
		return http.StatusPreconditionFailed
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.Unimplemented:
		return http.StatusNotImplemented
	default:
		return http.StatusInternalServerError
	}
}
