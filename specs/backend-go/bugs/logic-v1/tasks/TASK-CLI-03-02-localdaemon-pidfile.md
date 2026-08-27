# TASK-CLI-03-02: `localdaemon.pidfile` — PID file read/write/liveness check

**From Solution:** SOL-CLI-03
**Priority:** P0 — `ComposeSupervisor` (TASK-CLI-03-03) depends on this
**Service:** `orca-cli`
**File:** `backend-go/cmd/orca-cli/internal/localdaemon/pidfile.go`
**Depends on:** TASK-CLI-01-06 (scaffold — establishes the module)
**Status:** `[ ]` TODO

---

## Context

BR-CLI-10's PID file, scoped honestly per SOL-CLI-03: it records the local supervisor process's own PID (the one process that corresponds to "the `orca serve` invocation"), not any of the 17 individual service PIDs — there is no single "the daemon process" among N containers. A crashed supervisor with a stale PID file must self-correct to "stopped", not report a false positive.

## Changes to make

`backend-go/cmd/orca-cli/internal/localdaemon/pidfile.go`:

```go
// Package localdaemon wraps the repo-root docker-compose stack for
// `orca serve --daemon`/`orca daemon status --local`/`orca daemon stop
// --local` — a thin local-convenience layer, not a production daemon
// model. See this package's compose_supervisor.go doc comment for why.
package localdaemon

import (
	"os"
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
	if err := os.MkdirAll(dirOf(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0600)
}

func dirOf(path string) string {
	i := strings.LastIndexByte(path, '/')
	if i < 0 {
		return "."
	}
	return path[:i]
}
```

`processAlive(pid int) bool` needs a platform-specific implementation per `AGENTS.md`'s cross-platform-support rule — two files:

`backend-go/cmd/orca-cli/internal/localdaemon/process_unix.go` (`//go:build !windows`):

```go
//go:build !windows

package localdaemon

import "syscall"

// processAlive sends signal 0 — delivers no actual signal, just checks
// whether the kernel would allow sending one (i.e. the pid exists and is
// owned by this user or root).
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
```

`backend-go/cmd/orca-cli/internal/localdaemon/process_windows.go` (`//go:build windows`):

```go
//go:build windows

package localdaemon

import "golang.org/x/sys/windows"

// processAlive on Windows: os.FindProcess/Process.Signal has no real
// liveness-probe semantics there (Signal only reliably supports os.Kill,
// which would actually terminate the process) — OpenProcess succeeding
// with PROCESS_QUERY_LIMITED_INFORMATION is the standard non-destructive
// existence check.
func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	return true
}
```

This is a new `golang.org/x/sys` dependency for this module (grep the repo first — `grep -rl golang.org/x/sys backend-go/*/go.sum` — to confirm no existing pin conflicts before adding it to `go.mod`). No existing `//go:build windows` file elsewhere in this repo to match against, confirmed by search — this is new precedent, so keep the platform split minimal (just this one function) rather than building out a broader Windows process-management abstraction here.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./cmd/orca-cli/...
go test ./cmd/orca-cli/internal/localdaemon/... -run TestPidfile -v
```

Expected new test `pidfile_test.go`: round-trip write/read returns the same PID and `running=true` for the test's own PID (`os.Getpid()`); a corrupt file (non-numeric content) and an empty file both return `running=false`, not a panic/error; a PID file naming a definitely-dead PID (e.g. a very high, almost-certainly-unused number, or a PID obtained by spawning and then waiting on a short-lived child process) returns `running=false`.
