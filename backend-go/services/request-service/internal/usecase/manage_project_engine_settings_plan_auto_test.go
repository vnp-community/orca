package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

func boolPtr(b bool) *bool { return &b }

func TestManageEngineSettings_PlanAutoGenerateStartsOnAndNilKeepsWhatIsStored(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()

	fresh, err := k.uc.GetProjectEngineSettings(adminCtx(tA), project, false)
	if err != nil || !fresh.Settings.PlanAutoGenerate {
		t.Fatalf("a project without a row reads as on: %+v %v", fresh.Settings, err)
	}
	first, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native"})
	if err != nil || !first.Settings.PlanAutoGenerate {
		t.Fatalf("first save without the field must be on: %+v %v", first.Settings, err)
	}
	off, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native", PlanAutoGenerate: boolPtr(false), ExpectedVersion: 1})
	if err != nil || off.Settings.PlanAutoGenerate {
		t.Fatalf("explicit false: %+v %v", off.Settings, err)
	}
	// An older client changes the Solution agent and sends no plan_auto_generate: the switch must not move.
	old, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native", SolutionAgent: true, ExpectedVersion: 2})
	if err != nil || old.Settings.PlanAutoGenerate || !old.Settings.SolutionAgent {
		t.Fatalf("an older client flipped the switch: %+v %v", old.Settings, err)
	}
	on, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native", PlanAutoGenerate: boolPtr(true), ExpectedVersion: 3})
	if err != nil || !on.Settings.PlanAutoGenerate {
		t.Fatalf("explicit true: %+v %v", on.Settings, err)
	}
	stored, _, _ := k.repo.Get(context.Background(), project)
	if !stored.PlanAutoGenerate || stored.Version != 4 {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestManageEngineSettings_ExplicitFalseOnAProjectWithoutARowIsStored(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()
	view, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native", PlanAutoGenerate: boolPtr(false)})
	if err != nil || view.Settings.PlanAutoGenerate {
		t.Fatalf("view = %+v %v", view.Settings, err)
	}
}

func TestManageEngineSettings_PlanAutoGenerateIsAudited(t *testing.T) {
	k := newSettingsKit()
	if _, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: uuid.NewString(), Engine: "native", PlanAutoGenerate: boolPtr(false)}); err != nil {
		t.Fatal(err)
	}
	if len(k.outbox.rows) != 1 {
		t.Fatalf("audit rows = %d", len(k.outbox.rows))
	}
	if !strings.Contains(k.outbox.rows[0].MetadataJSON, `"plan_auto_generate":false`) {
		t.Fatalf("audit metadata = %s", k.outbox.rows[0].MetadataJSON)
	}
}

func TestManageEngineSettings_StaleVersionStillConflictsWithTheNewField(t *testing.T) {
	k := newSettingsKit()
	project := uuid.NewString()
	if _, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native"}); err != nil {
		t.Fatal(err)
	}
	_, err := k.uc.SetProjectEngineSettings(adminCtx(tA), SetProjectEngineSettingsInput{ProjectID: project, Engine: "native", PlanAutoGenerate: boolPtr(false)})
	requireCode(t, err, "REQUEST_ENGINE_SETTINGS_VERSION_CONFLICT")
}

func TestDefaultedEngineSettings_ReadsAsPlanAutoOnForAProjectWithoutARow(t *testing.T) {
	inner := &memEngineSettings{rows: map[string]domain.ProjectEngineSettings{}}
	repo := NewDefaultedEngineSettings(inner, domain.EngineDefaults{Engine: domain.EngineOpenSpec})
	got, found, err := repo.Get(context.Background(), "p1")
	if err != nil || !found || !got.PlanAutoGenerate || !got.FromDeploymentDefault {
		t.Fatalf("got %+v found=%v err=%v", got, found, err)
	}
}
