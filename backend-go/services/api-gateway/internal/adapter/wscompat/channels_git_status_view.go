package wscompat

import gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"

// gitStatusEntryView/gitStatusResultView mirror the frontend's
// GitStatusEntry/GitStatusResult (frontend/src/shared/git-status-types.ts).
// git.status must answer with `entries`, never the raw proto's `files`:
// useGit.ts iterates `status.entries` and crashed the Git tab on the raw shape
// (BUG-020).
type gitStatusEntryView struct {
	Path           string `json:"path"`
	Status         string `json:"status"`
	Area           string `json:"area"`
	OldPath        string `json:"oldPath,omitempty"`
	ConflictKind   string `json:"conflictKind,omitempty"`
	ConflictStatus string `json:"conflictStatus,omitempty"`
	Added          *int32 `json:"added,omitempty"`
	Removed        *int32 `json:"removed,omitempty"`
}

type gitStatusUpstreamView struct {
	HasUpstream  bool   `json:"hasUpstream"`
	UpstreamName string `json:"upstreamName,omitempty"`
	Ahead        int32  `json:"ahead"`
	Behind       int32  `json:"behind"`
}

type gitStatusResultView struct {
	Entries           []gitStatusEntryView   `json:"entries"`
	ConflictOperation string                 `json:"conflictOperation"`
	Head              string                 `json:"head,omitempty"`
	Branch            string                 `json:"branch,omitempty"`
	UpstreamStatus    *gitStatusUpstreamView `json:"upstreamStatus,omitempty"`
	DidHitLimit       bool                   `json:"didHitLimit,omitempty"`
	StatusLength      int32                  `json:"statusLength,omitempty"`
}

func toGitStatusResultView(resp *gitgatewayv1.GetStatusResponse) gitStatusResultView {
	files := resp.GetFiles()
	entries := make([]gitStatusEntryView, 0, len(files))
	for _, f := range files {
		entries = append(entries, gitStatusEntryView{
			Path:           f.GetPath(),
			Status:         frontendFileStatus(f.GetState()),
			Area:           stagingAreaOf(f),
			OldPath:        f.GetOldPath(),
			ConflictKind:   f.GetConflictKind(),
			ConflictStatus: f.GetConflictStatus(),
			Added:          f.Added,
			Removed:        f.Removed,
		})
	}
	view := gitStatusResultView{
		Entries:           entries,
		ConflictOperation: resp.GetConflictOperation(),
		Head:              resp.GetHead(),
		Branch:            resp.GetBranch(),
		DidHitLimit:       resp.GetDidHitLimit(),
		StatusLength:      resp.GetStatusLength(),
	}
	if view.ConflictOperation == "" {
		view.ConflictOperation = "unknown" // GitConflictOperation has no empty value
	}
	if u := resp.GetUpstream(); u != nil {
		view.UpstreamStatus = &gitStatusUpstreamView{
			HasUpstream: u.GetHasUpstream(), UpstreamName: u.GetUpstreamName(), Ahead: u.GetAhead(), Behind: u.GetBehind(),
		}
	}
	return view
}

// stagingAreaOf falls back for executors that do not report an area.
func stagingAreaOf(f *gitgatewayv1.FileStatus) string {
	if area := f.GetArea(); area != "" {
		return area
	}
	if f.GetState() == "untracked" {
		return "untracked"
	}
	return "unstaged"
}

// frontendFileStatus maps the host-local executor's "conflicted" onto the
// frontend's GitFileStatus, which has no such value (conflicts travel in
// conflictKind).
func frontendFileStatus(state string) string {
	if state == "conflicted" {
		return "modified"
	}
	return state
}
