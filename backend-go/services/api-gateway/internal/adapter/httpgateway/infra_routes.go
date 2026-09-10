package httpgateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// mountInfraRoutes wires REST->gRPC proxy routes for infra-fleet-service,
// following the same hand-written translation pattern as
// mountUsageRoutes (usage_routes.go) — see that function's doc comment for
// why this isn't grpc-gateway-generated. tenant_id/user_id are always taken
// from identityFromContext, never trusted from the request body or query,
// per api-gateway.md §9.
func mountInfraRoutes(r chi.Router, client infrafleetv1.InfraFleetServiceClient) {
	r.Route("/v1/infra", func(sub chi.Router) {
		sub.Post("/dev-servers", handleRegisterDevServer(client))
		sub.Get("/dev-servers", handleListDevServers(client))
		sub.Post("/connections/resolve", handleResolveConnection(client))
		sub.Post("/connections", handleCreateConnection(client))
		sub.Post("/ssh-targets", handleCreateSshTarget(client))
		sub.Get("/health", handleGetFleetHealth(client))
		sub.Post("/workspaces/scan-ports", handleScanWorkspacePorts(client))
		sub.Post("/relay", handleRelay(client))
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

// registerDevServerRequestBody is the REST request shape for POST
// /v1/infra/dev-servers — tenant_id is deliberately absent: it comes from
// the validated Identity, never trusted from the request body.
type registerDevServerRequestBody struct {
	Host        string `json:"host"`
	Mode        string `json:"mode"`
	SshTargetID string `json:"ssh_target_id"`
}

func handleRegisterDevServer(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body registerDevServerRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.RegisterDevServer(ctx, &infrafleetv1.RegisterDevServerRequest{
			TenantId:    identity.TenantID,
			Host:        body.Host,
			Mode:        parseConnectionMode(body.Mode),
			SshTargetId: body.SshTargetID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp.GetDevServer())
	}
}

func handleListDevServers(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ListDevServers(ctx, &infrafleetv1.ListDevServersRequest{})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// resolveConnectionRequestBody is the REST request shape for POST
// /v1/infra/connections/resolve.
type resolveConnectionRequestBody struct {
	ConnectionID string `json:"connection_id"`
}

func handleResolveConnection(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body resolveConnectionRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}
		if body.ConnectionID == "" {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "connection_id is required")
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ResolveConnection(ctx, &infrafleetv1.ResolveConnectionRequest{
			ConnectionId: body.ConnectionID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// createConnectionRequestBody is the REST request shape for POST
// /v1/infra/connections.
type createConnectionRequestBody struct {
	DevServerID string `json:"dev_server_id"`
	RepoPath    string `json:"repo_path"`
	WorktreeID  string `json:"worktree_id"`
}

func handleCreateConnection(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createConnectionRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateConnection(ctx, &infrafleetv1.CreateConnectionRequest{
			DevServerId: body.DevServerID,
			RepoPath:    body.RepoPath,
			WorktreeId:  body.WorktreeID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

// createSshTargetRequestBody is the REST request shape for POST
// /v1/infra/ssh-targets — tenant_id is deliberately absent: it comes from
// the validated Identity, never trusted from the request body.
type createSshTargetRequestBody struct {
	Host         string `json:"host"`
	User         string `json:"user"`
	VaultSshRole string `json:"vault_ssh_role"`
}

func handleCreateSshTarget(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body createSshTargetRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.CreateSshTarget(ctx, &infrafleetv1.CreateSshTargetRequest{
			TenantId:     identity.TenantID,
			Host:         body.Host,
			User:         body.User,
			VaultSshRole: body.VaultSshRole,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func handleGetFleetHealth(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.GetFleetHealth(ctx, &infrafleetv1.GetFleetHealthRequest{
			TenantId: identity.TenantID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// scanWorkspacePortsRequestBody is the REST request shape for POST
// /v1/infra/workspaces/scan-ports.
type scanWorkspacePortsRequestBody struct {
	ConnectionID string `json:"connection_id"`
	WorktreeID   string `json:"worktree_id"`
}

func handleScanWorkspacePorts(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body scanWorkspacePortsRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.ScanWorkspacePorts(ctx, &infrafleetv1.ScanWorkspacePortsRequest{
			ConnectionId: body.ConnectionID,
			WorktreeId:   body.WorktreeID,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// relayRequestBody is the REST request shape for POST /v1/infra/relay.
type relayRequestBody struct {
	ConnectionID string `json:"connection_id"`
	Method       string `json:"method"`
	ParamsJSON   string `json:"params_json"`
}

func handleRelay(client infrafleetv1.InfraFleetServiceClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, _ := identityFromContext(r.Context())

		var body relayRequestBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "INVALID_ARGUMENT", "invalid JSON body: "+err.Error())
			return
		}

		ctx := gatewaygrpc.AttachIdentity(r.Context(), identity)
		resp, err := client.Relay(ctx, &infrafleetv1.RelayRequest{
			ConnectionId: body.ConnectionID,
			Method:       body.Method,
			ParamsJson:   body.ParamsJSON,
		})
		if err != nil {
			writeGRPCError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func parseConnectionMode(v string) infrafleetv1.ConnectionMode {
	switch v {
	case "relay_ssh":
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_SSH
	case "relay_websocket":
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_RELAY_WEBSOCKET
	case "direct_websocket":
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_DIRECT_WEBSOCKET
	default:
		return infrafleetv1.ConnectionMode_CONNECTION_MODE_UNSPECIFIED
	}
}
