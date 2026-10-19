package grpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// agentExecCeilingMs is the longest the agent lets one agent.exec run; asking for more would only be cut short there.
const agentExecCeilingMs = 5 * 60 * 1000

// DevServerExecutor runs a command on the project's dev server through infra-fleet's relay (the same road as
// agent.execPrompt), so local, SSH and WSL dev servers behave alike. The binary and arguments travel as separate JSON
// fields; no shell is involved.
type DevServerExecutor struct {
	infra infrafleetv1.InfraFleetServiceClient
}

var _ usecase.DevServerExecutor = (*DevServerExecutor)(nil)

func NewDevServerExecutor(infra infrafleetv1.InfraFleetServiceClient) *DevServerExecutor {
	return &DevServerExecutor{infra: infra}
}

type agentExecParams struct {
	Binary    string            `json:"binary"`
	Args      []string          `json:"args"`
	Cwd       string            `json:"cwd,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	TimeoutMS int               `json:"timeoutMs,omitempty"`
}

type agentExecResult struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   *int   `json:"exitCode"`
	TimedOut   bool   `json:"timedOut"`
	SpawnError string `json:"spawnError"`
}

func (e *DevServerExecutor) Exec(ctx context.Context, conn usecase.AnalysisConnection, in usecase.ExecInput) (usecase.ExecResult, error) {
	if in.Binary == "" {
		return usecase.ExecResult{}, errors.New("grpcclient: agent.exec needs a binary")
	}
	tctx, err := withTenantMetadata(ctx)
	if err != nil {
		return usecase.ExecResult{}, err
	}
	timeout := in.TimeoutMs
	if timeout <= 0 || timeout > agentExecCeilingMs {
		timeout = agentExecCeilingMs
	}
	tctx, cancel := context.WithTimeout(tctx, time.Duration(timeout)*time.Millisecond+slackOverTimeout)
	defer cancel()
	args := in.Args
	if args == nil {
		args = []string{}
	}
	raw, err := json.Marshal(agentExecParams{Binary: in.Binary, Args: args, Cwd: in.Cwd, Env: in.Env, TimeoutMS: timeout})
	if err != nil {
		return usecase.ExecResult{}, fmt.Errorf("grpcclient: marshal agent.exec params: %w", err)
	}
	var resultJSON string
	if conn.DevServerID != "" {
		resp, rerr := e.infra.RelayByDevServer(tctx, &infrafleetv1.RelayByDevServerRequest{DevServerId: conn.DevServerID, Method: "agent.exec", ParamsJson: string(raw)})
		resultJSON, err = resp.GetResultJson(), rerr
	} else {
		resp, rerr := e.infra.Relay(tctx, &infrafleetv1.RelayRequest{ConnectionId: conn.ConnectionID, Method: "agent.exec", ParamsJson: string(raw)})
		resultJSON, err = resp.GetResultJson(), rerr
	}
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || status.Code(err) == codes.DeadlineExceeded {
			return usecase.ExecResult{TimedOut: true, ExitCode: -1}, nil
		}
		return usecase.ExecResult{}, fmt.Errorf("grpcclient: relay agent.exec: %w", err)
	}
	var r agentExecResult
	if err := json.Unmarshal([]byte(resultJSON), &r); err != nil {
		return usecase.ExecResult{}, fmt.Errorf("grpcclient: decode agent.exec result: %w", err)
	}
	out := usecase.ExecResult{Stdout: truncateUTF8(r.Stdout, maxStdoutBytes), Stderr: truncateUTF8(r.Stderr, 16*1024), TimedOut: r.TimedOut, SpawnError: r.SpawnError, ExitCode: -1}
	if r.ExitCode != nil { // no exit code means the process never completed
		out.ExitCode = *r.ExitCode
	}
	return out, nil
}
