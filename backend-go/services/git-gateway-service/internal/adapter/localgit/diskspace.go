package localgit

import "golang.org/x/sys/unix"

// checkFreeSpace is [A3]'s soft warning check, local-dispatch only — relay
// dispatch has no Dev Server Agent disk-usage RPC (same absence BUG-009/
// SOL-009 documents for the agent's fs.* method set), so this is never
// called for a relay-resolved worktree. A statfs failure fails OPEN (ok=true)
// — a broken disk-space check must never block worktree creation.
func checkFreeSpace(parentDir string, minBytes uint64) (ok bool, availableBytes uint64, err error) {
	var stat unix.Statfs_t
	if statErr := unix.Statfs(parentDir, &stat); statErr != nil {
		return true, 0, statErr
	}
	available := stat.Bavail * uint64(stat.Bsize)
	return available >= minBytes, available, nil
}
