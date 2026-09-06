package usecase

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"

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
	outbox     OutboxWriter
	agent      DevServerAgentClient
	logger     *slog.Logger
}

// outbox may be nil — see Execute's "alerting is best-effort" doc comment;
// a nil outbox just means the transition still gets logged, never enqueued
// (used by the few tests/composition paths with no eventbus configured).
func NewPollFleetHealth(devServers FleetHealthPollerRepository, writer FleetHealthWriter, outbox OutboxWriter, agent DevServerAgentClient, logger *slog.Logger) *PollFleetHealth {
	if logger == nil {
		logger = slog.Default()
	}
	return &PollFleetHealth{devServers: devServers, writer: writer, outbox: outbox, agent: agent, logger: logger}
}

// Execute polls every registered dev server once and persists its sample.
// A per-dev-server failure (Health erroring, or the write itself failing)
// is logged and skipped — one unreachable/misbehaving target must never
// stop the rest of the fleet from being polled in the same pass.
//
// Admin alerting on a reachable=true -> false transition is best-effort:
// the WARN log line is unconditional (visible via `docker logs` today,
// zero new infra — this is what live incidents this session were actually
// diagnosed from), the outbox enqueue is attempted alongside it but a
// failure there is only logged, never allowed to fail the poll. See
// domain.DevServerDisconnectedSubject's doc comment for why the outbox
// event carries no recipient user IDs yet.
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

		// Read the previous sample BEFORE UpsertFleetHealth overwrites it —
		// this is the only way to see a true->false edge at all, since
		// UpsertFleetHealth is a plain upsert with no history. A read
		// failure (e.g. no sample exists yet for a brand-new dev server)
		// just disables the transition check for this one poll; it must
		// never block recording the current sample.
		previous, hadPrevious, err := uc.writer.GetDevServerHealth(ctx, ds.ID)
		if err != nil {
			uc.logger.WarnContext(ctx, "poll_fleet_health: reading previous sample failed, skipping transition check",
				slog.String("devServerId", ds.ID), slog.Any("error", err))
			hadPrevious = false
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
			continue
		}

		if hadPrevious && previous.Reachable && !sample.Reachable {
			uc.alertDevServerDisconnected(ctx, ds)
		}
	}

	return nil
}

// alertDevServerDisconnected fires exactly once per true->false edge (never
// on repeated false samples — Execute only calls this when the PREVIOUS
// sample was reachable) — a dev server down for an hour alerts once, not
// every 30s.
func (uc *PollFleetHealth) alertDevServerDisconnected(ctx context.Context, ds domain.DevServer) {
	uc.logger.WarnContext(ctx, "dev server disconnected",
		slog.String("devServerId", ds.ID), slog.String("host", ds.Host), slog.String("tenantId", ds.TenantID))

	if uc.outbox == nil {
		return
	}
	payload, err := json.Marshal(domain.DevServerDisconnectedPayload{
		DevServerID: ds.ID, Host: ds.Host, TenantID: ds.TenantID,
	})
	if err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: marshaling disconnect alert payload failed",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
		return
	}
	event := domain.OutboxEvent{
		ID:          uuid.NewString(),
		TenantID:    ds.TenantID,
		Subject:     domain.DevServerDisconnectedSubject,
		OccurredAt:  time.Now().UTC(),
		PayloadJSON: payload,
	}
	if err := uc.outbox.InsertOutboxEvent(ctx, event); err != nil {
		uc.logger.WarnContext(ctx, "poll_fleet_health: enqueuing disconnect alert failed",
			slog.String("devServerId", ds.ID), slog.Any("error", err))
	}
}
