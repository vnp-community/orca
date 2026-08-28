package localdaemon

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// runComposeCmd is a seam for tests — production code always shells out to
// the real `docker` binary; tests override this var to an injectable
// command-runner stub instead of invoking real docker, per this package's
// own test file's doc comment.
var runComposeCmd = func(ctx context.Context, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "docker", args...)
}

// ComposeSupervisor shells out to `docker compose` against the repo-root
// docker-compose.yml — it never starts a Go process directly and never
// embeds service logic; the compose file remains the one source of truth
// for what "all services" means, so this supervisor can't drift from
// `make dev-up`'s own definition of the stack.
type ComposeSupervisor struct {
	ComposeFile string // repo-root docker-compose.yml
	PidFile     string // ~/.local/share/orca/daemon.pid — BR-CLI-10
}

type Status struct {
	Running bool
	PID     int
}

// Start brings the compose stack up. Records THIS supervisor process's own
// PID (os.Getpid()) — not any individual service's — see pidfile.go's doc
// comment for why: there is no single "the daemon process" among N
// containers.
func (s *ComposeSupervisor) Start(ctx context.Context) error {
	if pid, running := readPIDFile(s.PidFile); running {
		return fmt.Errorf("orca serve already running (pid %d) — stop it first or use --force", pid)
	}
	if _, err := os.Stat(s.ComposeFile); err != nil {
		return fmt.Errorf("compose file not found at %s — see 10-deployment-infrastructure.md's Local development section: %w", s.ComposeFile, err)
	}

	cmd := runComposeCmd(ctx, "compose", "-f", s.ComposeFile, "up", "-d")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("starting compose stack: %w", err)
	}
	return writePIDFile(s.PidFile, os.Getpid())
}

// Stop tears the compose stack down. Removes the PID file even if
// `docker compose down` fails — best-effort cleanup: a failed teardown
// should not permanently wedge Start into refusing to run again.
func (s *ComposeSupervisor) Stop(ctx context.Context) error {
	defer os.Remove(s.PidFile)
	cmd := runComposeCmd(ctx, "compose", "-f", s.ComposeFile, "down")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("stopping compose stack: %w", err)
	}
	return nil
}

// Status reports the local supervisor's recorded PID and whether it's
// still alive — cross-checks readPIDFile's liveness probe so a crashed
// supervisor with a stale PID file self-corrects to "not running" rather
// than a false positive.
func (s *ComposeSupervisor) Status() (Status, error) {
	pid, running := readPIDFile(s.PidFile)
	return Status{Running: running, PID: pid}, nil
}
