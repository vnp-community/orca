package devserveragent

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// agentExecutor is the narrow slice of usecase.DevServerAgentClient
// TerraformRunner actually needs — deliberately smaller than the full
// interface (Go's "accept small interfaces" convention) so *Client
// satisfies it without dragging in every terminal/screencast/ephemeral-VM
// method a test double would otherwise have to stub.
type agentExecutor interface {
	Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error)
}

// TerraformRunner implements usecase.TerraformRunner over the same generic
// DevServerAgentClient.Exec passthrough usecase.Relay uses (CR-FLEET-002 §
// "Hướng A") — no new transport, just a typed call to the agent's
// terraform.apply JSON-RPC method (agent/src/relay/agent-terraform-handler.ts,
// TASK-BE-FLEET-007).
//
// SECURITY: this adapter passes no credentials — see
// usecase.ApplyTerraformPlan's doc comment and TASK-BE-FLEET-009 (design +
// security review, not yet implemented).
type TerraformRunner struct {
	agent agentExecutor
}

func NewTerraformRunner(agent agentExecutor) *TerraformRunner {
	return &TerraformRunner{agent: agent}
}

// var _ usecase.TerraformRunner = (*TerraformRunner)(nil) enforces the
// contract at compile time — this type exists specifically to satisfy that
// port, and a signature drift should fail the build here, not at
// cmd/server/main.go's call site.
var _ usecase.TerraformRunner = (*TerraformRunner)(nil)

func (r *TerraformRunner) Apply(ctx context.Context, controlDevServer domain.DevServer, workingDir, varsFile string) (string, error) {
	params := map[string]any{"workingDir": workingDir}
	if varsFile != "" {
		params["varsFile"] = varsFile
	}
	result, err := r.agent.Exec(ctx, controlDevServer, "terraform.apply", params)
	if err != nil {
		return "", fmt.Errorf("devserveragent: terraform.apply: %w", err)
	}
	outputJSON, ok := result["outputJson"].(string)
	if !ok {
		return "", fmt.Errorf("devserveragent: terraform.apply: response missing outputJson field")
	}
	return outputJSON, nil
}
