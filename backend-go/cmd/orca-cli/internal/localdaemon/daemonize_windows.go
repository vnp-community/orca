//go:build windows

package localdaemon

import (
	"os"
	"os/exec"
	"syscall"
)

// Daemonize on Windows uses CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS —
// the Windows-native equivalent of setsid-based detaching, per AGENTS.md's
// cross-platform-support rule (no POSIX-only assumption).
func Daemonize(args []string) error {
	cmd := exec.Command(os.Args[0], args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000200 | 0x00000008} // CREATE_NEW_PROCESS_GROUP | DETACHED_PROCESS
	return cmd.Start()
}
