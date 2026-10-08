package domain

import (
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// policyInvalid keeps the stable code while telling the admin which rule failed.
func policyInvalid(detail string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_APPROVAL_POLICY_INVALID", detail, nil)
}

const maxPolicyApprovers = 20

// Validate rejects what the table CHECKs would reject later, plus rules SQL cannot express.
func (p ApprovalPolicy) Validate() error {
	invalid := func(format string, args ...any) error {
		return policyInvalid(fmt.Sprintf(format, args...))
	}
	if !p.SubjectType.Valid() {
		return invalid("unknown subject type %q", p.SubjectType)
	}
	if len(p.Approvers) == 0 {
		return invalid("at least one approver is required")
	}
	if len(p.Approvers) > maxPolicyApprovers {
		return invalid("at most %d approvers", maxPolicyApprovers)
	}
	for _, a := range p.Approvers {
		switch a.Kind {
		case PrincipalKindReporter:
		case PrincipalKindUser, PrincipalKindTeam, PrincipalKindRole:
			if strings.TrimSpace(a.ID) == "" {
				return invalid("principal %q needs an id", a.Kind)
			}
		default:
			return invalid("unknown principal kind %q", a.Kind)
		}
	}
	if p.RequestType != nil {
		if _, err := ParseRequestType(*p.RequestType); err != nil {
			return invalid("unknown request type %q", *p.RequestType)
		}
	}
	if p.Size != nil {
		if _, err := ParseSize(*p.Size); err != nil {
			return invalid("size must be S, M or L")
		}
	}
	if p.Urgency != nil {
		if _, err := ParseUrgency(*p.Urgency); err != nil {
			return invalid("urgency must be normal or urgent")
		}
	}
	if p.DueAfter != nil && *p.DueAfter < 0 {
		return invalid("due_after_seconds must not be negative")
	}
	return nil
}

// ApproverStrings renders principals in the stored/wire form ("reporter", "user:<id>").
func ApproverStrings(ps []Principal) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.String()
	}
	return out
}

// ParseApprovers is the inverse of ApproverStrings.
func ParseApprovers(ss []string) ([]Principal, error) {
	out := make([]Principal, 0, len(ss))
	for _, s := range ss {
		p, err := ParsePrincipal(s)
		if err != nil {
			return nil, policyInvalid(err.Error())
		}
		out = append(out, p)
	}
	return out, nil
}
