package grpc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type memEngineRows struct {
	rows map[string]domain.ProjectEngineSettings
}

func (m *memEngineRows) Get(_ context.Context, id string) (domain.ProjectEngineSettings, bool, error) {
	s, ok := m.rows[id]
	return s, ok, nil
}

func (m *memEngineRows) Upsert(_ context.Context, s domain.ProjectEngineSettings, expected int64) (domain.ProjectEngineSettings, error) {
	if cur, ok := m.rows[s.ProjectID]; (expected == 0) == ok || (ok && cur.Version != expected) {
		return s, domain.ErrEngineSettingsVersionConflict
	}
	s.Version = expected + 1
	m.rows[s.ProjectID] = s
	return s, nil
}

func engineSettingsServer() *Server {
	return (&Server{}).WithEngineSettings(usecase.NewManageProjectEngineSettings(&memEngineRows{rows: map[string]domain.ProjectEngineSettings{}}))
}

func TestEngineSettingsRPC_UnimplementedUntilWired(t *testing.T) {
	_, err := (&Server{}).GetProjectEngineSettings(context.Background(), &requestv1.GetProjectEngineSettingsRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("err = %v", err)
	}
	_, err = (&Server{}).SetProjectEngineSettings(context.Background(), &requestv1.SetProjectEngineSettingsRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("err = %v", err)
	}
}

func TestEngineSettingsRPC_AdminSetsBothSwitchesAndAnyoneReadsThem(t *testing.T) {
	s := engineSettingsServer()
	project := uuid.NewString()
	set, err := s.SetProjectEngineSettings(roleCtx("t1", "admin"), &requestv1.SetProjectEngineSettingsRequest{ProjectId: project, SolutionEngine: "openspec", SolutionAgent: true})
	if err != nil || set.GetSettings().GetVersion() != 1 || !set.GetSettings().GetSolutionAgent() || set.GetSettings().GetSolutionEngine() != "openspec" {
		t.Fatalf("set: %v %v", set, err)
	}
	// No readiness gate is wired here, so the response says the check did not run instead of staying silent.
	if got := set.GetPreflight().GetWarnings(); len(got) != 1 || got[0] != "preflight_not_run" || set.GetPreflight().GetRan() {
		t.Fatalf("preflight = %v", set.GetPreflight())
	}
	got, err := s.GetProjectEngineSettings(roleCtx("t1", "member"), &requestv1.GetProjectEngineSettingsRequest{ProjectId: project})
	if err != nil || !got.GetSettings().GetSolutionAgent() || got.GetPreflight() != nil {
		t.Fatalf("get: %v %v", got, err)
	}
}

func TestEngineSettingsRPC_MapsRefusalsToStatusCodes(t *testing.T) {
	s := engineSettingsServer()
	project := uuid.NewString()
	if _, err := s.SetProjectEngineSettings(roleCtx("t1", "member"), &requestv1.SetProjectEngineSettingsRequest{ProjectId: project, SolutionEngine: "openspec"}); status.Code(err) != codes.PermissionDenied {
		t.Fatalf("member: %v", err)
	}
	if _, err := s.SetProjectEngineSettings(roleCtx("t1", "admin"), &requestv1.SetProjectEngineSettingsRequest{ProjectId: project, SolutionEngine: "magic"}); status.Code(err) != codes.InvalidArgument {
		t.Fatalf("bad engine: %v", err)
	}
	if _, err := s.SetProjectEngineSettings(roleCtx("t1", "admin"), &requestv1.SetProjectEngineSettingsRequest{ProjectId: project, SolutionEngine: "native"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetProjectEngineSettings(roleCtx("t1", "admin"), &requestv1.SetProjectEngineSettingsRequest{ProjectId: project, SolutionEngine: "native"}); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("second insert: %v", err)
	}
}

func TestEngineSettingsRPC_PlanAutoGenerateMapsBothWaysAndAbsentKeepsTheStoredValue(t *testing.T) {
	s := engineSettingsServer()
	project := uuid.NewString()
	off := false
	set, err := s.SetProjectEngineSettings(roleCtx("t1", "admin"), &requestv1.SetProjectEngineSettingsRequest{ProjectId: project, SolutionEngine: "native", PlanAutoGenerate: &off})
	if err != nil || set.GetSettings().GetPlanAutoGenerate() {
		t.Fatalf("explicit false: %v %v", set, err)
	}
	// An older client sends no plan_auto_generate: the switch stays off.
	set, err = s.SetProjectEngineSettings(roleCtx("t1", "admin"), &requestv1.SetProjectEngineSettingsRequest{ProjectId: project, SolutionEngine: "native", SolutionAgent: true, ExpectedVersion: 1})
	if err != nil || set.GetSettings().GetPlanAutoGenerate() || !set.GetSettings().GetSolutionAgent() {
		t.Fatalf("absent field: %v %v", set, err)
	}
	got, err := s.GetProjectEngineSettings(roleCtx("t1", "member"), &requestv1.GetProjectEngineSettingsRequest{ProjectId: project})
	if err != nil || got.GetSettings().GetPlanAutoGenerate() {
		t.Fatalf("get: %v %v", got, err)
	}
	// A project without a row reads as on.
	fresh, err := s.GetProjectEngineSettings(roleCtx("t1", "member"), &requestv1.GetProjectEngineSettingsRequest{ProjectId: uuid.NewString()})
	if err != nil || !fresh.GetSettings().GetPlanAutoGenerate() {
		t.Fatalf("fresh: %v %v", fresh, err)
	}
	on := true
	set, err = s.SetProjectEngineSettings(roleCtx("t1", "admin"), &requestv1.SetProjectEngineSettingsRequest{ProjectId: project, SolutionEngine: "native", PlanAutoGenerate: &on, ExpectedVersion: 2})
	if err != nil || !set.GetSettings().GetPlanAutoGenerate() {
		t.Fatalf("explicit true: %v %v", set, err)
	}
}
