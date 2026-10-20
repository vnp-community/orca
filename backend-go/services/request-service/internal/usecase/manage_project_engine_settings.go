package usecase

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type SetProjectEngineSettingsInput struct {
	ProjectID     string
	Engine        string
	MinVersion    string
	SolutionAgent bool
	// ExecutionMode is step_by_step or continuous; empty keeps the mode the project already has.
	ExecutionMode string
	// PlanAutoGenerate nil keeps the stored value (true for a project without a row), so an older client that does not
	// send it never flips the switch.
	PlanAutoGenerate *bool
	// OutcomeReview is off or manual; empty keeps the stored choice and "default" clears it so the deployment default applies again.
	OutcomeReview   string
	ExpectedVersion int64
}

// EngineSettingsView is a project's settings and, when asked for, whether its dev server can run what is switched on.
type EngineSettingsView struct {
	Settings domain.ProjectEngineSettings
	// Preflight is nil when no check ran.
	Preflight *PreflightReport
	Warnings  []string
	// OutcomeReviewEnabled is whether the review is on for the project now: its choice, else the deployment default, never past the kill switch.
	OutcomeReviewEnabled bool
}

// ManageProjectEngineSettings reads and changes the per-project switches: which engine writes the Plan and whether the
// Solution is written by the read-only agent. Both default to off.
type ManageProjectEngineSettings struct {
	repo EngineSettingsRepository
	gate *EngineReadinessGate
	// reviewCfg decides whether a project without its own choice has the outcome review on, so the check can look at it.
	reviewCfg domain.OutcomeReviewConfig
	audit     *AuditRecorder
	tx        TxRunner
	now       func() time.Time
}

