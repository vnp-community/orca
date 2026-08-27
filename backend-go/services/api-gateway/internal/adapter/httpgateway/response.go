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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}
