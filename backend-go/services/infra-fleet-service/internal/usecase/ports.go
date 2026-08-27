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
}

// ConnectionRepository is the persistence port for the write side of
// infra.connections (migrations/0002_connections) — the real routing model
// that replaced the connectionId==dev_server.id equation. Kept separate from
// ConnectionResolver (the read side) the same way DevServerRepository and
// ConnectionResolver already are two narrow ports over one Repository.
type ConnectionRepository interface {
	CreateConnection(ctx context.Context, conn domain.Connection) (domain.Connection, error)
	// GetActiveByDevServer returns the most recent non-closed connection
	// bound to devServerID, if any — backs ssh.getState's local read.
	// found=false, err=nil means "no active connection", not an error.
	GetActiveByDevServer(ctx context.Context, tenantID, devServerID string) (conn domain.Connection, found bool, err error)
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

// FleetHealthPort is the read port over fleet health samples. The
// health-polling writer side (the 30s-cadence poller from
// specs/backend-go/services/infra-fleet-service.md §8) is not implemented in
// this scaffold — see this service's README "Known gaps".
type FleetHealthPort interface {
	GetFleetHealth(ctx context.Context, tenantID string) ([]domain.DevServerHealth, error)
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
	Cwd   string
	Shell string
	Cols  int32
	Rows  int32
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
