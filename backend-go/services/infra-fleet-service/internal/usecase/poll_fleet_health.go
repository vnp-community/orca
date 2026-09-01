package usecase

import (
	"context"
	"log/slog"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// PollFleetHealth is the write side of the "30s-cadence poller"
// specs/backend-go/services/infra-fleet-service.md §8 calls for — never
// implemented before this pass (see this service's README "Known gaps"),
// which meant infra.fleet_health had zero rows for every dev server,
// forever, and DevServerReachability.IsReachable (the gate
// dispatchExecutorForRepo and friends use to route git ops to a dev server
// vs execute locally) always returned false — live-reproduced on
// b15.openledger.vn: a genuinely connected, live agent still couldn't be
// used for git dispatch because no sample ever existed for it to read.
//
// Scope cut from the full spec: this only measures reachability
// (DevServerAgentClient.Health — an agent-level handshake check, the same
// primitive IsDevServerConnected's sibling IsConnected peeks at without
// dialing) for every registered dev server, every poll. CPU/RAM/disk/
// latency stay at zero (a valid, honest "not measured yet" input per
// domain.NewDevServerHealth's own invariants — the SSH-exec-based resource
// sampler HasFleetHealthPort.GetFleetHealth's doc comment references is a
// separate, materially larger feature, not addressed here). §8's
// leader-election-per-target/distributed-lock requirement for horizontally
// scaled replicas is also not addressed — this deployment runs a single
// infra-fleet-service instance, and the spec itself flags multi-replica
// fan-out as "an open design question for the Go rewrite's implementation
// phase, not resolved by this doc" — every replica polling every target
// would only ever be a minor efficiency concern (redundant polls), never a
// correctness one, so it's a reasonable follow-on rather than a blocker
// here.
type PollFleetHealth struct {
	devServers FleetHealthPollerRepository
	writer     FleetHealthWriter
	agent      DevServerAgentClient
	logger     *slog.Logger
}

func NewPollFleetHealth(devServers FleetHealthPollerRepository, writer FleetHealthWriter, agent DevServerAgentClient, logger *slog.Logger) *PollFleetHealth {
	if logger == nil {
		logger = slog.Default()
	}
	return &PollFleetHealth{devServers: devServers, writer: writer, agent: agent, logger: logger}
}

// Execute polls every registered dev server once and persists its sample.
// A per-dev-server failure (Health erroring, or the write itself failing)
// is logged and skipped — one unreachable/misbehaving target must never
// stop the rest of the fleet from being polled in the same pass.
func (uc *PollFleetHealth) Execute(ctx context.Context) error {
	devServers, err := uc.devServers.ListAllDevServers(ctx)
	if err != nil {
		return err
	}

	for _, ds := range devServers {
		reachable, err := uc.agent.Health(ctx, ds)
		if err != nil {
			uc.logger.WarnContext(ctx, "poll_fleet_health: health check failed, recording unreachable",
				slog.String("devServerId", ds.ID), slog.Any("error", err))
			reachable = false
		}

		sample, err := domain.NewDevServerHealth(ds.ID, reachable, 0, 0, 0, 0)
		if err != nil {
			uc.logger.WarnContext(ctx, "poll_fleet_health: constructing sample failed, skipping",
				slog.String("devServerId", ds.ID), slog.Any("error", err))
			continue
		}

		if err := uc.writer.UpsertFleetHealth(ctx, sample); err != nil {
			uc.logger.WarnContext(ctx, "poll_fleet_health: persisting sample failed",
				slog.String("devServerId", ds.ID), slog.Any("error", err))
		}
	}

	return nil
}
