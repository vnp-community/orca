package usecase

import (
	"github.com/stablyai/orca-go/prompts/analysis"
	"github.com/stablyai/orca-go/prompts/clarification_answers"
	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// SolutionPromptInput carries only data the model may see. It has no credential field by design: a prompt must never hold secrets.
type SolutionPromptInput struct {
	Kind           domain.SolutionKind
	Request        domain.Request
	ProjectName    string
	RepoURL        string
	MinOptions     int
	PriorArtifacts []PriorArtifact
	// Answers are the reviewer's answers to earlier open questions of this Request.
	Answers []PromptAnswer
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

// RenderSolutionPrompt maps the domain input to prompts/analysis. The wording lives in the templates, not here.
func RenderSolutionPrompt(in SolutionPromptInput) (string, error) {
	return builtPrompt(analysis.Render(analysis.Input{
		Kind: string(in.Kind), Request: promptRequest(in.Request), Project: promptProject(in.ProjectName, in.RepoURL),
		MinOptions: in.MinOptions, PriorArtifacts: promptPriorArtifacts(in.PriorArtifacts), Answers: promptAnswers(in.Answers),
		Feedback: in.Feedback, RetryNote: in.RetryNote, AgentSolution: in.AgentSolution, UseKnowledge: in.UseKnowledge,
		Language: in.Language,
	}))
}

func promptPriorArtifacts(arts []PriorArtifact) []analysis.PriorArtifact {
	out := make([]analysis.PriorArtifact, len(arts))
	for i, a := range arts {
		out[i] = analysis.PriorArtifact{Kind: a.Kind, Status: a.Status, Content: a.Content, Feedback: a.Feedback}
	}
	return out
}

func promptAnswers(answers []PromptAnswer) []clarification_answers.Answer {
	out := make([]clarification_answers.Answer, len(answers))
	for i, a := range answers {
		out[i] = clarification_answers.Answer{Key: a.Key, Question: a.Question, Answer: a.Answer}
	}
	return out
}
