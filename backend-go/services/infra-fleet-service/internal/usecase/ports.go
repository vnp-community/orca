// Package usecase holds infra-fleet-service's application services and the
// ports they need — defined here, implemented in internal/adapter/*, per
// the Dependency Inversion convention in
// specs/backend-go/architecture/03-clean-architecture-guidelines.md.
//
// DevServerAgentClient is defined here rather than in adapter/devserveragent
// deliberately — per
// specs/backend-go/services/infra-fleet-service.md §6 ("usecase/ ... defines
// DevServerAgentClient port here (not adapter/)"), the wire-protocol client
// is an outbound adapter like any other, and the usecase layer must not
// depend on its concrete package.
package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/eventbus"
	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

// DevServerRepository is the persistence port for the dev server registry.
// Implemented by internal/adapter/postgres against this service's own
// database — see specs/backend-go/architecture/05-data-architecture.md's
// database-per-service rule.
type DevServerRepository interface {
	Register(ctx context.Context, devServer domain.DevServer) (domain.DevServer, error)
	Get(ctx context.Context, tenantID, id string) (domain.DevServer, error)
	List(ctx context.Context, tenantID string) ([]domain.DevServer, error)
	// FindBySshTarget returns the DevServer whose ssh_target_id matches
	// sshTargetID, if one has been registered yet. found=false, err=nil
	// means "no dev server bound to this SSH target yet" — the caller
	// (usecase.EstablishConnection) is responsible for constructing and
	// Register()-ing a new relay-ssh-mode DevServer in that case,
	// generating its ID with uuid.NewString() the same way
	// register_dev_server.go's usecase already does — ID generation stays
	// in the usecase layer, not this adapter, matching every other `New*`
	// call site in this service.
	FindBySshTarget(ctx context.Context, tenantID, sshTargetID string) (ds domain.DevServer, found bool, err error)
	// UpdateProvisionResult persists the outcome of one provisioning
	// attempt — status plus the handshake facts SOL-FLEET-04 needs
	// surfaced. Called once per server at the end of bulkProvisionOne
	// (TASK-FLEET-02-05) and after EstablishConnection's handshake
	// (TASK-FLEET-04-03), success or failure.
	UpdateProvisionResult(ctx context.Context, tenantID, id string, status domain.DevServerStatus, info HandshakeInfo, provisionedAt time.Time) error
	// ListAllForPolling is cross-tenant by design (the poller is not
	// answering one tenant's request), unlike every other
	// DevServerRepository method's tenantID parameter.
	ListAllForPolling(ctx context.Context) ([]domain.DevServer, error)
	// ListByTag returns tenantID's dev servers carrying tag exactly — backs
	// usecase.ListDevServersByTag / workflow-service's "fleet:tag:<tag>"
	// dispatch-target shape (TASK-WF-02-02).
	ListByTag(ctx context.Context, tenantID, tag string) ([]domain.DevServer, error)
}

// HandshakeInfo is a usecase-owned mirror of
// adapter/devserveragent.HandshakeInfo — duplicated here rather than
// imported, since adapter/devserveragent already imports this package (to
// implement DevServerAgentClient) and importing it back would create an
// import cycle. adapter/devserveragent.Client.LastHandshakeInfo and
// adapter/sshrelay's provisioner both convert into this shape at the
// usecase boundary.
type HandshakeInfo struct {
	Platform     string
	Arch         string
	NodeVersion  string
	AgentVersion string
}

// Provisioner is BulkProvisionFleet's narrow port onto relay-ssh
// provisioning (adapter/sshrelay.Provisioner's SSH-connect ->
// prereq-check -> deploy -> handshake pipeline) — deliberately not
// DevServerAgentClient (whose Health/Exec double as an implicit
// provision-on-demand for every OTHER usecase in this service):
// BulkProvisionFleet needs the provisioning outcome itself (handshake
// facts, prereq shortfall) as its primary result, not a side-effect of
// some other call.
type Provisioner interface {
	// Provision runs the full pipeline for devServer. A prereq shortfall
	// does NOT make this return an error — deploy is still attempted
	// (BL-FLEET-02's "does not abort the pipeline"), and prereqsMet=false
	// on an otherwise-successful call reports it, so bulkProvisionOne can
	// tell "deployed but degraded" apart from "deploy/handshake genuinely
	// failed" (a non-nil err, which DOES still consume a retry attempt).
	Provision(ctx context.Context, devServer domain.DevServer) (info HandshakeInfo, prereqsMet bool, err error)
}

