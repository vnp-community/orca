//go:build !windows

package localdaemon

import "syscall"

// processAlive sends signal 0 — delivers no actual signal, just checks
// whether the kernel would allow sending one (i.e. the pid exists and is
// owned by this user or root).
func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}
