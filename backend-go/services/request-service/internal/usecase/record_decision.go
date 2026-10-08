package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type RecordDecisionInput struct {
	RequestID     string
	RequestNumber int64
	SolutionID    string
	// OptionsJSON is the solution options document; its options become the decision's choices.
	OptionsJSON  []byte
	ChosenOption string
	ChooserID    string
	Rationale    string
	// SubjectDigest is the digest of the pending approval after this choice (DigestOptions with the chosen index).
	SubjectDigest string
	// ReporterID and SelfChoiceAllowed carry the approval setting: with self approval off, the reporter cannot choose.
	ReporterID        string
	SelfChoiceAllowed bool
	// HighRiskServices overrides the proposed threshold of affected services (0 uses the default).
	HighRiskServices int
}

// DecisionPayload is the wire shape of orca.request.decision.recorded|confirmed; it holds no rationale.
type DecisionPayload struct {
	DecisionID string `json:"decision_id"`
	RequestID  string `json:"request_id"`
	DisplayID  string `json:"display_id"`
	SubjectID  string `json:"subject_id"`
	RiskLevel  string `json:"risk_level"`
	Status     string `json:"status"`
}

// RecordDecision writes the choice of a solution option inside the transaction of ChooseSolutionOption,
// so a refused choice (for example a missing rationale) leaves no trace anywhere.
type RecordDecision struct {
	decisions        DecisionRepository
	outbox           OutboxWriter
	clock            func() time.Time
	highRiskServices int
}

// WithHighRiskServices sets the default service-count threshold (REQUEST_DECISION_HIGH_RISK_SERVICES);
// an input's own HighRiskServices still wins.
func (uc *RecordDecision) WithHighRiskServices(n int) *RecordDecision {
	uc.highRiskServices = n
	return uc
}

func NewRecordDecision(decisions DecisionRepository, outbox OutboxWriter) *RecordDecision {
	return &RecordDecision{decisions: decisions, outbox: outbox, clock: func() time.Time { return time.Now().UTC() }}
}

func (uc *RecordDecision) WithClock(c func() time.Time) *RecordDecision {
	uc.clock = c
	return uc
}

func (uc *RecordDecision) Execute(ctx context.Context, in RecordDecisionInput) (domain.Decision, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return domain.Decision{}, domain.ErrRequestTenantRequired()
	}
	if !in.SelfChoiceAllowed && in.ReporterID != "" && in.ChooserID == in.ReporterID {
		return domain.Decision{}, domain.ErrDecisionSelfChoiceForbidden()
	}
	threshold := in.HighRiskServices
	if threshold == 0 {
		threshold = uc.highRiskServices
	}
	options, risks, recommended, reason, err := decisionOptions(in.OptionsJSON, threshold)
	if err != nil {
		return domain.Decision{}, err
	}
	risk, ok := risks[in.ChosenOption]
	if !ok {
		return domain.Decision{}, domain.ErrClarificationInvalidAnswer("decision", "unknown option "+in.ChosenOption)
	}
	now := uc.clock()
	d, err := uc.decisions.GetLiveBySubject(ctx, domain.DecisionSubjectSolutionOption, in.SolutionID)
	if err != nil {
		return domain.Decision{}, err
	}
	created := d == nil
	if created {
		seq, err := uc.decisions.NextSeq(ctx, in.RequestID)
		if err != nil {
			return domain.Decision{}, err
		}
		d = &domain.Decision{
			ID: uuid.NewString(), TenantID: mustTenant(ctx), RequestID: in.RequestID, Seq: seq,
			SubjectKind: domain.DecisionSubjectSolutionOption, SubjectID: in.SolutionID,
			Question: fmt.Sprintf("Chọn phương án cho %s", domain.FormatRequestID(in.RequestNumber)),
			Options:  options, RecommendedOptionID: recommended, RecommendationReason: reason,
			Status: domain.DecisionStatusOpen, RiskLevel: domain.RiskNormal, Version: 1, CreatedAt: now,
		}
		if err := uc.decisions.Insert(ctx, *d); err != nil {
			return domain.Decision{}, err
		}
	}
	action, err := d.Choose(in.ChosenOption, in.ChooserID, in.Rationale, risk, in.SubjectDigest, now)
	if err != nil {
		return domain.Decision{}, err
	}
	saved, err := uc.decisions.Update(ctx, *d, d.Version)
	if err != nil {
		return domain.Decision{}, err
	}
	if err := uc.decisions.AppendHistory(ctx, domain.DecisionHistory{
		ID: uuid.NewString(), DecisionID: saved.ID, Action: action, OptionID: in.ChosenOption, ActorID: in.ChooserID, Rationale: saved.Rationale, At: now,
	}); err != nil {
		return domain.Decision{}, err
	}
	ev, err := NewOutboxEvent(ctx, domain.SubjectDecisionRecorded, DecisionPayload{
		DecisionID: saved.ID, RequestID: saved.RequestID, DisplayID: saved.DisplayID(in.RequestNumber), SubjectID: saved.SubjectID,
		RiskLevel: string(saved.RiskLevel), Status: string(saved.Status),
	})
	if err != nil {
		return domain.Decision{}, err
	}
	ev.OccurredAt = now
	return saved, uc.outbox.InsertOutboxEvent(ctx, ev)
}

func mustTenant(ctx context.Context) string {
	id, _ := tenant.TenantID(ctx)
	return id
}

// decisionOptions derives the decision's choices, each option's risk level, and the recommendation from a solution options document.
func decisionOptions(raw []byte, highRiskServices int) ([]domain.DecisionOption, map[string]domain.RiskLevel, string, string, error) {
	var doc struct {
		Options        []json.RawMessage `json:"options"`
		Recommendation struct {
			OptionID string `json:"option_id"`
			Reason   string `json:"reason"`
		} `json:"recommendation"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, nil, "", "", domain.ErrRequestContentInvalid("solution options: " + err.Error())
	}
	risks := map[string]domain.RiskLevel{}
	var out []domain.DecisionOption
	for _, o := range doc.Options {
		var head struct {
			ID      string `json:"id"`
			Title   string `json:"title"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal(o, &head); err != nil || head.ID == "" {
			return nil, nil, "", "", domain.ErrRequestContentInvalid("a solution option has no id")
		}
		signals, err := domain.RiskSignalsFromOption(o)
		if err != nil {
			return nil, nil, "", "", err
		}
		level := domain.DecisionRisk{}.Assess(signals, highRiskServices)
		risks[head.ID] = level
		opt := domain.DecisionOption{ID: head.ID, Label: head.Title, Summary: head.Summary, Risk: domain.DecisionRiskInfo{Level: string(level)}}
		if signals.BreakingChange {
			opt.Risk.Reasons = append(opt.Risk.Reasons, "breaking_change")
		}
		if signals.HighSeverity {
			opt.Risk.Reasons = append(opt.Risk.Reasons, "high_severity_risk")
		}
		if signals.AffectedService > 0 && level == domain.RiskHigh && !signals.BreakingChange && !signals.HighSeverity {
			opt.Risk.Reasons = append(opt.Risk.Reasons, "many_services")
		}
		out = append(out, opt)
	}
	return out, risks, doc.Recommendation.OptionID, doc.Recommendation.Reason, nil
}
