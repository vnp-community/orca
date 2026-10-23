package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// TenantLister names the tenants a background sweep visits; the retention store already answers it.
type TenantLister interface {
	ListTenants(ctx context.Context) ([]string, error)
}

// proposalSweepBatch bounds how many worktrees one tenant's pass looks at.
const proposalSweepBatch = 200

// ProposalJanitor removes the worktrees of OpenSpec proposals that nothing will write to again, and only those. The branch
// always stays, so a proposal remains reviewable after its worktree is gone. A worktree is removed only when its branch is
// the Request's own proposal branch, and never while a plan run of the Request is running.
type ProposalJanitor struct {
	Changes   OpenSpecChangeRepository
	Requests  RequestReader
	Runs      AnalysisRunStore
	Workspace ProposalWorkspace
	Tenants   TenantLister
	Retention time.Duration
	Now       func() time.Time
	Log       *slog.Logger
}

// ErrProposalRunActive refuses a cleanup while a plan run writes in the worktree.
var ErrProposalRunActive = errors.New("a plan run of this Request is writing in the proposal worktree")

// ProposalRelease says what a cleanup did.
type ProposalRelease struct {
	// Removed is false when there was no worktree to remove.
	Removed   bool
	Abandoned bool
	Branch    string
}

func (j *ProposalJanitor) now() time.Time {
	if j.Now != nil {
		return j.Now().UTC()
	}
	return time.Now().UTC()
}

func (j *ProposalJanitor) retention() time.Duration {
	if j.Retention > 0 {
		return j.Retention
	}
	return domain.DefaultProposalRetention
}

func (j *ProposalJanitor) log() *slog.Logger {
	if j.Log != nil {
		return j.Log
	}
	return slog.Default()
}

// SweepSummary is what one pass did, for the log.
type SweepSummary struct {
	Tenants, Looked, Removed, Abandoned, Failed int
}

// Sweep visits every tenant once. One tenant's trouble never stops the others.
func (j *ProposalJanitor) Sweep(ctx context.Context) (SweepSummary, error) {
	var sum SweepSummary
	tenants, err := j.Tenants.ListTenants(ctx)
	if err != nil {
		return sum, fmt.Errorf("list tenants: %w", err)
	}
	for _, id := range tenants {
		if ctx.Err() != nil {
			return sum, ctx.Err()
		}
		sum.Tenants++
		j.sweepTenant(tenant.WithActorType(tenant.WithTenantID(ctx, id), tenant.ActorSystem), &sum)
	}
	return sum, nil
}

func (j *ProposalJanitor) sweepTenant(ctx context.Context, sum *SweepSummary) {
	cands, err := j.Changes.ListWithWorkspace(ctx, proposalSweepBatch)
	if err != nil {
		j.log().WarnContext(ctx, "proposal cleanup could not list worktrees", slog.Any("error", err))
		sum.Failed++
		return
	}
	for _, c := range cands {
		sum.Looked++
		active, err := j.planRunActive(ctx, c.Change.RequestID)
		if err != nil {
			sum.Failed++
			continue
		}
		action := domain.DecideProposalCleanup(c.Change.Status, c.RequestStatus, c.UpdatedAt, j.now(), j.retention(), active)
		if !action.Remove {
			continue
		}
		rel, err := j.release(ctx, c.Change, action.Abandon)
		switch {
		case err != nil:
			j.log().WarnContext(ctx, "proposal worktree not removed", slog.String("request_id", c.Change.RequestID), slog.Any("error", err))
			sum.Failed++
		default:
			if rel.Removed {
				sum.Removed++
			}
			if rel.Abandoned {
				sum.Abandoned++
			}
		}
	}
}

// ReleaseOnCancel is the immediate cleanup of a cancelled Request: the change is abandoned and its worktree removed. It
// reports nothing wrong when the Request has no change.
func (j *ProposalJanitor) ReleaseOnCancel(ctx context.Context, requestID string) error {
	change, found, err := j.Changes.GetByRequest(ctx, requestID)
	if err != nil || !found || (change.WorkspaceID == "" && change.Status != domain.ChangeStatusPreparing && change.Status != domain.ChangeStatusReady) {
		return err
	}
	active, err := j.planRunActive(ctx, requestID)
	if err != nil || active {
		return err // a running run is cancelled with the Request; the sweep removes the leftover
	}
	_, err = j.release(ctx, change, true)
	return err
}

// Cleanup is the manual one (`orca request cleanup`): remove the worktree of a Request whatever the policy says, unless a
// plan run is writing in it. The change keeps its status; a cancelled Request's change is abandoned as usual.
func (j *ProposalJanitor) Cleanup(ctx context.Context, requestID string) (ProposalRelease, error) {
	change, found, err := j.Changes.GetByRequest(ctx, requestID)
	if err != nil {
		return ProposalRelease{}, err
	}
	if !found {
		return ProposalRelease{}, nil
	}
	active, err := j.planRunActive(ctx, requestID)
	if err != nil {
		return ProposalRelease{}, err
	}
	if active {
		return ProposalRelease{Branch: change.Branch}, ErrProposalRunActive
	}
	req, err := j.Requests.Get(ctx, requestID)
	if err != nil {
		return ProposalRelease{}, err
	}
	return j.release(ctx, change, req.Status == domain.RequestStatusCancelled)
}

// planRunActive is true while a plan run of the Request is running; such a run owns the worktree.
func (j *ProposalJanitor) planRunActive(ctx context.Context, requestID string) (bool, error) {
	runs, err := j.Runs.ListRecent(ctx, requestID, 10)
	if err != nil {
		return false, err
	}
	for _, r := range runs {
		if r.Kind == domain.RunKindPlan && r.Status == domain.RunStatusRunning {
			return true, nil
		}
	}
	return false, nil
}

// release removes the worktree, then records it. A worktree that is not the Request's own proposal branch is left alone.
func (j *ProposalJanitor) release(ctx context.Context, change domain.OpenSpecChange, abandon bool) (ProposalRelease, error) {
	out := ProposalRelease{Branch: change.Branch}
	if change.WorkspaceID != "" {
		req, err := j.Requests.Get(ctx, change.RequestID)
		if err != nil {
			return out, err
		}
		if !domain.IsProposalBranch(change.Branch, req.Number, req.ID) {
			return out, fmt.Errorf("the change names branch %q, not %q or %q: its worktree is not removed", change.Branch,
				domain.ProposalBranch(req.Number), domain.ProposalBranchFor(req.Number, req.ID))
		}
		if err := j.Workspace.Remove(ctx, Workspace{WorktreeID: change.WorkspaceID, Branch: change.Branch}); err != nil {
			return out, fmt.Errorf("remove the proposal worktree: %w", err)
		}
		out.Removed = true
	}
	change.ReleaseWorkspace()
	if abandon && change.Abandon() == nil {
		out.Abandoned = true
	}
	if _, err := j.Changes.Upsert(ctx, change); err != nil {
		return out, fmt.Errorf("record the released worktree: %w", err)
	}
	return out, nil
}
