package grpcclient

import "github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"

// gitStatusWire mirrors the Dev Server Agent's git.status / git.submoduleStatus
// result (agent/src/shared/git-status-types.ts GitStatusResult). The rows are
// under "entries", not "files": decoding the agent reply straight into
// domain.GitStatus silently dropped every row, so the Source Control panel and
// the worktree-delete safety check saw a clean tree on every relayed host.
type gitStatusWire struct {
	Entries           []gitStatusEntryWire `json:"entries"`
	ConflictOperation string               `json:"conflictOperation"`
	Head              string               `json:"head"`
	Branch            string               `json:"branch"`
	UpstreamStatus    *struct {
		HasUpstream  bool   `json:"hasUpstream"`
		UpstreamName string `json:"upstreamName"`
		Ahead        int    `json:"ahead"`
		Behind       int    `json:"behind"`
	} `json:"upstreamStatus"`
	DidHitLimit  bool `json:"didHitLimit"`
	StatusLength int  `json:"statusLength"`
}

// gitStatusEntryWire is one GitUncommittedEntry. added/removed are pointers
// because the agent omits them for binary files and an untracked file only
// carries "added".
type gitStatusEntryWire struct {
	Path           string `json:"path"`
	Status         string `json:"status"`
	Area           string `json:"area"`
	OldPath        string `json:"oldPath"`
	ConflictKind   string `json:"conflictKind"`
	ConflictStatus string `json:"conflictStatus"`
	Added          *int   `json:"added"`
	Removed        *int   `json:"removed"`
}

func (w gitStatusWire) toDomain() domain.GitStatus {
	status := domain.GitStatus{
		Branch:            w.Branch,
		Files:             make([]domain.FileStatus, 0, len(w.Entries)),
		ConflictOperation: w.ConflictOperation,
		Head:              w.Head,
		DidHitLimit:       w.DidHitLimit,
		StatusLength:      w.StatusLength,
	}
	if u := w.UpstreamStatus; u != nil {
		status.Upstream = &domain.StatusUpstream{
			HasUpstream: u.HasUpstream, UpstreamName: u.UpstreamName, Ahead: u.Ahead, Behind: u.Behind,
		}
	}
	for _, e := range w.Entries {
		status.Files = append(status.Files, domain.FileStatus{
			Path: e.Path, State: domain.FileState(e.Status), Area: e.Area, OldPath: e.OldPath,
			ConflictKind: e.ConflictKind, ConflictStatus: e.ConflictStatus,
			Added: e.Added, Removed: e.Removed,
		})
	}
	return status
}
