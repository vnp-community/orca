// channels_browser_screencast.go registers browser.screencast — the remote
// headless-browser pane's live-view stream. TASK-036 shipped the other 12
// browser.* request/response ops (channels_browser.go) but left this one
// unimplemented, framing the whole feature as blocked on a frontend
// architecture decision. That framing was wrong for the web/server-mode
// build api-gateway actually targets: RemoteBrowserPagePane's
// window.api.runtimeEnvironments.subscribe(...) already reaches this
// package's /ws surface today, proven by the working accounts.subscribe
// (TASK-023) precedent — see this file's git history / PR description for
// the two research passes that found this. The real gap was narrower:
// browser.screencast itself didn't exist anywhere, and status.get
// hardcoded an empty capabilities list. Both are fixed by this file (plus
// this package's registerStatusChannels, see channels_repo_ssh_status_workspace.go).
//
// # Known, deliberate scope cuts
//
// BinaryStreamChannelHandler gives exactly one synchronous JSON ack plus an
// ongoing binary-send capability — there is no ongoing JSON push channel
// alongside it (that's StreamChannelHandler's shape; registry.go's four
// handler kinds are mutually exclusive). The OLD TS backend's
// browser.screencast sent BOTH an ongoing JSON event stream (ready/dialog/
// dialogClosed/end/error, via emit(...)) and binary frames (sendBinary)
// concurrently. Rather than add a 5th registry kind (real, separable infra
// work), this implementation:
//   - sends "ready" as the synchronous ack BinaryStreamChannelHandler
//     already returns (RemoteBrowserPagePane's onResponse receives it the
//     same way any invoke ack arrives);
//   - sends every frame via SendBinary — onBinary fires exactly as the
//     frontend already expects, no frontend change needed;
//   - does NOT forward dialog/dialogClosed events — a JS alert()/confirm()
//     during remote page navigation won't surface in the pane this pass;
//   - sends no explicit "end" frame on agent/browser closure or error —
//     the feed simply stops (last frame freezes). This matches the exact
//     same, already-documented limitation accounts.subscribe/
//     notifications.subscribe carry today ("ending this subscription is
//     always whole-connection teardown" — channels_accounts.go's own doc
//     comment), not a new regression.
package wscompat

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// browserScreencastArgs mirrors the OLD TS backend's Screencast Zod schema
// (backend/src/main/runtime/rpc/methods/browser-schemas.ts) field-for-field
// — "worktree" (not "worktreeId") matches every other browser.* channel's
// param name (channels_browser.go's registerBrowserRelay).
type browserScreencastArgs struct {
	Worktree           string   `json:"worktree"`
	Page               string   `json:"page"`
	Format             string   `json:"format"`
	Quality            *int32   `json:"quality"`
	MaxWidth           *int32   `json:"maxWidth"`
	MaxHeight          *int32   `json:"maxHeight"`
	ViewportWidth      *int32   `json:"viewportWidth"`
	ViewportHeight     *int32   `json:"viewportHeight"`
	DeviceScaleFactor  *float64 `json:"deviceScaleFactor"`
	Mobile             bool     `json:"mobile"`
	EveryNthFrame      *int32   `json:"everyNthFrame"`
	MinFrameIntervalMs *int32   `json:"minFrameIntervalMs"`
}

// Clamp bounds mirror orca-runtime-browser.ts's clampInteger/
// clampOptionalInteger/clampOptionalNumber calls exactly, so acceptance
// behavior is byte-identical to the pre-backend-go implementation.
func clampInt32(v *int32, lo, hi, def int32) int32 {
	if v == nil {
		return def
	}
	if *v < lo {
		return lo
	}
	if *v > hi {
		return hi
	}
	return *v
}

func clampOptionalInt32(v *int32, lo, hi int32) *int32 {
	if v == nil {
		return nil
	}
	c := clampInt32(v, lo, hi, lo)
	return &c
}

