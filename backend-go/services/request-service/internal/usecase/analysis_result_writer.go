package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// AnalysisResultWriter commits a finished analysis in one transaction: the run, the Solution, the events,
// the Approval and the Request transition succeed or fail together, so a crash never leaves a half-proposed Request.
type AnalysisResultWriter struct {
	requests   RequestRepository
	solutions  SolutionStore
	runs       AnalysisRunStore
	opener     SolutionApprovalOpener
	transition RequestTransitioner
	tx         TxRunner
	outbox     OutboxWriter
	owner      string
	now        func() time.Time
	artifacts  SolutionArtifacts
}

// WithArtifacts stamps provenance on each finished Solution and records its display ids and relations.
func (w *AnalysisResultWriter) WithArtifacts(a SolutionArtifacts) *AnalysisResultWriter {
	w.artifacts = a
	return w
}

func NewAnalysisResultWriter(requests RequestRepository, solutions SolutionStore, runs AnalysisRunStore, opener SolutionApprovalOpener,
	transition RequestTransitioner, tx TxRunner, outbox OutboxWriter, owner string) *AnalysisResultWriter {
	return &AnalysisResultWriter{requests: requests, solutions: solutions, runs: runs, opener: opener, transition: transition, tx: tx, outbox: outbox,
		owner: owner, now: func() time.Time { return time.Now().UTC() }}
}

type PersistAnalysisInput struct {
	Run domain.AnalysisRun
	// Doc is the validated, redacted document that becomes solutions.options.
	Doc []byte
	Raw string
}

type solutionProposedPayload struct {
	TenantID            string `json:"tenant_id"`
	RequestID           string `json:"request_id"`
	SolutionID          string `json:"solution_id"`
	Kind                string `json:"kind"`
	Mode                string `json:"mode"`
	OptionCount         int    `json:"option_count,omitempty"`
	RecommendedOptionID string `json:"recommended_option_id,omitempty"`
}

type solutionApprovedPayload struct {
	RequestID    string `json:"request_id"`
	SolutionID   string `json:"solution_id"`
	Kind         string `json:"kind"`
	ChosenOption *int   `json:"chosen_option,omitempty"`
	Mode         string `json:"mode,omitempty"`
	Auto         bool   `json:"auto,omitempty"`
}

func (w *AnalysisResultWriter) Persist(ctx context.Context, in PersistAnalysisInput) error {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return domain.ErrRequestTenantRequired()
	}
	run := in.Run
	return w.tx.InTx(ctx, func(ctx context.Context) error {
		req, err := w.requests.Get(ctx, run.RequestID)
		if err != nil {
			return err
		}
		if req.Status != domain.RequestStatusAnalyzing {
			// The request moved on (cancelled, retyped) while the model worked; the result no longer applies.
			return newFailure(run, "REQUEST_SOLUTION_REQUEST_MOVED", "request left analyzing while the analysis ran")
		}
		flow, err := domain.FlowFor(req.Type)
		if err != nil {
			return err
		}
		raw := RedactRaw(in.Raw)
		run.Succeed(w.now())
		run.RawOutput = &raw
		if ok, err := w.runs.FinishOwned(ctx, run, w.owner); err != nil {
			return err
		} else if !ok {
			return ErrLeaseLost
		}

		sol, err := w.solutions.Get(ctx, run.SolutionID)
		if err != nil {
			return err
		}
		gate := flow.AnalysisGate
		if gate == domain.GateNone {
			err = sol.AutoApprove(in.Doc)
		} else {
			err = sol.Propose(in.Doc)
		}
		if err != nil {
			return err
		}
		if w.artifacts != nil {
			if err := w.artifacts.Annotate(req, run, &sol); err != nil {
				return err
			}
		}
		saved, err := w.solutions.Update(ctx, sol, sol.Version)
		if err != nil {
			return err
		}
		retired := w.openSiblings(ctx, req.ID, saved)
		if _, err := w.solutions.SupersedeOpen(ctx, req.ID, saved.Kind, saved.ID); err != nil {
			return err
		}
		if w.artifacts != nil {
			if err := w.artifacts.Record(ctx, req, saved, run, retired); err != nil {
				return err
			}
		}
		if err := w.emitProposed(ctx, tenantID, saved, run); err != nil {
			return err
		}
		if gate == domain.GateNone {
			if err := w.emit(ctx, domain.SubjectSolutionApproved, solutionApprovedPayload{
				RequestID: req.ID, SolutionID: saved.ID, Kind: string(saved.Kind), Mode: string(run.Mode), Auto: true,
			}); err != nil {
				return err
			}
		}
		from := domain.RequestStatusAnalyzing
		if _, err := w.transition.Execute(ctx, TransitionInput{
			RequestID: req.ID, Trigger: domain.TriggerAnalysisReady, ExpectedFrom: &from, ActorID: run.ActorID, ActorKind: domain.ActorKindSystem,
		}); err != nil {
			return err
		}
		if gate == domain.GateNone {
			return nil
		}
		// Opened after the transition: an approval records the status it was opened in and is only valid there.
		subject, _ := domain.SubjectTypeForGate(gate)
		digest, err := domain.DigestOptions(saved.OptionsJSON, nil)
		if err != nil {
			return err
		}
		moved, err := w.requests.Get(ctx, req.ID)
		if err != nil {
			return err
		}
		return w.opener.Open(ctx, OpenSolutionApprovalInput{Request: moved, SubjectType: subject, SubjectID: saved.ID, Digest: digest})
	})
}

