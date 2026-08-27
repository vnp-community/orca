# TASK-CLI-02-05: `api-gateway` — `/v1/worktrees/{worktreeId}/agent/*` REST routes

**From Solution:** SOL-CLI-02
**Priority:** P1
**Service:** `api-gateway`
**File:** `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go`
**Depends on:** TASK-CLI-02-04
**Status:** `[ ]` TODO

---

## Context

`agentStatus`/`wait`/`send`/scrollback are each one request/response, no server-push framing — per `api-gateway.md` §3's REST-vs-WS split these belong on REST, not a new WS channel. Resource-nested per that section's "nested along the logical-FK relationships" rule: `/v1/worktrees/{worktreeId}/agent/*`, not a flat `?worktreeId=` param, and not nested under `/v1/infra` (the resource is "this worktree's agent"). All four handlers share one resolve-then-call step (`resolveAgentPtyID`), following `infra_routes.go`'s existing hand-written REST->gRPC pattern.

## Changes to make

In `backend-go/services/api-gateway/internal/adapter/httpgateway/infra_routes.go`:

```go
func mountInfraRoutes(r chi.Router, client infrafleetv1.InfraFleetServiceClient) {
	r.Route("/v1/infra", func(sub chi.Router) {
		// ... existing routes unchanged ...
	})

	r.Route("/v1/worktrees/{worktreeId}/agent", func(sub chi.Router) {
		sub.Get("/status", handleGetAgentStatus(client))
		sub.Post("/wait", handleWaitAgent(client))
		sub.Post("/send", handleSendAgentInput(client))
		sub.Get("/snapshot", handleGetAgentSnapshot(client))
	})
}

// errNoActiveAgentSession is resolveAgentPtyID's sentinel for "no live
// terminal session for this worktree" — a finished/never-started agent run
// is expected state, not a bug, so it maps to 404 NOT_FOUND, distinct from
// a genuine gRPC error (writeGRPCError's passthrough path).
var errNoActiveAgentSession = errors.New("no active agent session for this worktree")

// resolveAgentPtyID is the shared first step all four handlers below open
// with.
func resolveAgentPtyID(ctx context.Context, client infrafleetv1.InfraFleetServiceClient, worktreeID string) (string, error) {
	resp, err := client.GetAgentTerminalSession(ctx, &infrafleetv1.GetAgentTerminalSessionRequest{WorktreeId: worktreeID})
	if err != nil {
		return "", err
	}
	if !resp.GetFound() {
		return "", errNoActiveAgentSession
	}
	return resp.GetSession().GetPtyId(), nil
}

func writeAgentResolveError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNoActiveAgentSession) {
		writeJSONError(w, http.StatusNotFound, "NOT_FOUND", "no active agent session for this worktree — has an agent been started?")
		return
	}
	writeGRPCError(w, err)
}

func handleGetAgentStatus(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		ptyID, err := resolveAgentPtyID(ctx, client, chi.URLParam(r, "worktreeId"))
		if err != nil {
			writeAgentResolveError(w, err)
			return
		}
		resp, err := client.GetTerminalAgentStatus(ctx, &infrafleetv1.GetTerminalAgentStatusRequest{PtyId: ptyID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

type waitAgentRequestBody struct {
	TimeoutMs int32 `json:"timeout_ms"`
}

func handleWaitAgent(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		ptyID, err := resolveAgentPtyID(ctx, client, chi.URLParam(r, "worktreeId"))
		if err != nil {
			writeAgentResolveError(w, err)
			return
		}
		var body waitAgentRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}
		resp, err := client.WaitTerminalSession(ctx, &infrafleetv1.WaitTerminalSessionRequest{PtyId: ptyID, TimeoutMs: body.TimeoutMs})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

type sendAgentInputRequestBody struct {
	Text string `json:"text"`
}

func handleSendAgentInput(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		ptyID, err := resolveAgentPtyID(ctx, client, chi.URLParam(r, "worktreeId"))
		if err != nil {
			writeAgentResolveError(w, err)
			return
		}
		var body sendAgentInputRequestBody
		if !decodeJSONBody(w, r, &body) {
			return
		}
		if _, err := client.SendTerminalInput(ctx, &infrafleetv1.SendTerminalInputRequest{PtyId: ptyID, Data: []byte(body.Text)}); err != nil {
			writeGRPCError(w, err)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

func handleGetAgentSnapshot(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())
		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		ptyID, err := resolveAgentPtyID(ctx, client, chi.URLParam(r, "worktreeId"))
		if err != nil {
			writeAgentResolveError(w, err)
			return
		}
		resp, err := client.GetTerminalScrollback(ctx, &infrafleetv1.GetTerminalScrollbackRequest{PtyId: ptyID})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		// BR-CLI-06's "flat scrollback file" — text/plain, not JSON, so
		// `orca snapshot --output result.txt` is a literal body copy.
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if resp.GetTruncated() {
			w.Header().Set("X-Orca-Snapshot-Truncated", "true")
		}
		_, _ = w.Write([]byte(resp.GetText()))
	}
}
```

Add `"errors"` to this file's imports.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/httpgateway/... -run TestInfraRoutes -v
```

Expected new cases in `infra_routes_test.go`, one per route, using a fake `InfraFleetServiceClient`: `GetAgentTerminalSession{found:false}` -> `404` with the documented message, and `GetTerminalAgentStatus`/`WaitTerminalSession`/`SendTerminalInput`/`GetTerminalScrollback` never called afterward (regression guard against calling a stale/wrong `pty_id`); `GET .../snapshot` asserts `Content-Type: text/plain` and the truncated header only when the fake response says `truncated=true`.
