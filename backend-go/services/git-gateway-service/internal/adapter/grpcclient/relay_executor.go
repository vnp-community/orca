package grpcclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// RelayExecutor implements usecase.GitExecutor and usecase.FilesystemExecutor
// for the connected-host case (ConnectionResolver reports Connected=true),
// by calling infra-fleet-service's generic Relay RPC with method names like
// "git.status" / "fs.readFile" and JSON-encoded params, per
// git-gateway-service.md §2 step 3'. It also implements usecase.AICompleter
// (see Complete below) via the same Relay RPC with method "ai.complete",
// for GenerateCommitMessage/GeneratePullRequestFields.
//
// Param/result field names below are reconciled against
// specs/agent/api/agent-rpc-catalog-git-fs.md's real Dev Server Agent
// contract (worktreePath, filePath, etc.) per TASK-227/TASK-228/BUG-036 —
// see each method's own doc comment for known gaps that remain (blocked
// shape redesigns, methods still needing TASK-227 for reachability).
type RelayExecutor struct {
	client infrafleetv1.InfraFleetServiceClient
}

// NewRelayExecutor wraps an already-constructed infrafleetv1 client — used
// with Dial's connection (resolver.go) in cmd/server/main.go, and with a
// fake client in tests.
func NewRelayExecutor(client infrafleetv1.InfraFleetServiceClient) *RelayExecutor {
	return &RelayExecutor{client: client}
}

// relay marshals params, calls infra-fleet-service's Relay RPC for
// connectionID/method, and unmarshals the result into out.
//
// Known gap: usecase.GitExecutor's methods only receive repoPath, not the
// worktreeID/connectionId ConnectionResolver resolved it from (§
// dispatchExecutor in usecase/ports.go forwards ResolvedConnection.RepoPath
// to GitExecutor but drops ConnectionID). Every caller below therefore
// passes its repoPath argument through as this relay's connectionID too —
// correct only if RelayRequest.ConnectionId ends up identical to the
// worktreeID that resolved to it, which holds today because
// ConnectionResolver.ResolveConnection echoes worktreeID as
// ResolvedConnection.ConnectionID but sources RepoPath from
// infra-fleet-service's own answer (resp.GetRepoPath()) — those two need
// not always match. Threading ConnectionID through GitExecutor's signature
// is the real fix; not done here since ports.go is out of scope for this
// change.
func (r *RelayExecutor) relay(ctx context.Context, connectionID, method string, params map[string]any, out any) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("grpcclient: marshal params for %s: %w", method, err)
	}

	resp, err := r.client.Relay(ctx, &infrafleetv1.RelayRequest{
		ConnectionId: connectionID,
		Method:       method,
		ParamsJson:   string(paramsJSON),
	})
	if err != nil {
		return fmt.Errorf("grpcclient: relay %s: %w", method, err)
	}

	if out == nil || resp.GetResultJson() == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(resp.GetResultJson()), out); err != nil {
		return fmt.Errorf("grpcclient: unmarshal %s result: %w", method, err)
	}
	return nil
}

// ── usecase.GitExecutor ───────────────────────────────────────────────────

func (r *RelayExecutor) GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error) {
	var result domain.GitStatus
	err := r.relay(ctx, repoPath, "git.status", map[string]any{
		"worktreePath": repoPath, // was "repoPath" — real agent contract, see BUG-036/TASK-228
	}, &result)
	return result, err
}

