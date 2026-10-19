package usecase

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type defaultsProbeStore struct {
	row   *domain.ProjectEngineSettings
	wrote []domain.ProjectEngineSettings
}

func (f *defaultsProbeStore) Get(context.Context, string) (domain.ProjectEngineSettings, bool, error) {
	if f.row == nil {
		return domain.ProjectEngineSettings{}, false, nil
	}
	return *f.row, true, nil
}

func (f *defaultsProbeStore) Upsert(_ context.Context, s domain.ProjectEngineSettings, _ int64) (domain.ProjectEngineSettings, error) {
	f.wrote = append(f.wrote, s)
	return s, nil
}

func TestDefaultedEngineSettings_AProjectWithoutARowRunsWithTheDeploymentDefaults(t *testing.T) {
	repo := NewDefaultedEngineSettings(&defaultsProbeStore{}, domain.EngineDefaults{Engine: domain.EngineOpenSpec, SolutionAgent: true})
	s, found, err := repo.Get(context.Background(), "p1")
	if err != nil || !found || s.Engine != domain.EngineOpenSpec || !s.SolutionAgent || s.Version != 0 {
		t.Fatalf("settings=%+v found=%v err=%v", s, found, err)
	}
}

func TestDefaultedEngineSettings_ASavedRowWinsEvenWhenItTurnsEverythingOff(t *testing.T) {
	saved := domain.ProjectEngineSettings{ProjectID: "p1", Engine: domain.EngineNative, Version: 3}
	repo := NewDefaultedEngineSettings(&defaultsProbeStore{row: &saved}, domain.EngineDefaults{Engine: domain.EngineOpenSpec, SolutionAgent: true})
	s, _, _ := repo.Get(context.Background(), "p1")
	if s.Engine != domain.EngineNative || s.SolutionAgent || s.Version != 3 {
		t.Fatalf("a saved row must win: %+v", s)
	}
}

func TestDefaultedEngineSettings_NativeAndOffChangesNothing(t *testing.T) {
	inner := &defaultsProbeStore{}
	if got := NewDefaultedEngineSettings(inner, domain.EngineDefaults{Engine: domain.EngineNative}); got != EngineSettingsRepository(inner) {
		t.Fatal("the plain defaults must not wrap the store")
	}
}

func TestDefaultedEngineSettings_AWriteGoesToTheStoreUnchanged(t *testing.T) {
	inner := &defaultsProbeStore{}
	repo := NewDefaultedEngineSettings(inner, domain.EngineDefaults{Engine: domain.EngineOpenSpec, SolutionAgent: true})
	want := domain.ProjectEngineSettings{ProjectID: "p1", Engine: domain.EngineNative}
	if _, err := repo.Upsert(context.Background(), want, 0); err != nil || len(inner.wrote) != 1 || inner.wrote[0] != want {
		t.Fatalf("wrote=%v err=%v", inner.wrote, err)
	}
}
