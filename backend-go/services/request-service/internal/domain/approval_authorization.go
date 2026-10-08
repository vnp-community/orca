package domain

type DecisionActor struct {
	UserID    string
	Role      string
	TeamIDs   []string
	IsMachine bool
}

type ApprovalAuthorization struct{}

func (a ApprovalAuthorization) Decide(actor DecisionActor, ap Approval, approvers []Principal, reporterID string) error {
	if actor.UserID == "" && !actor.IsMachine {
		return ErrNoUser
	}
	if actor.IsMachine {
		return ErrAgentForbidden
	}

	isApprover := false
	if actor.Role == "admin" {
		isApprover = true
	} else {
		for _, p := range approvers {
			switch p.Kind {
			case PrincipalKindUser:
				if p.ID == actor.UserID {
					isApprover = true
				}
			case PrincipalKindTeam:
				for _, tid := range actor.TeamIDs {
					if tid == p.ID {
						isApprover = true
					}
				}
			case PrincipalKindRole:
				if p.ID == actor.Role {
					isApprover = true
				}
			case PrincipalKindReporter:
				if actor.UserID == reporterID {
					isApprover = true
				}
			}
			if isApprover {
				break
			}
		}
	}

	if !isApprover {
		return ErrNotApprover
	}

	if !ap.SelfApprovalAllowed {
		// Separation of duties
		isRequester := false
		if actor.UserID == reporterID {
			isRequester = true
		}
		if ap.RequestedBy == actor.UserID && ap.RequestedBy != "system" {
			isRequester = true
		}
		if isRequester {
			return ErrSelfApprovalForbidden
		}
	}

	return nil
}

func EligibleApprovers(approvers []Principal, reporterID string, selfAllowed bool, expandedUsers []string) []string {
	if selfAllowed {
		return expandedUsers
	}

	var res []string
	for _, u := range expandedUsers {
		if u != reporterID {
			res = append(res, u)
		}
	}
	return res
}
