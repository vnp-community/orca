package httpgateway

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	scmintegrationv1 "github.com/stablyai/orca-go/proto/gen/go/orca/scmintegration/v1"
)

// mountSCMWebhookRoutes wires POST /v1/scm/webhooks/{provider} — deliberately
// mounted OUTSIDE the JWT-authenticated router group (see router.go's
// NewRouter doc comment for that group's contract): the caller is
// GitHub/GitLab's own servers, which never carry an Orca JWT. Authenticity
// is instead established by scm-integration-service's ReceiveWebhook RPC
// verifying the provider's own signature header (BUG-PI-03/SOL-PI-03).
func mountSCMWebhookRoutes(r chi.Router, client scmintegrationv1.ScmIntegrationServiceClient) {
	r.Post("/v1/scm/webhooks/{provider}", handleReceiveSCMWebhook(client))
}

// handleReceiveSCMWebhook forwards the raw request body and provider-specific
// signature headers straight through to ReceiveWebhook — no translation of
// the payload itself, since verification/parsing happens entirely inside
// scm-integration-service against the exact byte sequence received here.
func handleReceiveSCMWebhook(client scmintegrationv1.ScmIntegrationServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := chi.URLParam(r, "provider")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "failed to read request body")
			return
		}

		// GitHub signs with X-Hub-Signature-256 + X-GitHub-Delivery; GitLab
		// signs with X-Gitlab-Token + X-Gitlab-Event-UUID. Both are passed
		// through unconditionally — scm-integration-service picks the pair
		// that matches the {provider} path segment and ignores the other.
		signatureHeader := r.Header.Get("X-Hub-Signature-256")
		if signatureHeader == "" {
			signatureHeader = r.Header.Get("X-Gitlab-Token")
		}
		deliveryIDHeader := r.Header.Get("X-GitHub-Delivery")
		if deliveryIDHeader == "" {
			deliveryIDHeader = r.Header.Get("X-Gitlab-Event-UUID")
		}

		resp, err := client.ReceiveWebhook(r.Context(), &scmintegrationv1.ReceiveWebhookRequest{
			Provider: provider, RawBody: body,
			SignatureHeader:  signatureHeader,
			DeliveryIdHeader: deliveryIDHeader,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