// SshTargetRepository is the persistence port for SSH target registration.
type SshTargetRepository interface {
	Create(ctx context.Context, target domain.SshTarget) (domain.SshTarget, error)
	// Get fetches an SSH target scoped to tenantID — same tenant-join
	// requirement as DevServerRepository.Get, see
	// specs/backend-go/services/infra-fleet-service.md §9.
	Get(ctx context.Context, tenantID, id string) (domain.SshTarget, error)
	// List returns every SSH target registered for tenantID — backs
	// ssh.listTargets/ssh.getUserAccount.
	List(ctx context.Context, tenantID string) ([]domain.SshTarget, error)
	// Upsert inserts or updates by (tenant_id, host, user_name) — the
	// conflict target migrations/0007's unique index establishes.
	// updated=true means an existing row's vault_ssh_role/project/tags were
	// overwritten; updated=false means a new row was inserted.
	Upsert(ctx context.Context, target domain.SshTarget) (saved domain.SshTarget, updated bool, err error)
	// GetByHostUser is a narrow existence-probe used only by the
	// dry-run import path (usecase.ImportFleetInventory) — it does not
	// commit anything.
	GetByHostUser(ctx context.Context, tenantID, host, userName string) (domain.SshTarget, bool, error)
}

// ConnectionRepository is the persistence port for the write side of
// infra.connections (migrations/0002_connections) — the real routing model
// that replaced the connectionId==dev_server.id equation. Kept separate from
// ConnectionResolver (the read side) the same way DevServerRepository and
// ConnectionResolver already are two narrow ports over one Repository.
type ConnectionRepository interface {
	CreateConnection(ctx context.Context, conn domain.Connection) (domain.Connection, error)
	// CreateConnectionWithOutbox inserts a new connection binding and
	// enqueues event as an infra.outbox_events row — both in ONE Postgres
	// transaction (Epic G's transactional-outbox pattern, see
	// domain.OutboxEvent's doc comment). Used by EstablishConnection to
	// publish its ssh.connect event exactly once alongside the connection
	// write, mirroring usage-service's Repository.SaveSession(session,
	// event) shape. Kept as a separate method from CreateConnection rather
	// than adding an event parameter there, since CreateConnection's other
	// caller (the plain worktree-bind usecase) has no event to enqueue.
	CreateConnectionWithOutbox(ctx context.Context, conn domain.Connection, event domain.OutboxEvent) (domain.Connection, error)
	// GetActiveByDevServer returns the most recent non-closed connection
	// bound to devServerID, if any — backs ssh.getState's local read.
	// found=false, err=nil means "no active connection", not an error.
	GetActiveByDevServer(ctx context.Context, tenantID, devServerID string) (conn domain.Connection, found bool, err error)
	// UpdateStatus sets connectionID's status column — TeardownConnection's
	// "mark closed" step (BR-SSH-13), also usable by a future reconnect-state
	// writer to record "reconnecting"/"degraded".
	UpdateStatus(ctx context.Context, tenantID, connectionID, status string) error
	// GetDevServerByConnection resolves connectionID's owning DevServer —
	// TeardownConnection needs it to call DevServerAgentClient.CancelReconnect
	// by DevServer.ID, not by connection id. found=false, err=nil means the
	// connection row doesn't exist (already gone/never existed) — not an
	// error, TeardownConnection stays idempotent on it.
	GetDevServerByConnection(ctx context.Context, tenantID, connectionID string) (devServer domain.DevServer, found bool, err error)
}

// PortForwardRepository is the storage port for domain.PortForward —
// implemented by adapter/postgres.PortForwardStore (mirrors SshTargetStore's
// own-Go-value-not-the-same-as-Repository shape).
type PortForwardRepository interface {
	Create(ctx context.Context, pf domain.PortForward) (domain.PortForward, error)
	UpdateStatus(ctx context.Context, tenantID, id string, status domain.PortForwardStatus) error
	ListActiveByConnection(ctx context.Context, tenantID, connectionID string) ([]domain.PortForward, error)
}

// PortForwardEventPublisher publishes port-forward lifecycle events for a
// future push path to consume. Defined here (consumer-side) per this
// codebase's Dependency Inversion convention.
type PortForwardEventPublisher interface {
	Publish(ctx context.Context, event string, pf domain.PortForward)
}

// TunnelOpener narrows sshconn.Connection.Forward to what this package needs.
type TunnelOpener interface {
	Forward(localPort, remotePort int) (Tunnel, error)
}

// Tunnel narrows sshconn.Tunnel to its Close method — the only thing
// PollWorkspacePorts calls on it directly.
type Tunnel interface {
	Close() error
}

