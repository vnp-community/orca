package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/domain"
)

func TestCheckDevServerPreflight_GitMeetsMinTrueFor2392(t *testing.T) {
	ds := domain.DevServer{ID: "ds1", TenantID: "t1"}
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{
		"stdout": "GIT:git version 2.39.2\nNODE:v22.3.0\nDISK:10485760\nGH:gh version 2.40.0 (2024-01-01)\nPORT:FREE\n",
	}}
	uc := NewCheckDevServerPreflight(devRepo, agent)

	got, err := uc.Execute(context.Background(), "t1", "ds1", 3000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Git.MeetsMin {
		t.Errorf("expected Git.MeetsMin=true for 2.39.2, got %+v", got.Git)
	}
	if !got.Node.MeetsMin {
		t.Errorf("expected Node.MeetsMin=true for v22.3.0, got %+v", got.Node)
	}
	if !got.Disk.MeetsMin || got.Disk.FreeGB != 10 {
		t.Errorf("expected Disk.MeetsMin=true with FreeGB=10, got %+v", got.Disk)
	}
	if !got.GH.Installed || got.GH.Version != "gh version 2.40.0 (2024-01-01)" {
		t.Errorf("expected GH installed with version, got %+v", got.GH)
	}
	if !got.Port.Available || got.Port.Port != 3000 {
		t.Errorf("expected Port.Available=true, port=3000, got %+v", got.Port)
	}
}

func TestCheckDevServerPreflight_GitMeetsMinFalseFor2200(t *testing.T) {
	ds := domain.DevServer{ID: "ds1", TenantID: "t1"}
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	agent := &fakeDevServerAgentClient{execResult: map[string]any{
		"stdout": "GIT:git version 2.20.0\nNODE:v22.3.0\nDISK:10485760\nGH:\nPORT:BUSY\n",
	}}
	uc := NewCheckDevServerPreflight(devRepo, agent)

	got, err := uc.Execute(context.Background(), "t1", "ds1", 3000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Git.MeetsMin {
		t.Errorf("expected Git.MeetsMin=false for 2.20.0, got %+v", got.Git)
	}
	if !got.Git.Installed {
		t.Errorf("expected Git.Installed=true even though it's below the minimum, got %+v", got.Git)
	}
	if got.GH.Installed {
		t.Errorf("expected GH.Installed=false for an empty GH: line, got %+v", got.GH)
	}
	if got.Port.Available {
		t.Errorf("expected Port.Available=false for PORT:BUSY, got %+v", got.Port)
	}
}

func TestCheckDevServerPreflight_ExecErrorSurfacesAsPreflightFailed(t *testing.T) {
	ds := domain.DevServer{ID: "ds1", TenantID: "t1"}
	devRepo := &fakeDevServerRepository{byID: map[string]domain.DevServer{"ds1": ds}}
	agent := &fakeDevServerAgentClient{execErr: errors.New("agent unreachable")}
	uc := NewCheckDevServerPreflight(devRepo, agent)

	_, err := uc.Execute(context.Background(), "t1", "ds1", 3000)
	if err == nil {
		t.Fatal("expected an error when shell.exec fails")
	}
}

func TestParsePreflightOutput_MalformedOutputDegradesEveryFieldNeverPanics(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
	}{
		{"empty stdout", ""},
		{"garbage lines", "not a recognized line at all\nneither is this"},
		{"unparseable versions", "GIT:garbage\nNODE:garbage\nDISK:not-a-number\nGH:garbage\nPORT:garbage\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The only hard invariant: this must never panic regardless of
			// input shape. Beyond that, MeetsMin must never be true for
			// unparseable/absent version content.
			got := parsePreflightOutput(tt.stdout, 3000)
			if got.Git.MeetsMin || got.Node.MeetsMin {
				t.Errorf("expected MeetsMin=false for unparseable/absent version content, got git=%+v node=%+v", got.Git, got.Node)
			}
			if got.Disk.MeetsMin {
				t.Errorf("expected Disk.MeetsMin=false, got %+v", got.Disk)
			}
			if got.Port.Available {
				t.Errorf("expected Port.Available=false, got %+v", got.Port)
			}
		})
	}
}

func TestParsePreflightOutput_PortFreeAndBusy(t *testing.T) {
	free := parsePreflightOutput("PORT:FREE\n", 8080)
	if !free.Port.Available || free.Port.Port != 8080 {
		t.Errorf("expected PORT:FREE to parse as available=true port=8080, got %+v", free.Port)
	}
	busy := parsePreflightOutput("PORT:BUSY\n", 8080)
	if busy.Port.Available {
		t.Errorf("expected PORT:BUSY to parse as available=false, got %+v", busy.Port)
	}
}
