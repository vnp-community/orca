package wscompat

import "strings"

// Browsing a directory with thousands of entries (e.g. macOS $TMPDIR) made
// the browser dialog freeze and the reply huge; the list is capped here and
// the caller is told via truncated/totalEntries (additive JSON fields).
const (
	browseDirDefaultLimit = 500
	browseDirMaxLimit     = 2000
)

// browseDirOptions are the optional, backward-compatible request fields of
// devServer.browseDir and files.browseServerDir.
type browseDirOptions struct {
	Limit           int    `json:"limit"`
	DirectoriesOnly bool   `json:"directoriesOnly"`
	Query           string `json:"query"`
}

func (o browseDirOptions) effectiveLimit() int {
	switch {
	case o.Limit <= 0:
		return browseDirDefaultLimit
	case o.Limit > browseDirMaxLimit:
		return browseDirMaxLimit
	default:
		return o.Limit
	}
}

// browseDirRelayEntry is one entry of the agent's fs.readDir reply.
type browseDirRelayEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// buildBrowseDirView applies the filters then the cap, keeping the agent's
// order (directories first). totalEntries counts entries after filtering.
func buildBrowseDirView(resolvedPath string, relay []browseDirRelayEntry, opts browseDirOptions) devServerBrowseDirResultView {
	query := strings.ToLower(opts.Query)
	limit := opts.effectiveLimit()
	entries := make([]devServerBrowseDirEntryView, 0, min(len(relay), limit))
	total := 0
	for _, e := range relay {
		isDir := e.Type == "directory"
		if opts.DirectoriesOnly && !isDir {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(e.Name), query) {
			continue
		}
		total++
		if len(entries) < limit {
			// fs.readDir does not report symlink-ness — see
			// devServer.browseDir's doc comment (channels.go).
			entries = append(entries, devServerBrowseDirEntryView{Name: e.Name, IsDirectory: isDir})
		}
	}
	return devServerBrowseDirResultView{ResolvedPath: resolvedPath, Entries: entries, Truncated: total > len(entries), TotalEntries: total}
}
