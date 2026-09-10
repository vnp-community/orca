// Package httpgateway implements api-gateway's inbound REST edge: a chi
// router combining the one real reverse-proxy path (usage-service) with
// documented 501 stubs for every other downstream service, behind the
// shared auth + rate-limit middleware. See
// specs/backend-go/services/api-gateway.md §3, §6, §9.
package httpgateway

import (
	"encoding/json"
	"net/http"
)

// errorBody is the JSON shape every error response in this package uses —
// both the real usage-service path's mapped gRPC errors and the 501 stub
// responses, so clients handle one error shape regardless of route.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeJSON encodes v as the HTTP response body via plain encoding/json —
// deliberately NOT protojson-aware. An earlier version of this function
// special-cased proto.Message values through protojson (camelCase field
// names, RFC3339 timestamps, enum-as-string) to fix
// specs/backend-go/bugs/missing-v2/'s admin_routes.go finding — reverted
// after the real test suite showed the blast radius was much wider than
// intended: every OTHER route in this package that passes a raw proto
// response straight to writeJSON (ai_provider_routes.go, infra_routes.go,
// notification_routes.go, orchestration_routes.go, ...) already has
// passing tests asserting today's plain-encoding/json shape (numeric enums,
// {seconds,nanos} timestamps) — changing this function globally broke
// ~10 of them at once. The admin_routes.go/auth_admin_routes.go fix for
// /admin/api/stats and /admin/api/users instead shapes its own
// route-local structs (userJSON/usersListJSON) explicitly, scoped to only
// the routes that actually needed it — see that file's doc comments.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
