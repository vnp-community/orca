package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// CleanupWorktreesConfig is the STEP_TYPE_CLEANUP_WORKTREES step's config
// shape — BL-AT-04/BR-AT-13. RunID is optional: when the dispatching
// automation run's ID is known (automation-service's RunNow threads it
// through), the audit report (BR-AT-14) is written against that row;
// left empty, the audit write is skipped (there is no automation_runs row
// to satisfy WriteCleanupReport's FK against).
type CleanupWorktreesConfig struct {
	ProjectID      string   `json:"project_id"`
	StatusIn       []string `json:"status_in"`
	OlderThanHours float64  `json:"older_than_hours"`
	DryRun         bool     `json:"dry_run"` // BR-AT-13
	RunID          string   `json:"run_id"`
}

// cleanupSummary is CleanupWorktreesStepExecutor's StepResult.OutputJSON
// shape.
type cleanupSummary struct {
	Deleted int            `json:"deleted"`
	Skipped int            `json:"skipped"`
	Entries []CleanupEntry `json:"entries"`
}

// CleanupWorktreesStepExecutor is a sixth StepExecutor implementation
// running natively in-process in workflow-service (no execution-plane
// relay needed, unlike Agent/Shell/Notification) — lists candidate
// worktrees via project-service's filtered ListWorktrees, deletes each via
// git-gateway-service.RemoveWorktree with force=false, allow_open_pr=false
// UNCONDITIONALLY (an automated bulk delete must never bypass either
// safety rule — BR-AT-11/BR-AT-12 still apply, enforced server-side in
// git-gateway-service), and writes an audit report (best-effort).
type CleanupWorktreesStepExecutor struct {
	projects ProjectClient
	gitgw    GitGatewayClient
	auditLog CleanupAuditWriter
	logger   *slog.Logger
}

func NewCleanupWorktreesStepExecutor(projects ProjectClient, gitgw GitGatewayClient, auditLog CleanupAuditWriter) *CleanupWorktreesStepExecutor {
	return &CleanupWorktreesStepExecutor{projects: projects, gitgw: gitgw, auditLog: auditLog, logger: slog.Default()}
}

var _ domain.StepExecutor = (*CleanupWorktreesStepExecutor)(nil)

func (e *CleanupWorktreesStepExecutor) Execute(ctx context.Context, stepConfigJSON string) (domain.StepResult, error) {
	var cfg CleanupWorktreesConfig
	if err := json.Unmarshal([]byte(stepConfigJSON), &cfg); err != nil {
		return domain.StepResult{}, fmt.Errorf("usecase: cleanup_worktrees: invalid step config JSON: %w", err)
	}

	olderThan := time.Now().UTC().Add(-time.Duration(cfg.OlderThanHours * float64(time.Hour)))
	candidates, err := e.projects.ListWorktrees(ctx, cfg.ProjectID, cfg.StatusIn, olderThan)
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("usecase: cleanup_worktrees: list worktrees: %w", err)
	}

	var deleted, skipped int
	entries := make([]CleanupEntry, 0, len(candidates))
	for _, wt := range candidates {
		if cfg.DryRun {
			entries = append(entries, CleanupEntry{WorktreeID: wt.ID, Action: "would_delete"})
			continue
		}
		// force=false, allow_open_pr=false — UNCONDITIONALLY. An automated
		// bulk delete must never silently bypass either safety rule.
		err := e.gitgw.RemoveWorktree(ctx, wt.ID, false, false)
		switch {
		case err == nil:
			deleted++
			entries = append(entries, CleanupEntry{WorktreeID: wt.ID, Action: "deleted"})
		case errors.Is(err, ErrWorktreeRemovalBlocked):
			skipped++
			entries = append(entries, CleanupEntry{WorktreeID: wt.ID, Action: "skipped", Reason: err.Error()})
		default:
			skipped++
			entries = append(entries, CleanupEntry{WorktreeID: wt.ID, Action: "skipped", Reason: "removal failed: " + err.Error()})
		}
	}

	// BR-AT-14 — best-effort, mirrors this codebase's existing
	// bookkeeping-failure posture: a report-write failure must not fail the
	// step itself. Skipped entirely when RunID is empty (see
	// CleanupWorktreesConfig's doc comment).
	if cfg.RunID != "" {
		if err := e.auditLog.WriteCleanupReport(ctx, cfg.RunID, entries); err != nil {
			e.logger.ErrorContext(ctx, "cleanup_worktrees: failed to write audit report", slog.Any("error", err), slog.String("run_id", cfg.RunID))
		}
	}

	output, err := json.Marshal(cleanupSummary{Deleted: deleted, Skipped: skipped, Entries: entries})
	if err != nil {
		return domain.StepResult{}, fmt.Errorf("usecase: cleanup_worktrees: marshal output: %w", err)
	}
	return domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: string(output)}, nil
}
