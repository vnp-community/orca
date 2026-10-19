package localgit

import (
	"strings"

	"github.com/stablyai/orca-go/services/git-gateway-service/internal/domain"
)

// porcelainStatusArgs lists untracked files one by one (the default mode
// collapses a new directory into "tests/", hiding the files Source Control
// must show) and keeps non-ASCII paths unescaped. Both flags predate Git 2.25.
var porcelainStatusArgs = []string{
	"-c", "core.quotePath=false", "status", "--porcelain=v1", "-b", "--untracked-files=all",
}

// parsePorcelainStatus parses `git status --porcelain=v1 -b` output. The
// first line is "## <branch>[...tracking info]"; subsequent lines are two
// status-code characters (index, worktree), a space, and the path. A path
// changed in both the index and the working tree yields two rows, one per
// area, like the Dev Server Agent's git.status.
func parsePorcelainStatus(out string) domain.GitStatus {
	var status domain.GitStatus
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		if i == 0 && strings.HasPrefix(line, "## ") {
			status.Branch = parseBranchLine(strings.TrimPrefix(line, "## "))
			continue
		}
		if len(line) < 4 {
			continue
		}
		status.Files = append(status.Files, porcelainRows(line[:2], line[3:])...)
	}
	return status
}

// porcelainRows turns one `XY path` line into its status rows.
func porcelainRows(code, rawPath string) []domain.FileStatus {
	path, oldPath := splitRenamedPath(rawPath)
	if code == "??" {
		return []domain.FileStatus{{Path: path, State: domain.FileStateUntracked, Area: "untracked"}}
	}
	if kind := conflictKindFromPorcelainCode(code); kind != "" {
		return []domain.FileStatus{{
			Path: path, State: domain.FileStateConflicted, Area: "unstaged",
			ConflictKind: kind, ConflictStatus: "unresolved",
		}}
	}
	var rows []domain.FileStatus
	if x := code[0]; x != ' ' && x != '!' {
		rows = append(rows, domain.FileStatus{Path: path, State: fileStateFromPorcelainChar(x), Area: "staged", OldPath: oldPath})
	}
	if y := code[1]; y != ' ' && y != '!' {
		rows = append(rows, domain.FileStatus{Path: path, State: fileStateFromPorcelainChar(y), Area: "unstaged", OldPath: oldPath})
	}
	return rows
}

// splitRenamedPath splits v1's "old -> new" form of a rename/copy row.
func splitRenamedPath(raw string) (path, oldPath string) {
	if old, renamed, ok := strings.Cut(raw, " -> "); ok {
		return renamed, old
	}
	return raw, ""
}

// parseBranchLine extracts the branch name from porcelain -b's header line,
// stripping tracking info (e.g. "main...origin/main [ahead 1]" -> "main")
// and the "no branch" / detached-HEAD case's own formatting.
func parseBranchLine(header string) string {
	if idx := strings.Index(header, "..."); idx >= 0 {
		header = header[:idx]
	}
	if idx := strings.Index(header, " "); idx >= 0 {
		header = header[:idx]
	}
	return header
}

// conflictKindFromPorcelainCode mirrors the agent's parseConflictKind; empty
// means the code is not an unmerged state.
func conflictKindFromPorcelainCode(code string) string {
	switch code {
	case "UU":
		return "both_modified"
	case "AA":
		return "both_added"
	case "DD":
		return "both_deleted"
	case "AU":
		return "added_by_us"
	case "UA":
		return "added_by_them"
	case "DU":
		return "deleted_by_us"
	case "UD":
		return "deleted_by_them"
	}
	return ""
}

// fileStateFromPorcelainChar maps one side (index or worktree) of a porcelain
// v1 XY code to domain.FileState; unknown letters (T typechange...) read as
// modified, like the agent's parseStatusChar.
func fileStateFromPorcelainChar(c byte) domain.FileState {
	switch c {
	case 'A':
		return domain.FileStateAdded
	case 'D':
		return domain.FileStateDeleted
	case 'R':
		return domain.FileStateRenamed
	case 'C':
		return domain.FileStateCopied
	default:
		return domain.FileStateModified
	}
}
