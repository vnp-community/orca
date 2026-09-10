package command

import (
	"context"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/localdaemon"
)

type DaemonMode int

const (
	ModeRemote DaemonMode = iota // default: --api-url points at a deployed api-gateway
	ModeLocal                    // --local: this host's docker-compose stack
)

type DaemonStatusResult struct {
	Mode   string `json:"mode"`
	Status string `json:"status"`
	Ready  bool   `json:"ready,omitempty"`
	PID    int    `json:"pid,omitempty"`
}

// RunDaemonStatus never lets the two modes bleed into each other: ModeLocal
// never calls cli (nil-safe — the caller passes nil for cli in --local
// mode so a coding mistake here fails loudly, not silently), ModeRemote
// never touches sup.
func RunDaemonStatus(ctx context.Context, mode DaemonMode, cli *apiclient.Client, sup *localdaemon.ComposeSupervisor) (DaemonStatusResult, error) {
	if mode == ModeRemote {
		health, err := cli.GetHealth(ctx)
		if err != nil {
			return DaemonStatusResult{}, err // exit 1 — unreachable gateway
		}
		status := "healthy"
		if !health.Healthy {
			status = "unhealthy"
		}
		return DaemonStatusResult{Mode: "remote", Status: status, Ready: health.Ready}, nil
	}

	status, err := sup.Status()
	if err != nil {
		return DaemonStatusResult{}, err
	}
	state := "stopped"
	if status.Running {
		state = "running"
	}
	return DaemonStatusResult{Mode: "local", Status: state, PID: status.PID}, nil
}
