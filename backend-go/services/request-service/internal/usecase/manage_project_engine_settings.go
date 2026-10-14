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
	ProjectID       string
	Engine          string
	MinVersion      string
	SolutionAgent   bool
	ExpectedVersion int64
}

// EngineSettingsView is a project's settings and, when asked for, whether its dev server can run what is switched on.
type EngineSettingsView struct {
	Settings domain.ProjectEngineSettings
	// Preflight is nil when no check ran.
	Preflight *PreflightReport
	Warnings  []string
}

// ManageProjectEngineSettings reads and changes the per-project switches: which engine writes the Plan and whether the
// Solution is written by the read-only agent. Both default to off.
type ManageProjectEngineSettings struct {
	repo  EngineSettingsRepository
	gate  *EngineReadinessGate
	audit *AuditRecorder
	tx    TxRunner
	now   func() time.Time
}

func NewManageProjectEngineSettings(repo EngineSettingsRepository) *ManageProjectEngineSettings {
	return &ManageProjectEngineSettings{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

// WithReadiness lets a read ask for the preflight and a write run it for what was switched on.
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
		settings = domain.ProjectEngineSettings{ProjectID: projectID, Engine: domain.EngineNative}
	}
	view := EngineSettingsView{Settings: settings}
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
			Metadata: map[string]any{"solution_engine": string(s.Engine), "solution_agent": s.SolutionAgent, "version": s.Version},
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
	view := EngineSettingsView{Settings: s}
	view.Preflight, view.Warnings = uc.preflight(ctx, s)
	return view, nil
}

// preflight checks only what the settings switch on, and never blocks: a setting that cannot run yet is saved with the
// reasons, and the old mode keeps working until the dev server is ready.
func (uc *ManageProjectEngineSettings) preflight(ctx context.Context, s domain.ProjectEngineSettings) (*PreflightReport, []string) {
	openspec := s.Engine == domain.EngineOpenSpec
	if !openspec && !s.SolutionAgent {
		return nil, nil
	}
	if uc.gate == nil {
		return nil, []string{"preflight_not_run"}
	}
	rep, err := uc.gate.CheckFor(ctx, s.ProjectID, &s, openspec)
	if err != nil {
		return nil, []string{"preflight_not_run"}
	}
	return &rep, nil
}