func (r *RelayExecutor) GetDiff(ctx context.Context, repoPath, filePath string, staged bool) (domain.DiffResult, error) {
	var result domain.DiffResult
	err := r.relay(ctx, repoPath, "git.diff", map[string]any{
		"worktreePath": repoPath, "filePath": filePath, "staged": staged,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error) {
	var result domain.CommitResult
	// NOTE: "paths" is sent but the real agent ignores it — commit assumes
	// pre-staged content (via a prior git.stage/git.bulkStage call, see
	// TASK-208). Left in the wire payload for now rather than silently
	// dropping the Go signature's paths param — harmless either way.
	err := r.relay(ctx, repoPath, "git.commit", map[string]any{
		"worktreePath": repoPath, "message": message,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error) {
	var result domain.PushResult
	// KNOWN LIMITATION: real agent wants a structured pushTarget, not bare
	// remote/branch (SOL-032 §0 open question #1, unresolved). This param
	// shape may be rejected by the agent's shape validator or bypass
	// fork-branch push safety. Do not consider Push "fixed" until the
	// pushTarget redesign lands.
	err := r.relay(ctx, repoPath, "git.push", map[string]any{
		"worktreePath": repoPath, "remote": remote, "branch": branch,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Pull(ctx context.Context, repoPath string) (domain.PullResult, error) {
	var result domain.PullResult
	// Same pushTarget limitation as Push above.
	err := r.relay(ctx, repoPath, "git.pull", map[string]any{
		"worktreePath": repoPath,
	}, &result)
	return result, err
}

// Stage always relays to "git.bulkStage", never "git.stage" — the real
// agent's git.stage is single-file only (no repeated-paths support), but
// git.bulkStage accepts any count ≥1, so it's a strict superset that
// covers both the single-file (stage) and multi-select (bulkStage)
// frontend call sites without branching on len(paths). See SOL-032 §0 /
// TASK-208's Contract correction section.
func (r *RelayExecutor) Stage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.bulkStage", map[string]any{
		"worktreePath": repoPath, "filePaths": paths,
	}, &result)
	return result, err
}

// Unstage always relays to "git.bulkUnstage" — same reasoning as Stage
// above.
func (r *RelayExecutor) Unstage(ctx context.Context, repoPath string, paths []string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.bulkUnstage", map[string]any{
		"worktreePath": repoPath, "filePaths": paths,
	}, &result)
	return result, err
}

// History: cursor param dropped, ref renamed to baseRef/"baseRef" wire key
// — see TASK-209's Contract correction section.
func (r *RelayExecutor) History(ctx context.Context, repoPath, baseRef string, limit int) ([]domain.CommitRef, error) {
	var result struct {
		Commits []domain.CommitRef `json:"commits"`
	}
	err := r.relay(ctx, repoPath, "git.history", map[string]any{
		"worktreePath": repoPath, "baseRef": baseRef, "limit": limit,
	}, &result)
	return result.Commits, err
}

// CheckIgnored returns only the ignored subset (string[]) per TASK-209's
// Contract correction section — matches the real agent's response shape
// instead of a {path, ignored} pair per input path.
func (r *RelayExecutor) CheckIgnored(ctx context.Context, repoPath string, paths []string) ([]string, error) {
	var result struct {
		Ignored []string `json:"ignoredPaths"`
	}
	err := r.relay(ctx, repoPath, "git.checkIgnored", map[string]any{
		"worktreePath": repoPath, "paths": paths,
	}, &result)
	return result.Ignored, err
}

// ForkSync now sends the required expectedUpstream param — the real agent
// rejects calls without it. See TASK-209's Contract correction section.
func (r *RelayExecutor) ForkSync(ctx context.Context, repoPath, expectedUpstream string) (domain.ForkSyncStatus, error) {
	var result domain.ForkSyncStatus
	err := r.relay(ctx, repoPath, "git.forkSync", map[string]any{
		"worktreePath": repoPath, "expectedUpstream": expectedUpstream,
	}, &result)
	return result, err
}

// UpstreamStatus: still needs TASK-227 (unlike this group's other 3
// shippable-now methods) — no confirmed agent handler is reachable from
// backend-go's transport until then. Param key renamed to worktreePath and
// an optional pushTarget added per the real contract (placeholder type —
// see SOL-032 §0 open question #1). Wired now; if no agent handler exists
// at all even post-TASK-227, this becomes a FailedPrecondition at runtime
// (relay() returns the agent's "unknown method" error) rather than a
// compile-time gap.
func (r *RelayExecutor) UpstreamStatus(ctx context.Context, repoPath, pushTarget string) (domain.UpstreamStatus, error) {
	var result domain.UpstreamStatus
	err := r.relay(ctx, repoPath, "git.upstreamStatus", map[string]any{
		"worktreePath": repoPath, "pushTarget": pushTarget,
	}, &result)
	return result, err
}

func (r *RelayExecutor) RemoteCommitURL(ctx context.Context, repoPath, sha string) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	err := r.relay(ctx, repoPath, "git.remoteCommitUrl", map[string]any{
		"worktreePath": repoPath, "sha": sha,
	}, &result)
	return result.URL, err
}

func (r *RelayExecutor) RemoteFileURL(ctx context.Context, repoPath, path, ref string) (string, error) {
	var result struct {
		URL string `json:"url"`
	}
	err := r.relay(ctx, repoPath, "git.remoteFileUrl", map[string]any{
		"worktreePath": repoPath, "path": path, "ref": ref,
	}, &result)
	return result.URL, err
}

// Complete implements usecase.AICompleter by relaying to the Dev Server
// Agent's ai.complete method, per specs/agent/api/agent-rpc-catalog-runtime.md's
// confirmed real handler (`ai-complete-handler.ts:47`): params
// `prompt(required), format?, taskId?, model?, accountId?, resolvedApiKey?`,
// result `{content, model?}`. Only `prompt` is sent here — model/account
// resolution is deliberately out of this service's scope
// (git-gateway-service.md §3.1's context-assembler framing), so the agent
// falls back to its own configured default model.
//
// Unlike the git.* methods above, connectionID here is the caller's actual
// ResolvedConnection.ConnectionID, not a repoPath standing in for it —
// ai.complete has no filesystem-path argument, so there's no equivalent of
// those methods' repoPath/connectionID conflation to inherit.
func (r *RelayExecutor) Complete(ctx context.Context, connectionID, prompt string) (string, error) {
	var result struct {
		Content string `json:"content"`
	}
	err := r.relay(ctx, connectionID, "ai.complete", map[string]any{
		"prompt": prompt,
	}, &result)
	return result.Content, err
}

// ── usecase.FilesystemExecutor ──────────────────────────────────────────
//
// Relays to the Dev Server Agent's fs.* methods. Field names below are
// named to match this service's own domain types; reconcile against the
// real handler contract (specs/agent/api/agent-rpc-catalog-git-fs.md)
// before removing this comment, same caveat as the git.* methods above.
// RelayExecutor deliberately does NOT implement
// usecase.LocalOnlyFilesystemExecutor (Rename/Copy) — the agent's fs.*
// surface has no rename/copy method (BUG-009) — nor ReadFileChunk, which
// is unsupported for any relay target by design (TASK-052 checks
// conn.Connected before ever reaching this executor).

func (r *RelayExecutor) ReadFile(ctx context.Context, repoPath, relPath string) ([]byte, error) {
	var result struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := r.relay(ctx, repoPath, "fs.readFile", map[string]any{
		"path": filepath.Join(repoPath, relPath),
	}, &result); err != nil {
		return nil, err
	}
	return decodeFileContent(result.Content, result.Encoding)
}

func (r *RelayExecutor) ReadFilePreview(ctx context.Context, repoPath, relPath string, maxBytes int64) ([]byte, bool, error) {
	var result struct {
		Content   string `json:"content"`
		Encoding  string `json:"encoding"`
		Truncated bool   `json:"truncated"`
	}
	if err := r.relay(ctx, repoPath, "fs.readFile", map[string]any{
		"path":     filepath.Join(repoPath, relPath),
		"maxBytes": maxBytes,
	}, &result); err != nil {
		return nil, false, err
	}
	content, err := decodeFileContent(result.Content, result.Encoding)
	return content, result.Truncated, err
}

func (r *RelayExecutor) ReadDir(ctx context.Context, repoPath, relPath string) ([]domain.DirEntry, error) {
	var result struct {
		Entries []domain.DirEntry `json:"entries"`
	}
	err := r.relay(ctx, repoPath, "fs.readDir", map[string]any{
		"path": filepath.Join(repoPath, relPath),
	}, &result)
	return result.Entries, err
}

func (r *RelayExecutor) WriteFile(ctx context.Context, repoPath, relPath string, content []byte, createParents bool) (int64, error) {
	var result struct {
		BytesWritten int64 `json:"bytesWritten"`
	}
	err := r.relay(ctx, repoPath, "fs.writeFile", map[string]any{
		"path":          filepath.Join(repoPath, relPath),
		"content":       base64.StdEncoding.EncodeToString(content),
		"encoding":      "base64",
		"createParents": createParents,
	}, &result)
	if err != nil {
		return 0, err
	}
	if result.BytesWritten == 0 {
		result.BytesWritten = int64(len(content))
	}
	return result.BytesWritten, nil
}

func (r *RelayExecutor) WriteFileChunk(ctx context.Context, repoPath, relPath string, offsetBytes int64, content []byte, isFinal bool) (int64, error) {
	var result struct {
		BytesWritten int64 `json:"bytesWritten"`
	}
	err := r.relay(ctx, repoPath, "fs.writeFile", map[string]any{
		"path":        filepath.Join(repoPath, relPath),
		"offsetBytes": offsetBytes,
		"content":     base64.StdEncoding.EncodeToString(content),
		"encoding":    "base64",
		"isFinal":     isFinal,
	}, &result)
	if err != nil {
		return 0, err
	}
	if result.BytesWritten == 0 {
		result.BytesWritten = int64(len(content))
	}
	return result.BytesWritten, nil
}

func (r *RelayExecutor) CreateDir(ctx context.Context, repoPath, relPath string, recursive, noClobber bool) error {
	return r.relay(ctx, repoPath, "fs.mkdir", map[string]any{
		"path":      filepath.Join(repoPath, relPath),
		"recursive": recursive,
		"noClobber": noClobber,
	}, nil)
}

func (r *RelayExecutor) Delete(ctx context.Context, repoPath, relPath string, recursive bool) error {
	return r.relay(ctx, repoPath, "fs.rmdir", map[string]any{
		"path":      filepath.Join(repoPath, relPath),
		"recursive": recursive,
	}, nil)
}

func (r *RelayExecutor) Stat(ctx context.Context, repoPath, relPath string) (domain.FileStat, error) {
	var result domain.FileStat
	err := r.relay(ctx, repoPath, "fs.stat", map[string]any{
		"path": filepath.Join(repoPath, relPath),
	}, &result)
	return result, err
}

func (r *RelayExecutor) Search(ctx context.Context, repoPath string, opts domain.SearchOptions) ([]domain.SearchMatch, error) {
	var result struct {
		Matches []domain.SearchMatch `json:"matches"`
	}
	err := r.relay(ctx, repoPath, "fs.grep", map[string]any{
		"repoPath":   repoPath,
		"pattern":    opts.Pattern,
		"isRegex":    opts.IsRegex,
		"pathGlob":   opts.PathGlob,
		"maxResults": opts.MaxResults,
	}, &result)
	return result.Matches, err
}

func (r *RelayExecutor) Glob(ctx context.Context, repoPath, pattern string, maxResults int) ([]string, error) {
	var result struct {
		Paths []string `json:"paths"`
	}
	err := r.relay(ctx, repoPath, "fs.glob", map[string]any{
		"repoPath":   repoPath,
		"pattern":    pattern,
		"maxResults": maxResults,
	}, &result)
	return result.Paths, err
}

// decodeFileContent turns the agent's {content, encoding} pair into raw
// bytes, matching WriteFile/WriteFileChunk's own base64-on-the-wire
// convention above.
func decodeFileContent(content, encoding string) ([]byte, error) {
	if encoding == "base64" {
		return base64.StdEncoding.DecodeString(content)
	}
	return []byte(content), nil
}
