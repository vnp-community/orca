package domain

import "testing"

func TestApprovalAuthorization_Decide(t *testing.T) {
	auth := ApprovalAuthorization{}

	// Stub test
	err := auth.Decide(DecisionActor{UserID: "u1"}, Approval{SelfApprovalAllowed: true}, []Principal{{Kind: PrincipalKindUser, ID: "u1"}}, "r1")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	err = auth.Decide(DecisionActor{UserID: "r1"}, Approval{SelfApprovalAllowed: false}, []Principal{{Kind: PrincipalKindUser, ID: "r1"}}, "r1")
	if err != ErrSelfApprovalForbidden {
		t.Errorf("expected ErrSelfApprovalForbidden, got %v", err)
	}
}
