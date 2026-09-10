package localdaemon

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stubComposeCmd overrides runComposeCmd for the duration of the test with
// a fake command-runner seam (per this task's "injectable command-runner
// seam" alternative to a PATH-injected stub script) — never shells out to
// real docker. It reports how many times it was invoked and lets the test
// pick success ("true") or failure ("false").
func stubComposeCmd(t *testing.T, succeed bool) *int {
	t.Helper()
	calls := 0
	orig := runComposeCmd
	runComposeCmd = func(ctx context.Context, args ...string) *exec.Cmd {
		calls++
		if succeed {
			return exec.CommandContext(ctx, "true")
		}
		return exec.CommandContext(ctx, "false")
	}
	t.Cleanup(func() { runComposeCmd = orig })
	return &calls
}

func writeTempComposeFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(path, []byte("services: {}\n"), 0644); err != nil {
		t.Fatalf("writing fake compose file: %v", err)
	}
	return path
}

// spawnDeadPID spawns and waits on a short-lived child process, returning
// its now-dead PID — a definitively-dead PID for a stale-pidfile test.
func spawnDeadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn child process in this environment: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("waiting for child process: %v", err)
	}
	return pid
}

func TestComposeSupervisor_Start_RefusesSecondStart_WithoutInvokingDockerAgain(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "daemon.pid")
	if err := writePIDFile(pidFile, os.Getpid()); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}

	calls := stubComposeCmd(t, true)
	s := &ComposeSupervisor{ComposeFile: writeTempComposeFile(t), PidFile: pidFile}

	err := s.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("Start() error = %v, want an 'already running' error", err)
	}
	if *calls != 0 {
		t.Fatalf("runComposeCmd called %d times, want 0", *calls)
	}
}

func TestComposeSupervisor_Start_StalePIDFile_AllowsFreshStart(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "daemon.pid")
	if err := writePIDFile(pidFile, spawnDeadPID(t)); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}

	calls := stubComposeCmd(t, true)
	s := &ComposeSupervisor{ComposeFile: writeTempComposeFile(t), PidFile: pidFile}

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v, want nil (stale pidfile must not block a fresh start)", err)
	}
	if *calls != 1 {
		t.Fatalf("runComposeCmd called %d times, want 1", *calls)
	}

	gotPID, running := readPIDFile(pidFile)
	if !running || gotPID != os.Getpid() {
		t.Fatalf("pidfile after Start: pid=%d running=%v, want this test process's own pid and running=true", gotPID, running)
	}
}

func TestComposeSupervisor_Stop_RemovesPIDFile_EvenWhenComposeDownFails(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "daemon.pid")
	if err := writePIDFile(pidFile, os.Getpid()); err != nil {
		t.Fatalf("writePIDFile: %v", err)
	}

	stubComposeCmd(t, false) // `docker compose down` stub exits non-zero
	s := &ComposeSupervisor{ComposeFile: writeTempComposeFile(t), PidFile: pidFile}

	err := s.Stop(context.Background())
	if err == nil {
		t.Fatal("Stop() error = nil, want a non-nil error propagated from the failed compose-down stub")
	}
	if _, statErr := os.Stat(pidFile); !os.IsNotExist(statErr) {
		t.Fatalf("pidfile still exists after Stop() despite the failed teardown: statErr=%v", statErr)
	}
}

func TestComposeSupervisor_Start_MissingComposeFile_FailsFastWithoutInvokingDocker(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "daemon.pid")
	missingComposeFile := filepath.Join(dir, "does-not-exist-compose.yml")

	calls := stubComposeCmd(t, true)
	s := &ComposeSupervisor{ComposeFile: missingComposeFile, PidFile: pidFile}

	err := s.Start(context.Background())
	if err == nil {
		t.Fatal("Start() error = nil, want a non-nil error for a missing compose file")
	}
	if !strings.Contains(err.Error(), "compose file not found at") {
		t.Fatalf("Start() error = %q, want it to contain %q", err.Error(), "compose file not found at")
	}
	if !strings.Contains(err.Error(), "10-deployment-infrastructure.md") {
		t.Fatalf("Start() error = %q, want it to reference 10-deployment-infrastructure.md", err.Error())
	}
	if *calls != 0 {
		t.Fatalf("runComposeCmd called %d times, want 0", *calls)
	}
}
