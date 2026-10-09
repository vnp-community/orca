package wscompat

import (
	"encoding/json"
	"testing"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// The frontend parser reads camelCase keys and string enums; this pins the view so a proto change cannot silently break the panel.
func TestClarificationViewOf_UsesFrontendShape(t *testing.T) {
	c := &requestv1.Clarification{
		Id: "c1", DisplayId: "CL-1", RequestId: "r1", Source: "readiness",
		Status: requestv1.ClarificationStatus_CLARIFICATION_STATUS_OPEN, ResumeStatus: "analyzing", Round: 1, Version: 2,
		Questions: []*requestv1.ClarificationQuestion{{
			Id: "q1", Seq: 1, QuestionKey: "acceptance_criteria", Kind: requestv1.QuestionKind_QUESTION_KIND_SINGLE_CHOICE,
			Prompt: "p", Required: true, OptionsJson: `[{"value":"a","label":"A"}]`, SuggestedDefaultJson: `"x"`, AnswerJson: "not json", ValueType: "number",
		}},
	}
	raw, err := json.Marshal(clarificationViewOf(c))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["status"] != "open" || got["displayId"] != "CL-1" || got["requestId"] != "r1" || got["resumeStatus"] != "analyzing" {
		t.Fatalf("unexpected top-level keys: %s", raw)
	}
	q := got["questions"].([]any)[0].(map[string]any)
	if q["kind"] != "single_choice" || q["questionKey"] != "acceptance_criteria" || q["required"] != true {
		t.Fatalf("unexpected question keys: %s", raw)
	}
	if q["valueType"] != "number" {
		t.Fatalf("valueType must reach the client, got %v", q["valueType"])
	}
	if _, ok := q["options"].([]any); !ok {
		t.Fatalf("options should be a JSON array, got %v", q["options"])
	}
	if _, present := q["answer"]; present {
		t.Fatalf("malformed answer_json must be dropped, got %v", q["answer"])
	}
}

func TestClarificationStatusFilter(t *testing.T) {
	for in, want := range map[string]requestv1.ClarificationStatus{
		"": requestv1.ClarificationStatus_CLARIFICATION_STATUS_UNSPECIFIED, "open": requestv1.ClarificationStatus_CLARIFICATION_STATUS_OPEN,
		"cancelled": requestv1.ClarificationStatus_CLARIFICATION_STATUS_CANCELLED,
	} {
		if got, err := clarificationStatusFilter(in); err != nil || got != want {
			t.Errorf("%q: got %v, %v", in, got, err)
		}
	}
	if _, err := clarificationStatusFilter("bogus"); err == nil {
		t.Error("an unknown status must be rejected")
	}
}