// ConnectionResolver is THE core coordination/execution dispatch primitive
// of this service — see
// specs/backend-go/services/infra-fleet-service.md §7's sequence diagram.
// Every "does this worktree/session have a connectionId" branch in the
// system reduces to a call through this port: found means relay to that
// DevServer, not-found means the caller executes locally.
//
// tenantID is threaded through explicitly (mirroring usage-service's
// Repository port convention) rather than pulled from ctx inside this
// interface's implementations, even though callers extract it from ctx via
// common/tenant first — see specs/backend-go/services/infra-fleet-service.md
// §9's "ResolveConnection must join through tenant_id on every lookup"
// requirement: an explicit parameter makes that join impossible to forget
// at any implementation's call site, and keeps the port trivially fakeable
// in tests without a context-plumbing helper.
type ConnectionResolver interface {
	// ResolveConnection looks up connectionID within tenantID's scope.
	// connected=false with a nil error means "no dev server owns this
	// connectionId" — the caller's cue to execute locally, not an error
	// condition. conn carries the per-connection metadata (RepoPath,
	// WorktreeID) callers like git-gateway-service's RelayExecutor need
	// alongside devServer — zero-value when connected is false.
	ResolveConnection(ctx context.Context, tenantID, connectionID string) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)

	// ResolveConnectionByDevServer finds the most recently created
	// connection row bound to devServerID within tenantID's scope — the
	// reverse lookup direction from ResolveConnection. Same
	// connected=false/nil-error "nothing bound yet" convention.
	ResolveConnectionByDevServer(ctx context.Context, tenantID, devServerID string) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)

	// ResolveConnectionByWorktree finds the connection row currently bound
	// to worktreeID within tenantID's scope. Same
	// connected=false/nil-error convention.
	ResolveConnectionByWorktree(ctx context.Context, tenantID, worktreeID string) (connected bool, devServer domain.DevServer, conn domain.Connection, err error)
}

// FleetHealthPort is the read port over fleet health samples.
type FleetHealthPort interface {
	GetFleetHealth(ctx context.Context, tenantID string) ([]domain.DevServerHealth, error)
}

// FleetHealthWriter is the write side PollFleetHealth (TASK-FLEET-03-05)
// needs — split from FleetHealthPort the same way other narrow ports
// already split a single Repository's read/write concerns in this file.
type FleetHealthWriter interface {
	UpsertFleetHealth(ctx context.Context, sample domain.DevServerHealth) error
	// GetPrevious reads the last-persisted sample for devServerID —
	// PollFleetHealth diffs against it to detect a status_change (BL-FLEET-03's
	// poll-flow step 4). found=false means no prior sample exists yet.
	GetPrevious(ctx context.Context, devServerID string) (sample domain.DevServerHealth, found bool, err error)
}

// PollLockPort wraps a Postgres session-level advisory lock keyed by a hash
// of devServerID — TryLock is non-blocking (pg_try_advisory_lock, not
// pg_advisory_lock): a replica that loses the race skips this server this
// tick rather than queueing, so a multi-replica poller never double-polls
// the same dev server concurrently.
type PollLockPort interface {
	// TryLock returns locked=false (with a nil unlock, nil err) when
	// another poller already holds the lock for devServerID — the caller
	// skips this server this tick. When locked=true, unlock MUST be called
	// exactly once to release the advisory lock.
	TryLock(ctx context.Context, devServerID string) (locked bool, unlock func(), err error)
}

// HealthEventPublisher fans a status_change out onto the event bus (see
// adapter/eventbus's health publisher) — fire-and-forget from
// PollFleetHealth's perspective, hence no error return.
type HealthEventPublisher interface {
	PublishStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus)
}

// WebhookAlerter delivers a status_change to BL-FLEET-03's configured
// webhook endpoint — also fire-and-forget from PollFleetHealth's
// perspective (a webhook delivery failure must never fail the poll tick).
type WebhookAlerter interface {
	NotifyStatusChange(ctx context.Context, ds domain.DevServer, from, to domain.HealthStatus, sample domain.DevServerHealth)
}

// MetricsCollector receives every poll sample so Prometheus scrapes read
// from an in-process cache instead of re-querying Postgres per scrape —
// declared here (consumer-side), implemented by
// adapter/metrics.FleetCollector, per this codebase's Dependency Inversion
// convention (usecase must not import adapter packages).
type MetricsCollector interface {
	Update(devServerID, host string, sample domain.DevServerHealth)
}

// BrowserProfileRepository is the persistence port for browser profile
// metadata (infra.browser_profiles, TASK-032) — Postgres-only; the 3
// live-agent profile operations (profileClearDefaultCookies/
// profileDetectBrowsers/profileImportFromBrowser) do NOT go through this
// port, they relay via DevServerAgentClient/Relay instead (see TASK-034).
type BrowserProfileRepository interface {
	List(ctx context.Context, tenantID, devServerID string) ([]domain.BrowserProfile, error)
	Create(ctx context.Context, profile domain.BrowserProfile) (domain.BrowserProfile, error)
	Delete(ctx context.Context, tenantID, id string) error
}

