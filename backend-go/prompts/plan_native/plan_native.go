// Package plan_native renders the prompt that asks a model for a Plan as one JSON document (engine native).
package plan_native

import (
	"embed"

	"github.com/stablyai/orca-go/prompts/render"
	"github.com/stablyai/orca-go/prompts/request_context"
)

// ID names this prompt for provenance.
const ID = "plan_native/plan"

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(request_context.Funcs(), files, request_context.Files)

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
	// RetryNote is why the previous answer was rejected.
	RetryNote string
}

// Render builds the Plan prompt. Rules that only some types need (regression test, baseline checks, rollback) are
// added per type so the model is not told about gates that do not apply.
func Render(in Input) (string, error) {
	return set.Execute("plan.tmpl", in)
}
