// Package git_commit_message renders the ai.complete prompt that asks a model for a commit message, and the file-stats body
// that replaces the diff when a change is too large to send in full.
package git_commit_message

import (
	"embed"
	"strings"
	"text/template"

	"github.com/stablyai/orca-go/prompts/render"
)

// IDs name the two prompts for provenance.
const (
	ID          = "git_commit_message/commit_message"
	StatsOnlyID = "git_commit_message/stats_only"
)

// shaChars is how many characters of a commit id the style context shows.
const shaChars = 7

//go:embed *.tmpl
var files embed.FS

var set = render.NewSet(template.FuncMap{}, files)

// Commit is a recent commit shown so the model can match the project's style.
type Commit struct {
	SHA     string
	Message string
}

// Input is the already-fetched context. Diff is the staged diff, or the output of RenderStatsOnly. The prompt has no block
// that wraps the diff, so there is nothing to fence.
type Input struct {
	Branch string
	Recent []Commit
	// IssueRef is the ticket the message must cite in a trailing Refs line; empty means the prompt does not ask for one.
	IssueRef string
	Diff     string
}

type commitLine struct {
	SHA     string
	Summary string
}

type view struct {
	Input
	Lines []commitLine
}

// Render returns the prompt. A commit shows its first 7 bytes of id and the first line of its message.
func Render(in Input) (string, error) {
	v := view{Input: in}
	for _, c := range in.Recent {
		sha := c.SHA
		if len(sha) > shaChars {
			sha = sha[:shaChars]
		}
		v.Lines = append(v.Lines, commitLine{SHA: sha, Summary: firstLine(c.Message)})
	}
	return set.Execute("commit_message.tmpl", v)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// File is one changed file in the stats-only body.
type File struct {
	State string
	Path  string
}

// StatsOnlyInput is the change list sent instead of the diff.
type StatsOnlyInput struct {
	Files []File
}

// RenderStatsOnly returns one line per changed file and no diff content.
func RenderStatsOnly(in StatsOnlyInput) (string, error) {
	return set.Execute("stats_only.tmpl", in)
}
