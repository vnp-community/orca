// Package domain holds workflow-service's entities, value objects, and the
// StepExecutor strategy interface. Per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md this
// package has zero imports outside stdlib + other domain/ packages — no
// database, no gRPC, no framework, and (deliberately, see StepExecutor's
// doc comment) no infra-fleet-service relay client either.
package domain

import (
	"context"
	"errors"
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
	StepTypeAction       StepType = "action"   // TASK-WF-003-02
	StepTypeParallel     StepType = "parallel" // TASK-WF-003-03
)

// Valid reports whether t is one of the seven known step types.
func (t StepType) Valid() bool {
	switch t {
	case StepTypeAgent, StepTypeShell, StepTypeNotification, StepTypeWebhook, StepTypeCondition, StepTypeAction, StepTypeParallel:
		return true
	default:
		return false
	}
}

// ErrParallelStepFailed is returned by waveDispatcher.runParallelStep when
// at least one sub-step failed and ParallelStepConfig.AllowPartialFailure
// is false — same per-package sentinel-error convention as
// ErrTemplateNotFound et al. (template.go).
var ErrParallelStepFailed = errors.New("domain: parallel step failed: at least one sub-step did not complete")

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

// AgentStepConfig is the Agent step type's config shape — the prompt-driven
// agent invocation internal/adapter/infrafleetclient.AgentExecutor relays to
// infra-fleet-service's Relay RPC (workflow-service.md §4).
//
// ConnectionID is a new field added in this pass: nothing in this scaffold
// previously identified *which* infra-fleet-service connection (dev server +
// worktree binding) an agent/shell/notification step should target — an
// undocumented gap this build closes, naming the field to match
// infra-fleet-service's own ConnectionID/connectionId convention (see its
// internal/usecase/resolve_connection.go and relay.go).
type AgentStepConfig struct {
	ConnectionID string       `json:"connectionId"`
	Prompt       string       `json:"prompt"`
	WorktreePath string       `json:"worktreePath,omitempty"`
	TrustPreset  string       `json:"trustPreset,omitempty"`
	Provider     *ProviderPin `json:"provider,omitempty"`
}

// ProviderPin is an AgentStepConfig's optional explicit AI provider
// choice — set, it beats ai-provider-service's own priority-chain
// resolution (user > project > server); unset, ProviderResolver falls back
// to that chain. See usecase.ProviderResolver. A pointer field so "field
// entirely absent" (fall back to the chain) is distinguishable from
// "present with an empty AccountID" (also falls back) without a second
// boolean.
type ProviderPin struct {
	AccountID string `json:"accountId,omitempty"`
	Model     string `json:"model,omitempty"`
}

// ShellStepConfig is the Shell step type's config shape — a script relayed
// to infra-fleet-service's Relay RPC for execution on the target connection.
// ConnectionID: see AgentStepConfig's doc comment (same new-field rationale).
type ShellStepConfig struct {
	ConnectionID string            `json:"connectionId"`
	Script       string            `json:"script"`
	Env          map[string]string `json:"env,omitempty"`
}

// NotificationStepConfig is the Notification step type's config shape — a
// message relayed to infra-fleet-service's Relay RPC for dispatch.
// ConnectionID: see AgentStepConfig's doc comment (same new-field rationale).
type NotificationStepConfig struct {
	ConnectionID string `json:"connectionId"`
	Channel      string `json:"channel"`
	Message      string `json:"message"`
}

// ActionStepConfig is the Action step type's config shape — dispatches to
// one of a registered set of action handlers (git.*, github.*, jira.*,
// ...) by name. See stepexecutors.ActionExecutor's doc comment for which
// handlers are actually wired — the set of supported actions is a product
// decision, not fixed by this type.
type ActionStepConfig struct {
	Action string         `json:"action"` // "git.createBranch" | "github.createPR" | ...
	Params map[string]any `json:"params"`
}

// ParallelStepConfig is the Parallel step type's config shape — a single
// step's own nested fan-out to sub-steps, dispatched directly by
// waveDispatcher.runStep (not through StepExecutorRegistry, since it needs
// to recurse back into runStep itself for each sub-step — see
// wave_dispatcher.go's runParallelStep).
type ParallelStepConfig struct {
	Steps               []Step `json:"steps"`
	AllowPartialFailure bool   `json:"allowPartialFailure"`
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
