package httpgateway

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// cliTokenAudience is the fixed `aud` claim for every token minted through
// this route — never accepted from the request, so a caller cannot mint a
// token scoped to an arbitrary other service's audience.
const cliTokenAudience = "orca-cli"

// mountCliTokenRoutes wires POST/GET/DELETE /v1/auth/cli-tokens. Mounted in
// router.go's authed group (NOT mountAuthRoutes's unauthenticated group) —
// same convention as mountAuthAdminRoutes, see that function's doc comment.
func mountCliTokenRoutes(r chi.Router, client authv1.AuthServiceClient) {
	r.Route("/v1/auth/cli-tokens", func(sub chi.Router) {
		sub.Post("/", handleIssueCliToken(client))
		sub.Get("/", handleListCliTokens(client))
		sub.Delete("/{id}", handleRevokeCliToken(client)) // {id} = jti
	})
}

// issueCliTokenResponseBody deliberately has no request body counterpart —
// there is nothing a caller may specify; user_id and audience are both
// fixed server-side. See createUserRequestBody's doc comment
// (auth_admin_routes.go) for the same "never trust identity fields from the
// body" convention this mirrors.
type issueCliTokenResponseBody struct {
	JWT       string `json:"jwt"`
	ExpiresAt string `json:"expires_at"`
}

func handleIssueCliToken(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.IssueServiceToken(ctx, &authv1.IssueServiceTokenRequest{
			UserId:   identity.UserID, // ALWAYS the caller — never from body/query
			Audience: cliTokenAudience,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, issueCliTokenResponseBody{
			JWT:       resp.GetJwt(),
			ExpiresAt: resp.GetExpiresAt().AsTime().Format(http.TimeFormat),
		})
	}
}

// cliTokenJSON is one auth.issued_service_tokens row's REST shape — never
// the JWT itself (that was only ever returned once, at mint time).
type cliTokenJSON struct {
	JTI       string `json:"jti"`
	Audience  string `json:"audience"`
	IssuedAt  string `json:"issued_at"`
	ExpiresAt string `json:"expires_at"`
	// RevokedAt is omitted (empty string) for a currently-valid token.
	RevokedAt string `json:"revoked_at,omitempty"`
}

type listCliTokensResponseBody struct {
	Tokens []cliTokenJSON `json:"tokens"`
}

func toCliTokenJSON(t *authv1.CliToken) cliTokenJSON {
	out := cliTokenJSON{
		JTI:       t.GetJti(),
		Audience:  t.GetAudience(),
		IssuedAt:  t.GetIssuedAt().AsTime().Format(http.TimeFormat),
		ExpiresAt: t.GetExpiresAt().AsTime().Format(http.TimeFormat),
	}
	if t.GetRevokedAt() != nil {
		out.RevokedAt = t.GetRevokedAt().AsTime().Format(http.TimeFormat)
	}
	return out
}

func handleListCliTokens(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListCliTokens(ctx, &authv1.ListCliTokensRequest{UserId: identity.UserID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		out := make([]cliTokenJSON, 0, len(resp.GetTokens()))
		for _, t := range resp.GetTokens() {
			out = append(out, toCliTokenJSON(t))
		}
		writeJSON(w, http.StatusOK, listCliTokensResponseBody{Tokens: out})
	}
}

func handleRevokeCliToken(client authv1.AuthServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		jti := chi.URLParam(r, "id")
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		// auth-service's RevokeCliToken usecase PHẢI tự verify jti thuộc về
		// identity.UserID trước khi revoke — route này không tự check, mirror
		// cách RevokeSession hiện có uỷ quyền toàn bộ business rule cho usecase.
		if _, err := client.RevokeCliToken(ctx, &authv1.RevokeCliTokenRequest{Jti: jti, UserId: identity.UserID}); err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
