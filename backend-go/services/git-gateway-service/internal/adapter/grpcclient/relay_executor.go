package grpcclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// RelayExecutor implements usecase.GitExecutor for the connected-host case
// (ConnectionResolver reports Connected=true), by calling
// infra-fleet-service's generic Relay RPC with method names "git.status" /
// "git.diff" / "git.commit" / "git.push" / "git.pull" and JSON-encoded
// params, per git-gateway-service.md §2 step 3'. It also implements
// usecase.AICompleter (see Complete below) via the same Relay RPC with
// method "ai.complete", for GenerateCommitMessage.
//
// Best-effort param/result shape, not verified against a real agent: like
// infra-fleet-service's decodeOpenPorts (internal/usecase/scan_workspace_ports.go),
// there is no live Dev Server Agent in this environment to confirm these
// JSON field names against. specs/agent/api/agent-rpc-catalog-git-fs.md
// documents the Part B SSH Relay Daemon's actual git.* handler contract —
// it uses different field names (worktreePath, filePath, pushTarget, etc.)
// and result shapes (e.g. git.status -> {entries[],branch?,...} rather than
// this service's domain.GitStatus{Branch,Files}) than what's used below.
// The params/results here are instead named to match this service's own
// domain types 1:1 (repoPath, staged, message, paths, remote, branch;
// results decoded straight into domain.GitStatus/DiffResult/etc.'s own
// field names) on the assumption that the real Dev Server Agent-facing
// relay method names (git.status etc.) are a distinct, git-gateway-specific
// contract from the catalog's SSH-relay-daemon methods of the same name —
// reconcile this against the real agent handler before removing this
// comment.
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

func (r *RelayExecutor) GetStatus(ctx context.Context, repoPath string) (domain.GitStatus, error) {
	var result domain.GitStatus
	err := r.relay(ctx, repoPath, "git.status", map[string]any{
		"repoPath": repoPath,
	}, &result)
	return result, err
}

func (r *RelayExecutor) GetDiff(ctx context.Context, repoPath string, staged bool) (domain.DiffResult, error) {
	var result domain.DiffResult
	err := r.relay(ctx, repoPath, "git.diff", map[string]any{
		"repoPath": repoPath,
		"staged":   staged,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Commit(ctx context.Context, repoPath, message string, paths []string) (domain.CommitResult, error) {
	var result domain.CommitResult
	err := r.relay(ctx, repoPath, "git.commit", map[string]any{
		"repoPath": repoPath,
		"message":  message,
		"paths":    paths,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Push(ctx context.Context, repoPath, remote, branch string) (domain.PushResult, error) {
	var result domain.PushResult
	err := r.relay(ctx, repoPath, "git.push", map[string]any{
		"repoPath": repoPath,
		"remote":   remote,
		"branch":   branch,
	}, &result)
	return result, err
}

func (r *RelayExecutor) Pull(ctx context.Context, repoPath string) (domain.PullResult, error) {
	var result domain.PullResult
	err := r.relay(ctx, repoPath, "git.pull", map[string]any{
		"repoPath": repoPath,
	}, &result)
	return result, err
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
