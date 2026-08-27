package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/project-service/internal/domain"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// mobileWorktreeStatusPageSize bounds each ProjectRepository.List page
// while GetMobileWorktreeStatus walks every project for the tenant — the
// RPC itself takes no pagination params (BL-MB-04's response is the whole
// tenant-wide list), so this is purely an internal fetch-batch size.
const mobileWorktreeStatusPageSize = 200

// MobileWorktreeStatus mirrors project.proto's MobileWorktreeStatus message
// — one worktree's identity plus (when a bound dev server has a live,
// path-matched terminal session) its runtime status.
type MobileWorktreeStatus struct {
	ID, Name, Agent, Status string
	DurationMs              int64
	LastOutput              string
}

// MobileStatusResult carries GetMobileWorktreeStatusResponse's fields.
type MobileStatusResult struct {
	Worktrees   []MobileWorktreeStatus
	GeneratedAt time.Time
}

// GetMobileWorktreeStatus is the ONE composed-read call BL-MB-04 reduces to
// (SOL-MB-04) — project-service already depends on infra-fleet-service for
// dev-server binding validation, so this extends that existing edge rather
// than adding a new cross-service dependency, per api-gateway.md §2's ban
// on cross-service response orchestration in the gateway.
//
// The worktree<->PTY correlation key is Worktree.Path == TerminalSession.Cwd
// — a string-equality correlation, not a foreign key (neither domain type
// has an FK to the other today). A clean follow-up (adding worktree_id to
// SpawnTerminalSessionInput/terminal_sessions) would close this properly,
// but is not required to satisfy BL-MB-04's response shape.
//
// TerminalSession (the infrafleet proto message returned by
// ListTerminalSessions) does not itself carry AgentKind/AgentRunning/
// ReadyForInput — those live on GetTerminalAgentStatusResponse, a separate
// RPC keyed by ptyId. Rather than widen ListTerminalSessions' response (a
// proto follow-up beyond this task's scope), this issues one extra
// GetAgentStatus RPC per matched session — acceptable cost since this
// endpoint is polled by a mobile client at a human-perceptible interval,
// not a hot path.
type GetMobileWorktreeStatus struct {
	worktrees WorktreeRepository
	projects  ProjectRepository
	terminals TerminalStatusResolver
}

func NewGetMobileWorktreeStatus(worktrees WorktreeRepository, projects ProjectRepository, terminals TerminalStatusResolver) *GetMobileWorktreeStatus {
	return &GetMobileWorktreeStatus{worktrees: worktrees, projects: projects, terminals: terminals}
}

func (uc *GetMobileWorktreeStatus) Execute(ctx context.Context) (MobileStatusResult, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return MobileStatusResult{}, apperrors.New(apperrors.KindUnauthenticated, "PROJECT_NO_TENANT", "no tenant in request context", err)
	}

	projects, err := uc.listAllProjects(ctx, tenantID)
	if err != nil {
		return MobileStatusResult{}, err
	}

	var out []MobileWorktreeStatus
	byDevServer := map[string][]domain.Worktree{}
	for _, project := range projects {
		worktrees, err := uc.worktrees.ListWorktrees(ctx, project.ID)
		if err != nil {
			return MobileStatusResult{}, apperrors.New(apperrors.KindInternal, "PROJECT_MOBILE_STATUS_LIST_WORKTREES_FAILED", "failed to list worktrees", err)
		}
		for _, wt := range worktrees {
			if !wt.Active {
				continue // BL-MB-04 only reports live worktrees
			}
			if project.DevServerID == "" {
				// No bound dev server: nothing to correlate a PTY/agent
				// status against — still listed (not silently dropped),
				// just with empty runtime fields.
				out = append(out, MobileWorktreeStatus{ID: wt.ID, Name: wt.Branch})
				continue
			}
			byDevServer[project.DevServerID] = append(byDevServer[project.DevServerID], wt)
		}
	}

	for devServerID, wts := range byDevServer {
		sessions, err := uc.terminals.ListSessionsForDevServer(ctx, devServerID)
		if err != nil {
			// A degraded dev server shouldn't fail the whole response — its
			// worktrees report "unknown", every other dev server's worktrees
			// are unaffected.
			for _, wt := range wts {
				out = append(out, MobileWorktreeStatus{ID: wt.ID, Name: wt.Branch, Status: "unknown"})
			}
			continue
		}
		byPath := indexSessionsByCwd(sessions)
		for _, wt := range wts {
			session, ok := byPath[wt.Path]
			if !ok {
				out = append(out, MobileWorktreeStatus{ID: wt.ID, Name: wt.Branch, Status: "idle"})
				continue
			}
			out = append(out, uc.mobileStatusFromSession(ctx, wt, session))
		}
	}

	return MobileStatusResult{Worktrees: out, GeneratedAt: time.Now()}, nil
}

