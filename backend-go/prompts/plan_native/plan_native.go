// Package plan_native renders the prompt that asks a model for a Plan as one JSON document (engine native).
package plan_native

import (
	"embed"
	"text/template"

	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/prompts/render"
	"github.com/stablyai/orca-go/prompts/request_context"
)

// ID names this prompt for provenance.
const ID = "plan_native/plan"

//go:embed *.tmpl
var files embed.FS

// PriorPlanRunes caps the Plan shown to the model for a revision; a longer one is cut before it is fenced.
const PriorPlanRunes = 20000

var set = render.NewSet(mergeFuncs(request_context.Funcs(), template.FuncMap{"priorPlanRunes": func() int { return PriorPlanRunes }}), files, request_context.Files)

// Limits are the project's bounds on the Plan tree.
type Limits struct {
	MaxPhases            int
	MaxTasksPerContainer int
	MaxTasksTotal        int
	MaxTitleRunes        int
}

// Labels are the task labels the Plan policy checks for. The service passes its own constants so the prompt and the
// validator cannot name different labels.
type Labels struct {
	TestRegression  string
	CheckBaseline   string
	CheckAfter      string
	CheckTestsPre   string
	CheckTestsAfter string
	Rollback        string
}

// Input carries only data the model may see; like analysis.Input it has no field for a credential.
type Input struct {
	Request request_context.Request
	Project request_context.Project
	// Phases says the Plan is split into phases; otherwise the tasks sit directly in the Plan.
	Phases bool
	Limits Limits
	Labels Labels
	// Analysis is the approved Solution or diagnosis rendered as text; empty for types that skip analysis.
	Analysis string
	// Feedback is the reviewer's guidance when a Plan is regenerated.
	Feedback string
	// PriorPlan is the Plan being revised, as the JSON the model returned for it; empty for a first Plan.
	PriorPlan string
	// RetryNote is why the previous answer was rejected.
	RetryNote string
	// Language is the language the model writes its prose in; the zero value adds no instruction.
	Language output_language.Language
}

// Render builds the Plan prompt. Rules that only some types need (regression test, baseline checks, rollback) are
// added per type so the model is not told about gates that do not apply.
func Render(in Input) (string, error) {
	return set.Execute("plan.tmpl", in)
}

func mergeFuncs(a, b template.FuncMap) template.FuncMap {
	for k, v := range b {
		a[k] = v
	}
	return a
}
