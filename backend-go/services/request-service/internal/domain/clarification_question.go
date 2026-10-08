package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type QuestionKind string

const (
	QuestionKindText         QuestionKind = "text"
	QuestionKindSingleChoice QuestionKind = "single_choice"
	QuestionKindMultiChoice  QuestionKind = "multi_choice"
	QuestionKindFile         QuestionKind = "file"
	QuestionKindBoolean      QuestionKind = "boolean"
)

func AllQuestionKinds() []QuestionKind {
	return []QuestionKind{QuestionKindText, QuestionKindSingleChoice, QuestionKindMultiChoice, QuestionKindFile, QuestionKindBoolean}
}

const (
	MaxPromptRunes     = 1000
	MaxReasonRunes     = 500
	MaxTextAnswerRunes = 4000
	MaxFileAnswerBytes = 64 * 1024
	MaxQuestions       = 20

	AnswerSourceUser            = "user"
	AnswerSourceDefaultAccepted = "default_accepted"
)

type QuestionOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ClarificationQuestion struct {
	ID               string
	Seq              int
	QuestionKey      string
	Kind             QuestionKind
	Prompt           string
	Reason           string
	Options          []QuestionOption
	SuggestedDefault json.RawMessage
	Required         bool
	TargetPath       string
	Answer           json.RawMessage
	AnswerSource     string
	AnsweredBy       string
	AnsweredAt       *time.Time
}

func (q ClarificationQuestion) HasAnswer() bool { return len(q.Answer) > 0 }

func (q ClarificationQuestion) isChoice() bool {
	return q.Kind == QuestionKindSingleChoice || q.Kind == QuestionKindMultiChoice
}

func (q ClarificationQuestion) optionIDs() []string {
	ids := make([]string, 0, len(q.Options))
	for _, o := range q.Options {
		ids = append(ids, o.ID)
	}
	return ids
}

// ValidateQuestion checks what a caller may put in a question before it is stored.
func ValidateQuestion(q ClarificationQuestion) error {
	bad := func(reason string) error { return ErrClarificationInvalidAnswer(q.QuestionKey, reason) }
	if strings.TrimSpace(q.QuestionKey) == "" {
		return ErrClarificationInvalidAnswer("", "question_key is required")
	}
	known := false
	for _, k := range AllQuestionKinds() {
		known = known || k == q.Kind
	}
	if !known {
		return bad("unknown question kind " + string(q.Kind))
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(q.Prompt)); n < 1 || n > MaxPromptRunes {
		return bad(fmt.Sprintf("prompt must be 1..%d characters", MaxPromptRunes))
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(q.Reason)); n < 1 || n > MaxReasonRunes {
		return bad(fmt.Sprintf("reason must be 1..%d characters", MaxReasonRunes))
	}
	if q.isChoice() && len(q.Options) == 0 {
		return bad("a choice question needs options")
	}
	seen := map[string]bool{}
	for _, o := range q.Options {
		if o.ID == "" || o.Label == "" || seen[o.ID] {
			return bad("options need unique ids and labels")
		}
		seen[o.ID] = true
	}
	if len(q.SuggestedDefault) > 0 {
		if err := ValidateAnswer(q, q.SuggestedDefault); err != nil {
			return bad("suggested default is not a valid answer")
		}
	}
	return nil
}

// ValidateAnswer checks value against the question kind. Required-ness is the caller's business.
func ValidateAnswer(q ClarificationQuestion, value json.RawMessage) error {
	bad := func(reason string) error { return ErrClarificationInvalidAnswer(q.QuestionKey, reason) }
	switch q.Kind {
	case QuestionKindText:
		var s string
		if json.Unmarshal(value, &s) != nil {
			return bad("expected a JSON string")
		}
		if utf8.RuneCountInString(s) > MaxTextAnswerRunes {
			return bad(fmt.Sprintf("text is longer than %d characters", MaxTextAnswerRunes))
		}
		if q.Required && strings.TrimSpace(s) == "" {
			return bad("an answer is required")
		}
	case QuestionKindSingleChoice:
		var s string
		if json.Unmarshal(value, &s) != nil || !containsString(q.optionIDs(), s) {
			return bad("value must be one of the options")
		}
	case QuestionKindMultiChoice:
		var list []string
		if json.Unmarshal(value, &list) != nil {
			return bad("expected a JSON list of option ids")
		}
		if q.Required && len(list) == 0 {
			return bad("pick at least one option")
		}
		ids := q.optionIDs()
		for _, v := range list {
			if !containsString(ids, v) {
				return bad("value is not one of the options: " + v)
			}
		}
	case QuestionKindBoolean:
		var b bool
		if json.Unmarshal(value, &b) != nil {
			return bad("expected true or false")
		}
	case QuestionKindFile:
		var f struct {
			Filename string `json:"filename"`
			Mime     string `json:"mime"`
			Size     int64  `json:"size"`
			Text     string `json:"text"`
		}
		dec := json.NewDecoder(bytes.NewReader(value))
		dec.DisallowUnknownFields()
		if dec.Decode(&f) != nil || strings.TrimSpace(f.Filename) == "" {
			return bad("expected {filename, mime, size, text}")
		}
		// No upload store exists, so v1 takes text content only.
		if len(f.Text) > MaxFileAnswerBytes {
			return bad(fmt.Sprintf("file text is larger than %d bytes", MaxFileAnswerBytes))
		}
	default:
		return bad("unknown question kind")
	}
	return nil
}

