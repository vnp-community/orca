// Package domain holds workflow-service's entities, value objects, and the
// StepExecutor strategy interface. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework, and (deliberately, see StepExecutor's
// doc comment) no infra-fleet-service relay client either.
package domain

import (
	"context"
	"encoding/json"
)

// StepType is the discriminator for a workflow step's kind — mirrors
// workflowv1.StepType (proto) one-for-one, see
// specs/backend-go/services/workflow-service.md §4.
type StepType string

const (
	StepTypeUnspecified  StepType = ""
	StepTypeAgent        StepType = "agent"
	StepTypeShell        StepType = "shell"
	StepTypeNotification StepType = "notification"
	StepTypeWebhook      StepType = "webhook"
	StepTypeCondition    StepType = "condition"
	// StepTypeCleanupWorktrees is BL-AT-04's bulk policy-delete step — see
	// internal/usecase.CleanupWorktreesStepExecutor.
	StepTypeCleanupWorktrees StepType = "cleanup_worktrees"
	// StepTypeAction dispatches to a named, in-process action handler —
	// see ActionStepConfig's doc comment. BUG-WF-02 found this type
	// entirely absent (only 5 of 6 StepTypes were implemented).
	StepTypeAction StepType = "action"
	// StepTypeParallel fans a fixed list of sub-steps out concurrently —
	// see ParallelStepConfig's doc comment.
	StepTypeParallel StepType = "parallel"
)

// Valid reports whether t is one of the known step types.
func (t StepType) Valid() bool {
	switch t {
	case StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook, StepTypeCondition, StepTypeCleanupWorktrees, StepTypeAction, StepTypeParallel:
		return true
	default:
		return false
	}
}

// ActionStepConfig is the Action step type's config shape: dispatches to a
// named, in-process action handler (registered by
// internal/adapter/stepexecutors.ActionExecutor). Neither
// workflow-service.md §4 nor this task describes a concrete action
// catalog, so this wires the minimal, extensible type system — no handlers
// are registered by TASK-WF-02-07 itself — rather than inventing one:
// an `action` step is recognized and fails with a clear, typed error
// (usecase.ErrNoActionHandlerRegistered) instead of silently no-op-ing.
type ActionStepConfig struct {
	ActionName string          `json:"actionName"`
	Params     json.RawMessage `json:"params,omitempty"`
}

// ParallelStepConfig is the Parallel step type's config shape: fans
// SubSteps out concurrently and aggregates their results
// (Promise.allSettled + allowPartialFailure semantics — see
// stepexecutors.ParallelExecutor). SubSteps' own DependsOn is ignored:
// sub-steps always run together in one fan-out, not wave-computed among
// themselves the way a template's top-level Steps are.
type ParallelStepConfig struct {
	SubSteps            []Step `json:"subSteps"`
	AllowPartialFailure bool   `json:"allowPartialFailure,omitempty"`
}

// ResultStatus is a StepResult's outcome — "completed" and "failed" only;
// no in-progress/partial state, since a StepExecutor call is synchronous
// from this service's point of view (§3.1: ExecuteAdHocStep is
// synchronous, and a real Execute's wave dispatch awaits each step before
// gating the next wave).
type ResultStatus string

const (
	ResultStatusCompleted ResultStatus = "completed"
	ResultStatusFailed    ResultStatus = "failed"
)

// StepResult is the outcome of running one step through a StepExecutor —
// mirrors workflowv1.StepResult (proto) one-for-one.
type StepResult struct {
	Status     ResultStatus
	OutputJSON string
}

// Target is a dispatch-target string in one of four shapes — the
// orchestrator resolves it to a concrete connectionId before relaying:
//
//	"connection:<id>"   — direct passthrough, today's ConnectionID shape (back-compat)
//	"project:<id>"      — resolve via project-service.GetProject().dev_server_id, then infra-fleet-service.ResolveConnection
//	"server:<id>"       — resolve via infra-fleet-service.ResolveConnection(dev_server_id=<id>) directly
//	"fleet:tag:<tag>"   — load-balance across infra-fleet-service's healthy dev servers carrying <tag>
//
// ConnectionID is a deprecated alias: when Target is empty and
// ConnectionID is set, it's treated as "connection:<ConnectionID>" — see
// EffectiveTarget.

