//go:build !windows

package localdaemon

import (
	"os"
	"os/exec"
	"syscall"
)

// Daemonize re-execs the current process detached from the controlling
// terminal (setsid) — the standard POSIX double-fork-equivalent for a Go
// process, since Go's exec.Cmd has no direct "daemonize in place"
// primitive. Returns immediately in the parent; the child continues as
// the backgrounded supervisor.
func Daemonize(args []string) error {
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	return cmd.Start()
}
