// Package outcome_review_context renders the prompt of a context run: a read-only agent reads the knowledge graph and the SAD
// (and, after coding, the code) and returns the design and the diagram models of an Outcome Review.
package outcome_review_context

import (
	"embed"
	"fmt"
	"text/template"

	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/prompts/render"
	"github.com/stablyai/orca-go/prompts/request_context"
)

// ID names this prompt for provenance.
const ID = "outcome_review_context/context"

// Version is part of the design lock and the context digest: change it whenever the wording or the contract changes, so a design
// extracted with the old text is not reused.
const Version = "outcome_review_context/1"

// Modes of a run.
const (
	ModeDesign    = "design"
	ModeDelivered = "delivered"
)

// Rune caps of the text written by outside parties.
const (
	PathRunes        = 200
	ViolationRunes   = 300
	DesignLabelRunes = 60
	MaxChangedPaths  = 300
	MaxViolations    = 30
	MaxDesignRows    = 60
)

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(funcs(), files, request_context.Files)

func funcs() template.FuncMap {
	f := request_context.Funcs()
	f["pathRunes"] = func() int { return PathRunes }
	f["violationRunes"] = func() int { return ViolationRunes }
	f["labelRunes"] = func() int { return DesignLabelRunes }
	return f
}

// ChangedGroup is the changed files under one top-level directory, the hint of where to start reading.
type ChangedGroup struct {
	Dir     string
	Files   int
	Added   int
	Removed int
}

// DesignContainer is a row of the design that was already extracted, shown so the delta uses the same ids.
type DesignContainer struct {
	ID    string
	Label string
	Kind  string
}

// Head is the commit of one task branch.
type Head struct {
	Branch string
	Commit string
}

// Input carries only data the model may see. It has no credential field by design.
type Input struct {
	Mode    string
	Request request_context.Request
	Project request_context.Project
	// UseKnowledge says the agent can reach the knowledge graph; without it the run is code only.
	UseKnowledge bool
	// LockedDesign is set in delivered mode when the design was extracted before: the agent must not read the graph again.
	LockedDesign []DesignContainer
	BaseRef      string
	Heads        []Head
	Groups       []ChangedGroup
	// ChangedPaths is the changed file catalogue, cut to MaxChangedPaths by the caller.
	ChangedPaths []string
	// PlanTargets are the files the Plan says it will touch (design mode).
	PlanTargets []string
	// MaxFiles is the budget of files the agent may open.
	MaxFiles int
	// ResultNonce fences the result block.
	ResultNonce string
	// Violations are what Orca found wrong in the previous reply; empty on the first attempt.
	Violations []string
	Language   output_language.Language
}

type view struct {
	Input
	Version string
	Design  bool
	Locked  bool
}

// Render builds the prompt. The nonce and the mode are required: without them the reply cannot be read.
func Render(in Input) (string, error) {
	if err := render.Require(map[string]string{"ResultNonce": in.ResultNonce, "Mode": in.Mode}); err != nil {
		return "", err
	}
	if in.Mode != ModeDesign && in.Mode != ModeDelivered {
		return "", fmt.Errorf("outcome_review_context: mode %q is not design or delivered", in.Mode)
	}
	if in.MaxFiles < 1 {
		return "", fmt.Errorf("outcome_review_context: MaxFiles must be at least 1")
	}
	in.ChangedPaths = firstN(in.ChangedPaths, MaxChangedPaths)
	in.PlanTargets = firstN(in.PlanTargets, MaxChangedPaths)
	in.Violations = firstN(in.Violations, MaxViolations)
	in.LockedDesign = firstN(in.LockedDesign, MaxDesignRows)
	return set.Execute("context.tmpl", view{Input: in, Version: Version, Design: in.Mode == ModeDesign, Locked: in.Mode == ModeDelivered && len(in.LockedDesign) > 0})
}

func firstN[T any](in []T, n int) []T {
	if len(in) > n {
		return in[:n]
	}
	return in
}
