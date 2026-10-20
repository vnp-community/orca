package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

type stubEngineRows struct {
	row   domain.ProjectEngineSettings
	found bool
	err   error
	reads int
}

func (s *stubEngineRows) Get(context.Context, string) (domain.ProjectEngineSettings, bool, error) {
	s.reads++
	return s.row, s.found, s.err
}

func (s *stubEngineRows) Upsert(_ context.Context, row domain.ProjectEngineSettings, _ int64) (domain.ProjectEngineSettings, error) {
	return row, nil
}

func TestPlanAutoSwitch_Enabled(t *testing.T) {
	boom := errors.New("db down")
	cases := []struct {
		name      string
		sw        PlanAutoSwitch
		rows      *stubEngineRows
		want      bool
		wantErr   bool
		wantReads int
	}{
		{"env off", PlanAutoSwitch{EnvOn: false, CommitWired: true}, &stubEngineRows{}, false, false, 0},
		{"commit not wired", PlanAutoSwitch{EnvOn: true, CommitWired: false}, &stubEngineRows{}, false, false, 0},
		{"project without a row counts as on", PlanAutoSwitch{EnvOn: true, CommitWired: true}, &stubEngineRows{}, true, false, 1},
		{"project row on", PlanAutoSwitch{EnvOn: true, CommitWired: true}, &stubEngineRows{found: true, row: domain.ProjectEngineSettings{PlanAutoGenerate: true}}, true, false, 1},
		{"project row off", PlanAutoSwitch{EnvOn: true, CommitWired: true}, &stubEngineRows{found: true}, false, false, 1},
		{"a failed read never turns it on", PlanAutoSwitch{EnvOn: true, CommitWired: true}, &stubEngineRows{err: boom}, false, true, 1},
	}
	for _, c := range cases {
		c.sw.Settings = c.rows
		got, err := c.sw.Enabled(context.Background(), "p1")
		if got != c.want || (err != nil) != c.wantErr || c.rows.reads != c.wantReads {
			t.Errorf("%s: got (%v, %v) reads=%d, want (%v, err=%v) reads=%d", c.name, got, err, c.rows.reads, c.want, c.wantErr, c.wantReads)
		}
	}
}

func TestPlanAutoSwitch_WithoutAProjectStoreOnlyTheGlobalSwitchesApply(t *testing.T) {
	on := PlanAutoSwitch{EnvOn: true, CommitWired: true}
	if got, err := on.Enabled(context.Background(), "p1"); err != nil || !got {
		t.Fatalf("got (%v, %v)", got, err)
	}
	if !on.GloballyAvailable() || (PlanAutoSwitch{EnvOn: true}).GloballyAvailable() || (PlanAutoSwitch{CommitWired: true}).GloballyAvailable() {
		t.Fatal("GloballyAvailable needs both the env switch and a wired commit")
	}
}