// openSiblings are the proposed or rejected Solutions of the same kind that SupersedeOpen is about to retire.
func (w *AnalysisResultWriter) openSiblings(ctx context.Context, requestID string, keep domain.Solution) []string {
	if open := openSolutionIDs(ctx, w.solutions, requestID, keep.Kind, keep.ID); len(open) > 0 {
		return open
	}
	// A regenerate retires the old proposal when it starts, so by now nothing is open: the edge goes to the latest earlier one.
	all, err := w.solutions.ListByRequest(ctx, SolutionListFilter{RequestID: requestID, Kind: keep.Kind})
	if err != nil {
		return nil
	}
	var prev *domain.Solution
	for i := range all {
		if s := &all[i]; s.Status == domain.SolutionStatusSuperseded && s.Seq < keep.Seq && (prev == nil || s.Seq > prev.Seq) {
			prev = s
		}
	}
	if prev == nil {
		return nil
	}
	return []string{prev.ID}
}

// openSolutionIDs lists what SupersedeOpen would retire. Best effort: the supersedes edge and the Decision cleanup
// are conveniences, the status change itself never depends on this list.
func openSolutionIDs(ctx context.Context, store SolutionStore, requestID string, kind domain.SolutionKind, keepID string) []string {
	all, err := store.ListByRequest(ctx, SolutionListFilter{RequestID: requestID, Kind: kind})
	if err != nil {
		return nil
	}
	var out []string
	for _, s := range all {
		if s.ID != keepID && (s.Status == domain.SolutionStatusProposed || s.Status == domain.SolutionStatusRejected) {
			out = append(out, s.ID)
		}
	}
	return out
}

func (w *AnalysisResultWriter) emitProposed(ctx context.Context, tenantID string, sol domain.Solution, run domain.AnalysisRun) error {
	p := solutionProposedPayload{TenantID: tenantID, RequestID: sol.RequestID, SolutionID: sol.ID, Kind: string(sol.Kind), Mode: string(run.Mode)}
	if sol.Kind == domain.SolutionKindSolution {
		if o, err := domain.ParseSolutionOptions(sol.OptionsJSON); err == nil {
			p.OptionCount, p.RecommendedOptionID = len(o.Options), o.Recommendation.OptionID
		}
	}
	return w.emit(ctx, domain.SubjectSolutionProposed, p)
}

func (w *AnalysisResultWriter) emit(ctx context.Context, subject string, payload any) error {
	ev, err := NewOutboxEvent(ctx, subject, payload)
	if err != nil {
		return err
	}
	return w.outbox.InsertOutboxEvent(ctx, ev)
}
