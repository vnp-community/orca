package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// memEngineSettings keeps one row per project with the same optimistic-version rule as the real repositories.
type memEngineSettings struct {
	rows map[string]domain.ProjectEngineSettings
}

func (m *memEngineSettings) Get(_ context.Context, projectID string) (domain.ProjectEngineSettings, bool, error) {
	s, ok := m.rows[projectID]
	return s, ok, nil
}

func (m *memEngineSettings) Upsert(_ context.Context, s domain.ProjectEngineSettings, expected int64) (domain.ProjectEngineSettings, error) {
	cur, ok := m.rows[s.ProjectID]
	if (expected == 0 && ok) || (expected != 0 && (!ok || cur.Version != expected)) {
		return s, domain.ErrEngineSettingsVersionConflict
	}
	s.Version = expected + 1
	m.rows[s.ProjectID] = s
	return s, nil
}

type settingsKit struct {
	repo   *memEngineSettings
	uc     *ManageProjectEngineSettings
	outbox *memAuditOutbox
	sink   *memSink
	exec   *fakeExec
	caps   *profileCaps
	files  *fakeFileStat
}

func newSettingsKit() *settingsKit {
	k := &settingsKit{repo: &memEngineSettings{rows: map[string]domain.ProjectEngineSettings{}}, outbox: &memAuditOutbox{}}
	k.exec = &fakeExec{res: ExecResult{Stdout: "1.3.0"}}
	k.caps = &profileCaps{profile: fullProfile()}
	k.files = &fakeFileStat{info: PathInfo{Exists: true}}
	gate := NewEngineReadinessGate(worktreeConn, k.exec, "").WithCapabilities(k.caps).WithRepoFiles(k.files)
	var rec *AuditRecorder
	rec, k.sink = newRecorder(k.outbox, rollbackTx{k.outbox})
	k.uc = NewManageProjectEngineSettings(k.repo).WithReadiness(gate).WithAudit(rec, rollbackTx{k.outbox})
	return k
}

func memberCtx() context.Context {
	return tenant.WithRole(tenant.WithUserID(tenant.WithTenantID(context.Background(), tA), "b0000000-0000-4000-8000-000000000002"), "member")
}

func TestManageEngineSettings_OnlyAnAdministratorChangesThem(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()
	_, err := k.uc.SetProjectEngineSettings(memberCtx(), SetProjectEngineSettingsInput{ProjectID: project, Engine: "openspec"})
	if err == nil || !strings.Contains(err.Error(), "REQUEST_FORBIDDEN") {
		t.Fatalf("err = %v", err)
	}
	if len(k.repo.rows) != 0 || len(k.outbox.rows) != 0 {
		t.Fatal("a refused change left something behind")
	}
}

func TestManageEngineSettings_RefusesWhatItCannotRun(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()
	cases := map[string]SetProjectEngineSettingsInput{
		"unknown engine":      {ProjectID: project, Engine: "magic"},
		"bad minimum version": {ProjectID: project, Engine: "openspec", MinVersion: "latest"},
		"project not a uuid":  {ProjectID: "p1", Engine: "native"},
		"negative version":    {ProjectID: project, Engine: "native", ExpectedVersion: -1},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := k.uc.SetProjectEngineSettings(adminCtx(tA), in)
			if err == nil || !strings.Contains(err.Error(), "REQUEST_ENGINE_SETTINGS_INVALID") {
				t.Fatalf("err = %v", err)
			}
		})
	}
	if len(k.repo.rows) != 0 {
		t.Fatal("an invalid setting was saved")
	}
}

func TestManageEngineSettings_SavesBothSwitchesAuditsAndRunsTheCheckForWhatWasSwitchedOn(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()
	view, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "openspec", SolutionAgent: true})
	if err != nil {
		t.Fatal(err)
	}
	if view.Settings.Version != 1 || !view.Settings.SolutionAgent || view.Settings.Engine != domain.EngineOpenSpec {
		t.Fatalf("settings = %+v", view.Settings)
	}
	if view.Preflight == nil || !view.Preflight.OK || len(view.Preflight.Gaps) != 0 {
		t.Fatalf("preflight = %+v", view.Preflight)
	}
	if len(k.outbox.rows) != 1 || k.outbox.rows[0].Action != domain.AuditEngineSettingsSet || k.outbox.rows[0].TargetID != project {
		t.Fatalf("audit rows = %+v", k.outbox.rows)
	}
	got, _, _ := k.repo.Get(context.Background(), project)
	if !got.SolutionAgent || got.UpdatedBy == "" {
		t.Fatalf("stored = %+v", got)
	}
}

func TestManageEngineSettings_ASettingThatCannotRunYetIsStillSavedWithTheReasons(t *testing.T) {
	k := newSettingsKit()
	k.exec.res = ExecResult{ExitCode: 127}
	project := uuid.NewString()
	view, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "openspec"})
	if err != nil {
		t.Fatalf("saving must not depend on the check: %v", err)
	}
	if view.Preflight == nil || view.Preflight.OK || view.Preflight.Failures[0].Code != "REQUEST_ENGINE_OPENSPEC_MISSING" || view.Preflight.Failures[0].FixHint == "" {
		t.Fatalf("preflight = %+v", view.Preflight)
	}
}