// ApplyAnswers writes answered questions into the content by target_path. It never guesses:
// an unsupported path is an error, an empty path means the answer is recorded but not applied.
func ApplyAnswers(content RequestContent, qs []ClarificationQuestion) (RequestContent, error) {
	out, err := content.Apply(ContentPatch{})
	if err != nil {
		return RequestContent{}, err
	}
	for _, q := range qs {
		if !q.HasAnswer() || q.TargetPath == "" {
			continue
		}
		switch {
		case q.TargetPath == "title":
			var s string
			if json.Unmarshal(q.Answer, &s) != nil || strings.TrimSpace(s) == "" {
				return RequestContent{}, ErrClarificationInvalidAnswer(q.QuestionKey, "title needs a text answer")
			}
			out.Title = strings.TrimSpace(NormalizeNFC(s))
		case q.TargetPath == "body":
			var s string
			if json.Unmarshal(q.Answer, &s) != nil {
				return RequestContent{}, ErrClarificationInvalidAnswer(q.QuestionKey, "body needs a text answer")
			}
			s = NormalizeNFC(s)
			if strings.TrimSpace(out.Body) == "" {
				out.Body = s
			} else {
				out.Body = strings.TrimRight(out.Body, "\n") + "\n\n" + s
			}
		case q.TargetPath == "acceptance_criteria":
			var s string
			if json.Unmarshal(q.Answer, &s) != nil {
				return RequestContent{}, ErrClarificationInvalidAnswer(q.QuestionKey, "acceptance_criteria needs a text answer")
			}
			for _, line := range splitLines(s) {
				if _, err := out.AcceptanceCriteria.Add(line, ""); err != nil {
					return RequestContent{}, ErrClarificationInvalidAnswer(q.QuestionKey, err.Error())
				}
			}
		case strings.HasPrefix(q.TargetPath, "type_fields."):
			key := strings.TrimPrefix(q.TargetPath, "type_fields.")
			if key == "" || strings.ContainsAny(key, "./") || key == typeFieldACNext {
				return RequestContent{}, ErrClarificationInvalidAnswer(q.QuestionKey, "unsupported target_path "+q.TargetPath)
			}
			v, err := typeFieldValue(out.Type, key, q)
			if err != nil {
				return RequestContent{}, err
			}
			out.TypeFields[key] = v
		default:
			return RequestContent{}, ErrClarificationInvalidAnswer(q.QuestionKey, "unsupported target_path "+q.TargetPath)
		}
	}
	return out, nil
}

func splitLines(s string) []string {
	var out []string
	for _, l := range strings.Split(NormalizeNFC(s), "\n") {
		if l = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(l), "-*")); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// typeFieldValue shapes the answer after the rule for that key so the stored value validates as ready.
func typeFieldValue(t RequestType, key string, q ClarificationQuestion) (any, error) {
	var rule *FieldRule
	for _, r := range RequiredFields(t) {
		if r.Key == key {
			r := r
			rule = &r
		}
	}
	bad := func(reason string) error { return ErrClarificationInvalidAnswer(q.QuestionKey, reason) }
	var generic any
	if err := json.Unmarshal(q.Answer, &generic); err != nil {
		return nil, bad("answer is not JSON")
	}
	if rule == nil {
		return generic, nil // not a required key of this type: stored as given
	}
	switch rule.Kind {
	case FieldList:
		switch v := generic.(type) {
		case string:
			lines := splitLines(v)
			list := make([]any, len(lines))
			for i, l := range lines {
				list[i] = l
			}
			return list, nil
		case []any:
			return v, nil
		}
	case FieldString:
		if s, ok := generic.(string); ok {
			return strings.TrimSpace(NormalizeNFC(s)), nil
		}
	case FieldEnum:
		if s, ok := generic.(string); ok && containsString(rule.Enum, s) {
			return s, nil
		}
	case FieldBool:
		if b, ok := generic.(bool); ok {
			return b, nil
		}
	case FieldNumber:
		switch v := generic.(type) {
		case float64:
			return v, nil
		case string:
			if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return f, nil
			}
		}
	case FieldScalar:
		switch v := generic.(type) {
		case float64:
			return v, nil
		case string:
			return strings.TrimSpace(NormalizeNFC(v)), nil
		}
	}
	return nil, bad(fmt.Sprintf("the answer does not fit %s (%s)", rule.Key, rule.Kind))
}
