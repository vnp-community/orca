package grpcclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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

// relayStream is relay's server-streaming counterpart (TASK-PW-03-08,
// SOL-PW-03) — calls infra-fleet-service's RelayStream RPC and forwards
// each decoded frame to sink until a stream.end-typed frame is observed (or
// the stream ends without one, treated as a clean nil-error completion —
// mirrors devserveragent.Client.ExecStream's own "channel closed = done"
// contract on this method's other end of the relay).
func (r *RelayExecutor) relayStream(ctx context.Context, connectionID, method string, params map[string]any, sink func(domain.GitProgressLine) error) error {
	ctx, err := withTenantMetadata(ctx)
	if err != nil {
		return err
	}

	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("grpcclient: marshal params for %s: %w", method, err)
	}

	stream, err := r.client.RelayStream(ctx, &infrafleetv1.RelayStreamRequest{
		ConnectionId: connectionID,
		Method:       method,
		ParamsJson:   string(paramsJSON),
	})
	if err != nil {
		return fmt.Errorf("grpcclient: relay stream %s: %w", method, err)
	}

	for {
		frame, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("grpcclient: relay stream %s: %w", method, err)
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(frame.GetFrameJson()), &raw); err != nil {
			continue // malformed frame — skip rather than abort the whole stream
		}
		line := decodeGitProgressFrame(raw)
		if err := sink(line); err != nil {
			return err
		}
		if line.IsFinal {
			return nil
		}
	}
}

// decodeGitProgressFrame decodes one RelayStreamFrame.frame_json payload —
// the agent's git.execStream shape ({type:'stream.chunk',line,source?} /
// {type:'stream.end',exitCode}, specs/agent/api/agent-rpc-catalog-git-fs.md)
// — into domain.GitProgressLine. Any frame not typed "stream.end" is
// treated as a stream.chunk line, matching devserveragent.Client.ExecStream's
// own best-effort tolerance for the FLAGGED/unconfirmed exact field names.
func decodeGitProgressFrame(raw map[string]any) domain.GitProgressLine {
	frameType, _ := raw["type"].(string)
	if frameType != "stream.end" {
		line, _ := raw["line"].(string)
		source, _ := raw["source"].(string)
		return domain.GitProgressLine{Line: line, Source: source}
	}
	exitCode, _ := raw["exitCode"].(float64)
	return domain.GitProgressLine{
		IsFinal:  true,
		ExitCode: int32(exitCode),
		Success:  exitCode == 0,
	}
}

// ── usecase.StreamingGitExecutor ─────────────────────────────────────────

// PushStream relays to the agent's git.execStream with a `git push`
// argv — same remote/branch argument-building rule as Push above, plus the
// same pushTarget shape limitation noted there.
func (r *RelayExecutor) PushStream(ctx context.Context, repoPath, remote, branch string, sink func(domain.GitProgressLine) error) error {
	args := []string{"push"}
	if remote != "" {
		args = append(args, remote)
		if branch != "" {
			args = append(args, branch)
		}
	}
	return r.relayStream(ctx, repoPath, "git.execStream", map[string]any{"args": args, "cwd": repoPath}, sink)
}

