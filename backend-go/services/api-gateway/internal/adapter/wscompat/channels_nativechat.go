// registerNativeChatChannels relays nativeChat.readSession straight to the
// Dev Server Agent via infra-fleet-service's generic Relay RPC — mirrors
// git-gateway-service's RelayExecutor, the established pattern for "read
// something that lives on the user's dev server, not wherever this
// backend process runs." No owning gRPC service exists for this
// one-method namespace, and none is warranted (SOL-017's "no new
// service" rationale).
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

func registerNativeChatChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("nativeChat.readSession", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type readSessionArgs struct {
			Agent          string `json:"agent"`
			SessionID      string `json:"sessionId"`
			Limit          int    `json:"limit,omitempty"`
			TranscriptPath string `json:"transcriptPath,omitempty"`
			ConnectionID   string `json:"connectionId,omitempty"` // see companion-frontend-change note below
		}
		in, err := decodeArg[readSessionArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.ConnectionID == "" {
			// No relay target — fail closed with a clear message rather than
			// silently reading api-gateway's own filesystem (the exact bug
			// BUG-017 flags). A future connectionId-carrying frontend build
			// (NativeChatReadSessionArgs) resolves this branch away entirely.
			return nil, fmt.Errorf("nativeChat.readSession: connectionId is required — this backend never reads transcript files from its own host")
		}

		ctx = attachIdentity(ctx, id)
		rpcCtx, cancel := context.WithTimeout(ctx, groupRPCTimeout)
		defer cancel()

		paramsJSON, err := json.Marshal(map[string]any{
			"agent": in.Agent, "sessionId": in.SessionID,
			"limit": in.Limit, "transcriptPath": in.TranscriptPath,
		})
		if err != nil {
			return nil, err
		}
		resp, err := client.Relay(rpcCtx, &infrafleetv1.RelayRequest{
			ConnectionId: in.ConnectionID,
			Method:       "nativeChat.readSession",
			ParamsJson:   string(paramsJSON),
		})
		if err != nil {
			return nil, err
		}
		// result_json is passed through verbatim — the Dev Server Agent's
		// response already matches NativeChatReadSessionResult's wire shape
		// ({messages: [...]} | {error: string}).
		var result json.RawMessage
		if resp.GetResultJson() != "" {
			result = json.RawMessage(resp.GetResultJson())
		}
		return result, nil
	})
}
