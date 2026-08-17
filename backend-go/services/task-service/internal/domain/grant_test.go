package domain

import "testing"

func TestGrant_Matches(t *testing.T) {
	caller := CallerIdentity{UserID: "user-1", TeamIDs: []string{"team-a", "team-b"}, CompanyID: "company-1"}

	tests := []struct {
		name  string
		grant Grant
		want  bool
	}{
		{"owner grant matches caller by user id", Grant{SubjectID: "user-1", Level: GrantLevelOwner}, true},
		{"owner grant does not match a different user", Grant{SubjectID: "user-2", Level: GrantLevelOwner}, false},
		{"admin grant matches caller by user id", Grant{SubjectID: "user-1", Level: GrantLevelAdmin}, true},
		{"user grant matches caller by user id", Grant{SubjectID: "user-1", Level: GrantLevelUser}, true},
		{"team grant matches caller team membership", Grant{SubjectID: "team-a", Level: GrantLevelTeam}, true},
		{"team grant does not match a team caller isn't in", Grant{SubjectID: "team-z", Level: GrantLevelTeam}, false},
		{"company grant matches caller's company", Grant{SubjectID: "company-1", Level: GrantLevelCompany}, true},
		{"company grant does not match a different company", Grant{SubjectID: "company-2", Level: GrantLevelCompany}, false},
		{"empty subject never matches", Grant{SubjectID: "", Level: GrantLevelOwner}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.grant.Matches(caller); got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGrantLevel_PriorityOrdering(t *testing.T) {
	// owner > admin > user > team > company, per task-service.md §4.1 step 4.
	levels := []GrantLevel{GrantLevelOwner, GrantLevelAdmin, GrantLevelUser, GrantLevelTeam, GrantLevelCompany}
	for i := 0; i < len(levels)-1; i++ {
		if levels[i].priority() >= levels[i+1].priority() {
			t.Errorf("expected %v to outrank %v", levels[i], levels[i+1])
		}
	}
}