// listAllProjects walks ProjectRepository.List's id-cursor pagination to
// completion — GetMobileWorktreeStatus needs every project the tenant owns
// (to find every dev-server-bound worktree), not one page of them.
func (uc *GetMobileWorktreeStatus) listAllProjects(ctx context.Context, tenantID string) ([]domain.Project, error) {
	var all []domain.Project
	pageToken := ""
	for {
		projects, next, err := uc.projects.List(ctx, tenantID, pageToken, mobileWorktreeStatusPageSize)
		if err != nil {
			return nil, apperrors.New(apperrors.KindInternal, "PROJECT_MOBILE_STATUS_LIST_PROJECTS_FAILED", "failed to list projects", err)
		}
		all = append(all, projects...)
		if next == "" {
			break
		}
		pageToken = next
	}
	return all, nil
}

// mobileStatusFromSession composes one worktree's runtime fields from a
// path-matched TerminalSession plus a per-session GetAgentStatus call (see
// GetMobileWorktreeStatus's doc comment for why). A GetAgentStatus failure
// degrades this one worktree to Status "unknown" — Duration/LastOutput,
// already known from the successful ListSessionsForDevServer call, are kept
// rather than discarded.
func (uc *GetMobileWorktreeStatus) mobileStatusFromSession(ctx context.Context, wt domain.Worktree, session *infrafleetv1.TerminalSession) MobileWorktreeStatus {
	out := MobileWorktreeStatus{
		ID:         wt.ID,
		Name:       wt.Branch,
		DurationMs: durationFromSession(session),
		LastOutput: session.GetLastOutputPreview(),
	}
	status, err := uc.terminals.GetAgentStatus(ctx, session.GetPtyId())
	if err != nil {
		out.Status = "unknown"
		return out
	}
	out.Agent = status.GetAgentKind()
	out.Status = statusFromAgentStatus(status)
	return out
}

// durationFromSession is "now - TerminalSession.CreatedAt" per
// MobileWorktreeStatus.duration_ms's proto doc comment — computed only for
// a matched (non-idle) session.
func durationFromSession(session *infrafleetv1.TerminalSession) int64 {
	return time.Now().UnixMilli() - session.GetCreatedAtUnixMs()
}

// statusFromAgentStatus maps GetTerminalAgentStatusResponse onto
// MobileWorktreeStatus.status's enum: a matched session with no agent
// process running has finished its work ("completed"); a running agent is
// either quiescent-and-waiting ("waiting") or actively producing output
// ("running"). "idle" (no matched session) and "unknown" (a resolver
// failure) are assigned by the caller, not here.
func statusFromAgentStatus(status *infrafleetv1.GetTerminalAgentStatusResponse) string {
	switch {
	case !status.GetAgentRunning():
		return "completed"
	case status.GetReadyForInput():
		return "waiting"
	default:
		return "running"
	}
}

func indexSessionsByCwd(sessions []*infrafleetv1.TerminalSession) map[string]*infrafleetv1.TerminalSession {
	m := make(map[string]*infrafleetv1.TerminalSession, len(sessions))
	for _, s := range sessions {
		m[s.GetCwd()] = s
	}
	return m
}
