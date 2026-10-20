// Package plan_openspec renders the prompt that asks an agent to author the OpenSpec change of a Request (engine openspec).
package plan_openspec

import (
	"embed"
	"text/template"

	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/prompts/render"
	"github.com/stablyai/orca-go/prompts/request_context"
)

// ID names this prompt for provenance.
const ID = "plan_openspec/plan"

// RetryNoteRunes caps the findings of the previous attempt; they list parser and structure violations, which run long.
const RetryNoteRunes = 3000

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(mergeFuncs(request_context.Funcs(), template.FuncMap{"retryNoteRunes": func() int { return RetryNoteRunes }}), files, request_context.Files)

// Limits are the project's bounds on the Plan tree.
type Limits struct {
	MaxPhases            int
	MaxTasksPerContainer int
	MaxTitleRunes        int
}

// Input is everything the agent needs to author the OpenSpec change of a Request. It carries no knowledge-graph content:
// the agent looks that up itself with the tdd-knowledge skill.
type Input struct {
	Request request_context.Request
	Project request_context.Project
	// Ref is the Request reference written into tasks.md (request=<Ref>), for example REQ-142.
	Ref      string
	ChangeID string
	// Phases says the tasks are grouped into phases (## PH-1, ...); otherwise they sit under one ## TASKS heading.
	Phases bool
	Limits Limits
	// MinTasks and MaxTasks are the task count the Request's size calls for, inside the project's limits; see TaskRange.
	MinTasks, MaxTasks int
	// Analysis is the approved Solution rendered as text; empty for types that skip analysis.
	Analysis string
	Feedback string
	// PreviousFiles are the files of the version being revised, relative to the change directory. Empty on a first run,
	// and then the prompt says nothing about a revision.
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

// Render builds the prompt. ChangeID, Ref and ResultNonce are required: the agent writes under the change directory and
// the run is read back by the nonce, so a prompt without them would send it to the wrong place.
func Render(in Input) (string, error) {
	if err := render.Require(map[string]string{"ChangeID": in.ChangeID, "Ref": in.Ref, "ResultNonce": in.ResultNonce}); err != nil {
		return "", err
	}
	return set.Execute("plan.tmpl", in)
}

func mergeFuncs(a, b template.FuncMap) template.FuncMap {
	for k, v := range b {
		a[k] = v
	}
	return a
}
