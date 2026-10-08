package domain

import (
	"fmt"
	"strings"
)

// flattenProposalTasks lists top-level tasks first, then each phase's tasks; depends_on_indices point into this order.
func flattenProposalTasks(p PlanProposal) []TaskProposal {
	out := append([]TaskProposal(nil), p.Tasks...)
	for _, ph := range p.Phases {
		out = append(out, ph.Tasks...)
	}
	return out
}

// EffectiveLabels adds the pre-deploy gate label to irreversible steps, as the plan writer does when it saves them.
func EffectiveLabels(t TaskProposal) []string {
	labels := append([]string(nil), t.Labels...)
	if t.Irreversible && !containsString(labels, PolicyLabelGatePreDeploy) {
		labels = append(labels, PolicyLabelGatePreDeploy)
	}
	return labels
}

func hasEffectiveLabel(t TaskProposal, label string) bool {
	return containsString(EffectiveLabels(t), label)
}

// CheckRunbook enforces the ops_request plan rules: a rollback task exists, and every irreversible step has a
// rollback after it (transitively) or a "rollback_note:" line in its description.
func CheckRunbook(p PlanProposal) error {
	tasks := flattenProposalTasks(p)
	hasRollback := false
	for _, t := range tasks {
		if hasEffectiveLabel(t, PolicyLabelRollback) {
			hasRollback = true
		}
	}
	if !hasRollback {
		return ErrRunbookRollbackMissing("an ops_request plan needs at least one task labelled rollback")
	}
	for i, t := range tasks {
		if !t.Irreversible {
			continue
		}
		if !hasEffectiveLabel(t, PolicyLabelGatePreDeploy) {
			return ErrRunbookIrreversibleStepUngated(fmt.Sprintf("irreversible step %q has no %s gate", t.Title, PolicyLabelGatePreDeploy))
		}
		if hasRollbackNote(t.Description) || hasRollbackAfter(tasks, i) {
			continue
		}
		return ErrRunbookRollbackMissing(fmt.Sprintf("irreversible step %q has no rollback task after it and no rollback_note", t.Title))
	}
	return nil
}

// hasRollbackAfter is true when a rollback task depends, directly or through other tasks, on tasks[step].
func hasRollbackAfter(tasks []TaskProposal, step int) bool {
	reaches := make([]bool, len(tasks))
	reaches[step] = true
	for changed := true; changed; {
		changed = false
		for i, t := range tasks {
			if reaches[i] {
				continue
			}
			for _, d := range t.DependsOnIndices {
				if d >= 0 && d < len(tasks) && reaches[d] {
					reaches[i], changed = true, true
					break
				}
			}
		}
	}
	for i, t := range tasks {
		if i != step && reaches[i] && hasEffectiveLabel(t, PolicyLabelRollback) {
			return true
		}
	}
	return false
}

func hasRollbackNote(description string) bool {
	const prefix = "rollback_note:"
	for _, line := range strings.Split(description, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) && strings.TrimSpace(strings.TrimPrefix(line, prefix)) != "" {
			return true
		}
	}
	return false
}
