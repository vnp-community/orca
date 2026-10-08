package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ApprovalGates are the checks a solution or plan approval handler runs before it lets an approval through:
// the chosen option must be backed by an effective decision for the same digest, and blocking questions or
// assumptions needing confirmation must have been answered through a clarification. diagnosis, findings and
// answer documents have no options, so their handlers do not call CheckSolution.
type ApprovalGates struct {
	decisions      DecisionRepository
	clarifications ClarificationRepository
}

func NewApprovalGates(decisions DecisionRepository, clarifications ClarificationRepository) *ApprovalGates {
	return &ApprovalGates{decisions: decisions, clarifications: clarifications}
}

// CheckDecision: the live decision of the solution must be effective and carry digest.
func (g *ApprovalGates) CheckDecision(ctx context.Context, solutionID, digest string) error {
	d, err := g.decisions.GetLiveBySubject(ctx, domain.DecisionSubjectSolutionOption, solutionID)
	if err != nil {
		return err
	}
	switch {
	case d == nil:
		return domain.ErrDecisionNotEffective("no decision was recorded for this solution; choose an option first")
	case d.Status == domain.DecisionStatusChosen:
		return domain.ErrDecisionNotEffective("the chosen option is high risk: confirm it by typing its title first")
	case d.Status != domain.DecisionStatusEffective:
		return domain.ErrDecisionNotEffective("the decision is " + string(d.Status))
	case d.SubjectDigest != digest:
		return domain.ErrDecisionNotEffective("the decision no longer matches the solution; choose the option again")
	}
	return nil
}

// CheckSolution runs CheckDecision and then the blocking-question rule on the solution document.
func (g *ApprovalGates) CheckSolution(ctx context.Context, requestID, solutionID string, optionsJSON []byte, digest string) error {
	if err := g.CheckDecision(ctx, solutionID, digest); err != nil {
		return err
	}
	var doc struct {
		OpenQuestions []struct {
			ID       string `json:"id"`
			Blocking bool   `json:"blocking"`
		} `json:"open_questions"`
	}
	if err := json.Unmarshal(optionsJSON, &doc); err != nil {
		// Legacy documents list questions as plain strings and cannot block.
		return nil
	}
	var ids []string
	for _, q := range doc.OpenQuestions {
		if q.Blocking {
			ids = append(ids, q.ID)
		}
	}
	unanswered, err := g.unanswered(ctx, requestID, domain.ClarificationSourceSolutionOpenQuestion, solutionID, "open_question:", ids)
	if err != nil {
		return err
	}
	if len(unanswered) > 0 {
		return domain.ErrSolutionBlockingQuestions(unanswered)
	}
	return nil
}

// CheckPlan: assumptions with needs_confirmation must be answered through a plan_assumption clarification on planRef.
func (g *ApprovalGates) CheckPlan(ctx context.Context, requestID, planRef string, planJSON []byte) error {
	var doc struct {
		Assumptions []struct {
			ID                string `json:"id"`
			NeedsConfirmation bool   `json:"needs_confirmation"`
		} `json:"assumptions"`
	}
	if err := json.Unmarshal(planJSON, &doc); err != nil {
		return nil
	}
	var ids []string
	for _, a := range doc.Assumptions {
		if a.NeedsConfirmation {
			ids = append(ids, a.ID)
		}
	}
	unanswered, err := g.unanswered(ctx, requestID, domain.ClarificationSourcePlanAssumption, planRef, "assumption:", ids)
	if err != nil {
		return err
	}
	if len(unanswered) > 0 {
		return domain.ErrPlanUnconfirmedAssumptions(unanswered)
	}
	return nil
}

// unanswered returns the ids with no answered question keyed prefix+id in a clarification of that source and reference.
func (g *ApprovalGates) unanswered(ctx context.Context, requestID string, source domain.ClarificationSource, ref, prefix string, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	list, err := g.clarifications.List(ctx, ClarificationListFilter{RequestID: requestID, Status: domain.ClarificationStatusAnswered, Limit: 200})
	if err != nil {
		return nil, err
	}
	answered := map[string]bool{}
	for _, c := range list {
		if c.Source != source || c.SourceRef != ref {
			continue
		}
		for _, q := range c.Questions {
			if q.HasAnswer() {
				answered[q.QuestionKey] = true
			}
		}
	}
	var out []string
	for _, id := range ids {
		if !answered[prefix+id] {
			out = append(out, id)
		}
	}
	return out, nil
}
