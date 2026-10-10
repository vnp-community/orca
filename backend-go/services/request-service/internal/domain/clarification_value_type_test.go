package domain

import "testing"

func TestClarificationValueType(t *testing.T) {
	cases := []struct {
		name   string
		kind   QuestionKind
		target string
		want   string
	}{
		{"numeric field typed as text", QuestionKindText, "type_fields.time_box_hours", ValueTypeNumber},
		{"boolean field", QuestionKindBoolean, "type_fields.data_exposed", ValueTypeBoolean},
		{"list field", QuestionKindText, "type_fields.repro_steps", ValueTypeList},
		{"enum is a string", QuestionKindSingleChoice, "type_fields.severity", ValueTypeString},
		{"scalar metric is a string", QuestionKindText, "type_fields.current_value", ValueTypeString},
		{"acceptance criteria", QuestionKindText, "acceptance_criteria", ValueTypeList},
		{"multi choice is a list", QuestionKindMultiChoice, "", ValueTypeList},
		{"title", QuestionKindText, "title", ValueTypeString},
		{"unknown key defaults to string", QuestionKindText, "type_fields.made_up", ValueTypeString},
		{"no target", QuestionKindText, "", ValueTypeString},
	}
	for _, c := range cases {
		if got := ClarificationValueType(c.kind, c.target); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

// Every required key must resolve to the type its rule validates, so a hint can never promise a type the
// server then rejects.
func TestClarificationValueTypeMatchesEveryRequiredField(t *testing.T) {
	for rt, rules := range requiredFieldsByType {
		for _, r := range rules {
			got := ClarificationValueType(QuestionKindText, "type_fields."+r.Key)
			if want := valueTypeOfField(r.Kind); got != want && got != ValueTypeString {
				t.Errorf("%s.%s: hint %q, rule kind %s wants %q", rt, r.Key, got, r.Kind, want)
			}
		}
	}
}

// Clients read the unanswered ids from this text (gateway relays it as is), so its shape is part of the contract.
func TestBlockingErrorsListTheirIdsAfterTheColon(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{ErrSolutionBlockingQuestions([]string{"Q-1", "Q-2"}), "REQUEST_SOLUTION_BLOCKING_QUESTIONS: blocking open questions are unanswered: Q-1, Q-2"},
		{ErrPlanUnconfirmedAssumptions([]string{"A-1"}), "REQUEST_PLAN_UNCONFIRMED_ASSUMPTIONS: assumptions needing confirmation are unanswered: A-1"},
	}
	for _, c := range cases {
		if got := c.err.Error(); got != c.want {
			t.Errorf("got %q want %q", got, c.want)
		}
	}
}
