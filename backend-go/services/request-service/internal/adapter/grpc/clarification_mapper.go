package grpc

import (
	"encoding/json"
	"fmt"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var clarificationStatusToProto = map[domain.ClarificationStatus]requestv1.ClarificationStatus{
	domain.ClarificationStatusOpen:      requestv1.ClarificationStatus_CLARIFICATION_STATUS_OPEN,
	domain.ClarificationStatusAnswered:  requestv1.ClarificationStatus_CLARIFICATION_STATUS_ANSWERED,
	domain.ClarificationStatusExpired:   requestv1.ClarificationStatus_CLARIFICATION_STATUS_EXPIRED,
	domain.ClarificationStatusCancelled: requestv1.ClarificationStatus_CLARIFICATION_STATUS_CANCELLED,
}

var questionKindToProto = map[domain.QuestionKind]requestv1.QuestionKind{
	domain.QuestionKindText:         requestv1.QuestionKind_QUESTION_KIND_TEXT,
	domain.QuestionKindSingleChoice: requestv1.QuestionKind_QUESTION_KIND_SINGLE_CHOICE,
	domain.QuestionKindMultiChoice:  requestv1.QuestionKind_QUESTION_KIND_MULTI_CHOICE,
	domain.QuestionKindFile:         requestv1.QuestionKind_QUESTION_KIND_FILE,
	domain.QuestionKindBoolean:      requestv1.QuestionKind_QUESTION_KIND_BOOLEAN,
}

func clarificationStatusFromProto(s requestv1.ClarificationStatus) (domain.ClarificationStatus, error) {
	if s == requestv1.ClarificationStatus_CLARIFICATION_STATUS_UNSPECIFIED {
		return "", nil
	}
	for d, p := range clarificationStatusToProto {
		if p == s {
			return d, nil
		}
	}
	return "", domain.ErrClarificationStateNotAllowed(fmt.Sprintf("unknown clarification status %d", s))
}

func questionKindFromProto(k requestv1.QuestionKind) (domain.QuestionKind, error) {
	for d, p := range questionKindToProto {
		if p == k {
			return d, nil
		}
	}
	return "", domain.ErrClarificationInvalidAnswer("", fmt.Sprintf("unknown question kind %d", k))
}

func toProtoClarification(v usecase.ClarificationView) *requestv1.Clarification {
	c := v.Clarification
	out := &requestv1.Clarification{
		Id: c.ID, DisplayId: c.DisplayID(v.RequestNumber), RequestId: c.RequestID, Source: string(c.Source), SourceRef: c.SourceRef,
		Status: clarificationStatusToProto[c.Status], ResumeStatus: string(c.ResumeStatus), Round: int32(c.Round),
		DueAt: timestamppb.New(c.DueAt), Version: c.Version, CreatedAt: timestamppb.New(c.CreatedAt),
	}
	for _, q := range c.Questions {
		pq := &requestv1.ClarificationQuestion{
			Id: q.ID, Seq: int32(q.Seq), QuestionKey: q.QuestionKey, Kind: questionKindToProto[q.Kind], Prompt: q.Prompt, Reason: q.Reason,
			SuggestedDefaultJson: string(q.SuggestedDefault), Required: q.Required, TargetPath: q.TargetPath,
		}
		if len(q.Options) > 0 {
			b, _ := json.Marshal(q.Options)
			pq.OptionsJson = string(b)
		}
		// Answers can hold sensitive detail: only assignees, the asker and admins get them back.
		if v.AnswersVisible {
			pq.AnswerJson = string(q.Answer)
		}
		out.Questions = append(out.Questions, pq)
	}
	return out
}

func questionInputsFromProto(qs []*requestv1.ClarificationQuestion) ([]usecase.QuestionInput, error) {
	out := make([]usecase.QuestionInput, 0, len(qs))
	for _, q := range qs {
		kind, err := questionKindFromProto(q.GetKind())
		if err != nil {
			return nil, err
		}
		in := usecase.QuestionInput{
			QuestionKey: q.GetQuestionKey(), Kind: kind, Prompt: q.GetPrompt(), Reason: q.GetReason(), Required: q.GetRequired(), TargetPath: q.GetTargetPath(),
		}
		if s := q.GetOptionsJson(); s != "" {
			if err := json.Unmarshal([]byte(s), &in.Options); err != nil {
				return nil, domain.ErrClarificationInvalidAnswer(q.GetQuestionKey(), "options_json is not a list of {id,label}")
			}
		}
		if s := q.GetSuggestedDefaultJson(); s != "" {
			if !json.Valid([]byte(s)) {
				return nil, domain.ErrClarificationInvalidAnswer(q.GetQuestionKey(), "suggested_default_json is not JSON")
			}
			in.SuggestedDefault = json.RawMessage(s)
		}
		out = append(out, in)
	}
	return out, nil
}

func answerItemsFromProto(items []*requestv1.AnswerItem) ([]usecase.AnswerItem, error) {
	out := make([]usecase.AnswerItem, 0, len(items))
	for _, it := range items {
		a := usecase.AnswerItem{QuestionID: it.GetQuestionId(), AcceptDefault: it.GetAcceptDefault()}
		if !a.AcceptDefault {
			if !json.Valid([]byte(it.GetValueJson())) {
				return nil, domain.ErrClarificationInvalidAnswer(it.GetQuestionId(), "value_json is not JSON")
			}
			a.Value = json.RawMessage(it.GetValueJson())
		}
		out = append(out, a)
	}
	return out, nil
}

var decisionStatusToProto = map[domain.DecisionStatus]requestv1.DecisionStatus{
	domain.DecisionStatusOpen:       requestv1.DecisionStatus_DECISION_STATUS_OPEN,
	domain.DecisionStatusChosen:     requestv1.DecisionStatus_DECISION_STATUS_CHOSEN,
	domain.DecisionStatusEffective:  requestv1.DecisionStatus_DECISION_STATUS_EFFECTIVE,
	domain.DecisionStatusSuperseded: requestv1.DecisionStatus_DECISION_STATUS_SUPERSEDED,
}

func decisionStatusFromProto(s requestv1.DecisionStatus) (domain.DecisionStatus, error) {
	if s == requestv1.DecisionStatus_DECISION_STATUS_UNSPECIFIED {
		return "", nil
	}
	for d, p := range decisionStatusToProto {
		if p == s {
			return d, nil
		}
	}
	return "", domain.ErrDecisionStateInvalid("", domain.DecisionStatus(fmt.Sprint(int32(s))))
}

func toProtoDecision(d domain.Decision, requestNumber int64) *requestv1.Decision {
	opts, _ := json.Marshal(d.Options)
	out := &requestv1.Decision{
		Id: d.ID, DisplayId: d.DisplayID(requestNumber), RequestId: d.RequestID, SubjectKind: string(d.SubjectKind), SubjectId: d.SubjectID,
		SubjectDigest: d.SubjectDigest, Question: d.Question, OptionsJson: string(opts), RecommendedOptionId: d.RecommendedOptionID,
		RecommendationReason: d.RecommendationReason, ChosenOptionId: d.ChosenOptionID, ChooserId: d.ChooserID, Rationale: d.Rationale,
		RiskLevel: string(d.RiskLevel), ConfirmedBy: d.ConfirmedBy, Status: decisionStatusToProto[d.Status], Version: d.Version,
		CreatedAt: timestamppb.New(d.CreatedAt),
	}
	if d.ChosenAt != nil {
		out.ChosenAt = timestamppb.New(*d.ChosenAt)
	}
	if d.ConfirmedAt != nil {
		out.ConfirmedAt = timestamppb.New(*d.ConfirmedAt)
	}
	return out
}
