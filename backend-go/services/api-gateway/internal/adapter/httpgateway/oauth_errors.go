package httpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var grpcCodedMessage = regexp.MustCompile(`^([A-Z][A-Z0-9_]+): (.*)$`)

// splitCoded extracts "CODE: message" from a gRPC status. unavailable is true
// for transport failures, which must never be reported as the client's fault.
func splitCoded(err error) (code, msg string, unavailable bool) {
	st, ok := status.FromError(err)
	if !ok {
		return "", "", errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
	}
	switch st.Code() {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		unavailable = true
	}
	if m := grpcCodedMessage.FindStringSubmatch(st.Message()); m != nil {
		return m[1], m[2], unavailable
	}
	return "", st.Message(), unavailable
}

// oauthErrorName maps auth-service OAUTH_* codes to RFC 6749/7591/8707 error
// identifiers. Unknown codes become server_error so internals never leak.
func oauthErrorName(code string) string {
	switch code {
	case "OAUTH_INVALID_REQUEST":
		return "invalid_request"
	case "OAUTH_INVALID_CLIENT", "OAUTH_CLIENT_NOT_FOUND":
		return "invalid_client"
	case "OAUTH_INVALID_GRANT":
		return "invalid_grant"
	case "OAUTH_INVALID_SCOPE", "MCP_SCOPE_INVALID", "MCP_SCOPE_NOT_ALLOWED":
		return "invalid_scope"
	case "OAUTH_INVALID_TARGET":
		return "invalid_target"
	case "OAUTH_UNAUTHORIZED_CLIENT":
		return "unauthorized_client"
	case "OAUTH_UNSUPPORTED_GRANT_TYPE":
		return "unsupported_grant_type"
	case "OAUTH_UNSUPPORTED_RESPONSE_TYPE":
		return "unsupported_response_type"
	case "OAUTH_INVALID_REDIRECT_URI":
		return "invalid_redirect_uri"
	case "OAUTH_INVALID_CLIENT_METADATA":
		return "invalid_client_metadata"
	case "OAUTH_ACCESS_DENIED", "MCP_CLIENT_NOT_ALLOWED":
		return "access_denied"
	default:
		return "server_error"
	}
}

// oauthErrorBody is the RFC 6749 section 5.2 error response.
type oauthErrorBody struct {
	Error       string `json:"error"`
	Description string `json:"error_description,omitempty"`
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}

// writeOAuthJSONError renders a JSON OAuth error. Descriptions of internal
// failures are replaced by a generic text so SQL/service details never leak.
func writeOAuthJSONError(w http.ResponseWriter, err error) {
	code, msg, unavailable := splitCoded(err)
	name := oauthErrorName(code)
	setNoStore(w)
	switch {
	case unavailable:
		w.Header().Set("Retry-After", "2")
		writeJSON(w, http.StatusServiceUnavailable, oauthErrorBody{Error: "temporarily_unavailable", Description: "authorization server is temporarily unavailable"})
	case name == "server_error" || code == "OAUTH_INTERNAL":
		writeJSON(w, http.StatusInternalServerError, oauthErrorBody{Error: "server_error", Description: "internal error"})
	case name == "invalid_client":
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_client"`)
		writeJSON(w, http.StatusUnauthorized, oauthErrorBody{Error: name, Description: msg})
	default:
		writeJSON(w, http.StatusBadRequest, oauthErrorBody{Error: name, Description: msg})
	}
}

// decodeOAuthJSON reads a size-bounded JSON object; unknown fields are ignored.
func decodeOAuthJSON(w http.ResponseWriter, r *http.Request, max int64, dst any) error {
	return json.NewDecoder(http.MaxBytesReader(w, r.Body, max)).Decode(dst)
}
