package domain

import "testing"

func TestSelectPolicy(t *testing.T) {
	// Stub test
	p1 := ApprovalPolicy{Enabled: true, Priority: 1}
	p2 := ApprovalPolicy{Enabled: true, Priority: 2}

	p, ok := SelectPolicy([]ApprovalPolicy{p1, p2}, PolicyContext{})
	if !ok || p.Priority != 2 {
		t.Errorf("expected p2")
	}
}

func TestDefaultPolicy(t *testing.T) {
	// Stub test
	p := DefaultPolicy(SubjectPreDeploy, PolicyContext{})
	if len(p.Approvers) != 1 || p.Approvers[0].ID != "admin" {
		t.Errorf("unexpected default policy for pre_deploy")
	}
}
