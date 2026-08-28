// Package localdaemon wraps the repo-root docker-compose stack for
// `orca serve --daemon`/`orca daemon status --local`/`orca daemon stop
// --local` — a thin local-convenience layer, not a production daemon
// model. See this package's compose_supervisor.go doc comment for why.
package localdaemon

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// readPIDFile reads path and reports whether the recorded PID still
// corresponds to a live process. A missing, empty, or corrupt file is
// "not running", not an error — self-correcting rather than surfacing a
// stale-file crash to the caller.
func readPIDFile(path string) (pid int, running bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, processAlive(pid)
}

// writePIDFile records pid at path, creating parent directories as
// needed. 0600 — no sensitive data, but matches this CLI's other on-disk
// state (credentials.json) for consistency.
func writePIDFile(path string, pid int) error {
	// filepath.Dir (not a hand-rolled '/'-split) per AGENTS.md's
	// cross-platform-support rule — this path may come from
	// os.UserConfigDir() and use '\' on Windows.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0600)
}
