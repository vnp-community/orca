package domain

import "testing"

func TestParseTargetSpec_Project(t *testing.T) {
	spec, err := ParseTargetSpec("project:proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Kind != TargetKindProject || spec.ID != "proj-1" {
		t.Errorf("got %+v, want Kind=TargetKindProject ID=proj-1", spec)
	}
}

func TestParseTargetSpec_Server(t *testing.T) {
	spec, err := ParseTargetSpec("server:srv-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Kind != TargetKindServer || spec.ID != "srv-1" {
		t.Errorf("got %+v, want Kind=TargetKindServer ID=srv-1", spec)
	}
}

func TestParseTargetSpec_FleetTag(t *testing.T) {
	spec, err := ParseTargetSpec("fleet:tag:gpu-fleet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.Kind != TargetKindFleetTag || spec.Tag != "gpu-fleet" {
		t.Errorf("got %+v, want Kind=TargetKindFleetTag Tag=gpu-fleet", spec)
	}
}

func TestParseTargetSpec_UnknownPrefixRejected(t *testing.T) {
	_, err := ParseTargetSpec("literal-connection-id")
	if err != ErrUnknownTargetKind {
		t.Errorf("expected ErrUnknownTargetKind, got %v", err)
	}
}

func TestParseTargetSpec_EmptyStringRejected(t *testing.T) {
	_, err := ParseTargetSpec("")
	if err != ErrUnknownTargetKind {
		t.Errorf("expected ErrUnknownTargetKind for an empty spec, got %v", err)
	}
}