// DevServerAgentClient is the port to the Dev Server Agent execution plane —
// implemented by adapter/devserveragent against the EXISTING TS wire
// protocol (Option A, see
// specs/backend-go/architecture/08-inter-service-communication.md), NOT a
// new gRPC contract. Real for relay-websocket mode (Epic A, 2026-08-17);
// direct-websocket/relay-ssh still return ErrConnectionModeNotImplemented.
// See adapter/devserveragent's package doc comment and this service's
// README "Known gaps" for exactly what's still missing.
type DevServerAgentClient interface {
	// Exec dispatches one JSON-RPC method call (e.g. "ports.scan",
	// "pty.spawn", "preflight.check") to the agent over devServer's resolved
	// transport mode and returns its decoded result.
	Exec(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (map[string]any, error)
	// Health performs an agent-level reachability/handshake check, distinct
	// from the SSH-exec-based fleet health poll that GetFleetHealth reads.
	Health(ctx context.Context, devServer domain.DevServer) (bool, error)
	// LastHandshakeInfo returns the HandshakeInfo captured at the most
	// recent successful handshake for devServerID, if a live session
	// exists — a cheap in-memory lookup, no round trip to the remote host.
	// EstablishConnection (SOL-FLEET-04) uses this right after a
	// successful Health() call to persist platform/arch/node-version facts
	// without a second round trip.
	LastHandshakeInfo(devServerID string) (HandshakeInfo, bool)

	// --- Terminal/PTY (TASK-180..187) ---
	// The six methods below extend the same generic-Exec transport with
	// typed wrappers over the agent's real pty.* JSON-RPC catalog (pty.create/
	// write/resize/destroy/listProcesses — see
	// internal/adapter/devserveragent/methods.go and client.go's StreamPty).
	// Unlike Exec, these ARE method-name-aware, because the terminal RPC
	// surface (TASK-180's proto additions) needs typed request/response
	// shapes at the usecase layer, not a generic map[string]any passthrough.

	// SpawnPty calls pty.create, returning the agent-assigned pty id and the
	// effective cwd/cols/rows/shell it actually started with.
	SpawnPty(ctx context.Context, devServer domain.DevServer, in SpawnPtyInput) (SpawnPtyResult, error)
	// WritePty calls pty.write — raw input bytes for ptyID's stdin.
	WritePty(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error
	// ResizePty calls pty.resize.
	ResizePty(ctx context.Context, devServer domain.DevServer, ptyID string, cols, rows int32) error
	// KillPty calls pty.destroy.
	KillPty(ctx context.Context, devServer domain.DevServer, ptyID string, graceful bool) error
	// SendSignal calls the real pty.sendSignal RPC (confirmed in
	// agent/src/relay/agent-rpc-dispatch.ts's 'pty.sendSignal' case and its
	// real handler, agent/src/relay/pty-agent-bridge.ts's
	// handlePtySendSignal) — a dedicated signal-delivery primitive that
	// TASK-181's original port list omitted. signal must be one of the
	// agent's ALLOWED_SIGNALS (SIGTERM/SIGKILL/SIGINT/SIGHUP/SIGTSTP) — see
	// devserveragent/methods.go's SendSignal doc comment. Replaces the
	// former StopTerminalProcess-sends-Ctrl-C-via-WritePty workaround.
	SendSignal(ctx context.Context, devServer domain.DevServer, ptyID string, signal string) error
	// StreamPty subscribes to ptyID's pty.data/pty.exit/pty.replay
	// notifications over devServer's persistent session (see
	// devserveragent/session.go's notification demux) and returns a
	// receive-only event channel plus an unsubscribe func. unsubscribe MUST
	// be called exactly once by every caller (typically via defer) to
	// release the subscription and let the channel be closed — usecase.AttachPty
	// and usecase.WaitTerminalSession are the two callers.
	StreamPty(ctx context.Context, devServer domain.DevServer, ptyID string) (<-chan PtyEvent, func(), error)
	// AgentStatus answers both terminal.agentStatus and terminal.isRunningAgent
	// (see GetTerminalAgentStatusResponse's doc comment) — CONFIRMED derived
	// from the real pty.listProcesses RPC (there is still no dedicated
	// agent-status RPC in the catalog); ReadyForInput remains a heuristic
	// (AgentRunning's value) because pty.listProcesses's {id,cwd,title,pid}
	// shape carries no busy/idle signal — see
	// devserveragent/methods.go's AgentStatus doc comment.
	AgentStatus(ctx context.Context, devServer domain.DevServer, ptyID string) (AgentStatusResult, error)
	// InspectProcess answers InspectTerminalProcessRequest — Known=false
	// when the agent can't (or this adapter can't) answer, never a
	// fabricated zero value. Pid is real (agent/src/relay/pty-agent-bridge.ts's
	// handlePtyListProcesses now includes it) as of this pass — see
	// devserveragent/methods.go's InspectProcess doc comment.
	InspectProcess(ctx context.Context, devServer domain.DevServer, ptyID string) (InspectProcessResult, error)

	// ExecStream dispatches one streaming JSON-RPC method call (currently
	// only "git.execStream", TASK-PW-03-08/SOL-PW-03) — usecase.RelayStream's
	// only caller — whose result arrives as multiple frames instead of one.
	// Unlike StreamPty (an out-of-band notification demux keyed by pty id),
	// these are ordinary JSON-RPC response frames sharing one request id;
	// see devserveragent/session.go's callStream doc comment for the wire
	// mechanics. The returned channel closes once the agent's terminal
	// frame is observed, the session disconnects, or ctx is cancelled;
	// unsubscribe MUST still be called exactly once (typically via defer)
	// to release the pending-call slot on an early return.
	ExecStream(ctx context.Context, devServer domain.DevServer, method string, params map[string]any) (<-chan map[string]any, func(), error)

	// CancelReconnect stops devServerID's in-flight background-reconnect
	// loop (relay-websocket's backgroundReconnect or relay-ssh's
	// relaySSHReconnect) immediately — TeardownConnection's "Cancel" action
	// (BR-SSH-13). No-op if no reconnect loop is running or no session
	// exists for devServerID.
	CancelReconnect(devServerID string)

	// --- Agent sessions (TASK-AG-01..05) ---
	// SpawnAgent calls agent.spawn. Returns immediately once the agent
	// accepts the request ({ok:true, ptyId}) — output/exit arrive later as
	// agent.output/agent.exited notifications over the same StreamPty
	// subscription used for plain PTYs.
	SpawnAgent(ctx context.Context, devServer domain.DevServer, in SpawnAgentInput) (SpawnAgentResult, error)
	// KillAgent calls agent.kill — signal is "SIGTERM" (graceful) or
	// "SIGKILL" (force).
	KillAgent(ctx context.Context, devServer domain.DevServer, ptyID, signal string) error
	// SendAgentInput calls agent.sendInput — used for graceful Ctrl+C.
	SendAgentInput(ctx context.Context, devServer domain.DevServer, ptyID string, data []byte) error
	// StreamAgentHooks subscribes to every agent.hook notification on
	// devServer's persistent session — ONE long-lived subscription per
	// devServer connection (not per AgentSession, unlike StreamPty),
	// consumed by RecordAgentHookProviderSession.
	StreamAgentHooks(ctx context.Context, devServer domain.DevServer) (<-chan AgentHookEvent, func(), error)
}

// SpawnAgentInput mirrors agent.spawn's real param set 1:1
// (agent-spawner.ts's AgentSpawnRequest) — resolvedApiKey intentionally
// absent, see TASK-AG-01-04 (credential injection blocker).
type SpawnAgentInput struct {
	TaskID       string // this service's own session id, minted before calling
	UserID       string
	ModelID      string
	AccountID    string
	Cwd          string
	ResumeID     string // "" for a fresh start; set by ResumeAgentSession
	WorktreePath string
	BranchName   string
	Cols, Rows   int32
	TrustPreset  string
}

type SpawnAgentResult struct {
	PtyID string
}

// AgentHookEvent is one agent.hook notification, decoded from
// agent-hook-server.ts's AgentHookRelayEnvelope (only the fields this
// service consumes) — see DevServerAgentClient.StreamAgentHooks.
type AgentHookEvent struct {
	WorktreeID string
	// PtyID — TASK-AG-03-07's exact correlation key, empty on older agent
	// builds mid-rollout (the worktreeId fallback then applies, see
	// RecordAgentHookProviderSession.Handle).
	PtyID              string
	ProviderSessionKey string
	ProviderSessionID  string
}

// AgentSessionRepository persists AgentSession.
type AgentSessionRepository interface {
	// Create enforces BR-AG-01 (one non-terminal agent session per
	// worktree+user) via a partial unique constraint at the DB layer —
	// domain.ErrAgentAlreadyRunning on conflict, not a race-prone
	// check-then-insert.
	Create(ctx context.Context, s domain.AgentSession) (domain.AgentSession, error)
	Get(ctx context.Context, tenantID, sessionID string) (found bool, s domain.AgentSession, err error)
	// GetByPtyID — TASK-AG-03-07's exact join key: an agent.hook notification
	// carrying a ptyId correlates to its AgentSession directly, no worktree
	// fallback ambiguity. found=false, nil error means no session with this
	// pty_id for this tenant (not an error worth failing the hook pump over).
	GetByPtyID(ctx context.Context, tenantID, ptyID string) (found bool, s domain.AgentSession, err error)
	// LatestForWorktree — SELECT ... ORDER BY started_at DESC LIMIT 1, used
	// by ResumeAgentSession.
	LatestForWorktree(ctx context.Context, tenantID, worktreeID string) (found bool, s domain.AgentSession, err error)
	UpdateStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error
	MarkStopped(ctx context.Context, tenantID, sessionID string, now time.Time) error
	// MarkStoppedWithStatus is MarkStopped's exit-driven counterpart — sets
	// a terminal status ('stopped' or 'error', decided by the caller from
	// the pty's exit code) rather than always 'stopped'.
	MarkStoppedWithStatus(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus, now time.Time) error
	// MostRecentActiveForWorktree — the agent.hook correlation fallback
	// (TASK-AG-03-05's "genuine gap" option 2): most recent AgentSession in
	// spawning/running/idle/waiting status for worktreeID. found=false, nil
	// error means none — not an error worth failing the hook-notification
	// pump over.
	MostRecentActiveForWorktree(ctx context.Context, tenantID, worktreeID string) (found bool, s domain.AgentSession, err error)
	// UpdateProviderSession persists the CLI's own resumable session id,
	// captured from an agent.hook notification.
	UpdateProviderSession(ctx context.Context, tenantID, sessionID, providerSessionKey, providerSessionID string) error
}

// AIProviderResolverClient — infra-fleet-service's own client of
// ai-provider-service.ResolveProvider, same RPC
// git-gateway-service/internal/adapter/grpcclient/aiprovider_client.go
// already calls (second caller — a NEW edge on
// 02-microservices-decomposition.md's dependency graph, infra --> aiprov),
// extended with the projectID/excludeAccountID params SwitchAgentAccount
// needs (TASK-AG-04-02's additive ResolveProviderRequest.exclude_account_id
// field). The port is defined here but has no caller until TASK-AG-04-03
// (SwitchAgentAccount) — StartAgentSession itself never calls Resolve, see
// TASK-AG-01-07's context.
type AIProviderResolverClient interface {
	ResolveProvider(ctx context.Context, tenantID, userID, projectID, excludeAccountID string) (providerType, accountID, status string, err error)
}

// WriteActivityChecker answers "is this worktree mid-file-write right now?"
// — BR-AG-06. No implementation exists in this pass; see TASK-AG-02-06 for
// the open design question on where/whether to build one. A nil
// WriteActivityChecker (or one returning an error) must never block a kill
// — see KillAgentSession.Execute's fail-open handling below.
type WriteActivityChecker interface {
	HasInFlightWrite(ctx context.Context, worktreeID string) (bool, error)
}

// AgentStatusEventPublisher publishes agent-session lifecycle events for
// real-time delivery to the renderer (and, per BUG-MB-04, eventually
// mobile) — see TASK-AG-05-05 for the two concrete delivery paths
// (direct in-process push for statusChanged, outbox for rateLimited).
type AgentStatusEventPublisher interface {
	PublishStatusChanged(ctx context.Context, tenantID, sessionID string, status domain.AgentStatus) error
	PublishRateLimited(ctx context.Context, tenantID, sessionID string) error
}

// SpawnPtyInput carries pty.create's request fields.
type SpawnPtyInput struct {
	Cwd              string
	Shell            string
	Cols             int32
	Rows             int32
	ShellIntegration bool // BR-TM-13 — forwarded to the agent's pty.create, never inspected here
	// Command, when set, is the pty.create `command` param — a full shell
	// command line to run instead of an interactive prompt (TASK-INT-01-01,
	// github.startAuthLogin/gitlab.startAuthLogin).
	Command string
	// UserID is the pty.create `userId` param — engages the agent's
	// per-user GH_CONFIG_DIR/GLAB_CONFIG_DIR isolation when Command starts
	// with "gh "/"glab " (pty-handler.ts:680-699).
	UserID string
}

// SpawnPtyResult carries pty.create's response fields — Cols/Rows/Cwd/Shell
// are the agent's EFFECTIVE values (it applies its own defaults when the
// request left them empty/zero), not an echo of SpawnPtyInput.
type SpawnPtyResult struct {
	PtyID string
	Cols  int32
	Rows  int32
	Cwd   string
	Shell string
}

// PtyEvent is one event pushed by the Dev Server Agent's pty-daemon over a
// live pty.attach-style subscription — output bytes (pty.data/pty.replay) or
// a process-exit notification (pty.exit). Exactly one of the two shapes is
// populated: Exited=false means Data carries output bytes; Exited=true means
// the process ended and ExitCode is meaningful (Data is nil).
type PtyEvent struct {
	PtyID    string
	Data     []byte
	Exited   bool
	ExitCode int32
}

// AgentStatusResult carries GetTerminalAgentStatusResponse's fields.
type AgentStatusResult struct {
	AgentRunning  bool
	AgentKind     string
	ReadyForInput bool
	// LastOutputPreview is populated from the shared liveStates registry
	// (TASK-MB-04-02), BR-MB-15-truncated — empty when no live entry exists
	// for this ptyId (cross-pod case), an honest absence not an error.
	LastOutputPreview string
}

// InspectProcessResult carries InspectTerminalProcessResponse's fields.
type InspectProcessResult struct {
	Known   bool
	Pid     int32
	Command string
	Cwd     string
}

// TerminalSessionRepository is the persistence port for infra.terminal_sessions
// (migrations/0005_terminal_sessions) — the record of every PTY this service
// has spawned through the agent, independent of the agent's own in-memory
// pty-daemon state (which does not survive this service's restart, unlike
// Postgres). tenantID is threaded explicitly on every method, matching
// ConnectionResolver/SshTargetResolver's convention (see those ports' doc
// comments) — an explicit parameter makes the tenant join impossible to
// forget at any implementation's call site.
type TerminalSessionRepository interface {
	// Create inserts a new terminal session row and returns the persisted
	// value.
	Create(ctx context.Context, session domain.TerminalSession) (domain.TerminalSession, error)
	// Get fetches one session scoped to tenantID. found=false with a nil
	// error means "no such session for this tenant" — the caller's cue to
	// return a not-found error, not that Get itself failed — mirrors
	// ConnectionResolver.ResolveConnection's bool-found convention.
	Get(ctx context.Context, tenantID, ptyID string) (found bool, session domain.TerminalSession, err error)
	// List returns every OPEN session (closed_at IS NULL) for tenantID,
	// optionally narrowed to one connectionID — empty connectionID means
	// every dev server's sessions for this tenant, per
	// ListTerminalSessionsRequest's doc comment.
	List(ctx context.Context, tenantID, connectionID string) ([]domain.TerminalSession, error)
	// Touch bumps last_active_at — called by FocusTerminalSession and every
	// usecase that observes real activity on a session (resize, kill).
	Touch(ctx context.Context, tenantID, ptyID string, now time.Time) error
	// Close sets closed_at — called by KillTerminalSession. Idempotent: a
	// session already closed simply gets a newer closed_at, not an error, so
	// a duplicate/racing close request never fails the caller.
	Close(ctx context.Context, tenantID, ptyID string, closedAt time.Time) error
}

// TerminalScrollbackSnapshotRepository is the persistence port for
// infra.terminal_scrollback_snapshots (migrations/0007) — parallel in shape
// to TerminalSessionRepository, tenantID threaded explicitly on every
// method for the same reason that port's doc comment gives: an explicit
// parameter makes the tenant join impossible to forget at any
// implementation's call site.
type TerminalScrollbackSnapshotRepository interface {
	// Upsert writes or replaces the (tenantID, worktreeID, paneKey) row.
	Upsert(ctx context.Context, snap domain.TerminalScrollbackSnapshot) error
	// Get returns found=false, nil error when no snapshot exists yet for
	// this pane — mirrors ConnectionResolver's found-bool convention.
	Get(ctx context.Context, tenantID, worktreeID, paneKey string) (found bool, snap domain.TerminalScrollbackSnapshot, err error)
	// SumUncompressedBytes returns the current total across every pane for
	// worktreeID, EXCLUDING paneKey itself (the row Upsert is about to
	// replace) — backs BR-TM-10's per-worktree cap check.
	SumUncompressedBytes(ctx context.Context, tenantID, worktreeID, excludePaneKey string) (int64, error)
	// DeleteByWorktree removes every pane's snapshot for worktreeID — backs
	// git-gateway-service's RemoveWorktree cleanup hook.
	DeleteByWorktree(ctx context.Context, tenantID, worktreeID string) error
	// DeleteExpired removes every row with updated_at older than
	// domain.ScrollbackSnapshotTTL — backs BR-TM-12's sweep, called from a
	// scheduled job the same way fleet_health_samples' retention prune is.
	DeleteExpired(ctx context.Context, olderThan time.Time) (deletedCount int, err error)
}

// Clock abstracts time.Now for deterministic tests.
type Clock interface{ Now() time.Time }

// RealClock is the real Clock, wired in cmd/server/main.go.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

// CredentialBrokerClient is infra-fleet-service's port to
// credential-broker-service — used ONLY for relay-websocket agent tokens
// (SOL-AWS-01). Unlike most services' identically-named ports, this one
// DOES call the plaintext-returning ResolveCredential RPC: relay-websocket
// mode requires Orca to present the token outbound as an Authorization
// header, not merely compare against a stored hash the way
// direct-websocket's TokenHash branch does. See adapter/grpcclient's doc
// comment for the full justification before touching this port.
type CredentialBrokerClient interface {
	// WriteCredential writes envelope (raw token bytes) under
	// (tenantID, ownerID=devServerID, CREDENTIAL_CATEGORY_DEV_SERVER_AGENT_TOKEN)
	// and returns a reference — the plaintext itself is never returned.
	WriteCredential(ctx context.Context, tenantID, ownerID string, envelope []byte) (CredentialRef, error)
	// ResolveCredential returns the plaintext bytes for credentialRefID —
	// called once per dial (never cached across process restarts), see
	// SOL-AWS-01's "resolve on every dial" guarantee.
	ResolveCredential(ctx context.Context, credentialRefID string) ([]byte, error)
}

// CredentialRef is what CredentialBrokerClient.WriteCredential returns —
// an opaque pointer, never the secret itself.
type CredentialRef struct {
	ID string
}

// AgentTokenRepository is the persistence port for infra.agent_tokens
// (migrations/0007_agent_tokens, TASK-AWS-03-01) — BL-AWS-03's persistent,
// named, per-DevServer agent token set. tenantID is threaded explicitly on
// every method, matching ConnectionResolver/TerminalSessionRepository's
// convention.
type AgentTokenRepository interface {
	// CountActive returns the number of non-revoked tokens for devServerID
	// — enforces domain.MaxActiveAgentTokensPerDevServer.
	CountActive(ctx context.Context, tenantID, devServerID string) (int, error)
	// Insert persists a new token row. Callers must set exactly one of
	// TokenHash/CredentialRefID (domain.AgentToken's own invariant,
	// enforced again by the table's exactly_one_secret_ref CHECK).
	Insert(ctx context.Context, t domain.AgentToken) error
	// ListActive returns every non-revoked token for devServerID, newest
	// first — backs ListAgentTokens.
	ListActive(ctx context.Context, tenantID, devServerID string) ([]domain.AgentToken, error)
	// FindActiveByHash looks up a non-revoked direct-websocket token by its
	// SHA-256 hash — the agentwsserver handshake fallback's read path
	// (TASK-AWS-03-06). found=false, err=nil means "no such active token".
	FindActiveByHash(ctx context.Context, hash string) (t domain.AgentToken, found bool, err error)
	// ActiveForDevServer returns the most-recently-created non-revoked
	// token for a relay-websocket DevServer — SOL-AWS-01's per-dial
	// resolution read. Relay-websocket DevServers are expected to carry
	// exactly one active token in ordinary operation. found=false, err=nil
	// means "no active token registered yet".
	ActiveForDevServer(ctx context.Context, tenantID, devServerID string) (t domain.AgentToken, found bool, err error)
	// TouchLastUsed bumps last_used_at — called best-effort on a successful
	// handshake/dial, never blocks the caller on its result.
	TouchLastUsed(ctx context.Context, id string) error
	// Revoke sets revoked_at and returns the updated row.
	Revoke(ctx context.Context, tenantID, id string) (domain.AgentToken, error)
}

// HandshakeInfoProvider is ResolveConnection's optional read port for the
// connected session's self-reported Node.js version (TASK-INT-03-02,
// SOL-INT-03) — implemented by devserveragent.Client via a small
// composition-root adapter (see cmd/server/main.go). nil (or a miss)
// leaves ResolveConnectionOutput.NodeVersion empty rather than erroring —
// a connection with no live session, or one that predates this field, is
// not a failure condition for ResolveConnection's own contract.
type HandshakeInfoProvider interface {
	// NodeVersionFor returns devServerID's live session's self-reported
	// Node.js version. found=false means no live session right now (or a
	// session that never sent one) — never an error.
	NodeVersionFor(devServerID string) (nodeVersion string, found bool)
}

// LiveSessionCloser closes any live direct-websocket session currently
// authenticated with a given agent token — RevokeAgentToken's
// immediate-effect guarantee (TASK-AWS-03-06's usecase calls this after
// AgentTokenRepository.Revoke). Implemented by devserveragent.Client, which
// already tracks one live session per devServerID.
type LiveSessionCloser interface {
	// CloseSessionsForDevServerToken closes any direct-websocket session on
	// devServerID currently authenticated as tokenID, with WS close code
	// 1008 and a "token revoked" reason — see SOL-AWS-02 for why 1008, not
	// the never-implemented 4001.
	CloseSessionsForDevServerToken(ctx context.Context, devServerID, tokenID string) (closed int, err error)
}

// QueuedPromptRepository is the persistence port for infra.queued_prompts
// (migrations/0008_queued_prompts) — durable storage for a mobile-dispatched
// prompt held until the agent becomes ready (BR-MB-10), outliving the
// per-pod in-memory liveStates registry AttachPty/GetTerminalAgentStatus
// share. One row per PtyID; Get/GetAndDelete's found=false with a nil error
// means "no prompt currently queued", matching this codebase's other
// found-bool repository conventions (see TerminalSessionRepository.Get).
type QueuedPromptRepository interface {
	Get(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error)
	Upsert(ctx context.Context, p domain.QueuedPrompt) error
	Delete(ctx context.Context, ptyID string) error
	// GetAndDelete atomically reads and removes the row — see
	// postgres.QueuedPromptStore.GetAndDelete's doc comment for the
	// double-delivery race it guards against.
	GetAndDelete(ctx context.Context, ptyID string) (domain.QueuedPrompt, bool, error)
}

// LifecycleEventPublisher publishes terminal-session agent-lifecycle
// events for notification-service to translate into mobile pushes
// (BL-MB-02). Best-effort — a publish failure must never fail the PTY
// relay loop itself.
type LifecycleEventPublisher interface {
	PublishAgentLifecycle(ctx context.Context, tenantID, subject string, payload eventbus.AgentLifecyclePayload) error
}
