package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// trustPresetDefault is the only trust preset this client ever sends. task-service's SimpleExecutor sends "full";
// analysis must not, and the type of AgentPromptInput gives callers no way to ask for it.
const trustPresetDefault = "default"

const (
	// maxStdoutBytes caps what is read into memory; the agent is asked for slightly more so a JSON block at the end survives.
	maxStdoutBytes    = 256 * 1024
	maxOutputBytesAsk = maxStdoutBytes + 64*1024
	// slackOverTimeout lets the agent report its own timeout before the RPC deadline hides it.
	slackOverTimeout = 30 * time.Second
)

// AgentPromptRelay calls agent.execPrompt through infra-fleet's Relay (or RelayByDevServer when the project has no
// infra connection), so local, SSH and WSL dev servers behave the same.
type AgentPromptRelay struct {
	infra infrafleetv1.InfraFleetServiceClient
}

var _ usecase.AgentPromptRunner = (*AgentPromptRelay)(nil)

func NewAgentPromptRelay(infra infrafleetv1.InfraFleetServiceClient) *AgentPromptRelay {
	return &AgentPromptRelay{infra: infra}
}

type agentExecPromptParams struct {
	StepID       string            `json:"stepId"`
	Prompt       string            `json:"prompt"`
	WorktreePath string            `json:"worktreePath"`
	TrustPreset  string            `json:"trustPreset,omitempty"`
	Env          map[string]string `json:"env"`
	TimeoutMS    int               `json:"timeoutMs,omitempty"`
	// The fields below only exist for agents that advertise agent.execPrompt.readonly; old agents ignore unknown keys,
	// which is why the caller decides before sending.
	AccessMode     string `json:"accessMode,omitempty"`
	WorkspaceKind  string `json:"workspaceKind,omitempty"`
	ReportChanges  bool   `json:"reportChanges,omitempty"`
	MaxOutputBytes int    `json:"maxOutputBytes,omitempty"`
}

// buildAgentParams fixes trust and env: the env holds two ids and never a token or credential.
func buildAgentParams(in usecase.AgentPromptInput) agentExecPromptParams {
	p := agentExecPromptParams{
		StepID: in.StepID, Prompt: in.Prompt, WorktreePath: in.RepoPath, TimeoutMS: in.TimeoutMS,
		Env: map[string]string{"ORCA_REQUEST_ID": in.RequestID, "ORCA_PROJECT_ID": in.ProjectID},
	}
	if in.ReadOnlyEnforced {
		// With accessMode=readonly the agent drops any trust preset, so none is sent.
		p.AccessMode, p.WorkspaceKind, p.ReportChanges, p.MaxOutputBytes = "readonly", "repo_root", true, maxOutputBytesAsk
		return p
	}
	p.TrustPreset = trustPresetDefault
	return p
}

type agentExecPromptResult struct {
	Stdout   string   `json:"stdout"`
	Stderr   string   `json:"stderr"`
	ExitCode *int     `json:"exitCode"`
	TimedOut bool     `json:"timedOut"`
	Warnings []string `json:"warnings"`
	Applied  *struct {
		AccessMode string `json:"accessMode"`
	} `json:"applied"`
	Changes *struct {
		Available    bool              `json:"available"`
		HeadMoved    bool              `json:"headMoved"`
		ChangedFiles []json.RawMessage `json:"changedFiles"`
	} `json:"changes"`
}

func (r *AgentPromptRelay) ExecPrompt(ctx context.Context, conn usecase.AnalysisConnection, in usecase.AgentPromptInput) (usecase.AgentPromptResult, error) {
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return usecase.AgentPromptResult{}, err
	}
	if in.TimeoutMS > 0 {
		var cancel context.CancelFunc
		tctx, cancel = context.WithTimeout(tctx, time.Duration(in.TimeoutMS)*time.Millisecond+slackOverTimeout)
		defer cancel()
	}
	raw, err := json.Marshal(buildAgentParams(in))
	if err != nil {
		return usecase.AgentPromptResult{}, fmt.Errorf("grpcclient: marshal agent.execPrompt params: %w", err)
	}
	var resultJSON string
	if conn.DevServerID != "" {
		resp, rerr := r.infra.RelayByDevServer(tctx, &infrafleetv1.RelayByDevServerRequest{DevServerId: conn.DevServerID, Method: "agent.execPrompt", ParamsJson: string(raw)})
		resultJSON, err = resp.GetResultJson(), rerr
	} else {
		resp, rerr := r.infra.Relay(tctx, &infrafleetv1.RelayRequest{ConnectionId: conn.ConnectionID, Method: "agent.execPrompt", ParamsJson: string(raw)})
		resultJSON, err = resp.GetResultJson(), rerr
	}
	if err != nil {
		return usecase.AgentPromptResult{}, classifyAgentRelayError(err)
	}
	return parseAgentResult(resultJSON)
}

// classifyAgentRelayError turns the agent's refusal of read-only into a typed error. Relay may flatten the JSON-RPC
// error data to a message, so the reason code is matched in the text.
func classifyAgentRelayError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
		return context.DeadlineExceeded
	}
	if strings.Contains(err.Error(), "READONLY_MODE_UNSUPPORTED") {
		return usecase.ErrAgentReadonlyUnsupported
	}
	return fmt.Errorf("grpcclient: relay agent.execPrompt: %w", err)
}

func parseAgentResult(resultJSON string) (usecase.AgentPromptResult, error) {
	var r agentExecPromptResult
	if err := json.Unmarshal([]byte(resultJSON), &r); err != nil {
		return usecase.AgentPromptResult{}, fmt.Errorf("grpcclient: decode agent.execPrompt result: %w", err)
	}
	out := usecase.AgentPromptResult{Stdout: truncateUTF8(r.Stdout, maxStdoutBytes), Stderr: truncateUTF8(r.Stderr, 16*1024), TimedOut: r.TimedOut, Warnings: r.Warnings}
	if r.ExitCode == nil {
		out.ExitCode = -1 // no exit code means the process never completed
	} else {
		out.ExitCode = *r.ExitCode
	}
	if r.Applied != nil {
		out.AppliedAccessMode = r.Applied.AccessMode
	}
	if r.Changes != nil {
		out.ChangesAvailable, out.HeadMoved, out.ChangedFiles = r.Changes.Available, r.Changes.HeadMoved, len(r.Changes.ChangedFiles)
	}
	return out, nil
}

func truncateUTF8(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