// AgentStepConfig is the Agent step type's config shape — the prompt-driven
// agent invocation internal/adapter/infrafleetclient.AgentExecutor relays to
// infra-fleet-service's Relay RPC (workflow-service.md §4).
//
// ConnectionID is a new field added in an earlier pass: nothing in this
// scaffold previously identified *which* infra-fleet-service connection
// (dev server + worktree binding) an agent/shell/notification step should
// target — an undocumented gap that build closed, naming the field to
// match infra-fleet-service's own ConnectionID/connectionId convention
// (see its internal/usecase/resolve_connection.go and relay.go). Target
// (this pass) supersedes it with the four-shape resolver-friendly string
// above; ConnectionID stays as a deprecated back-compat alias.
type AgentStepConfig struct {
	Target       string `json:"target,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"` // deprecated, see Target's doc comment
	Prompt       string `json:"prompt"`
	WorktreePath string `json:"worktreePath,omitempty"`
	TrustPreset  string `json:"trustPreset,omitempty"`
	UserID       string `json:"userId,omitempty"`    // whose profile to resolve; empty = legacy passthrough, see below
	ProjectID    string `json:"projectId,omitempty"` // for GetProjectContext + ORCA_PROJECT_*
	// Provider pins a specific ai-provider-service account, bypassing the
	// priority cascade — workflow-service.md §7: an explicit
	// step.config.provider.accountId pin (validated active) beats
	// ai-provider-service's priority-chain resolution.
	Provider *ProviderPin `json:"provider,omitempty"`
	Model    string       `json:"model,omitempty"` // pass-through param, not resolved server-side
}

// ProviderPin explicitly pins an agent step to a specific
// ai-provider-service account, bypassing ai-provider-service's own
// priority-cascade resolution — see AgentStepConfig.Provider's doc comment.
type ProviderPin struct {
	AccountID string `json:"accountId"`
}

// EffectiveTarget resolves AgentStepConfig's dispatch target: Target when
// set, else ConnectionID mapped to its "connection:<id>" equivalent (the
// deprecated back-compat path), else empty (execute locally, unchanged
// from before Target existed).
func (c AgentStepConfig) EffectiveTarget() string {
	if c.Target != "" {
		return c.Target
	}
	if c.ConnectionID != "" {
		return "connection:" + c.ConnectionID
	}
	return ""
}

// ShellStepConfig is the Shell step type's config shape — a script relayed
// to infra-fleet-service's Relay RPC for execution on the target connection.
// Target/ConnectionID: see AgentStepConfig's doc comment (identical
// resolver-target shape; Shell has no Provider/Model — those are
// agent-specific).
type ShellStepConfig struct {
	Target       string            `json:"target,omitempty"`
	ConnectionID string            `json:"connectionId,omitempty"` // deprecated, see AgentStepConfig.Target's doc comment
	Script       string            `json:"script"`
	Env          map[string]string `json:"env,omitempty"`
}

func (c ShellStepConfig) EffectiveTarget() string {
	if c.Target != "" {
		return c.Target
	}
	if c.ConnectionID != "" {
		return "connection:" + c.ConnectionID
	}
	return ""
}

// NotificationStepConfig is the Notification step type's config shape — a
// message relayed to infra-fleet-service's Relay RPC for dispatch.
// Target/ConnectionID: see AgentStepConfig's doc comment.
type NotificationStepConfig struct {
	Target       string `json:"target,omitempty"`
	ConnectionID string `json:"connectionId,omitempty"` // deprecated, see AgentStepConfig.Target's doc comment
	Channel      string `json:"channel"`
	Message      string `json:"message"`
}

func (c NotificationStepConfig) EffectiveTarget() string {
	if c.Target != "" {
		return c.Target
	}
	if c.ConnectionID != "" {
		return "connection:" + c.ConnectionID
	}
	return ""
}

// ExecutionEvent is a step/execution-level lifecycle event fanned out to
// live StreamExecutionEvents subscribers — mirrors workflowv1.ExecutionEvent
// (proto, added TASK-WF-02-01) one-for-one.
type ExecutionEvent struct {
	ExecutionID string
	StepID      string // empty for execution-level events
	Type        string // step.output | step.completed | execution.completed
	PayloadJSON string
	OccurredAt  int64 // unix ms
}

// StepExecutor is the domain-level strategy interface each step type
// implements — one Execute per StepType, dispatched by
// usecase.StepExecutorRegistry.Resolve.
//
// Deviation note: specs/backend-go/services/workflow-service.md §4 places
// this interface in usecase/, per
// architecture/03-clean-architecture-guidelines.md's general "interface
// lives with its consumer" rule. It's defined here instead, per this
// service's explicit build instructions: the strategy itself (dispatch by
// StepType to a pure Execute(ctx, config) -> (StepResult, error) contract)
// is a pure domain concept independent of any adapter, even though four of
// five concrete implementations happen to need I/O. The port that *resolves*
// a StepType to a StepExecutor (usecase.StepExecutorRegistry) still lives in
// usecase/, matching the general rule for the one piece that's genuinely
// usecase-shaped.
type StepExecutor interface {
	Execute(ctx context.Context, stepConfigJSON string) (StepResult, error)
}
