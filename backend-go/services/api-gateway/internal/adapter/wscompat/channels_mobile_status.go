package wscompat

import (
	"context"
	"encoding/json"
	"reflect"
	"time"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// mobileStatusPollInterval is BR-MB-14's poll-and-diff cadence — worktree/
// agent status changes aren't published as domain events, so
// mobile.statusSubscribe polls project-service on this interval rather than
// bridging a true event stream (api-gateway.md §2's stateless-by-design
// principle: this server does no foreground-detection of its own, the
// mobile client enforces "foreground only" by closing the stream itself).
// A var, not a const, so tests can shrink it — mirrors
// channels_accounts.go's accountsSubscribePollInterval.
var mobileStatusPollInterval = 5 * time.Second

// registerMobileStatusChannels wires mobile.status (BR-MB-16's pull-to-
// refresh) and mobile.statusSubscribe (BR-MB-14's live update while
// foregrounded). Called from RegisterMobileChannels (channels_mobile_dispatch.go).
func registerMobileStatusChannels(r *Registry, client projectv1.ProjectServiceClient, devices DeviceSecretResolver) {
	r.Register("mobile.status", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		if id.DeviceID == "" {
			return nil, errNotAMobileSession // mirrors mobile.dispatch (TASK-MB-03-06)
		}
		gctx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetMobileWorktreeStatus(gctx, &projectv1.GetMobileWorktreeStatusRequest{})
		if err != nil {
			return nil, err
		}
		secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID) // BR-MB-13: encrypt in transit
		if err != nil {
			return nil, err
		}
		return sealMobileEnvelope(resp, secret) // shared helper: mobile_envelope.go
	})

	r.RegisterStream("mobile.statusSubscribe", func(ctx context.Context, id Identity, args []json.RawMessage) (<-chan PushEvent, error) {
		if id.DeviceID == "" {
			return nil, errNotAMobileSession
		}
		secret, err := devices.ResolveSharedSecret(ctx, id.DeviceID)
		if err != nil {
			return nil, err
		}
		out := make(chan PushEvent)
		go pollAndDiffMobileStatus(ctx, client, id, secret, out)
		return out, nil
	})
}

// pollAndDiffMobileStatus polls GetMobileWorktreeStatus every
// mobileStatusPollInterval and only emits a PushEvent when the computed
// worktree view actually changed since the last poll — a transient RPC
// error just skips that tick (best-effort poll, not a hard failure).
func pollAndDiffMobileStatus(ctx context.Context, client projectv1.ProjectServiceClient, id Identity, secret []byte, out chan<- PushEvent) {
	defer close(out)
	ticker := time.NewTicker(mobileStatusPollInterval)
	defer ticker.Stop()
	var lastSent *projectv1.GetMobileWorktreeStatusResponse
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			gctx := gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
			resp, err := client.GetMobileWorktreeStatus(gctx, &projectv1.GetMobileWorktreeStatusRequest{})
			if err != nil {
				continue // best-effort poll — a transient error just skips this tick
			}
			if lastSent != nil && worktreeStatusesEqual(lastSent, resp) {
				continue // no change — regression guard: two consecutive identical polls produce exactly one PushEvent, not two
			}
			envelope, err := sealMobileEnvelope(resp, secret)
			if err != nil {
				continue
			}
			select {
			case out <- PushEvent{Channel: "mobile.statusEvent", Args: []any{envelope}}:
				lastSent = resp
			case <-ctx.Done():
				return
			}
		}
	}
}

// worktreeStatusesEqual compares two responses' worktree lists only —
// GeneratedAtUnixMs is deliberately excluded, or every poll would count as
// "changed" and defeat the diffing.
func worktreeStatusesEqual(a, b *projectv1.GetMobileWorktreeStatusResponse) bool {
	return reflect.DeepEqual(a.GetWorktrees(), b.GetWorktrees())
}
