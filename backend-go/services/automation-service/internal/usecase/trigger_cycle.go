package usecase

import (
	"context"
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/automation-service/internal/domain"
)

// actionToEvents is the fixed, closed mapping from an action's StepType to
// the event(s) it could emit — mirrors EventName's closed set. Modeled on
// workflow-service's BuildWaves/ErrCyclicDependency precedent (BR-AT-10).
//
// Only StepTypeAgent maps to a real event today (agent:completed/
// agent:error) — this service's StepType set is the closed 5-value one
// (agent/shell/notification/webhook/condition; see domain/automation.go).
// worktree:created/pr:merged/issue:assigned have no corresponding StepType
// in that set yet (a dedicated create-worktree/create-pr step type doesn't
// exist), so they add no edges — the same "commit/notify/cleanup emit none
// of the 5 documented events" case SOL-AT-03 already calls out.
var actionToEvents = map[domain.StepType][]domain.EventName{
	domain.StepTypeAgent: {domain.EventAgentCompleted, domain.EventAgentError},
}

// DetectTriggerCycle builds a directed graph over tenantID's event-triggered
// automations (including candidate, which overrides its own prior version
// on an update) and returns AUTOMATION_TRIGGER_CYCLE naming the offending
// automation IDs if one exists. A same-automation self-reference (X == Y)
// is the degenerate single-node case of this same graph — no separate guard
// needed.
func DetectTriggerCycle(ctx context.Context, repo AutomationRepository, tenantID string, candidate domain.Automation) error {
	all, err := repo.ListEventTriggered(ctx, tenantID)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "AUTOMATION_TRIGGER_CYCLE_CHECK_FAILED", "failed to list event-triggered automations", err)
	}
	nodes := replaceOrAppend(all, candidate)

	graph := map[string][]string{} // automation ID -> IDs it can trigger
	byEvent := map[domain.EventName][]string{}
	for _, a := range nodes {
		if a.TriggerType == domain.TriggerTypeEvent {
			byEvent[a.TriggerEvent] = append(byEvent[a.TriggerEvent], a.ID)
		}
	}
	for _, a := range nodes {
		for _, action := range a.Actions {
			for _, ev := range actionToEvents[action.StepType] {
				graph[a.ID] = append(graph[a.ID], byEvent[ev]...)
			}
		}
	}

	if cycle := findCycle(nodes, graph); cycle != nil {
		return apperrors.New(apperrors.KindFailedPrecondition, "AUTOMATION_TRIGGER_CYCLE",
			fmt.Sprintf("cyclic automation trigger chain: %v", cycle), nil)
	}
	return nil
}

// replaceOrAppend returns all with candidate substituted for its
// prior-version entry (matched by ID), or appended if not present —
// candidate always wins so a not-yet-persisted create/update is checked
// against the graph it would actually produce.
func replaceOrAppend(all []domain.Automation, candidate domain.Automation) []domain.Automation {
	out := make([]domain.Automation, 0, len(all)+1)
	replaced := false
	for _, a := range all {
		if a.ID == candidate.ID {
			out = append(out, candidate)
			replaced = true
			continue
		}
		out = append(out, a)
	}
	if !replaced {
		out = append(out, candidate)
	}
	return out
}

// findCycle runs Kahn's algorithm over graph (restricted to nodes) and
// returns the automation IDs still un-orderable (i.e. participating in a
// cycle, including a self-loop) — nil if the graph is a DAG.
func findCycle(nodes []domain.Automation, graph map[string][]string) []string {
	inDegree := make(map[string]int, len(nodes))
	for _, n := range nodes {
		inDegree[n.ID] = 0
	}
	for _, targets := range graph {
		for _, t := range targets {
			if _, ok := inDegree[t]; ok {
				inDegree[t]++
			}
		}
	}

	queue := make([]string, 0, len(nodes))
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		for _, t := range graph[id] {
			if _, ok := inDegree[t]; !ok {
				continue
			}
			inDegree[t]--
			if inDegree[t] == 0 {
				queue = append(queue, t)
			}
		}
	}
	if visited == len(nodes) {
		return nil
	}

	remaining := make([]string, 0, len(nodes)-visited)
	for id, deg := range inDegree {
		if deg > 0 {
			remaining = append(remaining, id)
		}
	}
	return remaining
}
