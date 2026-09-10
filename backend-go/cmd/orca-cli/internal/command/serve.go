package command

import (
	"context"
	"errors"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/localdaemon"
)

var errServeRequiresLocal = errors.New("orca serve --daemon requires --local — there is no single daemon process to start against a GitOps-managed deployment")

// RunServe starts the local compose stack. --daemon backgrounds the
// process (handled by main.go's platform-specific daemonize step, see
// below); RunServe itself is the foreground body either way — Start
// already returns once `docker compose up -d` reports the containers
// launched, matching "exits once the compose stack reports healthy" per
// SOL-CLI-03's design (a fuller health-poll-before-returning loop can be
// layered on here using apiclient.GetHealth once TASK-CLI-03-01 is
// available to this package without an import cycle).
func RunServe(ctx context.Context, local bool, sup *localdaemon.ComposeSupervisor) error {
	if !local {
		return errServeRequiresLocal
	}
	return sup.Start(ctx)
}
