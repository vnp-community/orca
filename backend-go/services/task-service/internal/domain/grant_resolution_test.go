package domain

import "testing"

func TestResolveGrant_FindsGrantOnTargetTaskItself(t *testing.T) {
	caller := CallerIdentity{UserID: "user-1"}
	chain := []string{"task-1", "task-0-parent", "task-root"}
	grants := map[string][]Grant{
		"task-1": {{TaskID: "task-1", SubjectID: "user-1", Level: GrantLevelAdmin, ApplyTree: false}},
	}

	level, found := ResolveGrant(chain, grants, caller, 0)
	if !found {
		t.Fatal("expected a grant to be found")
	}
	if level != GrantLevelAdmin {
		t.Errorf("expected GrantLevelAdmin, got %v", level)
	}
}

func TestResolveGrant_TargetTaskGrantAppliesEvenWithoutApplyTree(t *testing.T) {
	// A grant directly on the task itself must count regardless of
	// ApplyTree — apply_tree only governs inheritance to descendants, not
	// whether the grant applies to the task it's directly attached to.
	caller := CallerIdentity{UserID: "user-1"}
	chain := []string{"task-1"}
	grants := map[string][]Grant{
		"task-1": {{TaskID: "task-1", SubjectID: "user-1", Level: GrantLevelUser, ApplyTree: false}},
	}

	level, found := ResolveGrant(chain, grants, caller, 0)
	if !found || level != GrantLevelUser {
		t.Fatalf("expected GrantLevelUser found=true, got %v found=%v", level, found)
	}
}

func TestResolveGrant_FallsThroughMultipleAncestorLevels(t *testing.T) {
	// No grant at task-1 or its immediate parent; the grandparent has an
	// inherited (apply_tree=true) grant that must be found by continuing
	// the walk past the immediate parent.
	caller := CallerIdentity{UserID: "user-1"}
	chain := []string{"task-1", "parent", "grandparent", "root"}
	grants := map[string][]Grant{
		"grandparent": {{TaskID: "grandparent", SubjectID: "user-1", Level: GrantLevelUser, ApplyTree: true}},
	}

	level, found := ResolveGrant(chain, grants, caller, 0)
	if !found {
		t.Fatal("expected the grandparent's inherited grant to be found")
	}
	if level != GrantLevelUser {
		t.Errorf("expected GrantLevelUser, got %v", level)
	}
}

func TestResolveGrant_ApplyTreeFalseStopsInheritance(t *testing.T) {
	// The parent has a grant, but ApplyTree=false — it must NOT apply to
	// task-1, a descendant. No other grant exists, so resolution defaults
	// to not-found.
	caller := CallerIdentity{UserID: "user-1"}
	chain := []string{"task-1", "parent", "root"}
	grants := map[string][]Grant{
		"parent": {{TaskID: "parent", SubjectID: "user-1", Level: GrantLevelOwner, ApplyTree: false}},
	}

	_, found := ResolveGrant(chain, grants, caller, 0)
	if found {
		t.Error("expected apply_tree=false on an ancestor to NOT be inherited")
	}
}

func TestResolveGrant_ApplyTreeFalseAtOneAncestorDoesNotBlockADeeperOne(t *testing.T) {
	// The immediate parent's grant is not inherited (ApplyTree=false), but
	// the grandparent's IS (ApplyTree=true) — the walk must continue past
	// the non-inherited ancestor rather than stopping there.
	caller := CallerIdentity{UserID: "user-1"}
	chain := []string{"task-1", "parent", "grandparent"}
	grants := map[string][]Grant{
		"parent":      {{TaskID: "parent", SubjectID: "user-1", Level: GrantLevelOwner, ApplyTree: false}},
		"grandparent": {{TaskID: "grandparent", SubjectID: "user-1", Level: GrantLevelAdmin, ApplyTree: true}},
	}

	level, found := ResolveGrant(chain, grants, caller, 0)
	if !found || level != GrantLevelAdmin {
		t.Fatalf("expected GrantLevelAdmin found=true, got %v found=%v", level, found)
	}
}

