// Package task_execution renders the prompt that runs one task on the agent and the instruction that tells the agent how to
// end a contract run with a result block.
package task_execution

import (
	"embed"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// IDs name the two prompts for provenance.
const (
	ID            = "task_execution/execute"
	ResultBlockID = "task_execution/result_block_suffix"
)

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{}, files)

// Task is the part of a task the prompt shows. PromptTemplate, when set, replaces the generic opener and is used as written.
type Task struct {
	Title          string
	Description    string
	AIContext      string
	PromptTemplate string
}

// Parent is the task this one is a subtask of.
type Parent struct {
	Title       string
	Description string
}

// Dependency is a task this one depends on that has already finished.
type Dependency struct {
	Title       string
	Description string
}

// Input is everything the execute prompt says. The text is the author's own task text, sent to the agent as written: the
// prompt has no block that wraps it, so there is nothing to fence.
type Input struct {
	Task          Task
	Parent        *Parent
	CompletedDeps []Dependency
}

// Render returns the prompt before `{{outputs.<taskId>.*}}` is resolved: that substitution is data, not wording, and the
// service runs it on the result.
func Render(in Input) (string, error) {
	if err := render.Require(map[string]string{"Task.Title": in.Task.Title}); err != nil {
		return "", err
	}
	return set.Execute("execute.tmpl", in)
}

// ResultBlockInput says how a contract run must report back. Nonce is empty only when the caller has none to give; the
// markers are then printed with nothing after them, as before.
type ResultBlockInput struct {
	Nonce               string
	RequirePassingCheck bool
	KnowledgeHub        bool
}

// RenderResultBlock returns the instruction appended to the execute prompt of a contract run. The schema in it is the one
// the service's result parser reads.
func RenderResultBlock(in ResultBlockInput) (string, error) {
	return set.Execute("result_block_suffix.tmpl", in)
}
