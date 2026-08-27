package infrafleetclient

import (
	"context"
	"encoding/json"
	"fmt"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// notificationSendMethod is the Relay method name NotificationExecutor
// uses — the Relay proto's own doc comment names "notification.send" as the
// example method for notification steps. Best-effort, not verified against
// a live Dev Server Agent — see AgentExecutor's agentExecMethod doc comment
// for the sibling caveat on this same relay contract.
const notificationSendMethod = "notification.send"

// notificationSendParams is the params_json payload NotificationExecutor
// sends — mirrors TS's StepExecutors.ts executeNotification() shape
// (channel + message).
type notificationSendParams struct {
	Channel string `json:"channel"`
	Message string `json:"message"`
}

// notificationResult is the best-effort shape this package assumes
// notification.send decodes to. TS's executeNotification() never inspects
// its relay result at all (always returns exitCode: 0 — fire and forget);
// this executor is slightly stricter, treating a non-empty "error" field as
// a business-level failure so a caller relying on ExecuteAdHocStep's result
// can actually observe a send failure, at the cost of assuming the field
// exists — unverified against a live agent, same caveat as its siblings.
type notificationResult struct {
	Error string `json:"error,omitempty"`
}

// notificationResultOutput is what Execute marshals into
// StepResult.OutputJSON.
type notificationResultOutput struct {
	Sent  bool   `json:"sent"`
	Error string `json:"error,omitempty"`
}

// NotificationExecutor is the real Notification step executor — relays a
// channel+message notification to infra-fleet-service's Relay RPC.
type NotificationExecutor struct {
	client infrafleetv1.InfraFleetServiceClient
}

// NewNotificationExecutor wraps an already-constructed infrafleetv1
// client — used by cmd/server/main.go (real dial) and by tests (fake
// client).
func NewNotificationExecutor(client infrafleetv1.InfraFleetServiceClient) *NotificationExecutor {
	return &NotificationExecutor{client: client}
}

var _ domain.StepExecutor = (*NotificationExecutor)(nil)

func (e *NotificationExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg domain.NotificationStepConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: notification: invalid step config JSON: %w", err)
	}

	var result notificationResult
	if err := relay(ctx, e.client, cfg.ConnectionID, notificationSendMethod, notificationSendParams{
		Channel: cfg.Channel,
		Message: cfg.Message,
	}, &result); err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: notification: %w", err)
	}

	status := domain.ResultStatusCompleted
	sent := true
	if result.Error != "" {
		status = domain.ResultStatusFailed
		sent = false
	}

	output, err := json.Marshal(notificationResultOutput{Sent: sent, Error: result.Error})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("infrafleetclient: notification: marshal output: %w", err)
	}
	return domain.StepResult{Status: status, OutputJSON: string(output)}, nil
}
