package usecase

import (
	"context"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
)

// RequestVisibility says which Requests a caller may see. ListTasks and ListExecutionStates only check the tenant,
// so the backlog views filter Requests here before they ask task-service anything.
type RequestVisibility interface {
	Filter(ctx context.Context, actor domain.DecisionActor, requests []domain.Request) ([]domain.Request, error)
}

// ProjectMembership answers "is this user a member of the project". An unknown project or a denied lookup is false;
// only an outage is an error.
type ProjectMembership interface {
	IsMember(ctx context.Context, projectID, userID string) (bool, error)
}

// MemberRequestVisibility is the interim rule of CR-REQ-035 2.3: an admin sees everything, the reporter sees their own
// Requests, a project member sees the Requests of that project. Without a user nothing is visible.
type MemberRequestVisibility struct {
	Members ProjectMembership
}

var _ RequestVisibility = (*MemberRequestVisibility)(nil)

func (v *MemberRequestVisibility) Filter(ctx context.Context, actor domain.DecisionActor, requests []domain.Request) ([]domain.Request, error) {
	if actor.UserID == "" {
		return nil, nil
	}
	if actor.Role == "admin" {
		return requests, nil
	}
	memberOf := map[string]bool{} // one membership lookup per project, not per Request
	var out []domain.Request
	for _, r := range requests {
		if r.ReporterID == actor.UserID {
			out = append(out, r)
			continue
		}
		if r.ProjectID == "" || v.Members == nil {
			continue
		}
		member, seen := memberOf[r.ProjectID]
		if !seen {
			var err error
			if member, err = v.Members.IsMember(ctx, r.ProjectID, actor.UserID); err != nil {
				return nil, err
			}
			memberOf[r.ProjectID] = member
		}
		if member {
			out = append(out, r)
		}
	}
	return out, nil
}
