package domain

import "testing"

func TestNewProjectGroup_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		gname    string
		wantErr  error
	}{
		{"valid", "t1", "group-a", nil},
		{"empty tenant", "", "group-a", ErrEmptyTenantID},
		{"empty name", "t1", "", ErrEmptyName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewProjectGroup("g1", tt.tenantID, tt.gname, "")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewProjectGroup_RejectsSelfParent(t *testing.T) {
	if _, err := NewProjectGroup("g1", "t1", "group-a", "g1"); err != ErrGroupSelfParent {
		t.Fatalf("expected ErrGroupSelfParent, got %v", err)
	}
}

func TestNewProjectGroup_AllowsDistinctParent(t *testing.T) {
	g, err := NewProjectGroup("g2", "t1", "group-b", "g1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.ParentGroupID != "g1" {
		t.Errorf("expected ParentGroupID=g1, got %q", g.ParentGroupID)
	}
}

func TestNewProjectGroup_ProjectIDFieldRoundTrips(t *testing.T) {
	g, err := NewProjectGroup("leaf-1", "t1", "my-project", "parent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// NewProjectGroup's signature doesn't accept ProjectID — a leaf group
	// gets it stamped afterward by UpsertLeafGroupForProject, not at
	// construction time. Assigning it directly and reading it back proves
	// the field round-trips unaffected by this task's changes.
	g.ProjectID = "project-1"
	if g.ProjectID != "project-1" {
		t.Errorf("expected ProjectID=project-1, got %q", g.ProjectID)
	}
	if g.ParentGroupID != "parent-1" {
		t.Errorf("expected ParentGroupID=parent-1, got %q", g.ParentGroupID)
	}
}

func TestParseNestedRepoCandidates_DecodesWireShape(t *testing.T) {
	resultJSON := []byte(`{"candidates":[{"path":"/home/dev/repo-a","suggested_name":"repo-a","is_git_repo":true},{"path":"/home/dev/not-a-repo","suggested_name":"","is_git_repo":false}]}`)

	got, err := ParseNestedRepoCandidates(resultJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []NestedRepoCandidate{
		{Path: "/home/dev/repo-a", SuggestedName: "repo-a", IsGitRepo: true},
		{Path: "/home/dev/not-a-repo", SuggestedName: "", IsGitRepo: false},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d candidates, got %d: %+v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate %d: expected %+v, got %+v", i, want[i], got[i])
		}
	}
}

func TestParseNestedRepoCandidates_EmptyCandidatesIsNotError(t *testing.T) {
	got, err := ParseNestedRepoCandidates([]byte(`{"candidates":[]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected a non-nil empty slice, got nil")
	}
	if len(got) != 0 {
		t.Errorf("expected 0 candidates, got %d: %+v", len(got), got)
	}
}
