# TASK-CR-03-05: Add REST mirror `POST /v1/annotations/send-to-agent`

**From Solution:** SOL-CR-03
**Priority:** P2
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/annotation_routes.go`
**Depends on:** TASK-CR-03-03
**Status:** `[ ]` TODO

---

## Context

Every other `annotation.*` wscompat channel has a REST mirror in
`annotation_routes.go` (`mountAnnotationRoutes`). `annotation.sendToAgent`
gets the same treatment for parity with clients that use the REST surface
instead of the WS RPC surface.

## Changes to make

Add the route:

```go
func mountAnnotationRoutes(r chi.Router, client annotationv1.AnnotationServiceClient, gitClient gitgatewayv1.GitGatewayServiceClient) {
	r.Route("/v1/annotations", func(sub chi.Router) {
		sub.Post("/", handleCreateAnnotation(client))
		sub.Get("/", handleListAnnotations(client))
		sub.Put("/{id}", handleUpdateAnnotation(client))
		sub.Patch("/{id}", handleUpdateAnnotation(client))
		sub.Delete("/{id}", handleDeleteAnnotation(client))
		sub.Post("/mark-sent", handleMarkAnnotationsSent(client)) // from TASK-CR-02-07
		sub.Post("/send-to-agent", handleSendToAgent(client, gitClient))
	})
}
```

`mountAnnotationRoutes` gains a `gitClient` parameter — update its one call
site (wherever `httpgateway`'s router composition root calls it) to pass
the already-dialed `gitgatewayv1.GitGatewayServiceClient`.

```go
type sendToAgentRequestBody struct {
	WorktreeID   string `json:"worktree_id"`
	PtyID        string `json:"pty_id"`
	WorktreeName string `json:"worktree_name"`
}

// handleSendToAgent is annotation.sendToAgent's REST mirror. It duplicates
// the composition logic in channels_annotation_send.go rather than sharing
// it directly, matching this file's existing pattern of hand-written
// request/response translation per transport (see mountAnnotationRoutes'
// doc comment) — a future cleanup could extract the shared orchestration
// into a small internal helper both transports call, but that's out of
// scope for this bug fix.
func handleSendToAgent(client annotationv1.AnnotationServiceClient, gitClient gitgatewayv1.GitGatewayServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body sendToAgentRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.WorktreeID == "" || body.PtyID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "worktree_id and pty_id are required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		result, err := sendReviewFeedbackToAgent(ctx, client, gitClient, body.WorktreeID, body.PtyID, body.WorktreeName)
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}
```

Extract the body of `annotation.sendToAgent`'s handler (TASK-CR-03-03) into
a shared, transport-agnostic function
`sendReviewFeedbackToAgent(ctx, annotationClient, gitClient, worktreeID,
ptyID, worktreeName string) (map[string]any, error)` in
`channels_annotation_send.go`, and have both the wscompat handler and this
REST handler call it — this avoids the two transports' logic drifting
apart, superseding the "duplicates the composition logic" note above with
the cleaner shared-helper shape once this task lands.

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go build ./...
go test ./internal/adapter/httpgateway/... -run TestSendToAgent -v
```

Add a test asserting `POST /v1/annotations/send-to-agent` forwards
`worktree_id`/`pty_id`/`worktree_name` and returns the same
`sent`/`prompt`/`annotations` shape `annotation.sendToAgent` returns over
WS.
