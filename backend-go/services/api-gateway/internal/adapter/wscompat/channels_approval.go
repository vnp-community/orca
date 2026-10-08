package wscompat

import (
	"context"
	"encoding/json"
	"strings"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

type approvalDecisionArgs struct {
	ID              string `json:"id"`
	ExpectedVersion int64  `json:"expectedVersion"`
	ExpectedDigest  string `json:"expectedDigest"`
	Comment         string `json:"comment"`
}

type approvalDecisionResult struct {
	Approval      ApprovalView `json:"approval"`
	RequestStatus string       `json:"requestStatus"`
}

// registerApproval has no approval.request channel: approvals are created by the state machine.
func (c requestChannels) registerApproval(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout, approval: true}
	c.handle(r, "approval.get", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[requestIDArgs](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.appr.GetApproval(ctx, &requestv1.GetApprovalRequest{Id: in.ID})
		if err != nil {
			return nil, err
		}
		return struct {
			Approval ApprovalView `json:"approval"`
		}{ApprovalViewOf(resp.GetApproval())}, nil
	})
	c.handle(r, "approval.list", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			RequestID   string `json:"requestId"`
			SubjectType string `json:"subjectType"`
			Status      string `json:"status"`
			PageSize    int32  `json:"pageSize"`
			PageToken   string `json:"pageToken"`
		}](args)
		if err != nil {
			return nil, err
		}
		subject, status, err := parseApprovalFilters(in.SubjectType, in.Status)
		if err != nil {
			return nil, err
		}
		resp, err := c.appr.ListApprovals(ctx, &requestv1.ListApprovalsRequest{
			RequestId: in.RequestID, SubjectType: subject, Status: status,
			PageSize: clampRequestPageSize(in.PageSize), PageToken: in.PageToken,
		})
		if err != nil {
			return nil, err
		}
		return approvalPage{ApprovalViews(resp.GetApprovals()), resp.GetNextPageToken()}, nil
	})
	// The user is never an argument: request-service takes it from the session metadata.
	c.handle(r, "approval.listPending", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			SubjectType string `json:"subjectType"`
			PageSize    int32  `json:"pageSize"`
			PageToken   string `json:"pageToken"`
		}](args)
		if err != nil {
			return nil, err
		}
		subject, _, err := parseApprovalFilters(in.SubjectType, "")
		if err != nil {
			return nil, err
		}
		resp, err := c.appr.ListPendingForUser(ctx, &requestv1.ListPendingForUserRequest{
			SubjectType: subject, PageSize: clampRequestPageSize(in.PageSize), PageToken: in.PageToken,
		})
		if err != nil {
			return nil, err
		}
		return approvalPage{ApprovalViews(resp.GetApprovals()), resp.GetNextPageToken()}, nil
	})
	c.handle(r, "approval.approve", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeDecision(args)
		if err != nil {
			return nil, err
		}
		resp, err := c.appr.Approve(ctx, &requestv1.ApproveRequest{
			Id: in.ID, Comment: in.Comment, ExpectedDigest: in.ExpectedDigest, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return approvalDecisionResult{ApprovalViewOf(resp.GetApproval()), resp.GetRequestStatus()}, nil
	})
	c.handle(r, "approval.reject", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeDecision(args)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.Comment) == "" {
			return nil, requestInputError{msg: "REQUEST_APPROVAL_COMMENT_REQUIRED: a comment is required to reject"}
		}
		resp, err := c.appr.Reject(ctx, &requestv1.RejectRequest{
			Id: in.ID, Comment: in.Comment, ExpectedDigest: in.ExpectedDigest, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return approvalDecisionResult{ApprovalViewOf(resp.GetApproval()), resp.GetRequestStatus()}, nil
	})
	c.handle(r, "approval.cancel", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ID     string `json:"id"`
			Reason string `json:"reason"`
		}](args)
		if err != nil {
			return nil, err
		}
		resp, err := c.appr.Cancel(ctx, &requestv1.ApprovalServiceCancelRequest{Id: in.ID, Reason: in.Reason})
		if err != nil {
			return nil, err
		}
		return struct {
			Approval ApprovalView `json:"approval"`
		}{ApprovalViewOf(resp.GetApproval())}, nil
	})
}

type approvalPage struct {
	Approvals     []ApprovalView `json:"approvals"`
	NextPageToken string         `json:"nextPageToken"`
}

// decodeDecision enforces the optimistic-lock pair before any RPC: a decision
// without the version and digest the person saw could approve changed content.
func decodeDecision(args []json.RawMessage) (approvalDecisionArgs, error) {
	in, err := decodeRequestArgs[approvalDecisionArgs](args)
	if err != nil {
		return in, err
	}
	if in.ExpectedVersion < 1 || strings.TrimSpace(in.ExpectedDigest) == "" {
		return in, invalidRequestArg("expectedVersion and expectedDigest are required")
	}
	return in, nil
}

func parseApprovalFilters(subjectType, status string) (requestv1.ApprovalSubjectType, requestv1.ApprovalStatus, error) {
	s, err := parseProtoEnum("subjectType", "APPROVAL_SUBJECT_TYPE_", map[string]int32(requestv1.ApprovalSubjectType_value), subjectType)
	if err != nil {
		return 0, 0, err
	}
	st, err := parseProtoEnum("status", "APPROVAL_STATUS_", map[string]int32(requestv1.ApprovalStatus_value), status)
	if err != nil {
		return 0, 0, err
	}
	return requestv1.ApprovalSubjectType(s), requestv1.ApprovalStatus(st), nil
}
