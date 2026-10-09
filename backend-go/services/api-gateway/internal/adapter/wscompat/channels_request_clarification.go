package wscompat

import (
	"context"
	"encoding/json"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// ClarificationQuestionView and ClarificationView use camelCase keys and string enums, the shape
// frontend/src/shared/request-artifact-parsers.ts reads; raw proto structs would leak snake_case and numeric enums.
type ClarificationQuestionView struct {
	ID               string `json:"id"`
	Seq              int32  `json:"seq"`
	QuestionKey      string `json:"questionKey"`
	Kind             string `json:"kind"`
	Prompt           string `json:"prompt"`
	Reason           string `json:"reason,omitempty"`
	Options          any    `json:"options,omitempty"`
	SuggestedDefault any    `json:"suggestedDefault,omitempty"`
	Required         bool   `json:"required"`
	TargetPath       string `json:"targetPath,omitempty"`
	// ValueType (string|number|boolean|list) tells the UI which input to render and how to encode valueJson.
	ValueType string `json:"valueType,omitempty"`
	Answer    any    `json:"answer,omitempty"`
}

type ClarificationView struct {
	ID           string                      `json:"id"`
	DisplayID    string                      `json:"displayId"`
	RequestID    string                      `json:"requestId"`
	Source       string                      `json:"source"`
	SourceRef    string                      `json:"sourceRef,omitempty"`
	Status       string                      `json:"status"`
	ResumeStatus string                      `json:"resumeStatus,omitempty"`
	Round        int32                       `json:"round"`
	Questions    []ClarificationQuestionView `json:"questions"`
	DueAt        string                      `json:"dueAt,omitempty"`
	Version      int64                       `json:"version"`
	CreatedAt    string                      `json:"createdAt,omitempty"`
}

// rawJSONOrNil keeps a stored JSON column as a value, and drops it when empty or malformed instead of failing the whole view.
func rawJSONOrNil(s string) any {
	if s == "" {
		return nil
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return nil
	}
	return v
}

func clarificationViewOf(c *requestv1.Clarification) ClarificationView {
	qs := make([]ClarificationQuestionView, 0, len(c.GetQuestions()))
	for _, q := range c.GetQuestions() {
		qs = append(qs, ClarificationQuestionView{
			ID: q.GetId(), Seq: q.GetSeq(), QuestionKey: q.GetQuestionKey(),
			Kind:   enumLower(q.GetKind().String(), "QUESTION_KIND_"),
			Prompt: q.GetPrompt(), Reason: q.GetReason(),
			Options: rawJSONOrNil(q.GetOptionsJson()), SuggestedDefault: rawJSONOrNil(q.GetSuggestedDefaultJson()),
			Required: q.GetRequired(), TargetPath: q.GetTargetPath(), ValueType: q.GetValueType(), Answer: rawJSONOrNil(q.GetAnswerJson()),
		})
	}
	return ClarificationView{
		ID: c.GetId(), DisplayID: c.GetDisplayId(), RequestID: c.GetRequestId(),
		Source: c.GetSource(), SourceRef: c.GetSourceRef(),
		Status:       enumLower(c.GetStatus().String(), "CLARIFICATION_STATUS_"),
		ResumeStatus: c.GetResumeStatus(), Round: c.GetRound(), Questions: qs,
		DueAt: rfc3339(c.GetDueAt()), Version: c.GetVersion(), CreatedAt: rfc3339(c.GetCreatedAt()),
	}
}

func clarificationStatusFilter(s string) (requestv1.ClarificationStatus, error) {
	switch s {
	case "":
		return requestv1.ClarificationStatus_CLARIFICATION_STATUS_UNSPECIFIED, nil
	case "open":
		return requestv1.ClarificationStatus_CLARIFICATION_STATUS_OPEN, nil
	case "answered":
		return requestv1.ClarificationStatus_CLARIFICATION_STATUS_ANSWERED, nil
	case "expired":
		return requestv1.ClarificationStatus_CLARIFICATION_STATUS_EXPIRED, nil
	case "cancelled":
		return requestv1.ClarificationStatus_CLARIFICATION_STATUS_CANCELLED, nil
	}
	return 0, invalidRequestArg("status must be open, answered, expired or cancelled")
}

// registerClarification exposes the Clarification RPCs of CR-REQ-028. request-service already parks a Request in
// awaiting_information after type confirmation; without these channels no client can answer and the Request stays blocked.
func (c requestChannels) registerClarification(r *Registry) {
	opts := requestChannelOpts{timeout: requestRPCTimeout}
	c.handle(r, "clarification.list", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			RequestID string `json:"requestId"`
			Status    string `json:"status"`
			PageSize  int32  `json:"pageSize"`
			PageToken string `json:"pageToken"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.RequestID == "" {
			return nil, invalidRequestArg("requestId is required")
		}
		st, err := clarificationStatusFilter(in.Status)
		if err != nil {
			return nil, err
		}
		resp, err := c.req.ListClarifications(ctx, &requestv1.ListClarificationsRequest{
			RequestId: in.RequestID, Status: st, PageSize: clampRequestPageSize(in.PageSize), PageToken: in.PageToken,
		})
		if err != nil {
			return nil, err
		}
		views := make([]ClarificationView, 0, len(resp.GetClarifications()))
		for _, cl := range resp.GetClarifications() {
			views = append(views, clarificationViewOf(cl))
		}
		return struct {
			Clarifications []ClarificationView `json:"clarifications"`
			NextPageToken  string              `json:"nextPageToken"`
		}{views, resp.GetNextPageToken()}, nil
	})
	c.handle(r, "clarification.get", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ClarificationID string `json:"clarificationId"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.ClarificationID == "" {
			return nil, invalidRequestArg("clarificationId is required")
		}
		resp, err := c.req.GetClarification(ctx, &requestv1.GetClarificationRequest{Id: in.ClarificationID})
		if err != nil {
			return nil, err
		}
		return struct {
			Clarification ClarificationView `json:"clarification"`
		}{clarificationViewOf(resp.GetClarification())}, nil
	})
	c.handle(r, "clarification.answer", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ClarificationID string `json:"clarificationId"`
			Answers         []struct {
				QuestionID    string `json:"questionId"`
				ValueJSON     string `json:"valueJson"`
				AcceptDefault bool   `json:"acceptDefault"`
			} `json:"answers"`
			Complete        bool  `json:"complete"`
			ExpectedVersion int64 `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.ClarificationID == "" {
			return nil, invalidRequestArg("clarificationId is required")
		}
		items := make([]*requestv1.AnswerItem, 0, len(in.Answers))
		for _, a := range in.Answers {
			items = append(items, &requestv1.AnswerItem{QuestionId: a.QuestionID, ValueJson: a.ValueJSON, AcceptDefault: a.AcceptDefault})
		}
		resp, err := c.req.AnswerClarification(ctx, &requestv1.AnswerClarificationRequest{
			ClarificationId: in.ClarificationID, Answers: items, Complete: in.Complete, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Clarification   ClarificationView `json:"clarification"`
			RequestStatus   string            `json:"requestStatus"`
			RequestRevision int32             `json:"requestRevision"`
			StillMissing    bool              `json:"stillMissing"`
		}{clarificationViewOf(resp.GetClarification()), resp.GetRequestStatus(), resp.GetRequestRevision(), resp.GetStillMissing()}, nil
	})
	c.handle(r, "clarification.cancel", opts, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			ClarificationID string `json:"clarificationId"`
			Reason          string `json:"reason"`
			ExpectedVersion int64  `json:"expectedVersion"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.ClarificationID == "" {
			return nil, invalidRequestArg("clarificationId is required")
		}
		resp, err := c.req.CancelClarification(ctx, &requestv1.CancelClarificationRequest{
			Id: in.ClarificationID, Reason: in.Reason, ExpectedVersion: in.ExpectedVersion,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Clarification ClarificationView `json:"clarification"`
		}{clarificationViewOf(resp.GetClarification())}, nil
	})
}
