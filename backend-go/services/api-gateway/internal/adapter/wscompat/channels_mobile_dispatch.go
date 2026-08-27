// mobile.* channels — kept in this SEPARATE file (plus its sibling
// channels_mobile_status.go), following channels_push.go's established
// convention, so these edits never touch the shared, high-churn channels.go.
package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// errNotAMobileSession guards every mobile.* channel — a plain browser
// session's Identity has no DeviceID (registry.go), so this rejects before
// any decrypt/RPC work happens.
var errNotAMobileSession = errors.New("wscompat: this channel requires a paired-device (mobile) identity")

// DeviceSecretResolver resolves a paired device's E2E shared secret — the
// same secret auth-service minted during CompleteDevicePairing (SOL-MB-01),
// via auth-service's internal-only ResolveDeviceSharedSecret RPC.
type DeviceSecretResolver interface {
	ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error)
}

type mobileDispatchArgs struct {
	PtyID         string `json:"ptyId"`
	EncryptedBody string `json:"encryptedBody"` // base64 NaCl secretbox ciphertext
	Nonce         string `json:"nonce"`
	Overwrite     bool   `json:"overwrite"`
}

type dispatchOutcomeView struct {
	Outcome                     string `json:"outcome"`
	ExistingQueuedPromptPreview string `json:"existingQueuedPromptPreview,omitempty"`
}

// RegisterMobileChannels wires every mobile.* wscompat channel — called
// once from cmd/server/main.go's composition root, alongside
// RegisterRealChannels/RegisterPushChannels. Extended by TASK-MB-04-06 to
// also register mobile.status/mobile.statusSubscribe rather than adding a
// second composition-root call site.
func RegisterMobileChannels(r *Registry, infraFleetClient infrafleetv1.InfraFleetServiceClient, projectClient projectv1.ProjectServiceClient, devices DeviceSecretResolver) {
	registerMobileDispatchChannel(r, infraFleetClient, devices)
	registerMobileStatusChannels(r, projectClient, devices)
}

// registerMobileDispatchChannel wires mobile.dispatch: api-gateway.md §2
// keeps business logic (the gate/queue decision) out of wscompat — this
// channel only decrypts, then relays to infra-fleet-service.DispatchPrompt,
// which owns BR-MB-09..12.
func registerMobileDispatchChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient, devices DeviceSecretResolver) {
	r.Register("mobile.dispatch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[mobileDispatchArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if id.DeviceID == "" {
			return nil, errNotAMobileSession
		}
		secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID)
		if err != nil {
			return nil, err
		}
		prompt, err := unsealMobilePayload(in.EncryptedBody, in.Nonce, secret)
		if err != nil {
			return nil, fmt.Errorf("wscompat: decrypting mobile dispatch payload: %w", err)
		}

		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.DispatchPrompt(ctx, &infrafleetv1.DispatchPromptRequest{
			PtyId: in.PtyID, Prompt: prompt, Overwrite: in.Overwrite, DispatchedByDeviceId: id.DeviceID,
		})
		if err != nil {
			return nil, err
		}
		return dispatchOutcomeView{
			Outcome:                     resp.GetOutcome().String(),
			ExistingQueuedPromptPreview: resp.GetExistingQueuedPromptPreview(),
		}, nil
	})
}
