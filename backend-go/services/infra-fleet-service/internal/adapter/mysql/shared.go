package mysql

import (
	"github.com/stablyai/orca-go/common/outbox"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshrelay"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/usecase"
)

// rowScanner abstracts over *sql.Row/*sql.Rows — both expose Scan(...any)
// error, letting a single-row Get and a multi-row List share one
// column-order-sensitive scan function instead of duplicating it. Mirrors
// internal/adapter/postgres's identical local interface (defined there over
// pgx.Row/pgx.Rows instead).
type rowScanner interface {
	Scan(dest ...any) error
}

// Compile-time interface conformance checks for every port this package
// implements — internal/adapter/postgres only asserts 2 of its types this
// way; this package asserts all of them (checklist §3 of
// BE-DB-SOL-017-infra-fleet-service-mysql-tidb-adapter.md) so a signature
// mismatch against any usecase port fails the build immediately, not at
// cmd/server/main.go wiring time.
var (
	_ usecase.DevServerRepository                  = (*Repository)(nil)
	_ usecase.ConnectionRepository                 = (*Repository)(nil)
	_ usecase.ConnectionResolver                   = (*Repository)(nil)
	_ usecase.FleetHealthPort                      = (*Repository)(nil)
	_ usecase.FleetHealthWriter                    = (*Repository)(nil)
	_ usecase.PollLockPort                         = (*Repository)(nil)
	_ usecase.FleetConnectivityRepository          = (*Repository)(nil)
	_ usecase.FleetHealthPollerRepository          = (*Repository)(nil)
	_ usecase.OutboxWriter                         = (*Repository)(nil)
	_ outbox.Store                                 = (*Repository)(nil)
	_ usecase.SshTargetRepository                  = (*SshTargetStore)(nil)
	_ sshrelay.SshTargetResolver                   = (*SshTargetStore)(nil)
	_ usecase.PortForwardRepository                = (*PortForwardStore)(nil)
	_ usecase.DevServerGroupRepository             = (*DevServerGroupStore)(nil)
	_ usecase.DevServerGroupGrantRepository        = (*DevServerGroupGrantStore)(nil)
	_ usecase.DevServerAccessRequestRepository     = (*DevServerAccessRequestStore)(nil)
	_ usecase.BrowserProfileRepository             = (*BrowserProfileStore)(nil)
	_ usecase.AgentTokenRepository                 = (*AgentTokenStore)(nil)
	_ usecase.FleetDefinitionRepository            = (*FleetDefinitionStore)(nil)
	_ usecase.TerminalSessionRepository            = (*TerminalSessionStore)(nil)
	_ usecase.TerminalScrollbackSnapshotRepository = (*TerminalScrollbackSnapshotStore)(nil)
	_ usecase.QueuedPromptRepository               = (*QueuedPromptStore)(nil)
	_ usecase.EphemeralVmSshTargetRepository       = (*EphemeralVmSshTargetStore)(nil)
	_ outbox.Store                                 = (*AgentRateLimitedOutboxStore)(nil)
	_ usecase.AgentSessionRepository               = (*AgentSessionStore)(nil)
	_ usecase.EphemeralVmRuntimeRepository         = (*EphemeralVmRuntimeStore)(nil)
)
