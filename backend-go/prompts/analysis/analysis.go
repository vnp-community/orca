// Package analysis renders the prompts that ask for a Solution, a diagnosis, Findings or an Answer about a Request.
package analysis

import (
	"embed"

	"github.com/stablyai/orca-go/prompts/clarification_answers"
	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/prompts/render"
	"github.com/stablyai/orca-go/prompts/request_context"
)

// IDs name each prompt for provenance.
const (
	IDSolution      = "analysis/solution"
	IDAgentSolution = "analysis/agent_solution"
	IDDiagnosis     = "analysis/diagnosis"
	IDFindings      = "analysis/findings"
	IDAnswer        = "analysis/answer"
)

// Document kinds. Any other kind is rendered as a solution, like the prompt did before it moved here.
const (
	KindSolution  = "solution"
	KindDiagnosis = "diagnosis"
	KindFindings  = "findings"
	KindAnswer    = "answer"
)

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(request_context.Funcs(), files, request_context.Files)

// PriorArtifact is an earlier Solution of the same Request, shown so a regenerated answer does not repeat a rejected one.
type PriorArtifact struct {
	Kind     string
	Status   string
	Content  string
	Feedback string
}

// Input carries only data the model may see. It has no credential field by design: a prompt must never hold secrets.
type Input struct {
	Kind    string
	Request request_context.Request
	Project request_context.Project
	// MinOptions is the fewest options a Solution must offer; a value below 1 means 1.
	MinOptions     int
	PriorArtifacts []PriorArtifact
	// Answers are the reviewer's answers to earlier open questions of this Request.
	Answers []clarification_answers.Answer
	// Feedback is the reviewer's regenerate hint.
	Feedback string
	// RetryNote is the validation error of the previous attempt.
	RetryNote string
	// AgentSolution is set when a read-only agent, not ai.complete, writes the kind=solution proposal.
	AgentSolution bool
	// UseKnowledge says the agent can reach the project's knowledge graph; without it the agent works from the repository alone.
	UseKnowledge bool
	// Language is the language the model writes its prose in; the zero value adds no instruction.
	Language output_language.Language
}

// view is what the templates read: the Input plus the parts that need Go (a count, a cap over the whole block, a sub-render).
type view struct {
	Input
	MinOptions     int
	Prior          priorView
	Clarifications string
}

// Render builds the prompt for the Input's kind. Kinds other than solution are always read-only agent prompts.
func Render(in Input) (string, error) {
	answers, err := clarification_answers.Render(clarification_answers.Input{Answers: in.Answers})
	if err != nil {
		return "", err
	}
	prior, err := newPriorView(in.PriorArtifacts)
	if err != nil {
		return "", err
	}
	v := view{Input: in, MinOptions: max(in.MinOptions, 1), Prior: prior, Clarifications: answers}
	return set.Execute(templateFor(in), v)
}

func templateFor(in Input) string {
	switch in.Kind {
	case KindDiagnosis:
		return "diagnosis.tmpl"
	case KindFindings:
		return "findings.tmpl"
	case KindAnswer:
		return "answer.tmpl"
	}
	if in.AgentSolution {
		return "agent_solution.tmpl"
	}
	return "solution.tmpl"
}
