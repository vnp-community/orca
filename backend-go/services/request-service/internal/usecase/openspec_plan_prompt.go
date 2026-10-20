package usecase

import (
	"fmt"

	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/prompts/plan_openspec"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// OpenSpecPlanPromptInput is everything the agent needs to author the OpenSpec change of a Request. It carries no
// knowledge-graph content: the agent looks that up itself with the tdd-knowledge skill.
type OpenSpecPlanPromptInput struct {
	Request domain.Request
	// Ref is the Request reference written into tasks.md (request=<Ref>), for example REQ-142.
	Ref      string
	ChangeID string
	Shape    domain.PlanShape
	Limits   domain.PlanLimits
	// MinTasks and MaxTasks are the task count the Request's size calls for, inside Limits.
	MinTasks, MaxTasks int
	ProjectName        string
	RepoURL            string
	// Analysis is the approved Solution rendered as text; empty for types that skip analysis.
	Analysis string
	Feedback string
	// PreviousFiles are the files of the version being revised, relative to the change directory; empty on a first run.
	PreviousFiles []string
	// KnowledgeAvailable is false when the project has no usable knowledge graph; the prompt then says so instead of
	// sending the agent to a server that is not there.
	KnowledgeAvailable bool
	// ResultNonce fences the result block the agent ends with.
	ResultNonce string
	// RetryNote lists what was wrong with the previous attempt (parser violations, structure findings).
	RetryNote string
	// Language is the language the prose is written in; the zero value adds no instruction.
	Language output_language.Language
}

// TaskRangeFor is the number of tasks a Request of this size should be split into, inside the project's limits.
// The prompt and the bundle checker both use it, so what the model is told is what the checker enforces.
func TaskRangeFor(size domain.RequestSize, limits domain.PlanLimits) (minTasks, maxTasks int) {
	return plan_openspec.TaskRange(string(size), limits.MaxTasksTotal)
}

// OpenSpecRef is how a Request is named inside tasks.md.
func OpenSpecRef(number int64) string { return fmt.Sprintf("REQ-%d", number) }

// RenderOpenSpecPlanPrompt maps the domain input to prompts/plan_openspec.
func RenderOpenSpecPlanPrompt(in OpenSpecPlanPromptInput) (string, error) {
	return builtPrompt(plan_openspec.Render(plan_openspec.Input{
		Request: promptRequest(in.Request), Project: promptProject(in.ProjectName, in.RepoURL), Ref: in.Ref, ChangeID: in.ChangeID,
		Phases:   in.Shape.AllowsPhases(),
		Limits:   plan_openspec.Limits{MaxPhases: in.Limits.MaxPhases, MaxTasksPerContainer: in.Limits.MaxTasksPerContainer, MaxTitleRunes: in.Limits.MaxTitleRunes},
		MinTasks: in.MinTasks, MaxTasks: in.MaxTasks, Analysis: in.Analysis, Feedback: in.Feedback, PreviousFiles: in.PreviousFiles,
		KnowledgeAvailable: in.KnowledgeAvailable, ResultNonce: in.ResultNonce, RetryNote: in.RetryNote, Language: in.Language,
	}))
}