// PullStream relays to the agent's git.execStream with a `git pull` argv.
func (r *RelayExecutor) PullStream(ctx context.Context, repoPath string, sink func(domain.GitProgressLine) error) error {
	return r.relayStream(ctx, repoPath, "git.execStream", map[string]any{"args": []string{"pull"}, "cwd": repoPath}, sink)
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

// CreateWorktree/RemoveWorktree/FetchAndResolveRef/ListWorktreePaths below
// follow this file's existing relay(...) helper pattern exactly (SOL-031 /
// TASK-193). Same best-effort-param-shape caveat this file's doc comment
// already states for git.status/git.diff/etc. applies here — git.worktreeAdd/
// git.worktreeRemove/git.fetchRef/git.worktreeList are not verified against
// a real Dev Server Agent handler; reconcile before removing this note.

func (r *RelayExecutor) CreateWorktree(ctx context.Context, repoPath, branch, baseRef, targetPath string) (domain.WorktreeCreateResult, error) {
	var result domain.WorktreeCreateResult
	err := r.relay(ctx, repoPath, "git.worktreeAdd", map[string]any{
		"repoPath": repoPath, "branch": branch, "baseRef": baseRef, "targetPath": targetPath,
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
// pushTarget is now the real, structured PushTargetInput (TASK-207's type,
// SOL-032 §0 open question #1 resolved) instead of TASK-209's original
// placeholder string. Wired now; if no agent handler exists at all even
// post-TASK-227, this becomes a FailedPrecondition at runtime (relay()
// returns the agent's "unknown method" error) rather than a compile-time
// gap.
func (r *RelayExecutor) UpstreamStatus(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.UpstreamStatus, error) {
	var result domain.UpstreamStatus
	params := map[string]any{"worktreePath": repoPath}
	if pt := pushTargetParam(pushTarget); pt != nil {
		params["pushTarget"] = pt
	}
	err := r.relay(ctx, repoPath, "git.upstreamStatus", params, &result)
	return result, err
}

// Fetch relays to the real agent's git.fetch
// (specs/agent/api/agent-rpc-catalog-git-fs.md:155): always prunes
// (`git fetch --prune [remote]`), no separate prune flag — needed TASK-227
// (reachability) and PushTargetInput (both now resolved) per TASK-210's own
// Contract correction.
func (r *RelayExecutor) Fetch(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	params := map[string]any{"worktreePath": repoPath}
	if pt := pushTargetParam(pushTarget); pt != nil {
		params["pushTarget"] = pt
	}
	err := r.relay(ctx, repoPath, "git.fetch", params, &result)
	return result, err
}

// gitChangeEntryWire mirrors parseBranchDiff's {path, status, oldPath?,
// added?, removed?} entry shape on the wire
// (agent/src/relay/git-handler-utils.ts:107-134) — shared by
// CommitCompare/BranchCompare below.
type gitChangeEntryWire struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	OldPath string `json:"oldPath"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

func toDomainChangeEntries(entries []gitChangeEntryWire) []domain.GitChangeEntry {
	out := make([]domain.GitChangeEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, domain.GitChangeEntry{
			Path: e.Path, Status: e.Status, OldPath: e.OldPath, Added: e.Added, Removed: e.Removed,
		})
	}
	return out
}

// CommitCompare relays to the real agent's git.commitCompare exactly:
// worktreePath + commitId, response {summary{...}, entries[]} — matches
// commitCompareOp's real shape (agent/src/relay/git-handler-commit-diff-ops.ts:15-122,
// specs/agent/api/agent-rpc-catalog-git-fs.md:50/144), NOT TASK-209's
// original two-arbitrary-commits (baseSha/headSha) design.
func (r *RelayExecutor) CommitCompare(ctx context.Context, repoPath, commitID string) (domain.CommitCompareResult, error) {
	var result struct {
		Summary struct {
			CommitOID    string `json:"commitOid"`
			ParentOID    string `json:"parentOid"`
			CompareRef   string `json:"compareRef"`
			BaseRef      string `json:"baseRef"`
			ChangedFiles int    `json:"changedFiles"`
			Status       string `json:"status"`
			ErrorMessage string `json:"errorMessage"`
		} `json:"summary"`
		Entries []gitChangeEntryWire `json:"entries"`
	}
	err := r.relay(ctx, repoPath, "git.commitCompare", map[string]any{
		"worktreePath": repoPath, "commitId": commitID,
	}, &result)
	if err != nil {
		return domain.CommitCompareResult{}, err
	}
	return domain.CommitCompareResult{
		CommitOID: result.Summary.CommitOID, ParentOID: result.Summary.ParentOID,
		CompareRef: result.Summary.CompareRef, BaseRef: result.Summary.BaseRef,
		ChangedFiles: result.Summary.ChangedFiles, Status: result.Summary.Status,
		ErrorMessage: result.Summary.ErrorMessage, Entries: toDomainChangeEntries(result.Entries),
	}, nil
}

// BranchCompare relays to the real agent's git.branchCompare exactly:
// worktreePath + baseRef, response {summary{...}, entries[]} — matches
// branchCompareOp's real shape (agent/src/relay/git-handler-ops.ts:124-214,
// specs/agent/api/agent-rpc-catalog-git-fs.md:49/143), NOT TASK-209's
// original two-arbitrary-branches (baseBranch/headBranch) design.
func (r *RelayExecutor) BranchCompare(ctx context.Context, repoPath, baseRef string) (domain.BranchCompareResult, error) {
	var result struct {
		Summary struct {
			BaseRef      string `json:"baseRef"`
			BaseOID      string `json:"baseOid"`
			CompareRef   string `json:"compareRef"`
			HeadOID      string `json:"headOid"`
			MergeBase    string `json:"mergeBase"`
			ChangedFiles int    `json:"changedFiles"`
			CommitsAhead int    `json:"commitsAhead"`
			Status       string `json:"status"`
			ErrorMessage string `json:"errorMessage"`
		} `json:"summary"`
		Entries []gitChangeEntryWire `json:"entries"`
	}
	err := r.relay(ctx, repoPath, "git.branchCompare", map[string]any{
		"worktreePath": repoPath, "baseRef": baseRef,
	}, &result)
	if err != nil {
		return domain.BranchCompareResult{}, err
	}
	return domain.BranchCompareResult{
		BaseRef: result.Summary.BaseRef, BaseOID: result.Summary.BaseOID,
		CompareRef: result.Summary.CompareRef, HeadOID: result.Summary.HeadOID,
		MergeBase: result.Summary.MergeBase, ChangedFiles: result.Summary.ChangedFiles,
		CommitsAhead: result.Summary.CommitsAhead, Status: result.Summary.Status,
		ErrorMessage: result.Summary.ErrorMessage, Entries: toDomainChangeEntries(result.Entries),
	}, nil
}

// fileDiffResultWire mirrors buildDiffResult's response shape
// (agent/src/relay/git-diff-result.ts:5-38) — shared by CommitDiff/
// BranchDiff below. Binary content is intentionally not decoded here even
// when the agent sends a base64 preview payload — see
// localgit.buildFileDiffResult's doc comment for why (raw binary can't
// round-trip through this service's proto string fields).
type fileDiffResultWire struct {
	Kind             string `json:"kind"`
	OriginalContent  string `json:"originalContent"`
	ModifiedContent  string `json:"modifiedContent"`
	OriginalIsBinary bool   `json:"originalIsBinary"`
	ModifiedIsBinary bool   `json:"modifiedIsBinary"`
}

func toDomainFileDiff(w fileDiffResultWire) domain.FileDiffResult {
	if w.OriginalIsBinary || w.ModifiedIsBinary {
		return domain.FileDiffResult{Kind: "binary", OriginalIsBinary: w.OriginalIsBinary, ModifiedIsBinary: w.ModifiedIsBinary}
	}
	return domain.FileDiffResult{Kind: w.Kind, OriginalContent: w.OriginalContent, ModifiedContent: w.ModifiedContent}
}

// CommitDiff relays to the real agent's git.commitDiff exactly: worktreePath
// + commitOid + optional parentOid + REQUIRED filePath + optional oldPath,
// response a single diff-result object — matches commitDiffEntry's real
// shape (agent/src/relay/git-handler-commit-diff-ops.ts:124-160,
// specs/agent/api/agent-rpc-catalog-git-fs.md:52/146). Same class of
// per-file fix as GetDiff's own TASK-228 correction — TASK-209's original
// design had no filePath at all and assumed a whole-commit diff.
func (r *RelayExecutor) CommitDiff(ctx context.Context, repoPath, commitOID, parentOID, filePath, oldPath string) (domain.FileDiffResult, error) {
	params := map[string]any{"worktreePath": repoPath, "commitOid": commitOID, "filePath": filePath}
	if parentOID != "" {
		params["parentOid"] = parentOID
	}
	if oldPath != "" {
		params["oldPath"] = oldPath
	}
	var result fileDiffResultWire
	err := r.relay(ctx, repoPath, "git.commitDiff", params, &result)
	if err != nil {
		return domain.FileDiffResult{}, err
	}
	return toDomainFileDiff(result), nil
}

// BranchDiff relays to the real agent's git.branchDiff exactly: worktreePath
// + baseRef + REQUIRED filePath + optional oldPath, response a diff-result
// object — matches branchDiffEntries' real shape
// (agent/src/relay/git-handler-ops.ts:218-288,
// specs/agent/api/agent-rpc-catalog-git-fs.md:51/145). Same per-file fix as
// CommitDiff, plus the same base-ref-only-vs-two-sided fix as BranchCompare.
// includePatch is always sent true — without it the real agent returns
// empty-content placeholder entries (agent-git-handler-ops.ts:259-267),
// which this per-file RPC has no use for.
func (r *RelayExecutor) BranchDiff(ctx context.Context, repoPath, baseRef, filePath, oldPath string) (domain.FileDiffResult, error) {
	params := map[string]any{
		"worktreePath": repoPath, "baseRef": baseRef, "filePath": filePath, "includePatch": true,
	}
	if oldPath != "" {
		params["oldPath"] = oldPath
	}
	var result fileDiffResultWire
	err := r.relay(ctx, repoPath, "git.branchDiff", params, &result)
	if err != nil {
		return domain.FileDiffResult{}, err
	}
	return toDomainFileDiff(result), nil
}

// SubmoduleStatus relays to the real agent's git.submoduleStatus exactly:
// worktreePath + submodulePath + optional area, response a GitStatus-shaped
// object — matches handleGitSubmoduleStatus's real per-submodule shape
// (agent/src/relay/agent-git-handler-extended.ts:196-230,
// specs/agent/api/agent-rpc-catalog-git-fs.md:55/123), NOT TASK-209's
// original "list every submodule" design (SOL-032 §0 open question #3,
// resolved — see GitExecutor.SubmoduleStatus's doc comment for the
// frontend-caller citation that closes this question).
func (r *RelayExecutor) SubmoduleStatus(ctx context.Context, repoPath, submodulePath, area string) (domain.GitStatus, error) {
	params := map[string]any{"worktreePath": repoPath, "submodulePath": submodulePath}
	if area != "" {
		params["area"] = area
	}
	var result domain.GitStatus
	err := r.relay(ctx, repoPath, "git.submoduleStatus", params, &result)
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

// Clone and InitRepo have no repoPath/connectionId yet (they create the
// worktree) — destPath doubles as the relay's connectionID, consistent
// with this file's existing "repoPath doubles as connectionId" convention
// (see this file's doc comment's "Known gap").
func (r *RelayExecutor) Clone(ctx context.Context, url, destPath string) (string, string, error) {
	var result struct {
		WorktreePath  string `json:"worktreePath"`
		DefaultBranch string `json:"defaultBranch"`
	}
	err := r.relay(ctx, destPath, "git.clone", map[string]any{
		"url": url, "destPath": destPath,
	}, &result)
	return result.WorktreePath, result.DefaultBranch, err
}

func (r *RelayExecutor) InitRepo(ctx context.Context, destPath, defaultBranch string) (string, string, error) {
	var result struct {
		Path          string `json:"path"`
		DefaultBranch string `json:"defaultBranch"`
	}
	err := r.relay(ctx, destPath, "git.init", map[string]any{
		"destPath": destPath, "defaultBranch": defaultBranch,
	}, &result)
	return result.Path, result.DefaultBranch, err
}

func (r *RelayExecutor) BaseRefDefault(ctx context.Context, repoPath string) (string, error) {
	var result struct {
		Ref string `json:"ref"`
	}
	err := r.relay(ctx, repoPath, "git.baseRefDefault", map[string]any{"repoPath": repoPath}, &result)
	return result.Ref, err
}

func (r *RelayExecutor) SearchRefs(ctx context.Context, repoPath, query string) ([]string, error) {
	var result struct {
		Refs []string `json:"refs"`
	}
	err := r.relay(ctx, repoPath, "git.searchRefs", map[string]any{"repoPath": repoPath, "query": query}, &result)
	return result.Refs, err
}

func (r *RelayExecutor) CheckHooks(ctx context.Context, repoPath string) ([]string, bool, error) {
	var result struct {
		InstalledHooks   []string `json:"installedHooks"`
		OrcaHooksCurrent bool     `json:"orcaHooksCurrent"`
	}
	err := r.relay(ctx, repoPath, "git.checkHooks", map[string]any{"repoPath": repoPath}, &result)
	return result.InstalledHooks, result.OrcaHooksCurrent, err
}

func (r *RelayExecutor) ReadIssueCommand(ctx context.Context, repoPath string) (string, bool, error) {
	var result struct {
		Content string `json:"content"`
		Exists  bool   `json:"exists"`
	}
	err := r.relay(ctx, repoPath, "git.readIssueCommand", map[string]any{"repoPath": repoPath}, &result)
	return result.Content, result.Exists, err
}

func (r *RelayExecutor) WriteIssueCommand(ctx context.Context, repoPath, content string) error {
	return r.relay(ctx, repoPath, "git.writeIssueCommand", map[string]any{"repoPath": repoPath, "content": content}, nil)
}

func (r *RelayExecutor) ScanSetupScriptImports(ctx context.Context, repoPath string) ([]string, error) {
	var result struct {
		ImportedPaths []string `json:"importedPaths"`
	}
	err := r.relay(ctx, repoPath, "git.scanSetupScriptImports", map[string]any{"repoPath": repoPath}, &result)
	return result.ImportedPaths, err
}

func (r *RelayExecutor) RemoveWorktree(ctx context.Context, worktreePath string, force bool) error {
	return r.relay(ctx, worktreePath, "git.worktreeRemove", map[string]any{
		"worktreePath": worktreePath, "force": force,
	}, nil)
}

func (r *RelayExecutor) FetchAndResolveRef(ctx context.Context, repoPath, ref string) (string, error) {
	var result struct {
		SHA string `json:"sha"`
	}
	err := r.relay(ctx, repoPath, "git.fetchRef", map[string]any{
		"repoPath": repoPath, "ref": ref,
	}, &result)
	return result.SHA, err
}

func (r *RelayExecutor) ListWorktreePaths(ctx context.Context, repoPath string) ([]string, error) {
	var result struct {
		Paths []string `json:"paths"`
	}
	err := r.relay(ctx, repoPath, "git.worktreeList", map[string]any{
		"repoPath": repoPath,
	}, &result)
	return result.Paths, err
}

// ForceDeleteBranch implements the REQUIRED (TASK-194) GitExecutor method —
// see domain.ErrForceDeleteBranchUnsupported's doc comment for why the
// operational-fallback sentinel lives in internal/domain rather than here.
func (r *RelayExecutor) ForceDeleteBranch(ctx context.Context, repoPath, branch string) error {
	err := r.relay(ctx, repoPath, "git.forceDeleteBranch", map[string]any{
		"repoPath": repoPath, "branch": branch,
	}, nil)
	if err != nil && isMethodNotFoundError(err) {
		return fmt.Errorf("%w: %v", domain.ErrForceDeleteBranchUnsupported, err)
	}
	return err
}

// ── Group A — branch/ref operations (TASK-207) ─────────────────────────────
//
// Checkout/ListLocalBranches/FastForward/ConflictOperation below were
// redesigned against the real agent contract
// (specs/agent/api/agent-rpc-catalog-git-fs.md) rather than implemented as
// TASK-207's original sketch — see each method's own doc comment for
// citations. RebaseFromBase/AbortRebase/AbortMerge/Discard/BulkDiscard are
// the mechanical param-rename-only subset that sketch got right.

// Checkout relays to the real agent's git.checkout exactly: worktreePath +
// branch only, no create-branch semantics (agent-rpc-catalog-git-fs.md:132,
// agent/src/relay/git-handler.ts:702-719).
func (r *RelayExecutor) Checkout(ctx context.Context, repoPath, branch string) (domain.CheckoutResult, error) {
	var result domain.CheckoutResult
	err := r.relay(ctx, repoPath, "git.checkout", map[string]any{
		"worktreePath": repoPath, "branch": branch,
	}, &result)
	return result, err
}

// gitExecResult mirrors the real agent's git.exec response shape
// (agent-rpc-catalog-git-fs.md:180: "{stdout,stderr}") — used by
// ListLocalBranches below to compose a richer branch listing than the real
// agent's own git.localBranches RPC provides.
type gitExecResult struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}

// ListLocalBranches composes a richer per-branch listing (upstream/ahead/
// behind, not just names) than the real agent's own git.localBranches RPC
// returns ({current, branches[]} — names only, agent-rpc-catalog-git-fs.md:133,
// agent/src/relay/git-handler.ts:721-744). It does this via git.exec's
// for-each-ref subcommand instead: confirmed whitelisted with no extra
// restriction on Part B's exec whitelist
// (agent-rpc-catalog-git-fs.md:203-206) — the whitelist RelayExecutor's
// SSH-relay calls actually reach (Part B, not Part A's separate, broader
// git.exec surface). Same --format string as localgit.Executor's
// ListLocalBranches for parser parity between the two implementations.
func (r *RelayExecutor) ListLocalBranches(ctx context.Context, repoPath string) ([]domain.BranchInfo, error) {
	var result gitExecResult
	err := r.relay(ctx, repoPath, "git.exec", map[string]any{
		"args": []string{
			"for-each-ref", "--format=%(refname:short)\t%(upstream:short)\t%(upstream:track)\t%(HEAD)",
			"refs/heads/",
		},
		"cwd": repoPath,
	}, &result)
	if err != nil {
		return nil, err
	}
	return parseForEachRefBranches(result.Stdout), nil
}

// parseForEachRefBranches parses ListLocalBranches' for-each-ref --format
// output — same field layout as localgit.Executor.ListLocalBranches, kept
// as a separate copy in this package (not shared) since the two
// implementations sit in different packages and this parsing is small.
func parseForEachRefBranches(out string) []domain.BranchInfo {
	var branches []domain.BranchInfo
	// Split raw output (not TrimSpace(out)) before trimming each line — see
	// localgit.Executor.ListLocalBranches' identical comment: TrimSpace on
	// the whole blob would eat a legitimate trailing empty %(HEAD) field
	// whenever the alphabetically-last ref isn't the current branch.
	for _, rawLine := range strings.Split(out, "\n") {
		line := strings.TrimRight(rawLine, "\r")
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			continue
		}
		ahead, behind := parseAheadBehindTrack(fields[2])
		branches = append(branches, domain.BranchInfo{
			Name:      fields[0],
			Upstream:  fields[1],
			Ahead:     ahead,
			Behind:    behind,
			IsCurrent: fields[3] == "*",
		})
	}
	return branches
}

// parseAheadBehindTrack parses %(upstream:track)'s "[ahead N, behind M]"
// (or "[ahead N]" / "[behind M]" / "") format.
func parseAheadBehindTrack(track string) (ahead, behind int) {
	track = strings.Trim(track, "[]")
	for _, part := range strings.Split(track, ",") {
		part = strings.TrimSpace(part)
		var n int
		if _, err := fmt.Sscanf(part, "ahead %d", &n); err == nil {
			ahead = n
		}
		if _, err := fmt.Sscanf(part, "behind %d", &n); err == nil {
			behind = n
		}
	}
	return ahead, behind
}

// pushTargetParam converts an optional domain.PushTargetInput to the wire
// map the real agent's GitPushTarget shape expects
// (agent/src/shared/types.ts:551-557) — nil means omit the field entirely,
// matching resolveRelayPushTarget's undefined-pushTarget branch
// (agent/src/relay/git-handler-push-target.ts:164-166).
func pushTargetParam(pushTarget *domain.PushTargetInput) any {
	if pushTarget == nil {
		return nil
	}
	m := map[string]any{
		"remoteName": pushTarget.RemoteName,
		"branchName": pushTarget.BranchName,
	}
	if pushTarget.RemoteURL != "" {
		m["remoteUrl"] = pushTarget.RemoteURL
	}
	if pushTarget.RemoteCreated {
		m["remoteCreated"] = pushTarget.RemoteCreated
	}
	return m
}

// FastForward relays to the real agent's git.fastForward
// (`pullWithArgs(['--ff-only'])`, agent-rpc-catalog-git-fs.md:160,
// agent/src/relay/git-handler.ts:1190-1192), which takes the same optional
// structured pushTarget as push/pull/fetch (SOL-032 §0 open question #1) —
// not TASK-207's original plain branch-string sketch. pushTarget == nil
// omits the field so the agent resolves the worktree's configured push
// target itself.
func (r *RelayExecutor) FastForward(ctx context.Context, repoPath string, pushTarget *domain.PushTargetInput) (domain.FastForwardResult, error) {
	var result domain.FastForwardResult
	params := map[string]any{"worktreePath": repoPath}
	if pt := pushTargetParam(pushTarget); pt != nil {
		params["pushTarget"] = pt
	}
	err := r.relay(ctx, repoPath, "git.fastForward", params, &result)
	return result, err
}

func (r *RelayExecutor) RebaseFromBase(ctx context.Context, repoPath, baseRef string) (domain.RebaseResult, error) {
	var result domain.RebaseResult
	err := r.relay(ctx, repoPath, "git.rebaseFromBase", map[string]any{
		"worktreePath": repoPath, "baseRef": baseRef,
	}, &result)
	return result, err
}

func (r *RelayExecutor) AbortRebase(ctx context.Context, repoPath string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.abortRebase", map[string]any{"worktreePath": repoPath}, &result)
	return result, err
}

func (r *RelayExecutor) AbortMerge(ctx context.Context, repoPath string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.abortMerge", map[string]any{"worktreePath": repoPath}, &result)
	return result, err
}

// MergeBranch relays to "git.merge" — following this file's existing
// relay(...) helper pattern (see CreateWorktree above). Flagged as
// unverified against a real Dev Server Agent handler, matching this file's
// own existing doc-comment caveat for CreateWorktree/RemoveWorktree/etc.
func (r *RelayExecutor) MergeBranch(ctx context.Context, repoPath, branch, strategy, commitMessage string) (domain.MergeResult, error) {
	var result domain.MergeResult
	err := r.relay(ctx, repoPath, "git.merge", map[string]any{
		"repoPath": repoPath, "branch": branch, "strategy": strategy, "commitMessage": commitMessage,
	}, &result)
	return result, err
}

// ConflictOperation relays to the real agent's git.conflictOperation
// exactly: worktreePath only, response is the bare operation string
// ("merge"/"rebase"/"cherry-pick"/"unknown" —
// agent-rpc-catalog-git-fs.md:136, agent/src/relay/git-handler.ts:886-889).
// See ResolveConflict below for the per-file resolve op TASK-207's
// original sketch conflated with this one.
func (r *RelayExecutor) ConflictOperation(ctx context.Context, repoPath string) (string, error) {
	var result string
	err := r.relay(ctx, repoPath, "git.conflictOperation", map[string]any{"worktreePath": repoPath}, &result)
	return result, err
}

// ResolveConflict has no real agent RPC to relay to: Part B's git.exec
// whitelist — the whitelist this relay-connected (SSH) path actually
// reaches — explicitly excludes both `checkout` and `add`
// (agent-rpc-catalog-git-fs.md:203-227's "Not allowed at all" list), so
// there is no whitelisted way to compose ours/theirs/markResolved
// remotely. Returns domain.ErrConflictResolveUnsupportedOverRelay directly
// (no RPC round-trip attempted — this is a known, static limitation, not a
// runtime failure to probe for) — same operational-fallback shape as
// ForceDeleteBranch's domain.ErrForceDeleteBranchUnsupported. Only
// localgit.Executor.ResolveConflict does real work.
func (r *RelayExecutor) ResolveConflict(ctx context.Context, repoPath, path, operation string) (domain.SimpleResult, error) {
	return domain.SimpleResult{}, domain.ErrConflictResolveUnsupportedOverRelay
}

func (r *RelayExecutor) Discard(ctx context.Context, repoPath, filePath string) (domain.SimpleResult, error) {
	var result domain.SimpleResult
	err := r.relay(ctx, repoPath, "git.discard", map[string]any{
		"worktreePath": repoPath, "filePath": filePath,
	}, &result)
	return result, err
}

func (r *RelayExecutor) BulkDiscard(ctx context.Context, repoPath string, filePaths []string) (domain.BulkDiscardResult, error) {
	var result domain.BulkDiscardResult
	err := r.relay(ctx, repoPath, "git.bulkDiscard", map[string]any{
		"worktreePath": repoPath, "filePaths": filePaths,
	}, &result)
	return result, err
}

// isMethodNotFoundError is a placeholder heuristic for detecting an
// agent's "unknown method" response through the Relay RPC's error path —
// FLAGGED: confirm the real error shape RelayResponse/infra-fleet-service's
// Relay RPC surfaces for an unsupported agent method before finalizing;
// this may need to check a gRPC status code (codes.Unimplemented) rather
// than string-matching, depending on how infra-fleet-service's Relay
// usecase translates an agent-side JSON-RPC "method not found" today.
func isMethodNotFoundError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "method not found") || strings.Contains(msg, "unknown method")
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

// ReadDir relays via the agent's fs.readDir, whose FileTreeNode entry shape
// (agent/src/relay/fs-agent-extensions.ts:27-32) is {name, type:'file'|
// 'directory', size?} — neither field name matches domain.DirEntry's own
// json tags (`isDirectory`/`sizeBytes`), so this uses an explicit
// intermediate shape rather than the generic tag-based unmarshal
// TASK-PW-02-04 otherwise expected to "just work". SOL-PW-02: size is now
// threaded from the agent's real `size` field.
func (r *RelayExecutor) ReadDir(ctx context.Context, repoPath, relPath string) ([]domain.DirEntry, error) {
	var result struct {
		Entries []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Size int64  `json:"size"`
		} `json:"entries"`
	}
	err := r.relay(ctx, repoPath, "fs.readDir", map[string]any{
		"path": filepath.Join(repoPath, relPath),
	}, &result)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DirEntry, 0, len(result.Entries))
	for _, e := range result.Entries {
		out = append(out, domain.DirEntry{Name: e.Name, IsDirectory: e.Type == "directory", SizeBytes: e.Size})
	}
	return out, nil
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

// ── SOL-PW-03 — merge/stash/branch-write. Only reachable when the target
// connection is Part A (relay-websocket/direct-websocket); Part B's
// (relay-ssh) git.exec whitelist rejects checkout/merge/stash/branch -d
// outright (agent-rpc-catalog-git-fs.md's "Not allowed at all" list). The
// usecase layer's ConnectionResolver check is expected to prevent these
// methods from ever being called against a relay-ssh connection — none of
// them re-check mode themselves. ───────────────────────────────────────────

// MergeIntoBranch relays via git.exec's merge subcommand. Named
// MergeIntoBranch, not MergeBranch — that name is already taken by the
// worktree-into-base MergeBranch method above.
func (r *RelayExecutor) MergeIntoBranch(ctx context.Context, repoPath, branch string, noFF bool) (domain.MergeOutcome, error) {
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	var result gitExecResult
	err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result)
	if err != nil {
		return domain.MergeOutcome{}, err
	}
	return domain.MergeOutcome{Success: true, HadConflicts: strings.Contains(result.Stderr, "CONFLICT")}, nil
}

// StashPush relays via git.exec's stash push subcommand.
func (r *RelayExecutor) StashPush(ctx context.Context, repoPath, message string, includeUntracked bool) (domain.SimpleResult, error) {
	args := []string{"stash", "push"}
	if includeUntracked {
		args = append(args, "-u")
	}
	if message != "" {
		args = append(args, "-m", message)
	}
	var result gitExecResult
	if err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result); err != nil {
		return domain.SimpleResult{}, err
	}
	return domain.SimpleResult{Success: true}, nil
}

// StashPop relays via git.exec's stash pop subcommand.
func (r *RelayExecutor) StashPop(ctx context.Context, repoPath, stashRef string) (domain.MergeOutcome, error) {
	args := []string{"stash", "pop"}
	if stashRef != "" {
		args = append(args, stashRef)
	}
	var result gitExecResult
	err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result)
	if err != nil {
		return domain.MergeOutcome{}, err
	}
	return domain.MergeOutcome{Success: true, HadConflicts: strings.Contains(result.Stderr, "CONFLICT")}, nil
}

// CreateBranch composes two git.exec calls (branch then checkout)
// sequentially when checkout=true — `checkout -b`'s combined form is not
// on either Part's exec whitelist as a single flag-shape, so this always
// issues the two subcommands separately.
func (r *RelayExecutor) CreateBranch(ctx context.Context, repoPath, branch, baseRef string, checkout bool) (string, error) {
	args := []string{"branch", branch}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	var result gitExecResult
	if err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": args, "cwd": repoPath}, &result); err != nil {
		return "", err
	}
	if checkout {
		var coResult gitExecResult
		if err := r.relay(ctx, repoPath, "git.exec", map[string]any{"args": []string{"checkout", branch}, "cwd": repoPath}, &coResult); err != nil {
			return "", err
		}
	}
	return branch, nil
}

// DeleteBranch relays via git.exec's branch -d subcommand (soft delete).
func (r *RelayExecutor) DeleteBranch(ctx context.Context, repoPath, branch string) error {
	var result gitExecResult
	return r.relay(ctx, repoPath, "git.exec", map[string]any{"args": []string{"branch", "-d", branch}, "cwd": repoPath}, &result)
}
