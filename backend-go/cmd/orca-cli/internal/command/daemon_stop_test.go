package command

import (
	"context"
	"errors"
	"testing"
)

// TestDaemonStop_RemoteMode_ReturnsRefusalWithoutCallingSupervisor proves
// the GitOps-mode refusal is a first-class tested outcome: passing sup=nil
// for ModeRemote means a mistaken sup.Stop() call would nil-pointer-panic
// this test instead of silently succeeding.
func TestDaemonStop_RemoteMode_ReturnsRefusalWithoutCallingSupervisor(t *testing.T) {
	err := RunDaemonStop(context.Background(), ModeRemote, nil)

	if !errors.Is(err, ErrStopUnsupportedInGitOpsMode) {
		t.Fatalf("RunDaemonStop() error = %v, want ErrStopUnsupportedInGitOpsMode", err)
	}
}
