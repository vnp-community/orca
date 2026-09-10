package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeProjectClient/fakeInfraFleetPicker are minimal test doubles for
// ServerResolver's two outbound ports — no real network client needed,
// matching this codebase's existing port-per-dependency pattern.
type fakeProjectClient struct {
	devServerID string
	err         error
	gotID       string // captures the last id passed, for assertions
}

func (f *fakeProjectClient) GetProject(ctx context.Context, id string) (string, error) {
	f.gotID = id
	if f.err != nil {
		return "", f.err
	}
	return f.devServerID, nil
}

type fakeInfraFleetPicker struct {
	connectionID string
	err          error
	gotTag       string
}

func (f *fakeInfraFleetPicker) PickByTag(ctx context.Context, tag string) (string, error) {
	f.gotTag = tag
	if f.err != nil {
		return "", f.err
	}
	return f.connectionID, nil
}

func TestServerResolver_ResolvesTargetKindProject(t *testing.T) {
	project := &fakeProjectClient{devServerID: "ds-1"}
	r := NewServerResolver(project, &fakeInfraFleetPicker{})

	got, err := r.Resolve(context.Background(), domain.TargetSpec{Kind: domain.TargetKindProject, ID: "proj-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "ds-1" {
		t.Errorf("expected ds-1, got %q", got)
	}
	if project.gotID != "proj-1" {
		t.Errorf("expected GetProject called with proj-1, got %q", project.gotID)
	}
}

func TestServerResolver_ResolvesTargetKindProject_UnboundProjectReturnsEmpty(t *testing.T) {
	project := &fakeProjectClient{devServerID: ""}
	r := NewServerResolver(project, &fakeInfraFleetPicker{})

	got, err := r.Resolve(context.Background(), domain.TargetSpec{Kind: domain.TargetKindProject, ID: "proj-unbound"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty devServerId for an unbound project, got %q", got)
	}
}

func TestServerResolver_TargetKindProject_ClientErrorPropagates(t *testing.T) {
	project := &fakeProjectClient{err: errors.New("project-service unreachable")}
	r := NewServerResolver(project, &fakeInfraFleetPicker{})

	_, err := r.Resolve(context.Background(), domain.TargetSpec{Kind: domain.TargetKindProject, ID: "proj-1"})
	if err == nil {
		t.Fatal("expected the client error to propagate")
	}
}

// TestServerResolver_ResolvesTargetKindServer_NoClientCall covers
// TargetKindServer's pure-passthrough contract — no client call at all.
func TestServerResolver_ResolvesTargetKindServer_NoClientCall(t *testing.T) {
	project := &fakeProjectClient{}
	infraFleet := &fakeInfraFleetPicker{}
	r := NewServerResolver(project, infraFleet)

	got, err := r.Resolve(context.Background(), domain.TargetSpec{Kind: domain.TargetKindServer, ID: "srv-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "srv-1" {
		t.Errorf("expected the raw server id passed through unchanged, got %q", got)
	}
	if project.gotID != "" || infraFleet.gotTag != "" {
		t.Error("expected TargetKindServer to make no client calls at all")
	}
}

func TestServerResolver_ResolvesTargetKindFleetTag(t *testing.T) {
	infraFleet := &fakeInfraFleetPicker{connectionID: "conn-1"}
	r := NewServerResolver(&fakeProjectClient{}, infraFleet)

	got, err := r.Resolve(context.Background(), domain.TargetSpec{Kind: domain.TargetKindFleetTag, Tag: "gpu-fleet"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "conn-1" {
		t.Errorf("expected conn-1, got %q", got)
	}
	if infraFleet.gotTag != "gpu-fleet" {
		t.Errorf("expected PickByTag called with gpu-fleet, got %q", infraFleet.gotTag)
	}
}

func TestServerResolver_TargetKindFleetTag_PickerErrorPropagates(t *testing.T) {
	infraFleet := &fakeInfraFleetPicker{err: errors.New("no connected server for this tag")}
	r := NewServerResolver(&fakeProjectClient{}, infraFleet)

	_, err := r.Resolve(context.Background(), domain.TargetSpec{Kind: domain.TargetKindFleetTag, Tag: "gpu-fleet"})
	if err == nil {
		t.Fatal("expected the picker error to propagate")
	}
}
