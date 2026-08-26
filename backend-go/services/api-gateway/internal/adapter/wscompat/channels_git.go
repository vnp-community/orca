// New wscompat channel registrations for the git-gateway-service RPCs added
// by TASK-206/TASK-208/TASK-209(shippable subset)/TASK-210(shippable
// subset)/TASK-211/TASK-049..060. Kept in its own file rather than
// channels.go: other agents are wiring other namespaces in parallel in
// their own worktrees and will also want to add registrations to
// channels.go — editing that shared file directly across worktrees creates
// unmergeable conflicts. registerGitDeepChannels/registerFilesChannels are
// ready to be called from RegisterRealChannels once a separate integration
// pass merges every group's new channels_*.go file — see this repo's
// docs/execution-plan.md and this task's own final report for the exact
// one-line wiring still needed.
//
// git.status/git.diff already exist in channels.go's registerGitChannels;
// this file's git.diff registration below (with the filePath TASK-228
// added) is the correct, up-to-date one — when wired in, call
// registerGitDeepChannels AFTER registerGitChannels so this file's
// registration wins (Registry.Register simply overwrites the map entry).
package wscompat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

// registerGitDeepChannels wires every git.* channel this scope's tasks
// (TASK-206/208/209-shippable/210-shippable/211/212-Group-B/213-shippable)
// back with a real git-gateway-service RPC. Channels for RPCs this scope
// deliberately did NOT implement (TASK-207's branch/ref group;
// commitCompare/branchCompare/commitDiff/branchDiff/submoduleStatus/fetch —
// all flagged BLOCKED in their own task files) are NOT registered here —
// they keep falling through to notImplementedHandler until a future pass
// resolves their open design questions (see SOL-032 §0).
func registerGitDeepChannels(r *Registry, client gitgatewayv1.GitGatewayServiceClient) {
	// ── TASK-206: commit/push/pull/generateCommitMessage ────────────────

	r.Register("git.commit", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type commitArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Message    string   `json:"message"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[commitArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Commit(ctx, &gitgatewayv1.CommitRequest{
			WorktreeId: in.WorktreeID, Message: in.Message, Paths: in.Paths,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.push", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type pushArgs struct {
			WorktreeID string `json:"worktreeId"`
			Remote     string `json:"remote"`
			Branch     string `json:"branch"`
		}
		in, err := decodeArg[pushArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Push(ctx, &gitgatewayv1.PushRequest{
			WorktreeId: in.WorktreeID, Remote: in.Remote, Branch: in.Branch,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.pull", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type pullArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[pullArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Pull(ctx, &gitgatewayv1.PullRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.generateCommitMessage", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type genArgs struct {
			WorktreeID string `json:"worktreeId"`
		}
		in, err := decodeArg[genArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GenerateCommitMessage(ctx, &gitgatewayv1.GenerateCommitMessageRequest{WorktreeId: in.WorktreeID})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// ── TASK-228: git.diff, corrected to thread filePath through — this
	// registration supersedes channels.go's older git.diff (no filePath)
	// once both are wired in RegisterRealChannels, per this file's package
	// doc comment. ─────────────────────────────────────────────────────
	r.Register("git.diff", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type diffArgs struct {
			WorktreeID string `json:"worktreeId"`
			FilePath   string `json:"filePath"`
			Staged     bool   `json:"staged"`
		}
		in, err := decodeArg[diffArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GetDiff(ctx, &gitgatewayv1.GetDiffRequest{
			WorktreeId: in.WorktreeID, FilePath: in.FilePath, Staged: in.Staged,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// ── TASK-208/TASK-212 (Group B only — Group A/TASK-207 was
	// deliberately not implemented in this scope, see this repo's
	// git-gateway-service TASK-207 file's own Status line). git.stage/
	// git.bulkStage share one handler (both call the same
	// StageRequest.Paths-typed RPC); same for unstage. ──────────────────

	stageHandler := func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type stageArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[stageArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Stage(ctx, &gitgatewayv1.StageRequest{WorktreeId: in.WorktreeID, Paths: in.Paths})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	r.Register("git.stage", stageHandler)
	r.Register("git.bulkStage", stageHandler)

	unstageHandler := func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type unstageArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[unstageArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.Unstage(ctx, &gitgatewayv1.UnstageRequest{WorktreeId: in.WorktreeID, Paths: in.Paths})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	r.Register("git.unstage", unstageHandler)
	r.Register("git.bulkUnstage", unstageHandler)

	// ── TASK-209/TASK-213 (shippable-now subset only: history/
	// checkIgnored/forkSync/upstreamStatus — commitCompare/branchCompare/
	// commitDiff/branchDiff/submoduleStatus are BLOCKED, not implemented,
	// not wired). Request/response shapes below match TASK-209's Contract
	// correction section, NOT TASK-213's own inline sketch (written before
	// that correction) — cursor dropped, ref renamed baseRef,
	// checkIgnored returns only the ignored subset, forkSync requires
	// expectedUpstream, upstreamStatus takes an optional pushTarget. ────

	r.Register("git.history", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type historyArgs struct {
			WorktreeID string `json:"worktreeId"`
			BaseRef    string `json:"baseRef"`
			Limit      int32  `json:"limit"`
		}
		in, err := decodeArg[historyArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.History(ctx, &gitgatewayv1.HistoryRequest{
			WorktreeId: in.WorktreeID, BaseRef: in.BaseRef, Limit: in.Limit,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.checkIgnored", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type checkIgnoredArgs struct {
			WorktreeID string   `json:"worktreeId"`
			Paths      []string `json:"paths"`
		}
		in, err := decodeArg[checkIgnoredArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.CheckIgnored(ctx, &gitgatewayv1.CheckIgnoredRequest{WorktreeId: in.WorktreeID, Paths: in.Paths})
		if err != nil {
			return nil, err
		}
		return resp.GetIgnoredPaths(), nil
	})

	r.Register("git.forkSync", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type forkSyncArgs struct {
			WorktreeID       string `json:"worktreeId"`
			ExpectedUpstream string `json:"expectedUpstream"`
		}
		in, err := decodeArg[forkSyncArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.ForkSync(ctx, &gitgatewayv1.ForkSyncRequest{
			WorktreeId: in.WorktreeID, ExpectedUpstream: in.ExpectedUpstream,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.upstreamStatus", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type upstreamStatusArgs struct {
			WorktreeID string `json:"worktreeId"`
			PushTarget string `json:"pushTarget"`
		}
		in, err := decodeArg[upstreamStatusArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.UpstreamStatus(ctx, &gitgatewayv1.UpstreamStatusRequest{
			WorktreeId: in.WorktreeID, PushTarget: in.PushTarget,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// ── TASK-210/TASK-213 (shippable-now subset: remoteCommitUrl/
	// remoteFileUrl — fetch is BLOCKED, not implemented, not wired). ────

	r.Register("git.remoteCommitUrl", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type remoteCommitURLArgs struct {
			WorktreeID string `json:"worktreeId"`
			SHA        string `json:"sha"`
		}
		in, err := decodeArg[remoteCommitURLArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.RemoteCommitUrl(ctx, &gitgatewayv1.RemoteCommitUrlRequest{WorktreeId: in.WorktreeID, Sha: in.SHA})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	r.Register("git.remoteFileUrl", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type remoteFileURLArgs struct {
			WorktreeID string `json:"worktreeId"`
			Path       string `json:"path"`
			Ref        string `json:"ref"`
		}
		in, err := decodeArg[remoteFileURLArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.RemoteFileUrl(ctx, &gitgatewayv1.RemoteFileUrlRequest{WorktreeId: in.WorktreeID, Path: in.Path, Ref: in.Ref})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// ── TASK-211/TASK-213 (fully shippable) ──────────────────────────────

	r.Register("git.generatePullRequestFields", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type genPRFieldsArgs struct {
			WorktreeID string `json:"worktreeId"`
			BaseBranch string `json:"baseBranch"`
		}
		in, err := decodeArg[genPRFieldsArgs](args, 0)
		if err != nil {
			return nil, err
		}
		resp, err := client.GeneratePullRequestFields(ctx, &gitgatewayv1.GeneratePullRequestFieldsRequest{
			WorktreeId: in.WorktreeID, BaseBranch: in.BaseBranch,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// git.discoverCommitMessageModels reads from Identity (id.TenantID/
	// id.UserID) instead of args — mirrors SOL-033's automation.create
	// note on never trusting a client-supplied tenant; there's no
	// equivalent identity field to source elsewhere for this channel.
	r.Register("git.discoverCommitMessageModels", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		resp, err := client.DiscoverCommitMessageModels(ctx, &gitgatewayv1.DiscoverCommitMessageModelsRequest{
			TenantId: id.TenantID, UserId: id.UserID,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetModels(), nil
	})
}

// ── files.* (TASK-049..060) ─────────────────────────────────────────────
//
// files.commitUpload and files.unwatch are always-local renderer-side
// bookkeeping in the old backend (no fs I/O) — registered as local no-op
// acks below, not wired to git-gateway-service. Every other channel
// dispatches through GitGatewayServiceClient's FileIO RPC group (SOL-009),
// which itself resolves local-vs-relay per worktree — wscompat never makes
// that decision; it only forwards worktreeId + params.

// simpleFileOp wires a channel that decodes one JSON arg and issues one
// gRPC call, returning the response verbatim — the shape most of files.*'s
// RPC-backed channels share.
func simpleFileOp[Req any](
	r *Registry,
	channel string,
	call func(ctx context.Context, req Req) (any, error),
) {
	r.Register(channel, func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		in, err := decodeArg[Req](args, 0)
		if err != nil {
			return nil, err
		}
		return call(ctx, in)
	})
}

func registerFilesChannels(r *Registry, client gitgatewayv1.GitGatewayServiceClient) {
	type readArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
	}
	simpleFileOp(r, "files.read", func(ctx context.Context, in readArgs) (any, error) {
		resp, err := client.ReadFile(ctx, &gitgatewayv1.ReadFileRequest{WorktreeId: in.WorktreeID, Path: in.Path})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
	simpleFileOp(r, "files.stat", func(ctx context.Context, in readArgs) (any, error) {
		resp, err := client.StatFile(ctx, &gitgatewayv1.StatFileRequest{WorktreeId: in.WorktreeID, Path: in.Path})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})
	simpleFileOp(r, "files.readDir", func(ctx context.Context, in readArgs) (any, error) {
		resp, err := client.ReadDir(ctx, &gitgatewayv1.ReadDirRequest{WorktreeId: in.WorktreeID, Path: in.Path})
		if err != nil {
			return nil, err
		}
		return resp.GetEntries(), nil
	})

	type readChunkArgs struct {
		WorktreeID  string `json:"worktreeId"`
		Path        string `json:"path"`
		OffsetBytes int64  `json:"offsetBytes"`
		LengthBytes int64  `json:"lengthBytes"`
	}
	simpleFileOp(r, "files.readChunk", func(ctx context.Context, in readChunkArgs) (any, error) {
		resp, err := client.ReadFileChunk(ctx, &gitgatewayv1.ReadFileChunkRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, OffsetBytes: in.OffsetBytes, LengthBytes: in.LengthBytes,
		})
		if err != nil {
			return nil, err // FAILED_PRECONDITION over relay surfaces as-is
		}
		return resp, nil
	})

	type readPreviewArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
		MaxBytes   int64  `json:"maxBytes"`
	}
	simpleFileOp(r, "files.readPreview", func(ctx context.Context, in readPreviewArgs) (any, error) {
		resp, err := client.ReadFilePreview(ctx, &gitgatewayv1.ReadFilePreviewRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, MaxBytes: in.MaxBytes,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// files.write / files.writeBase64 collapse onto WriteFile with an
	// encoding switch, per TASK-049's proto note that this is one RPC with
	// a wire-encoding field, not two RPCs.
	type writeArgs struct {
		WorktreeID    string `json:"worktreeId"`
		Path          string `json:"path"`
		Content       string `json:"content"`
		Base64        bool   `json:"base64"` // true for files.writeBase64
		CreateParents bool   `json:"createParents"`
	}
	writeHandler := func(ctx context.Context, in writeArgs) (any, error) {
		content := []byte(in.Content)
		if in.Base64 {
			decoded, err := base64.StdEncoding.DecodeString(in.Content)
			if err != nil {
				return nil, fmt.Errorf("decoding base64 content: %w", err)
			}
			content = decoded
		}
		resp, err := client.WriteFile(ctx, &gitgatewayv1.WriteFileRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, Content: content, CreateParents: in.CreateParents,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	simpleFileOp(r, "files.write", writeHandler)
	simpleFileOp(r, "files.writeBase64", writeHandler)

	type writeChunkArgs struct {
		WorktreeID  string `json:"worktreeId"`
		Path        string `json:"path"`
		OffsetBytes int64  `json:"offsetBytes"`
		Content     string `json:"content"` // always base64 per files.writeBase64Chunk's contract
		IsFinal     bool   `json:"isFinal"`
	}
	simpleFileOp(r, "files.writeBase64Chunk", func(ctx context.Context, in writeChunkArgs) (any, error) {
		content, err := base64.StdEncoding.DecodeString(in.Content)
		if err != nil {
			return nil, fmt.Errorf("decoding base64 content: %w", err)
		}
		resp, err := client.WriteFileChunk(ctx, &gitgatewayv1.WriteFileChunkRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, OffsetBytes: in.OffsetBytes, Content: content, IsFinal: in.IsFinal,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	type createDirArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
		Recursive  bool   `json:"recursive"`
		NoClobber  bool   `json:"noClobber"` // true for files.createDirNoClobber
	}
	createDirHandler := func(ctx context.Context, in createDirArgs) (any, error) {
		resp, err := client.CreateDir(ctx, &gitgatewayv1.CreateDirRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, Recursive: in.Recursive, NoClobber: in.NoClobber,
		})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	simpleFileOp(r, "files.createDir", createDirHandler)
	simpleFileOp(r, "files.createDirNoClobber", createDirHandler)

	type deleteArgs struct {
		WorktreeID string `json:"worktreeId"`
		Path       string `json:"path"`
		Recursive  bool   `json:"recursive"`
	}
	simpleFileOp(r, "files.delete", func(ctx context.Context, in deleteArgs) (any, error) {
		if _, err := client.DeleteFile(ctx, &gitgatewayv1.DeleteFileRequest{
			WorktreeId: in.WorktreeID, Path: in.Path, Recursive: in.Recursive,
		}); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	})

	type searchArgs struct {
		WorktreeID string `json:"worktreeId"`
		Pattern    string `json:"pattern"`
		IsRegex    bool   `json:"isRegex"`
		PathGlob   string `json:"pathGlob"`
		MaxResults int32  `json:"maxResults"`
	}
	simpleFileOp(r, "files.search", func(ctx context.Context, in searchArgs) (any, error) {
		resp, err := client.SearchFiles(ctx, &gitgatewayv1.SearchFilesRequest{
			WorktreeId: in.WorktreeID, Pattern: in.Pattern, IsRegex: in.IsRegex, PathGlob: in.PathGlob, MaxResults: in.MaxResults,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetMatches(), nil
	})

	type listAllArgs struct {
		WorktreeID string `json:"worktreeId"`
		PathGlob   string `json:"pathGlob"`
		MaxResults int32  `json:"maxResults"`
	}
	simpleFileOp(r, "files.listAll", func(ctx context.Context, in listAllArgs) (any, error) {
		resp, err := client.ListAllFiles(ctx, &gitgatewayv1.ListAllFilesRequest{
			WorktreeId: in.WorktreeID, PathGlob: in.PathGlob, MaxResults: in.MaxResults,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetPaths(), nil
	})

	type listMarkdownArgs struct {
		WorktreeID string `json:"worktreeId"`
		MaxResults int32  `json:"maxResults"`
	}
	simpleFileOp(r, "files.listMarkdownDocuments", func(ctx context.Context, in listMarkdownArgs) (any, error) {
		resp, err := client.ListMarkdownDocuments(ctx, &gitgatewayv1.ListMarkdownDocumentsRequest{
			WorktreeId: in.WorktreeID, MaxResults: in.MaxResults,
		})
		if err != nil {
			return nil, err
		}
		return resp.GetPaths(), nil
	})

	type renameArgs struct {
		WorktreeID string `json:"worktreeId"`
		From       string `json:"fromPath"`
		To         string `json:"toPath"`
	}
	simpleFileOp(r, "files.rename", func(ctx context.Context, in renameArgs) (any, error) {
		resp, err := client.RenameFile(ctx, &gitgatewayv1.RenameFileRequest{WorktreeId: in.WorktreeID, FromPath: in.From, ToPath: in.To})
		if err != nil {
			return nil, err // FAILED_PRECONDITION over relay surfaces as-is
		}
		return resp, nil
	})
	simpleFileOp(r, "files.copy", func(ctx context.Context, in renameArgs) (any, error) {
		resp, err := client.CopyFile(ctx, &gitgatewayv1.CopyFileRequest{WorktreeId: in.WorktreeID, FromPath: in.From, ToPath: in.To})
		if err != nil {
			return nil, err
		}
		return resp, nil
	})

	// ── Always-local bookkeeping — no gRPC call, matches
	// crashReports.getLatestPending's in-process pattern. ────────────
	r.Register("files.commitUpload", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	r.Register("files.unwatch", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
}