func NewManageProjectEngineSettings(repo EngineSettingsRepository) *ManageProjectEngineSettings {
	return &ManageProjectEngineSettings{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

// WithReadiness lets a read ask for the preflight and a write run it for what was switched on.
// WithOutcomeReviewConfig lets the readiness check include the outcome review for projects that follow the deployment default.
func (uc *ManageProjectEngineSettings) WithOutcomeReviewConfig(c domain.OutcomeReviewConfig) *ManageProjectEngineSettings {
	uc.reviewCfg = c
	return uc
}

func (uc *ManageProjectEngineSettings) WithReadiness(g *EngineReadinessGate) *ManageProjectEngineSettings {
	uc.gate = g
	return uc
}

// WithAudit records each change in the same transaction as the write.
func (uc *ManageProjectEngineSettings) WithAudit(a *AuditRecorder, tx TxRunner) *ManageProjectEngineSettings {
	uc.audit, uc.tx = a, tx
	return uc
}

var minVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func (uc *ManageProjectEngineSettings) GetProjectEngineSettings(ctx context.Context, projectID string, check bool) (EngineSettingsView, error) {
	if _, err := tenant.RequireTenantID(ctx); err != nil {
		return EngineSettingsView{}, domain.ErrRequestTenantRequired()
	}
	if _, err := uuid.Parse(projectID); err != nil {
		return EngineSettingsView{}, domain.ErrEngineSettingsInvalid("project_id must be a UUID")
	}
	settings, ok, err := uc.repo.Get(ctx, projectID)
	if err != nil {
		return EngineSettingsView{}, err
	}
	if !ok {
		settings = domain.ProjectEngineSettings{ProjectID: projectID, Engine: domain.EngineNative, ExecutionMode: domain.ExecutionStepByStep, PlanAutoGenerate: true}
	}
	view := EngineSettingsView{Settings: settings, OutcomeReviewEnabled: uc.reviewCfg.Effective(settings.OutcomeReview).Enabled()}
	if check {
		view.Preflight, view.Warnings = uc.preflight(ctx, settings)
	}
	return view, nil
}

func (uc *ManageProjectEngineSettings) SetProjectEngineSettings(ctx context.Context, in SetProjectEngineSettingsInput) (EngineSettingsView, error) {
	tenantID, err := tenant.RequireTenantID(ctx)
	if err != nil {
		return EngineSettingsView{}, domain.ErrRequestTenantRequired()
	}
	if !callerIsAdmin(ctx) {
		return EngineSettingsView{}, errRequestForbidden("only an administrator changes a project's engine settings")
	}
	if _, err := uuid.Parse(in.ProjectID); err != nil {
		return EngineSettingsView{}, domain.ErrEngineSettingsInvalid("project_id must be a UUID")
	}
	engine := domain.EngineName(in.Engine)
	if engine == "" {
		engine = domain.EngineNative
	}
	if engine != domain.EngineNative && engine != domain.EngineOpenSpec {
		return EngineSettingsView{}, domain.ErrEngineSettingsInvalid(fmt.Sprintf("solution_engine must be native or openspec, got %q", in.Engine))
	}
	if in.MinVersion != "" && !minVersionPattern.MatchString(in.MinVersion) {
		return EngineSettingsView{}, domain.ErrEngineSettingsInvalid("openspec_min_version must look like 1.2.3")
	}
	if in.ExpectedVersion < 0 {
		return EngineSettingsView{}, domain.ErrEngineSettingsInvalid("expected_version cannot be negative")
	}
	s, err := domain.NewProjectEngineSettings(tenantID, in.ProjectID, engine, in.MinVersion, callerID(ctx), uc.now())
	if err != nil {
		return EngineSettingsView{}, domain.ErrEngineSettingsInvalid(err.Error())
	}
	s.SolutionAgent = in.SolutionAgent
	if in.ExecutionMode == "" || in.PlanAutoGenerate == nil || in.OutcomeReview == "" {
		// An older client that does not know a field must not flip a project back to its default.
		cur, found, err := uc.repo.Get(ctx, in.ProjectID)
		if err != nil {
			return EngineSettingsView{}, err
		}
		if found && in.ExecutionMode == "" {
			s.ExecutionMode = cur.ExecutionMode.OrDefault()
		}
		if found && in.PlanAutoGenerate == nil {
			s.PlanAutoGenerate = cur.PlanAutoGenerate
		}
		if found && in.OutcomeReview == "" {
			s.OutcomeReview = cur.OutcomeReview
		}
	}
	if in.ExecutionMode != "" {
		mode, err := domain.ParseExecutionMode(in.ExecutionMode)
		if err != nil {
			return EngineSettingsView{}, err
		}
		s.ExecutionMode = mode
	}
	if in.PlanAutoGenerate != nil {
		s.PlanAutoGenerate = *in.PlanAutoGenerate
	}
	switch in.OutcomeReview {
	case "":
	case "default":
		s.OutcomeReview = ""
	case string(domain.OutcomeReviewOff), string(domain.OutcomeReviewManual):
		s.OutcomeReview = domain.OutcomeReviewMode(in.OutcomeReview)
	default:
		return EngineSettingsView{}, domain.ErrEngineSettingsInvalid(fmt.Sprintf("outcome_review must be off, manual or default, got %q", in.OutcomeReview))
	}

	write := func(ctx context.Context) error {
		saved, err := uc.repo.Upsert(ctx, s, in.ExpectedVersion)
		if err != nil {
			return err
		}
		s = saved
		if uc.audit == nil {
			return nil
		}
		return uc.audit.Record(ctx, AuditEvent{
			Action: domain.AuditEngineSettingsSet, TargetType: "project", TargetID: s.ProjectID, Outcome: "allowed",
			Metadata: map[string]any{"solution_engine": string(s.Engine), "solution_agent": s.SolutionAgent, "execution_mode": string(s.ExecutionMode.OrDefault()), "plan_auto_generate": s.PlanAutoGenerate, "outcome_review": string(s.OutcomeReview), "version": s.Version},
		})
	}
	if uc.tx != nil {
		err = uc.tx.InTx(ctx, write)
	} else {
		err = write(ctx)
	}
	if err != nil {
		return EngineSettingsView{}, err
	}
	view := EngineSettingsView{Settings: s, OutcomeReviewEnabled: uc.reviewCfg.Effective(s.OutcomeReview).Enabled()}
	view.Preflight, view.Warnings = uc.preflight(ctx, s)
	return view, nil
}

// preflight checks only what the settings switch on, and never blocks: a setting that cannot run yet is saved with the
// reasons, and the old mode keeps working until the dev server is ready.
func (uc *ManageProjectEngineSettings) preflight(ctx context.Context, s domain.ProjectEngineSettings) (*PreflightReport, []string) {
	openspec := s.Engine == domain.EngineOpenSpec
	review := uc.reviewCfg.Effective(s.OutcomeReview).Enabled()
	if !openspec && !s.SolutionAgent && !review {
		return nil, nil
	}
	if uc.gate == nil {
		return nil, []string{"preflight_not_run"}
	}
	rep, err := uc.gate.CheckForFreshWithReview(ctx, s.ProjectID, &s, openspec, review)
	if err != nil {
		return nil, []string{"preflight_not_run"}
	}
	return &rep, nil
}
