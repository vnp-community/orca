package usecase

import (
	"sort"
	"strings"

	"github.com/stablyai/orca-go/prompts/outcome_review_context"
	"github.com/stablyai/orca-go/prompts/outcome_review_draw"
	"github.com/stablyai/orca-go/prompts/output_language"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// ReviewContextPromptVersion is part of the design lock and the context digest; it changes with the prompt's wording.
const ReviewContextPromptVersion = outcome_review_context.Version

// ReviewPromptInput is what a context run shows the agent. It holds no content from the agent's earlier replies except the
// violations Orca found in the last one.
type ReviewPromptInput struct {
	Mode         domain.ContextMode
	Request      domain.Request
	ProjectName  string
	RepoURL      string
	UseKnowledge bool
	// Locked is the design already extracted; set only in delivered mode, where the agent must not read the graph again.
	Locked     *domain.DesignModel
	Facts      domain.Facts
	MaxFiles   int
	Nonce      string
	Violations []string
	Language   output_language.Language
}

// RenderReviewContextPrompt maps the domain input to prompts/outcome_review_context. The wording lives in the templates.
func RenderReviewContextPrompt(in ReviewPromptInput) (string, error) {
	p := outcome_review_context.Input{
		Mode: string(in.Mode), Request: promptRequest(in.Request), Project: promptProject(in.ProjectName, in.RepoURL),
		UseKnowledge: in.UseKnowledge, BaseRef: in.Facts.Subject.BaseRef, MaxFiles: in.MaxFiles, ResultNonce: in.Nonce,
		Violations: in.Violations, Language: in.Language,
	}
	for _, h := range in.Facts.Subject.Heads {
		p.Heads = append(p.Heads, outcome_review_context.Head{Branch: h.Branch, Commit: h.Commit})
	}
	p.Groups, p.ChangedPaths = changedGroups(in.Facts.Changed)
	if in.Mode == domain.ContextModeDesign {
		p.PlanTargets = in.Facts.PlanTargets
	}
	if in.Locked != nil && in.Mode == domain.ContextModeDelivered {
		for _, c := range in.Locked.Containers {
			p.LockedDesign = append(p.LockedDesign, outcome_review_context.DesignContainer{ID: c.ID, Label: c.Label, Kind: c.Kind})
		}
	}
	return builtPrompt(outcome_review_context.Render(p))
}

// changedGroups gives the changed files grouped by their first directory (where to start reading) and the sorted catalogue.
func changedGroups(changed []domain.ChangedPath) ([]outcome_review_context.ChangedGroup, []string) {
	byDir := map[string]*outcome_review_context.ChangedGroup{}
	var paths []string
	for _, c := range changed {
		p := strings.TrimPrefix(strings.ReplaceAll(c.Path, "\\", "/"), "./")
		if p == "" {
			continue
		}
		dir := "."
		if i := strings.IndexByte(p, '/'); i > 0 {
			dir = p[:i]
		}
		g := byDir[dir]
		if g == nil {
			g = &outcome_review_context.ChangedGroup{Dir: dir}
			byDir[dir] = g
		}
		g.Files++
		g.Added += c.Added
		g.Removed += c.Removed
		paths = append(paths, p)
	}
	groups := make([]outcome_review_context.ChangedGroup, 0, len(byDir))
	for _, g := range byDir {
		groups = append(groups, *g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if groups[i].Files != groups[j].Files {
			return groups[i].Files > groups[j].Files
		}
		return groups[i].Dir < groups[j].Dir
	})
	sort.Strings(paths)
	return groups, paths
}

func drawPromptVersionOfModule() string { return outcome_review_draw.Version }
