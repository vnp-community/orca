package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

var (
	ErrNoUser                = apperrors.New(apperrors.KindUnauthenticated, "REQUEST_APPROVAL_NO_USER", "a signed-in user is required", nil)
	ErrAgentForbidden        = apperrors.New(apperrors.KindPermissionDenied, "REQUEST_APPROVAL_AGENT_FORBIDDEN", "automated callers cannot decide approvals", nil)
	ErrNotApprover           = apperrors.New(apperrors.KindPermissionDenied, "REQUEST_APPROVAL_NOT_APPROVER", "caller is not an approver", nil)
	ErrSelfApprovalForbidden = apperrors.New(apperrors.KindPermissionDenied, "REQUEST_APPROVAL_SELF_APPROVAL_FORBIDDEN", "the requester cannot approve their own request", nil)
)

type PrincipalKind string

const (
	PrincipalKindUser     PrincipalKind = "user"
	PrincipalKindTeam     PrincipalKind = "team"
	PrincipalKindRole     PrincipalKind = "role"
	PrincipalKindReporter PrincipalKind = "reporter"
)

type Principal struct {
	Kind PrincipalKind
	ID   string
}

func ParsePrincipal(s string) (Principal, error) {
	if s == "reporter" {
		return Principal{Kind: PrincipalKindReporter, ID: ""}, nil
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return Principal{}, errors.New("invalid principal format")
	}
	switch parts[0] {
	case "user":
		return Principal{Kind: PrincipalKindUser, ID: parts[1]}, nil
	case "team":
		return Principal{Kind: PrincipalKindTeam, ID: parts[1]}, nil
	case "role":
		return Principal{Kind: PrincipalKindRole, ID: parts[1]}, nil
	default:
		return Principal{}, errors.New("invalid principal kind")
	}
}

func (p Principal) String() string {
	if p.Kind == PrincipalKindReporter {
		return "reporter"
	}
	return string(p.Kind) + ":" + p.ID
}

type ApprovalPolicy struct {
	ID                    string
	TenantID              string
	ProjectID             *string
	SubjectType           SubjectType
	RequestType           *string
	Size                  *string
	Urgency               *string
	Approvers             []Principal
	AllowRequesterApprove bool
	DueAfter              *time.Duration
	Priority              int
	Enabled               bool
	Version               int64
	CreatedBy             string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (p ApprovalPolicy) Specificity() int {
	score := 0
	if p.ProjectID != nil {
		score++
	}
	if p.RequestType != nil {
		score++
	}
	if p.Size != nil {
		score++
	}
	if p.Urgency != nil {
		score++
	}
	return score
}

type PolicyContext struct {
	ProjectID   string
	RequestType string
	Size        string
	Urgency     string
}

func SelectPolicy(candidates []ApprovalPolicy, ctx PolicyContext) (ApprovalPolicy, bool) {
	var best *ApprovalPolicy
	for i, c := range candidates {
		if !c.Enabled {
			continue
		}
		if c.ProjectID != nil && *c.ProjectID != ctx.ProjectID {
			continue
		}
		if c.RequestType != nil && *c.RequestType != ctx.RequestType {
			continue
		}
		if c.Size != nil && *c.Size != ctx.Size {
			continue
		}
		if c.Urgency != nil && *c.Urgency != ctx.Urgency {
			continue
		}

		if best == nil {
			best = &candidates[i]
			continue
		}
		if c.Specificity() > best.Specificity() {
			best = &candidates[i]
		} else if c.Specificity() == best.Specificity() {
			if c.Priority > best.Priority {
				best = &candidates[i]
			} else if c.Priority == best.Priority {
				if c.CreatedAt.Before(best.CreatedAt) {
					best = &candidates[i]
				}
			}
		}
	}
	if best == nil {
		return ApprovalPolicy{}, false
	}
	return *best, true
}

func DefaultPolicy(st SubjectType, ctx PolicyContext) ApprovalPolicy {
	due2h := 2 * time.Hour
	due4h := 4 * time.Hour
	due8h := 8 * time.Hour
	due24h := 24 * time.Hour
	due72h := 72 * time.Hour
	due7d := 7 * 24 * time.Hour

	p := ApprovalPolicy{
		SubjectType: st,
		Enabled:     true,
	}

	switch st {
	case SubjectPreDeploy:
		p.Approvers = []Principal{{Kind: PrincipalKindRole, ID: "admin"}}
		p.AllowRequesterApprove = false
	default:
		p.Approvers = []Principal{
			{Kind: PrincipalKindReporter, ID: ""},
			{Kind: PrincipalKindRole, ID: "admin"},
		}
		p.AllowRequesterApprove = true
	}

	if st == SubjectRequestType {
		if ctx.RequestType == "hotfix" || ctx.RequestType == "security" {
			p.AllowRequesterApprove = false
		}
	} else if st == SubjectSolution || st == SubjectPlan || st == SubjectTaskList {
		if ctx.Size == "L" {
			p.AllowRequesterApprove = false
		}
	}

	switch ctx.Urgency {
	case "urgent":
		switch st {
		case SubjectAnswer, SubjectPreDeploy:
			p.DueAfter = &due2h
		case SubjectFindings:
			p.DueAfter = &due8h
		case SubjectPlan, SubjectPhase, SubjectTaskList:
			p.DueAfter = &due24h
		default:
			p.DueAfter = &due4h
		}
	default: // normal
		switch st {
		case SubjectAnswer, SubjectPreDeploy:
			p.DueAfter = &due24h
		case SubjectFindings:
			p.DueAfter = &due72h
		case SubjectPlan, SubjectPhase, SubjectTaskList:
			p.DueAfter = &due7d
		default:
			p.DueAfter = &due72h
		}
	}

	return p
}
