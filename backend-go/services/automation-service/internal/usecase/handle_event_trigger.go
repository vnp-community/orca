package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// HandleEventTriggerInput is one delivered event, as handed off by
// internal/adapter/eventbus.Consumer.
type HandleEventTriggerInput struct {
	EventID   string
	TenantID  string
	EventName domain.EventName
	Payload   string // raw JSON
}

// HandleEventTrigger is BL-AT-03's event-trigger dispatch: given an
// incoming event, list matching enabled event-triggered automations
// (ListByTrigger), apply BR-AT-09's filter, and dispatch each match via the
// existing RunNow interactor with a deterministic idempotency key so
// JetStream's at-least-once redelivery is harmless.
type HandleEventTrigger struct {
	automations AutomationRepository
	runNow      *RunNow
	logger      *slog.Logger
}

func NewHandleEventTrigger(automations AutomationRepository, runNow *RunNow, logger *slog.Logger) *HandleEventTrigger {
	if logger == nil {
		logger = slog.Default()
	}
	return &HandleEventTrigger{automations: automations, runNow: runNow, logger: logger}
}

func (uc *HandleEventTrigger) Execute(ctx context.Context, in HandleEventTriggerInput) error {
	tenantCtx := tenant.WithTenantID(ctx, in.TenantID)
	matches, err := uc.automations.ListByTrigger(tenantCtx, in.TenantID, in.EventName)
	if err != nil {
		return err
	}

	var payload map[string]any
	_ = json.Unmarshal([]byte(in.Payload), &payload) // malformed/empty payload just means no filter ever matches — see TriggerFilter.Matches' fail-safe-false contract

	for _, automation := range matches {
		if !automation.Enabled {
			continue // BR-AT-03
		}
		if automation.TriggerFilter != nil && !automation.TriggerFilter.Matches(payload) {
			continue // BR-AT-09
		}
		// Deterministic request_id from (event ID, automation ID) —
		// idempotent under JetStream's at-least-once redelivery, same
		// mechanism as the scheduler ticker's own request_id derivation.
		requestID := fmt.Sprintf("event:%s:%s", in.EventID, automation.ID)
		if _, err := uc.runNow.Execute(tenantCtx, RunNowInput{
			AutomationID: automation.ID, RequestID: requestID, Trigger: domain.RunTriggerEvent,
		}); err != nil {
			uc.logger.ErrorContext(ctx, "event-triggered dispatch failed", slog.Any("error", err), slog.String("automation_id", automation.ID), slog.String("event_id", in.EventID))
			// Log and continue — one automation's dispatch failure must not
			// block dispatching the rest of this tenant's matching
			// automations.
		}
	}
	return nil
}