func TestResolveGrant_PriorityWinsOverProximity(t *testing.T) {
	// task-1 itself grants "user" level; a distant ancestor inherits an
	// "owner" grant. Per §4.1 step 4, priority (owner) beats proximity
	// (the nearer "user" grant).
	caller := CallerIdentity{UserID: "user-1"}
	chain := []string{"task-1", "parent", "grandparent"}
	grants := map[string][]Grant{
		"task-1":      {{TaskID: "task-1", SubjectID: "user-1", Level: GrantLevelUser, ApplyTree: false}},
		"grandparent": {{TaskID: "grandparent", SubjectID: "user-1", Level: GrantLevelOwner, ApplyTree: true}},
	}

	level, found := ResolveGrant(chain, grants, caller, 0)
	if !found || level != GrantLevelOwner {
		t.Fatalf("expected the distant owner grant to win, got %v found=%v", level, found)
	}
}

func TestResolveGrant_TeamGrantResolvedViaCallerTeamIDs(t *testing.T) {
	caller := CallerIdentity{UserID: "user-1", TeamIDs: []string{"team-a"}}
	chain := []string{"task-1"}
	grants := map[string][]Grant{
		"task-1": {{TaskID: "task-1", SubjectID: "team-a", Level: GrantLevelTeam, ApplyTree: false}},
	}

	level, found := ResolveGrant(chain, grants, caller, 0)
	if !found || level != GrantLevelTeam {
		t.Fatalf("expected GrantLevelTeam found=true, got %v found=%v", level, found)
	}
}

func TestResolveGrant_NoMatchAnywhereDefaultsToDeny(t *testing.T) {
	caller := CallerIdentity{UserID: "user-1"}
	chain := []string{"task-1", "parent", "root"}
	grants := map[string][]Grant{
		"parent": {{TaskID: "parent", SubjectID: "someone-else", Level: GrantLevelOwner, ApplyTree: true}},
	}

	level, found := ResolveGrant(chain, grants, caller, 0)
	if found {
		t.Errorf("expected no match, got level=%v", level)
	}
	if level != GrantLevelUnspecified {
		t.Errorf("expected GrantLevelUnspecified on no-match, got %v", level)
	}
}

func TestResolveGrant_MaxDepthGuardStopsTheWalk(t *testing.T) {
	// A grant exists 3 hops up, but maxDepth=2 caps the walk at the target
	// task plus one ancestor — the deep grant must not be found.
	caller := CallerIdentity{UserID: "user-1"}
	chain := []string{"task-1", "parent", "grandparent", "great-grandparent"}
	grants := map[string][]Grant{
		"great-grandparent": {{TaskID: "great-grandparent", SubjectID: "user-1", Level: GrantLevelOwner, ApplyTree: true}},
	}

	_, found := ResolveGrant(chain, grants, caller, 2)
	if found {
		t.Error("expected the max-depth guard to prevent finding a grant beyond the cap")
	}
}

func TestResolveGrant_BestOfMultipleCandidatesAtSameTask(t *testing.T) {
	caller := CallerIdentity{UserID: "user-1", TeamIDs: []string{"team-a"}}
	chain := []string{"task-1"}
	grants := map[string][]Grant{
		"task-1": {
			{TaskID: "task-1", SubjectID: "team-a", Level: GrantLevelTeam, ApplyTree: false},
			{TaskID: "task-1", SubjectID: "user-1", Level: GrantLevelAdmin, ApplyTree: false},
		},
	}

	level, found := ResolveGrant(chain, grants, caller, 0)
	if !found || level != GrantLevelAdmin {
		t.Fatalf("expected GrantLevelAdmin (higher priority than team), got %v found=%v", level, found)
	}
}
