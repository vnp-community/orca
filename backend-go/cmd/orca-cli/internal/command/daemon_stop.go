package command

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/localdaemon"
)

// ErrStopUnsupportedInGitOpsMode is RunDaemonStop's remote-mode result —
// a deliberate refusal (see this task's Context), not a fallback stub.
var ErrStopUnsupportedInGitOpsMode = errors.New("stopping a GitOps-managed deployment from the CLI is not supported — use kubectl/ArgoCD instead")

func RunDaemonStop(ctx context.Context, mode DaemonMode, sup *localdaemon.ComposeSupervisor) error {
	if mode == ModeRemote {
		return ErrStopUnsupportedInGitOpsMode
	}
	return sup.Stop(ctx)
}
