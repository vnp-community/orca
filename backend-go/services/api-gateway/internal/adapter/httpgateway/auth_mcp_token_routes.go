package httpgateway

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/mcptokens"
)

// McpTokenRoutes serves POST/GET/DELETE /v1/auth/mcp-tokens, the REST twin of
// the mcp.token.* channels (CONTRACT section 3). Mounted in the authed group,
// whose validator rejects MCP-audience tokens, so a PAT can never mint more PATs.
type McpTokenRoutes struct {
	Service *mcptokens.Service
}

func (m *McpTokenRoutes) mount(r chi.Router) {
	r.Route("/v1/auth/mcp-tokens", func(sub chi.Router) {
		sub.Post("/", m.handleCreate)
		sub.Get("/", m.handleList)
		sub.Delete("/{id}", m.handleRevoke) // {id} = jti
	})
}

// createMcpTokenBody has no user/tenant/audience: identity comes from the
// session and the audience is fixed server-side.
type createMcpTokenBody struct {
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes"`
	ExpiresInDays int      `json:"expires_in_days"`
	ExpiresInDay2 int      `json:"expiresInDays"`
}

func (m *McpTokenRoutes) handleCreate(w http.ResponseWriter, r *http.Request) {
	identity, _ := identityFromContext(r.Context())
	var body createMcpTokenBody
	if !decodeJSONBody(w, r, &body) {
		return
	}
	days := body.ExpiresInDays
	if days == 0 {
		days = body.ExpiresInDay2
	}
	out, err := m.Service.Create(r.Context(), identity, mcptokens.CreateInput{Name: body.Name, Scopes: body.Scopes, ExpiresInDays: days})
	w.Header().Set("Cache-Control", "no-store") // the response may carry the one-time secret
	if err != nil {
		writeMcpTokenError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (m *McpTokenRoutes) handleList(w http.ResponseWriter, r *http.Request) {
	identity, _ := identityFromContext(r.Context())
	toks, err := m.Service.List(r.Context(), identity)
	if err != nil {
		writeMcpTokenError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tokens": toks})
}

func (m *McpTokenRoutes) handleRevoke(w http.ResponseWriter, r *http.Request) {
	identity, _ := identityFromContext(r.Context())
	if err := m.Service.Revoke(r.Context(), identity, chi.URLParam(r, "id")); err != nil {
		writeMcpTokenError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeMcpTokenError(w http.ResponseWriter, err error) {
	var e *mcptokens.Error
	if !errors.As(err, &e) {
		e = &mcptokens.Error{Code: "MCP_INTERNAL", Message: "internal error"}
	}
	status := http.StatusInternalServerError
	switch e.Code {
	case "MCP_TOKEN_TOO_LONG", "MCP_SCOPE_INVALID", "MCP_INVALID_ARGUMENT":
		status = http.StatusBadRequest
	case "MCP_SCOPE_NOT_ALLOWED", "MCP_DISABLED", "MCP_KILL_SWITCH_ACTIVE":
		status = http.StatusForbidden
	case "MCP_NOT_FOUND":
		status = http.StatusNotFound
	case "MCP_TOKEN_LIMIT":
		status = http.StatusConflict
	case "MCP_UNAVAILABLE":
		status = http.StatusServiceUnavailable
	case "MCP_TIMEOUT":
		status = http.StatusGatewayTimeout
	}
	writeJSONError(w, status, e.Code, e.Message)
}
