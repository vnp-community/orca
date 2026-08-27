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
