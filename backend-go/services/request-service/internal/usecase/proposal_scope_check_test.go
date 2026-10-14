package usecase

import (
	"strings"
	"testing"
)

func TestCheckProposalScope(t *testing.T) {
	const id = "add-ai"
	in := []string{
		"openspec/changes/add-ai/tasks.md",
		"openspec/changes/add-ai/specs/ai/spec.md",
		"openspec/changes/add-ai/./design.md",
		`openspec\changes\add-ai\proposal.md`,
	}
	if got := CheckProposalScope(in, id); len(got) != 0 {
		t.Fatalf("everything under the change directory is in scope, got %v", got)
	}
	out := []string{
		"backend/main.go",
		"openspec/changes/other/tasks.md",
		"openspec/changes/add-ai-evil/tasks.md",
		"openspec/changes/add-ai",
		"openspec/specs/ai/spec.md",
		"../etc/passwd",
		"/etc/passwd",
		"openspec/changes/add-ai/../other/tasks.md",
		"openspec/changes/add-ai/\x00x",
		"openspec/changes/add-ai/" + strings.Repeat("a", 600),
		".tdd-agent/config.toml",
		"docs/tdd/TDD-TMS.md",
	}
	got := CheckProposalScope(out, id)
	if len(got) != len(out) {
		t.Fatalf("every one of these is out of scope; %d of %d flagged: %v", len(got), len(out), got)
	}
}

func TestCheckProposalScope_EmptyChangeIDAllowsNothing(t *testing.T) {
	// A blank id would turn the prefix into "openspec/changes//": no real path starts with it.
	if got := CheckProposalScope([]string{"openspec/changes/x/tasks.md"}, ""); len(got) != 1 {
		t.Fatalf("got %v", got)
	}
}

func TestCheckProposalScope_NoChangesIsInScope(t *testing.T) {
	if got := CheckProposalScope(nil, "add-ai"); len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}
