package tddagentclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// ApprovalStore is the part of usecase.StepApprovalRepository the approval step uses.
type ApprovalStore interface {
	UpsertPendingStepApproval(ctx context.Context, a domain.StepApproval) (domain.StepApproval, error)
	LatestStepApproval(ctx context.Context, tenantID, executionID, stepID string) (domain.StepApproval, error)
	MarkStepApprovalSubmitted(ctx context.Context, tenantID, id string, at time.Time) error
}

const approvalKind = "tdd_agent"

// Approval waits for a person in Orca to decide the gate the run waits at, then
// delivers the decision to tdd-agent (TDD §7.5). Reading run_get as the Orca
// client records the review receipt tdd-agent requires before an approval.
func (h *Handlers) Approval(ctx context.Context, raw string) (domain.StepResult, error) {
	e, spec, err := h.env(ctx, ActionApproval, raw, 3*24*time.Hour)
	if err != nil {
		return finish(ActionApproval, nil, err)
	}
	out := map[string]any{"action": ActionApproval, "runId": e.p.RunID, "gate": e.p.Gate, "projectDir": e.projectDir}
	if e.p.Gate == "" || e.p.Role == "" {
		return finish(ActionApproval, out, fail(CodeInvalidParams, "gate and role are required"))
	}
	if !e.hasIdent {
		return finish(ActionApproval, out, fail(CodeInvalidParams, "an approval step must run inside a workflow execution"))
	}
	projectID := ""
	if len(e.target) > len("project:") && e.target[:len("project:")] == "project:" {
		projectID = e.target[len("project:"):]
	}
	var row domain.StepApproval
	started := time.Now()
	err = poll(ctx, h.clock, spec, h.cancelled(ctx, e), func() (bool, error) {
		v, err := h.runGet(ctx, e, e.p.RunID)
		if err != nil {
			return false, err
		}
		task := v.Tasks[e.p.Gate]
		if task.Status == "approved" {
			// approved through this step (possibly before a restart) or outside Orca, e.g. with the tdd-agent CLI
			if latest, lerr := h.approvals.LatestStepApproval(ctx, e.tenantID, e.ident.ExecutionID, e.ident.StepID); lerr == nil &&
				latest.Status == domain.StepApprovalApproved {
				out["approvedBy"], out["approvalId"], out["decisionHash"] = latest.ResolvedBy, latest.ID, latest.DecisionHash
				return true, h.markSubmitted(ctx, e, &latest)
			}
			out["approvedBy"] = "external"
			return true, nil
		}
		switch v.status() {
		case RunFailed, RunBlocked:
			return true, fail(CodeRunFailed, fmt.Sprintf("run %s: %v", v.status(), v.Run.Result.Blockers))
		case RunAwaitingApproval:
		default:
			return false, nil // the run has not reached the gate yet
		}
		if v.Run.Result.PendingGate != e.p.Gate {
			return true, fail(CodeUnexpectedGate, fmt.Sprintf("run waits for approval of %q, the template expects %q", v.Run.Result.PendingGate, e.p.Gate))
		}
		if task.DecisionHash == "" {
			return false, nil
		}
		subject, _ := json.Marshal(map[string]string{"runId": e.p.RunID, "gate": e.p.Gate, "role": e.p.Role})
		decision, _ := json.Marshal(task.Decision)
		pending, perr := domain.NewStepApproval(uuid.NewString(), e.tenantID, e.ident.ExecutionID, e.ident.StepID, projectID, approvalKind,
			subject, decision, task.DecisionHash, time.Now().UTC())
		if perr != nil {
			return true, fail(CodeInvalidParams, perr.Error())
		}
		if row, err = h.approvals.UpsertPendingStepApproval(ctx, pending); err != nil {
			return true, fmt.Errorf("tdd_agent.approval: store: %w", err)
		}
		out["approvalId"], out["decisionHash"] = row.ID, row.DecisionHash
		switch row.Status {
		case domain.StepApprovalPending:
			return false, nil
		case domain.StepApprovalRejected:
			h.metrics.approvalWait.WithLabelValues(e.p.Gate, "rejected").Observe(time.Since(row.CreatedAt).Seconds())
			out["rejectedBy"], out["rationale"] = row.ResolvedBy, row.Rationale
			return true, fail(CodeApprovalReject, "rejected in Orca: "+row.Rationale)
		case domain.StepApprovalApproved:
			return h.submit(ctx, e, &row, out)
		}
		return false, nil
	})
	h.cancelRun(ctx, e, e.p.RunID, err)
	h.metrics.waited(ActionApproval, started, err)
	return finish(ActionApproval, out, err)
}

// submit delivers an approval made in Orca; the run continues with the next advance step.
func (h *Handlers) submit(ctx context.Context, e stepEnv, row *domain.StepApproval, out map[string]any) (bool, error) {
	rationale := fmt.Sprintf("orca:user=%s exec=%s step=%s: %s", row.ResolvedBy, e.ident.ExecutionID, e.ident.StepID, row.Rationale)
	args := map[string]any{"run_id": e.p.RunID, "gate": e.p.Gate, "role": e.p.Role, "decision_hash": row.DecisionHash,
		"rationale": rationale, "advance": false}
	var res struct {
		Accepted  bool   `json:"accepted"`
		RunStatus string `json:"run_status"`
	}
	if err := h.call(ctx, e, "approval_submit", args, &res); err != nil {
		var te *ToolError
		if errors.As(err, &te) {
			if te.Code == "NOT_AWAITING_APPROVAL" {
				return false, nil // approved meanwhile (e.g. our own submit before a restart): the next look decides
			}
			return true, fail(CodeApprovalRefused, "tdd-agent refused the approval: "+te.Code+": "+te.Message)
		}
		return false, err
	}
	out["approvedBy"], out["runStatus"] = row.ResolvedBy, res.RunStatus
	h.metrics.approvalWait.WithLabelValues(e.p.Gate, "approved").Observe(time.Since(row.CreatedAt).Seconds())
	return true, h.markSubmitted(ctx, e, row)
}

func (h *Handlers) markSubmitted(ctx context.Context, e stepEnv, row *domain.StepApproval) error {
	if row.SubmittedAt != nil {
		return nil
	}
	now := time.Now().UTC()
	if err := h.approvals.MarkStepApprovalSubmitted(ctx, e.tenantID, row.ID, now); err != nil {
		return fmt.Errorf("tdd_agent.approval: mark submitted: %w", err)
	}
	row.MarkSubmitted(now)
	return nil
}
