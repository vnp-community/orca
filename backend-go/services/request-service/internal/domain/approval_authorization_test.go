package domain

import "testing"

func TestApprovalAuthorization_Decide(t *testing.T) {
	auth := ApprovalAuthorization{}
	users := func(ids ...string) []Principal {
		var ps []Principal
		for _, id := range ids {
			ps = append(ps, Principal{Kind: PrincipalKindUser, ID: id})
		}
		return ps
	}
	tests := []struct {
		name      string
		actor     DecisionActor
		ap        Approval
		approvers []Principal
		reporter  string
		want      error
	}{
		{"named user", DecisionActor{UserID: "u1"}, Approval{SelfApprovalAllowed: true}, users("u1"), "r1", nil},
		{"no user", DecisionActor{}, Approval{}, users("u1"), "r1", ErrNoUser},
		{"machine", DecisionActor{UserID: "u1", IsMachine: true}, Approval{SelfApprovalAllowed: true}, users("u1"), "r1", ErrAgentForbidden},
		{"outsider", DecisionActor{UserID: "u2"}, Approval{SelfApprovalAllowed: true}, users("u1"), "r1", ErrNotApprover},
		{"team member", DecisionActor{UserID: "u2", TeamIDs: []string{"t1"}}, Approval{SelfApprovalAllowed: true}, []Principal{{Kind: PrincipalKindTeam, ID: "t1"}}, "r1", nil},
		{"role", DecisionActor{UserID: "u2", Role: "lead"}, Approval{SelfApprovalAllowed: true}, []Principal{{Kind: PrincipalKindRole, ID: "lead"}}, "r1", nil},
		{"reporter principal", DecisionActor{UserID: "r1"}, Approval{SelfApprovalAllowed: true}, []Principal{{Kind: PrincipalKindReporter}}, "r1", nil},
		{"admin is always an approver", DecisionActor{UserID: "u9", Role: "admin"}, Approval{SelfApprovalAllowed: true}, users("u1"), "r1", nil},
		{"reporter blocked by separation", DecisionActor{UserID: "r1"}, Approval{SelfApprovalAllowed: false}, users("r1"), "r1", ErrSelfApprovalForbidden},
		{"admin reporter blocked too", DecisionActor{UserID: "r1", Role: "admin"}, Approval{SelfApprovalAllowed: false}, users("u1"), "r1", ErrSelfApprovalForbidden},
		{"requester blocked", DecisionActor{UserID: "u1"}, Approval{SelfApprovalAllowed: false, RequestedBy: "u1"}, users("u1"), "r1", ErrSelfApprovalForbidden},
		{"system requester does not block", DecisionActor{UserID: "u1"}, Approval{SelfApprovalAllowed: false, RequestedBy: "system"}, users("u1"), "r1", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := auth.Decide(tc.actor, tc.ap, tc.approvers, tc.reporter); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEligibleApprovers_DropsReporterOnlyWhenSelfApprovalIsOff(t *testing.T) {
	in := []string{"a", "r1", "b"}
	if got := EligibleApprovers(nil, "r1", true, in); len(got) != 3 {
		t.Errorf("self-approval allowed keeps everyone: %v", got)
	}
	if got := EligibleApprovers(nil, "r1", false, in); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("reporter dropped: %v", got)
	}
}

func TestParsePrincipal_RejectsEmptyIDs(t *testing.T) {
	for _, bad := range []string{"user:", "team: ", "role:", "x:1", "user", ""} {
		if _, err := ParsePrincipal(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
	if p, err := ParsePrincipal("role:admin"); err != nil || p.Kind != PrincipalKindRole || p.ID != "admin" {
		t.Errorf("%+v %v", p, err)
	}
}
