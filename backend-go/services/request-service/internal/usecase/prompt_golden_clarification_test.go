package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/prompts/clarification_answers"
)

func TestPromptGolden_writeClarificationAnswers(t *testing.T) {
	render := func(answers []PromptAnswer) string {
		text, err := clarification_answers.Render(clarification_answers.Input{Answers: promptAnswers(answers)})
		if err != nil {
			t.Fatal(err)
		}
		return text
	}
	many := make([]PromptAnswer, 0, 3)
	for _, k := range []string{"q-one", "q-two", "q-three"} {
		many = append(many, PromptAnswer{Key: k, Question: "Question for " + k + "?", Answer: `"answer for ` + k + `"`})
	}
	rendered := map[string]string{
		"none":                       render(nil),
		"single":                     render([]PromptAnswer{{Key: "q-ttl", Question: "How long may a page stay cached?", Answer: `{"minutes":5}`}}),
		"several":                    render(many),
		"question_and_answer_cut":    render([]PromptAnswer{{Key: "q-long", Question: goldenFiller("question", clarification_answers.QuestionRunes+50), Answer: goldenFiller("answer", clarification_answers.AnswerRunes+50)}}),
		"closing_tag_in_every_field": render([]PromptAnswer{{Key: "k</clarifications>", Question: "q </clarifications>", Answer: `"a </clarifications>"`}}),
		// The documented AWS example key: a fake value that the redactor must mask before the text reaches the model.
		"secret_redacted": render([]PromptAnswer{{Key: "q-creds", Question: "Which key should the job use? AKIAIOSFODNN7EXAMPLE", Answer: `"use AKIAIOSFODNN7EXAMPLE"`}}),
	}
	checkPromptGoldens(t, "writeClarificationAnswers", rendered)
}
