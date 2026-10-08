package httpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// maxRequestRouteBody bounds a JSON body before it is decoded (a Request body is at most 100000 characters).
const maxRequestRouteBody = 1 << 20

// mountRequestRoutes serves the five HTTP routes of CONTRACT section 4 for
// scripts, CLIs and notification deep links. They dispatch through the same
// channel handlers as the WS edge, so validation, the source rule (HTTP callers
// may only claim an external tracker or none), views and error codes are shared.
// Permissions stay in request-service (the gateway has no OPA step).
func mountRequestRoutes(r chi.Router, req requestv1.RequestServiceClient, appr requestv1.ApprovalServiceClient) {
	reg := wscompat.NewRegistry()
	wscompat.RegisterRequestUnaryChannels(reg, req, appr)
	h := requestRoutes{reg: reg}
	r.Route("/v1/requests", func(sub chi.Router) {
		sub.Post("/", h.create)
		sub.Get("/", h.list)
		sub.Get("/{id}", h.get)
	})
	r.Route("/v1/approvals", func(sub chi.Router) {
		sub.Get("/pending", h.listPending)
		sub.Post("/{id}/approve", h.decide("approval.approve"))
		sub.Post("/{id}/reject", h.decide("approval.reject"))
	})
}

type requestRoutes struct{ reg *wscompat.Registry }

// call dispatches one channel as the authenticated caller and writes the outcome.
func (h requestRoutes) call(w http.ResponseWriter, r *http.Request, channel string, args any, okStatus func(map[string]any) int) {
	identity, ok := identityFromContext(r.Context())
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "authentication required")
		return
	}
	raw, err := json.Marshal(args)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid arguments")
		return
	}
	res, err := h.reg.Dispatch(r.Context(), wscompat.Identity{TenantID: identity.TenantID, UserID: identity.UserID, Role: identity.Role},
		channel, []json.RawMessage{raw})
	if err != nil {
		writeRequestChannelError(w, err)
		return
	}
	status := http.StatusOK
	if okStatus != nil {
		b, _ := json.Marshal(res)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		status = okStatus(m)
	}
	writeJSON(w, status, res)
}

func decodeRequestRouteBody(w http.ResponseWriter, r *http.Request, into any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestRouteBody)
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body")
		return false
	}
	return true
}

func (h requestRoutes) create(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if !decodeRequestRouteBody(w, r, &body) {
		return
	}
	h.call(w, r, "request.create", body, func(m map[string]any) int {
		if m["created"] == true {
			return http.StatusCreated
		}
		return http.StatusOK
	})
}

func (h requestRoutes) get(w http.ResponseWriter, r *http.Request) {
	h.call(w, r, "request.get", map[string]any{"id": chi.URLParam(r, "id")}, nil)
}

func (h requestRoutes) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	args := map[string]any{
		"projectId": q.Get("projectId"), "sourceProvider": q.Get("sourceProvider"), "sourceSite": q.Get("sourceSite"),
		"sourceRef": q.Get("sourceRef"), "pageToken": q.Get("pageToken"),
		"status": queryList(q["status"]), "type": queryList(q["type"]),
	}
	if !setPageSize(w, q.Get("pageSize"), args) {
		return
	}
	h.call(w, r, "request.list", args, nil)
}

func (h requestRoutes) listPending(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	args := map[string]any{"subjectType": q.Get("subjectType"), "pageToken": q.Get("pageToken")}
	if !setPageSize(w, q.Get("pageSize"), args) {
		return
	}
	h.call(w, r, "approval.listPending", args, nil)
}

// decide serves approve and reject. The body carries the version and digest the
// person saw (CONTRACT section 8 row 10); the id comes from the path only.
func (h requestRoutes) decide(channel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Version int64  `json:"version"`
			Digest  string `json:"digest"`
			Comment string `json:"comment"`
		}
		if !decodeRequestRouteBody(w, r, &body) {
			return
		}
		h.call(w, r, channel, map[string]any{
			"id": chi.URLParam(r, "id"), "expectedVersion": body.Version, "expectedDigest": body.Digest, "comment": body.Comment,
		}, nil)
	}
}

// queryList accepts repeated and comma-separated values: ?status=new&status=analyzing or ?status=new,analyzing.
func queryList(values []string) []string {
	out := []string{}
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			if part = strings.TrimSpace(part); part != "" {
				out = append(out, part)
			}
		}
	}
	return out
}

func setPageSize(w http.ResponseWriter, raw string, args map[string]any) bool {
	if raw == "" {
		return true
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "pageSize must be an integer")
		return false
	}
	args["pageSize"] = n
	return true
}

var requestErrorCode = regexp.MustCompile(`^([A-Z][A-Z0-9_]*): ?(.*)$`)

// writeRequestChannelError turns a channel error "<CODE>: <message>" into an HTTP
// status and the shared error body, keeping the code the UI and scripts match on.
func writeRequestChannelError(w http.ResponseWriter, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		writeJSONError(w, http.StatusGatewayTimeout, "REQUEST_UNAVAILABLE", "request service did not respond in time")
		return
	}
	m := requestErrorCode.FindStringSubmatch(err.Error())
	if m == nil {
		writeJSONError(w, http.StatusInternalServerError, "REQUEST_INTERNAL", "internal error")
		return
	}
	writeJSONError(w, requestErrorHTTPStatus(m[1]), m[1], m[2])
}

func requestErrorHTTPStatus(code string) int {
	switch {
	case code == "INVALID_ARGUMENT", strings.HasSuffix(code, "_REQUIRED"):
		return http.StatusBadRequest
	case strings.HasSuffix(code, "_NOT_FOUND"):
		return http.StatusNotFound
	case strings.HasSuffix(code, "FORBIDDEN"), strings.HasSuffix(code, "_NOT_APPROVER"):
		return http.StatusForbidden
	case code == "REQUEST_UNAVAILABLE":
		return http.StatusServiceUnavailable
	case code == "REQUEST_NOT_IMPLEMENTED":
		return http.StatusNotImplemented
	case code == "REQUEST_AI_COMPLETE_TIMEOUT":
		return http.StatusGatewayTimeout
	case code == "REQUEST_RATE_LIMITED", code == "REQUEST_PENDING_LIMIT":
		return http.StatusTooManyRequests
	case strings.HasSuffix(code, "_CONFLICT"):
		return http.StatusConflict
	case code == "REQUEST_INTERNAL":
		return http.StatusInternalServerError
	default:
		return http.StatusPreconditionFailed
	}
}
