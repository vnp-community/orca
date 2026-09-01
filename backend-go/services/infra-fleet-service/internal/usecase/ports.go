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
	// FindByHostAndMode returns the DevServer matching host+mode for tenantID,
	// if one has been registered yet — the direct-websocket equivalent of
	// FindBySshTarget's find-or-create pattern (used by
	// ResolveDirectWebSocketDevServer). Scoped by mode as well as host
	// because relay-websocket rows also populate host with a real
	// ws://... URL — without the mode filter, an agent's external
	// devServerID string could theoretically collide with an unrelated
	// relay-websocket row's host value.
	FindByHostAndMode(ctx context.Context, tenantID, host string, mode domain.ConnectionMode) (ds domain.DevServer, found bool, err error)
	// UpdateApprovalStatus sets a dev server's approval_status — CR-DS-006
	// Phase 2. Returns the updated row; errors.Is(err, domain.ErrNotFound)-
	// style not-found signaling isn't needed here (same
	// tenant_id+id-scoped-update convention as every other mutation in this
	// service) — implementations return a plain error when 0 rows matched.
	UpdateApprovalStatus(ctx context.Context, tenantID, devServerID string, status domain.DevServerStatus) (domain.DevServer, error)
	// AssignGroup sets (or clears, when groupID == "") a dev server's
	// group_id — CR-DS-006 Phase 2.
	AssignGroup(ctx context.Context, tenantID, devServerID, groupID string) (domain.DevServer, error)
}

// DevServerGroupRepository is the persistence port for CR-DS-006's
// DevServerGroup tree — see docs/crs/v2/dev-server/
// CR-DS-006-dev-server-approval-and-grouping.md §3.2.
type DevServerGroupRepository interface {
	Create(ctx context.Context, group domain.DevServerGroup) (domain.DevServerGroup, error)
	// List returns every group registered for tenantID, in no particular
	// tree order — building the hierarchy from ParentGroupID is the
	// caller's job (same division of labor project-service's ProjectGroup
	// list endpoint uses).
	List(ctx context.Context, tenantID string) ([]domain.DevServerGroup, error)
}

// DevServerGroupGrantRepository is the persistence port for CR-DS-007's
// department/team ↔ group grants — see docs/crs/v2/dev-server/
// CR-DS-007-department-based-access-control.md.
type DevServerGroupGrantRepository interface {
	Create(ctx context.Context, grant domain.DevServerGroupGrant) (domain.DevServerGroupGrant, error)
	// Delete removes a grant by id, scoped to tenantID — a grantID from a
	// different tenant must never delete anything (same tenant-join
	// discipline as every other tenant-scoped mutation in this service).
	Delete(ctx context.Context, tenantID, grantID string) error
	// ListByGroup returns every grant on exactly this group (no
	// hierarchy/inheritance resolution — that's
	// usecase.ListDevServersForUser's job, composed from this plus
	// DevServerGroupRepository.List's parent_group_id chain).
	ListByGroup(ctx context.Context, tenantID, groupID string) ([]domain.DevServerGroupGrant, error)
	// ListAll returns every grant in tenantID — backs
	// ListDevServerGroupGrants's "empty group_id = every grant" case.
	ListAll(ctx context.Context, tenantID string) ([]domain.DevServerGroupGrant, error)
}

// DevServerAccessRequestRepository is the persistence port for CR-DS-008's
// access-request flow — see docs/crs/v2/dev-server/
// CR-DS-008-first-login-department-gate-and-access-request.md §2.3.
type DevServerAccessRequestRepository interface {
	Create(ctx context.Context, req domain.DevServerAccessRequest) (domain.DevServerAccessRequest, error)
	// Get fetches a request scoped to tenantID — same tenant-join
	// requirement as every other Get in this service.
	Get(ctx context.Context, tenantID, id string) (domain.DevServerAccessRequest, error)
	// ListPending returns every AccessRequestStatusPending request in
	// tenantID — backs the admin console's review queue.
	ListPending(ctx context.Context, tenantID string) ([]domain.DevServerAccessRequest, error)
	// UpdateStatus resolves a request (approved/rejected) — never reverts
	// an already-resolved request back to pending (usecase.ResolveAccessRequest
	// enforces that, not this port).
	UpdateStatus(ctx context.Context, tenantID, id string, status domain.AccessRequestStatus) (domain.DevServerAccessRequest, error)
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
	// IsConnected is Health's cheap, side-effect-free sibling — a pure peek
	// at whether devServerID already has a live session, never dialing one.
	// See devserveragent.Client.IsConnected's doc comment for why this
	// exists separately (safe to call in bulk, e.g. a dev server list).
	IsConnected(devServerID string) bool

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

	// --- Browser screencast (browser.screencast's live-view stream) ---

	// StreamScreencast starts a browser.screencast capture on the agent
	// (fire-and-forget browser.screencastStart dispatch, mirroring how
	// git.execStream/agent.spawn ack immediately then push further data via
	// notify) and subscribes to its ready/frame/ended/error notifications
	// over devServer's persistent session — same shape as StreamPty, except
	// there's no separate "spawn" step to call first: starting IS
	// subscribing, driven entirely by params.WorktreeID (the agent has no
	// pre-existing screencast identity to key off of the way a pty already
	// exists before StreamPty attaches to it). unsubscribe MUST be called
	// exactly once by the caller (usecase.AttachScreencast).
	StreamScreencast(ctx context.Context, devServer domain.DevServer, params ScreencastParams) (<-chan ScreencastEvent, func(), error)
}

// ScreencastParams carries browser.screencastStart's request fields —
// mirrors proto/orca/infrafleet/v1/infrafleet.proto's StartScreencast
// message 1:1 (clamping to the same bounds happens at the wscompat layer,
// before this usecase is ever reached, matching the OLD TS backend's
// clampInteger/clampOptionalInteger/clampOptionalNumber behavior).
type ScreencastParams struct {
	WorktreeID         string
	Page               string
	Format             string
	Quality            int32
	MaxWidth           int32
	MaxHeight          int32
	ViewportWidth      *int32
	ViewportHeight     *int32
	DeviceScaleFactor  *float64
	Mobile             bool
	EveryNthFrame      int32
	MinFrameIntervalMs int32
}

// ScreencastEvent is one event StreamScreencast's channel delivers —
// exactly one of Ready/Frame/Ended/ErrorMsg is meaningfully populated per
// value, mirroring PtyEvent's "one raw struct, narrow by which field is
// set" convention. Frame carries opaque, already-encoded bytes produced
// agent-side (encodeBrowserScreencastFrame) — this usecase and every layer
// above it never parses image bytes, only relays them.
type ScreencastEvent struct {
	Ready          bool
	SubscriptionID string
	BrowserPageID  string
	Format         string
	Frame          []byte
	Ended          bool
	ErrorMsg       string
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
