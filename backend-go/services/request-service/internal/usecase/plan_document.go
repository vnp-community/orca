package usecase

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// planDocument is the JSON shape a Plan proposal has on the wire between the model, request-service and the run's
// stored result. It mirrors domain.PlanProposal; keeping it separate keeps json tags out of the domain.
type planDocument struct {
	Title   string          `json:"title"`
	Summary string          `json:"summary,omitempty"`
	Phases  []phaseDocument `json:"phases,omitempty"`
	Tasks   []taskDocument  `json:"tasks,omitempty"`
	Notes   string          `json:"notes,omitempty"`
}

type phaseDocument struct {
	Title                 string         `json:"title"`
	Description           string         `json:"description,omitempty"`
	DependsOnPhaseIndices []int          `json:"depends_on_phase_indices,omitempty"`
	Tasks                 []taskDocument `json:"tasks"`
}

type taskDocument struct {
	Title            string   `json:"title"`
	Description      string   `json:"description,omitempty"`
	TaskType         string   `json:"task_type,omitempty"`
	EstimatedHours   float64  `json:"estimated_hours,omitempty"`
	PromptTemplate   string   `json:"prompt_template,omitempty"`
	DependsOnIndices []int    `json:"depends_on_indices,omitempty"`
	Labels           []string `json:"labels,omitempty"`
	Irreversible     bool     `json:"irreversible,omitempty"`
	// Satisfies lists the acceptance criteria a task serves (ac= in an OpenSpec tasks.md); the model-written plan leaves it empty.
	Satisfies []string `json:"satisfies,omitempty"`
}

func (d planDocument) proposal() domain.PlanProposal {
	p := domain.PlanProposal{Title: d.Title, Summary: d.Summary, Notes: d.Notes, Tasks: tasksOf(d.Tasks)}
	for _, ph := range d.Phases {
		p.Phases = append(p.Phases, domain.PhaseProposal{
			Title: ph.Title, Description: ph.Description, DependsOnPhaseIndices: ph.DependsOnPhaseIndices, Tasks: tasksOf(ph.Tasks),
		})
	}
	return p
}

func tasksOf(in []taskDocument) []domain.TaskProposal {
	var out []domain.TaskProposal
	for _, t := range in {
		out = append(out, domain.TaskProposal{
			Title: t.Title, Description: t.Description, TaskType: t.TaskType, EstimatedHours: t.EstimatedHours, PromptTemplate: t.PromptTemplate,
			DependsOnIndices: t.DependsOnIndices, Labels: t.Labels, Irreversible: t.Irreversible, Satisfies: t.Satisfies,
		})
	}
	return out
}

func documentOf(p domain.PlanProposal) planDocument {
	d := planDocument{Title: p.Title, Summary: p.Summary, Notes: p.Notes, Tasks: taskDocsOf(p.Tasks)}
	for _, ph := range p.Phases {
		d.Phases = append(d.Phases, phaseDocument{Title: ph.Title, Description: ph.Description, DependsOnPhaseIndices: ph.DependsOnPhaseIndices, Tasks: taskDocsOf(ph.Tasks)})
	}
	return d
}

func taskDocsOf(in []domain.TaskProposal) []taskDocument {
	var out []taskDocument
	for _, t := range in {
		out = append(out, taskDocument{
			Title: t.Title, Description: t.Description, TaskType: t.TaskType, EstimatedHours: t.EstimatedHours, PromptTemplate: t.PromptTemplate,
			DependsOnIndices: t.DependsOnIndices, Labels: t.Labels, Irreversible: t.Irreversible, Satisfies: t.Satisfies,
		})
	}
	return out
}

// ParsePlanOutput turns the model's text into a normalized proposal. It checks JSON shape only; the caller then
// validates the proposal against the Request's plan shape.
func ParsePlanOutput(text string) (domain.PlanProposal, error) {
	obj, err := ExtractJSONObject(text)
	if err != nil {
		return domain.PlanProposal{}, fmt.Errorf("the reply holds no JSON object: %w", err)
	}
	var doc planDocument
	if err := json.Unmarshal(obj, &doc); err != nil {
		return domain.PlanProposal{}, fmt.Errorf("the JSON does not match the Plan schema: %w", err)
	}
	return domain.NormalizePlanProposal(doc.proposal()), nil
}

// EncodePlanProposal is the canonical, secret-redacted JSON a successful run stores.
func EncodePlanProposal(p domain.PlanProposal) (string, error) {
	raw, err := json.Marshal(documentOf(p))
	if err != nil {
		return "", err
	}
	redacted, _, err := RedactDocument(raw)
	return string(redacted), err
}

// DecodePlanProposal reads back what EncodePlanProposal stored.
func DecodePlanProposal(stored string) (domain.PlanProposal, error) {
	if stored == "" {
		return domain.PlanProposal{}, errors.New("the run stored no proposal")
	}
	var doc planDocument
	if err := json.Unmarshal([]byte(stored), &doc); err != nil {
		return domain.PlanProposal{}, fmt.Errorf("stored proposal is unreadable: %w", err)
	}
	return domain.NormalizePlanProposal(doc.proposal()), nil
}
