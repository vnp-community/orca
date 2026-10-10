package wscompat

import (
	"context"
	"encoding/json"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// askQuestionArg is one question of clarification.request; options and suggestedDefault are JSON values, as the
// views return them, and travel to the service as JSON text.
type askQuestionArg struct {
	QuestionKey      string          `json:"questionKey"`
	Kind             string          `json:"kind"`
	Prompt           string          `json:"prompt"`
	Reason           string          `json:"reason"`
	Options          json.RawMessage `json:"options"`
	SuggestedDefault json.RawMessage `json:"suggestedDefault"`
	Required         bool            `json:"required"`
	TargetPath       string          `json:"targetPath"`
}

func questionKindArg(s string) (requestv1.QuestionKind, error) {
	switch s {
	case "text":
		return requestv1.QuestionKind_QUESTION_KIND_TEXT, nil
	case "single_choice":
		return requestv1.QuestionKind_QUESTION_KIND_SINGLE_CHOICE, nil
	case "multi_choice":
		return requestv1.QuestionKind_QUESTION_KIND_MULTI_CHOICE, nil
	case "file":
		return requestv1.QuestionKind_QUESTION_KIND_FILE, nil
	case "boolean":
		return requestv1.QuestionKind_QUESTION_KIND_BOOLEAN, nil
	}
	return 0, invalidRequestArg("kind must be text, single_choice, multi_choice, file or boolean")
}

func (a askQuestionArg) proto() (*requestv1.ClarificationQuestion, error) {
	kind, err := questionKindArg(a.Kind)
	if err != nil {
		return nil, err
	}
	q := &requestv1.ClarificationQuestion{
		QuestionKey: a.QuestionKey, Kind: kind, Prompt: a.Prompt, Reason: a.Reason, Required: a.Required, TargetPath: a.TargetPath,
	}
	if len(a.Options) > 0 && string(a.Options) != "null" {
		q.OptionsJson = string(a.Options)
	}
	if len(a.SuggestedDefault) > 0 && string(a.SuggestedDefault) != "null" {
		q.SuggestedDefaultJson = string(a.SuggestedDefault)
	}
	return q, nil
}

// registerClarificationAsk exposes the "ask again" of CR-REQ-028 section 2.5: a reviewer turns the open questions of a
// proposal into a Clarification. Without it a blocking open question of a Solution can never be answered and the
// approval stays refused with REQUEST_SOLUTION_BLOCKING_QUESTIONS. The service decides who may ask and from which
// states, and refuses the platform-only sources (readiness, task_blocked).
func (c requestChannels) registerClarificationAsk(r *Registry) {
	c.handle(r, "clarification.request", requestChannelOpts{timeout: requestRPCTimeout}, func(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
		in, err := decodeRequestArgs[struct {
			RequestID string           `json:"requestId"`
			Source    string           `json:"source"`
			SourceRef string           `json:"sourceRef"`
			Questions []askQuestionArg `json:"questions"`
		}](args)
		if err != nil {
			return nil, err
		}
		if in.RequestID == "" || in.Source == "" || len(in.Questions) == 0 {
			return nil, invalidRequestArg("requestId, source and at least one question are required")
		}
		qs := make([]*requestv1.ClarificationQuestion, 0, len(in.Questions))
		for _, a := range in.Questions {
			q, err := a.proto()
			if err != nil {
				return nil, err
			}
			qs = append(qs, q)
		}
		resp, err := c.req.RequestClarification(ctx, &requestv1.RequestClarificationRequest{
			RequestId: in.RequestID, Source: in.Source, SourceRef: in.SourceRef, Questions: qs,
		})
		if err != nil {
			return nil, err
		}
		return struct {
			Clarification ClarificationView `json:"clarification"`
		}{clarificationViewOf(resp.GetClarification())}, nil
	})
}
