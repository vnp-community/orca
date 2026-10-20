package usecase

import (
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/prompts/plan_native"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// PlanPromptInput carries only data the model may see; like SolutionPromptInput it has no field for a credential.
type PlanPromptInput struct {
	Request     domain.Request
	Shape       domain.PlanShape
	Limits      domain.PlanLimits
	ProjectName string
	RepoURL     string
	// Analysis is the approved Solution or diagnosis rendered as text; empty for types that skip analysis.
	Analysis string
	// Feedback is the reviewer's guidance when a Plan is regenerated.
	Feedback string
	// PriorPlan is the Plan being revised, as the run stored it; empty for a first Plan.
	PriorPlan string
	// RetryNote is why the previous answer was rejected.
	RetryNote string
	// Language is the language the model writes its prose in; the zero value adds no instruction.
	Language output_language.Language
}

// RenderPlanPrompt maps the domain input to prompts/plan_native. The labels come from the domain constants the Plan
// validator checks, so the prompt cannot ask for a label the validator does not know.
func RenderPlanPrompt(in PlanPromptInput) (string, error) {
	return builtPrompt(plan_native.Render(plan_native.Input{
		Request: promptRequest(in.Request), Project: promptProject(in.ProjectName, in.RepoURL), Phases: in.Shape.AllowsPhases(),
		Limits: plan_native.Limits{
			MaxPhases: in.Limits.MaxPhases, MaxTasksPerContainer: in.Limits.MaxTasksPerContainer,
			MaxTasksTotal: in.Limits.MaxTasksTotal, MaxTitleRunes: in.Limits.MaxTitleRunes,
		},
		Labels: plan_native.Labels{
			TestRegression: domain.PlanLabelTestRegression, CheckBaseline: domain.PolicyLabelCheckBaseline, CheckAfter: domain.PolicyLabelCheckAfter,
			CheckTestsPre: domain.PolicyLabelCheckTestsPre, CheckTestsAfter: domain.PolicyLabelCheckTestsAfter, Rollback: domain.PolicyLabelRollback,
		},
		Analysis: in.Analysis, Feedback: in.Feedback, PriorPlan: in.PriorPlan, RetryNote: in.RetryNote, Language: in.Language,
	}))
}

// approvedAnalysisText renders the approved Solution for the prompt: the chosen option in full when there was a choice,
// otherwise the whole document.
func approvedAnalysisText(s domain.Solution) string {
	if s.Kind == domain.SolutionKindSolution && s.ChosenOption != nil {
		if opts, err := domain.ParseSolutionOptions(s.OptionsJSON); err == nil && *s.ChosenOption >= 0 && *s.ChosenOption < len(opts.Options) {
			o := opts.Options[*s.ChosenOption]
			var b strings.Builder
			fmt.Fprintf(&b, "chosen option: %s\nsummary: %s\napproach:\n%s\n", o.Title, o.Summary, o.Approach)
			if o.Rollback != "" {
				fmt.Fprintf(&b, "rollback: %s\n", o.Rollback)
			}
			for _, a := range opts.Assumptions {
				fmt.Fprintf(&b, "assumption: %s\n", a.Text)
			}
			return b.String()
		}
	}
	return string(s.OptionsJSON)
}
