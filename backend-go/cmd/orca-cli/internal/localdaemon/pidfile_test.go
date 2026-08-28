package localdaemon

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestPidfile_RoundTrip_OwnPID_ReturnsRunningTrue proves write then read
// round-trips the same PID and reports running=true for the test
// process's own (definitely live) PID.
func TestPidfile_RoundTrip_OwnPID_ReturnsRunningTrue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "orca.pid")
	ownPID := os.Getpid()

	if err := writePIDFile(path, ownPID); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}

	gotPID, running := readPIDFile(path)
	if gotPID != ownPID {
		t.Fatalf("pid = %d, want %d", gotPID, ownPID)
	}
	if !running {
		t.Fatal("running = false, want true for the test's own PID")
	}
}

// TestPidfile_CorruptContent_ReturnsNotRunningWithoutPanic proves a
// non-numeric pidfile self-corrects to "not running" instead of panicking
// or erroring.
func TestPidfile_CorruptContent_ReturnsNotRunningWithoutPanic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orca.pid")
	if err := os.WriteFile(path, []byte("not-a-pid"), 0600); err != nil {
		t.Fatalf("writing corrupt pidfile: %v", err)
	}

	pid, running := readPIDFile(path)
	if running {
		t.Fatal("running = true, want false for corrupt content")
	}
	if pid != 0 {
		t.Fatalf("pid = %d, want 0", pid)
	}
}

// TestPidfile_EmptyFile_ReturnsNotRunning mirrors the corrupt-content case
// for a zero-byte file.
func TestPidfile_EmptyFile_ReturnsNotRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orca.pid")
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("writing empty pidfile: %v", err)
	}

	_, running := readPIDFile(path)
	if running {
		t.Fatal("running = true, want false for an empty file")
	}
}

// TestPidfile_MissingFile_ReturnsNotRunning proves a nonexistent path is
// "not running", not an error.
func TestPidfile_MissingFile_ReturnsNotRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.pid")

	pid, running := readPIDFile(path)
	if running || pid != 0 {
		t.Fatalf("pid=%d running=%v, want 0/false for a missing file", pid, running)
	}
}

// TestPidfile_DeadPID_ReturnsNotRunning spawns a short-lived child
// process, waits for it to exit, then proves its now-dead PID reads back
// as not running — a definitively-dead PID, not a guess at an unused
// number.
func TestPidfile_DeadPID_ReturnsNotRunning(t *testing.T) {
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn child process in this environment: %v", err)
	}
	deadPID := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("waiting for child process: %v", err)
	}

	path := filepath.Join(t.TempDir(), "orca.pid")
	if err := writePIDFile(path, deadPID); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}

	_, running := readPIDFile(path)
	if running {
		t.Fatalf("running = true for dead pid %d, want false", deadPID)
	}
}
