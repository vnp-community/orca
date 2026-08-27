// channels_terminal_scrollback.go registers terminal.scrollback.save /
// terminal.scrollback.restore — deliberately NOT part of
// terminal.multiplex's opcode set (see SOL-TM-03's "two distinct
// snapshot mechanisms" rationale: that protocol's SnapshotRequest resolves against a
// LIVE ptyId this flow structurally cannot have). Plain JSON channels,
// matching terminal.create/terminal.list's shape — this fires once per
// pane teardown/restore, not per keystroke, so there is no low-latency
// requirement forcing terminal.multiplex's binary framing.
package wscompat

import (
	"context"
	"encoding/json"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"

	gatewaygrpc "github.com/stablyai/orca-go/services/api-gateway/internal/adapter/grpc"
	"github.com/stablyai/orca-go/services/api-gateway/internal/usecase"
)

type scrollbackSaveArgs struct {
	WorktreeID string `json:"worktreeId"`
	PaneKey    string `json:"paneKey"`
	Cols       int32  `json:"cols"`
	Rows       int32  `json:"rows"`
	Data       string `json:"data"` // xterm SerializeAddon output, UTF-8 text
	LastTitle  string `json:"lastTitle"`
}

type scrollbackRestoreArgs struct {
	WorktreeID string `json:"worktreeId"`
	PaneKey    string `json:"paneKey"`
}

func registerTerminalScrollbackChannels(r *Registry, client infrafleetv1.InfraFleetServiceClient) {
	r.Register("terminal.scrollback.save", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[scrollbackSaveArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		_, err = client.SaveTerminalScrollbackSnapshot(ctx, &infrafleetv1.SaveTerminalScrollbackSnapshotRequest{
			WorktreeId: in.WorktreeID, PaneKey: in.PaneKey, Cols: in.Cols, Rows: in.Rows,
			Data: []byte(in.Data), LastTitle: in.LastTitle,
		})
		return map[string]bool{"ok": err == nil}, err
	})

	r.Register("terminal.scrollback.restore", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[scrollbackRestoreArgs](args, 0)
		if err != nil {
			return nil, err
		}
		ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
		resp, err := client.GetTerminalScrollbackSnapshot(ctx, &infrafleetv1.GetTerminalScrollbackSnapshotRequest{WorktreeId: in.WorktreeID, PaneKey: in.PaneKey})
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"found": resp.GetFound(), "cols": resp.GetCols(), "rows": resp.GetRows(),
			"data": string(resp.GetData()), "lastTitle": resp.GetLastTitle(),
			"updatedAt": resp.GetUpdatedAtUnixMs(),
		}, nil
	})
}
