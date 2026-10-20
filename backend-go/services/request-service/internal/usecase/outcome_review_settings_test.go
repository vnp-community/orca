package usecase

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type reviewEngineSettings struct {
	EngineSettingsRepository
	byProject map[string]domain.ProjectEngineSettings
}

func (m *reviewEngineSettings) Get(_ context.Context, id string) (domain.ProjectEngineSettings, bool, error) {
	s, ok := m.byProject[id]
	return s, ok, nil
}

func (m *reviewEngineSettings) Upsert(_ context.Context, s domain.ProjectEngineSettings, expected int64) (domain.ProjectEngineSettings, error) {
	if cur, ok := m.byProject[s.ProjectID]; ok && cur.Version != expected {
		return s, domain.ErrEngineSettingsVersionConflict
	}
	s.Version = expected + 1
	m.byProject[s.ProjectID] = s
	return s, nil
}

func reviewAdminCtx(t *testing.T) (context.Context, string) {
	t.Helper()
	tid := uuid.NewString()
	ctx := tenant.WithTenantID(context.Background(), tid)
	ctx = tenant.WithUserID(ctx, uuid.NewString())
	ctx = tenant.WithRole(ctx, "admin")
	return ctx, tid
}

func TestEngineSettings_OutcomeReviewChoice(t *testing.T) {
	ctx, _ := reviewAdminCtx(t)
	repo := &reviewEngineSettings{byProject: map[string]domain.ProjectEngineSettings{}}
	uc := NewManageProjectEngineSettings(repo)
	project := uuid.NewString()

	v, err := uc.SetProjectEngineSettings(ctx, SetProjectEngineSettingsInput{ProjectID: project, OutcomeReview: "manual"})
	if err != nil || v.Settings.OutcomeReview != domain.OutcomeReviewManual {
		t.Fatalf("choose manual: %+v err=%v", v.Settings, err)
	}
	v, err = uc.SetProjectEngineSettings(ctx, SetProjectEngineSettingsInput{ProjectID: project, ExpectedVersion: v.Settings.Version, SolutionAgent: false})
	if err != nil || v.Settings.OutcomeReview != domain.OutcomeReviewManual {
		t.Fatalf("an older client that does not send the field must keep the choice: %+v err=%v", v.Settings, err)
	}
	v, err = uc.SetProjectEngineSettings(ctx, SetProjectEngineSettingsInput{ProjectID: project, ExpectedVersion: v.Settings.Version, OutcomeReview: "off"})
	if err != nil || v.Settings.OutcomeReview != domain.OutcomeReviewOff {
		t.Fatalf("explicit off: %+v err=%v", v.Settings, err)
	}
	v, err = uc.SetProjectEngineSettings(ctx, SetProjectEngineSettingsInput{ProjectID: project, ExpectedVersion: v.Settings.Version, OutcomeReview: "default"})
	if err != nil || v.Settings.OutcomeReview != "" {
		t.Fatalf("default clears the choice: %+v err=%v", v.Settings, err)
	}
	if _, err := uc.SetProjectEngineSettings(ctx, SetProjectEngineSettingsInput{ProjectID: project, ExpectedVersion: v.Settings.Version, OutcomeReview: "auto"}); err == nil {
		t.Fatalf("there is no automatic mode; auto must be refused")
	}

	r := EngineSettingsReviewReader{Repo: repo}
	if m, err := r.OutcomeReviewMode(ctx, project); err != nil || m != "" {
		t.Fatalf("reader after clearing: %q %v", m, err)
	}
	if m, err := r.OutcomeReviewMode(ctx, uuid.NewString()); err != nil || m != "" {
		t.Fatalf("a project without a row has made no choice: %q %v", m, err)
	}
}

func TestOutcomeReviewConfig_EffectiveMode(t *testing.T) {
	cfg, warnings := domain.OutcomeReviewConfigFromEnv(func(k string) string {
		return map[string]string{"REQUEST_DEFAULT_OUTCOME_REVIEW": "manual"}[k]
	})
	if len(warnings) != 0 || cfg.Default != domain.OutcomeReviewManual {
		t.Fatalf("cfg %+v warnings %v", cfg, warnings)
	}
	if cfg.Effective("") != domain.OutcomeReviewManual || cfg.Effective(domain.OutcomeReviewOff) != domain.OutcomeReviewOff {
		t.Fatalf("a project's explicit off wins over a manual default; no choice follows the default")
	}
	cfg.KillSwitch = true
	if cfg.Effective(domain.OutcomeReviewManual) != domain.OutcomeReviewOff {
		t.Fatalf("the kill switch beats everything")
	}
	bad, warnings := domain.OutcomeReviewConfigFromEnv(func(k string) string {
		return map[string]string{"REQUEST_DEFAULT_OUTCOME_REVIEW": "auto", "REQUEST_REVIEW_KILL_SWITCH": "maybe"}[k]
	})
	if len(warnings) != 2 || bad.Default != domain.OutcomeReviewOff || bad.KillSwitch {
		t.Fatalf("unreadable values keep the safe answers and say so: %+v %v", bad, warnings)
	}
}