func clampOptionalFloat64(v *float64, lo, hi float64) *float64 {
	if v == nil {
		return nil
	}
	c := *v
	if c < lo {
		c = lo
	}
	if c > hi {
		c = hi
	}
	return &c
}

func toProtoStartScreencast(in browserScreencastArgs) *infrafleetv1.StartScreencast {
	format := in.Format
	if format != "png" {
		format = "jpeg" // mirrors the OLD backend's Screencast schema transform: anything but "png" collapses to "jpeg"
	}
	return &infrafleetv1.StartScreencast{
		WorktreeId:         in.Worktree,
		Page:               in.Page,
		Format:             format,
		Quality:            clampInt32(in.Quality, 10, 100, 70),
		MaxWidth:           clampInt32(in.MaxWidth, 320, 3840, 1440),
		MaxHeight:          clampInt32(in.MaxHeight, 240, 2160, 1200),
		ViewportWidth:      clampOptionalInt32(in.ViewportWidth, 320, 3840),
		ViewportHeight:     clampOptionalInt32(in.ViewportHeight, 240, 2160),
		DeviceScaleFactor:  clampOptionalFloat64(in.DeviceScaleFactor, 1, 4),
		Mobile:             in.Mobile,
		EveryNthFrame:      clampInt32(in.EveryNthFrame, 1, 10, 2),
		MinFrameIntervalMs: clampInt32(in.MinFrameIntervalMs, 0, 1000, 0),
	}
}

func registerBrowserScreencastChannel(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.RegisterBinaryStreamHandler("browser.screencast", func(ctx context.Context, id Identity, args []json.RawMessage, io BinaryStreamIO) (any, error) {
		in, err := decodeArg[browserScreencastArgs](args, 0)
		if err != nil {
			return nil, err
		}
		if in.Worktree == "" {
			return nil, fmt.Errorf("BROWSER_NO_WORKTREE: browser.screencast requires a worktree selector")
		}

		// attachContext (channels_terminal.go) — context.Background()-derived,
		// identity-already-attached, long-lived (NOT the 25s
		// invokeTimeout-bounded dispatchCtx) so the stream outlives this one
		// invoke's own deadline, same reasoning terminal.multiplex/AttachPty
		// already established.
		streamCtx, cancel := attachContext(id)
		stream, err := client.AttachScreencast(streamCtx)
		if err != nil {
			cancel()
			return nil, err
		}
		if err := stream.Send(&infrafleetv1.ScreencastClientFrame{
			Frame: &infrafleetv1.ScreencastClientFrame_Start{Start: toProtoStartScreencast(in)},
		}); err != nil {
			cancel()
			return nil, err
		}

		// Block for the first frame (ready or error) — bounded by the 25s
		// invokeTimeout on THIS call, same as any other synchronous ack; a
		// real CDP screencast start is sub-second.
		first, err := stream.Recv()
		if err != nil {
			cancel()
			return nil, err
		}
		ready := first.GetReady()
		if ready == nil {
			cancel()
			if e := first.GetError(); e != nil {
				return nil, fmt.Errorf("BROWSER_SCREENCAST_FAILED: %s", e.GetMessage())
			}
			return nil, fmt.Errorf("BROWSER_SCREENCAST_FAILED: unexpected first frame")
		}

		go func() {
			defer cancel()
			for {
				msg, err := stream.Recv()
				if err != nil {
					return
				}
				switch f := msg.GetFrame().(type) {
				case *infrafleetv1.ScreencastServerFrame_FrameData:
					if !io.SendBinary(f.FrameData.GetData()) {
						return // client gone — see this file's package doc comment on "no explicit end frame" scope cut
					}
				case *infrafleetv1.ScreencastServerFrame_Ended, *infrafleetv1.ScreencastServerFrame_Error:
					return
				}
			}
		}()

		return map[string]any{
			"type":           "ready",
			"subscriptionId": ready.GetSubscriptionId(),
			"browserPageId":  ready.GetBrowserPageId(),
			"format":         ready.GetFormat(),
		}, nil
	})
}