func TestManageEngineSettings_NoCheckWhenNothingIsSwitchedOn_AndNoGateMeansPreflightNotRun(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()
	view, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native"})
	if err != nil || view.Preflight != nil || len(view.Warnings) != 0 || len(k.exec.seen) != 0 {
		t.Fatalf("view=%+v err=%v probes=%d", view, err, len(k.exec.seen))
	}
	bare := NewManageProjectEngineSettings(&memEngineSettings{rows: map[string]domain.ProjectEngineSettings{}})
	view, err = bare.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: uuid.NewString(), Engine: "openspec"})
	if err != nil || view.Preflight != nil || len(view.Warnings) != 1 || view.Warnings[0] != "preflight_not_run" {
		t.Fatalf("view=%+v err=%v", view, err)
	}
}

func TestManageEngineSettings_AStaleVersionIsRefused_AndTurningASwitchOffIsSaved(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()
	ctx := adminCtx(tA)
	if _, err := k.uc.SetProjectEngineSettings(ctx, SetProjectEngineSettingsInput{ProjectID: project, Engine: "openspec", SolutionAgent: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := k.uc.SetProjectEngineSettings(ctx, SetProjectEngineSettingsInput{ProjectID: project, Engine: "native", ExpectedVersion: 7}); !errors.Is(err, domain.ErrEngineSettingsVersionConflict) {
		t.Fatalf("err = %v", err)
	}
	if _, err := k.uc.SetProjectEngineSettings(ctx, SetProjectEngineSettingsInput{ProjectID: project, Engine: "native", ExpectedVersion: 1}); err != nil {
		t.Fatal(err)
	}
	got, _, _ := k.repo.Get(context.Background(), project)
	if got.Engine != domain.EngineNative || got.SolutionAgent || got.Version != 2 {
		t.Fatalf("stored = %+v", got)
	}
}

func TestManageEngineSettings_GetReadsDefaultsAndRunsTheCheckOnlyWhenAsked(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()
	view, err := k.uc.GetProjectEngineSettings(memberCtx(), project, false)
	if err != nil || view.Settings.Engine != domain.EngineNative || view.Settings.SolutionAgent || view.Settings.Version != 0 || view.Preflight != nil {
		t.Fatalf("view=%+v err=%v", view, err)
	}
	k.files.info = PathInfo{} // the repo has no knowledge config; a gapped verdict is never cached
	if _, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "openspec"}); err != nil {
		t.Fatal(err)
	}
	k.exec.seen = nil
	if view, _ = k.uc.GetProjectEngineSettings(memberCtx(), project, false); view.Preflight != nil || len(k.exec.seen) != 0 {
		t.Fatalf("a plain read ran the check: %+v", view)
	}
	view, _ = k.uc.GetProjectEngineSettings(memberCtx(), project, true)
	if view.Preflight == nil || !view.Preflight.OK || len(view.Preflight.Gaps) != 1 || view.Preflight.Gaps[0].Code != "REQUEST_KNOWLEDGE_NOT_INITIALIZED" {
		t.Fatalf("preflight = %+v", view.Preflight)
	}
	if _, err := k.uc.GetProjectEngineSettings(memberCtx(), "nope", false); err == nil {
		t.Fatal("a malformed project id was accepted")
	}
}

// The tab of the Outcome Review shows from this flag, so it must be the effective one: the project's choice, else the deployment
// default, and never past the kill switch.
func TestManageEngineSettings_ReportsWhetherTheReviewIsOnNow(t *testing.T) {
	read := func(cfg domain.OutcomeReviewConfig, saved string) bool {
		k := newSettingsKit()
		k.uc = k.uc.WithOutcomeReviewConfig(cfg)
		project := uuid.NewString()
		if saved != "" {
			if _, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native", OutcomeReview: saved}); err != nil {
				t.Fatal(err)
			}
		}
		view, err := k.uc.GetProjectEngineSettings(memberCtx(), project, false)
		if err != nil {
			t.Fatal(err)
		}
		return view.OutcomeReviewEnabled
	}
	manual := domain.OutcomeReviewConfig{Default: domain.OutcomeReviewManual}
	off := domain.OutcomeReviewConfig{Default: domain.OutcomeReviewOff}
	killed := domain.OutcomeReviewConfig{Default: domain.OutcomeReviewManual, KillSwitch: true}
	for name, c := range map[string]struct {
		cfg   domain.OutcomeReviewConfig
		saved string
		want  bool
	}{
		"no choice follows a manual default": {manual, "", true},
		"no choice follows an off default":   {off, "", false},
		"a project's manual wins over off":   {off, "manual", true},
		"a project's off wins over manual":   {manual, "off", false},
		"the kill switch wins over all":      {killed, "manual", false},
	} {
		if got := read(c.cfg, c.saved); got != c.want {
			t.Errorf("%s: enabled=%v, want %v", name, got, c.want)
		}
	}
}
