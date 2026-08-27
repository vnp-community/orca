package httpgateway

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	aiproviderv1 "github.com/stablyai/orca-go/proto/gen/go/orca/aiprovider/v1"
)

// mountAIProviderRoutes wires the real REST->gRPC reverse-proxy path for
// ai-provider-service, following the same hand-written translation pattern
// as mountUsageRoutes (usage_routes.go) — no grpc-gateway codegen, see that
// file's doc comment for why.
//
// ai-provider-service owns provider account metadata + quota tracking only;
// it never stores or returns plaintext credential material (see
// aiprovider.proto's service doc comment and ResolveProviderResponse's).
// Every handler below writes back only the proto response message verbatim
// (or a field of it) — never an ad-hoc struct that could accidentally grow
// a secret field over time.
func mountAIProviderRoutes(r chi.Router, client aiproviderv1.AiProviderServiceClient) {
	r.Route("/v1/ai-providers", func(sub chi.Router) {
		sub.Post("/accounts", handleCreateAccount(client))
		sub.Get("/resolve", handleResolveProvider(client))
		sub.Post("/accounts/{id}/rotate-key", handleRotateKey(client))
		sub.Get("/usage-today", handleGetUsageToday(client))
	})
}

// createAccountRequestBody is the REST request shape for POST
// /v1/ai-providers/accounts — tenant_id is deliberately absent: it comes
// from the validated Identity, never trusted from the request body, per
// api-gateway.md §9.
type createAccountRequestBody struct {
	Type string `json:"type"`
}

func handleCreateAccount(client aiproviderv1.AiProviderServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createAccountRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateAccount(ctx, &aiproviderv1.CreateAccountRequest{
			TenantId: identity.TenantID,
			Type:     parseProviderType(body.Type),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetAccount())
	}
}

func handleResolveProvider(client aiproviderv1.AiProviderServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		userID := q.Get("user_id")
		if userID == "" {
			userID = identity.UserID
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ResolveProvider(ctx, &aiproviderv1.ResolveProviderRequest{
			TenantId:  identity.TenantID,
			UserId:    userID,
			ProjectId: q.Get("project_id"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		// SECURITY-CRITICAL: ResolveProviderResponse carries only a
		// ProviderAccount reference (credential_ref), never plaintext
		// credential material — write the proto response verbatim, never
		// augment it with extra fields. See aiprovider.proto's
		// ResolveProviderResponse doc comment.
		writeJSON(w, http.StatusOK, resp.GetAccount())
	}
}

func handleRotateKey(client aiproviderv1.AiProviderServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		accountID := chi.URLParam(r, "id")

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.RotateKey(ctx, &aiproviderv1.RotateKeyRequest{
			AccountId: accountID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp.GetAccount())
	}
}

func handleGetUsageToday(client aiproviderv1.AiProviderServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		q := r.URL.Query()

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetUsageToday(ctx, &aiproviderv1.GetUsageTodayRequest{
			AccountId: q.Get("account_id"),
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// parseProviderType accepts either the bare suffix (case-insensitive, e.g.
// "anthropic") or the full enum name (e.g. "PROVIDER_TYPE_ANTHROPIC") —
// mirrors parseStepType's leniency in automation_routes.go.
func parseProviderType(v string) aiproviderv1.ProviderType {
	name := strings.ToUpper(v)
	if !strings.HasPrefix(name, "PROVIDER_TYPE_") {
		name = "PROVIDER_TYPE_" + name
	}
	if n, ok := aiproviderv1.ProviderType_value[name]; ok {
		return aiproviderv1.ProviderType(n)
	}
	return aiproviderv1.ProviderType_PROVIDER_TYPE_UNSPECIFIED
}
