package domain

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type ValidationLevel string

const (
	// ValidationLevelDraft checks types and sizes only: Jira and GitHub requests arrive without AC.
	ValidationLevelDraft ValidationLevel = "draft"
	// ValidationLevelReady is the Definition of Ready (CR-REQ-028 calls it).
	ValidationLevelReady ValidationLevel = "ready"
)

const (
	CodeContentFieldRequired = "REQUEST_CONTENT_FIELD_REQUIRED"
	CodeContentFieldInvalid  = "REQUEST_CONTENT_FIELD_INVALID"
	CodeContentBodyTooShort  = "REQUEST_CONTENT_BODY_TOO_SHORT"
	CodeContentACRequired    = "REQUEST_CONTENT_AC_REQUIRED"
	CodeContentTypeRequired  = "REQUEST_CONTENT_TYPE_REQUIRED"
	minReadyBodyChars        = 20
	maxTitleRunes            = 500
	maxBodyBytes             = 100 * 1024
)

type FieldKind string

const (
	FieldString FieldKind = "string"
	FieldList   FieldKind = "list"
	FieldEnum   FieldKind = "enum"
	FieldBool   FieldKind = "bool"
	FieldNumber FieldKind = "number"
	// FieldScalar accepts a non-empty string or a number (metrics such as current_value).
	FieldScalar FieldKind = "scalar"
)

// FieldRule is one required key of type_fields. Blocking=false marks a recommended key that is
// reported but neither fails the ready check nor produces a question (CR-REQ-028 section 2.3).
type FieldRule struct {
	Key      string
	Kind     FieldKind
	Enum     []string
	Blocking bool
}

func (f FieldRule) Path() string { return "/type_fields/" + f.Key }

func rule(key string, kind FieldKind) FieldRule {
	return FieldRule{Key: key, Kind: kind, Blocking: true}
}

func enumRule(key string, values ...string) FieldRule {
	return FieldRule{Key: key, Kind: FieldEnum, Enum: values, Blocking: true}
}

var requiredFieldsByType = map[RequestType][]FieldRule{
	RequestTypeBug: {rule("repro_steps", FieldList), rule("actual", FieldString), rule("expected", FieldString),
		rule("environment", FieldString), enumRule("severity", "low", "medium", "high", "critical")},
	RequestTypeChangeRequest: changeFields(),
	RequestTypeRefactor:      changeFields(),
	RequestTypeSecurity: {rule("affected_components", FieldList), enumRule("exploitability", "low", "medium", "high"),
		rule("data_exposed", FieldBool)},
	RequestTypePerformance: {rule("metric", FieldString), rule("current_value", FieldScalar), rule("target_value", FieldScalar), rule("unit", FieldString)},
	RequestTypeOpsRequest:  {rule("target_environment", FieldString), rule("window", FieldString), rule("rollback_plan", FieldString)},
	RequestTypeHotfix:      {rule("production_impact", FieldString), rule("started_at", FieldString)},
	RequestTypeSpike:       {rule("question", FieldString), rule("time_box_hours", FieldNumber)},
	RequestTypeQuestion:    {rule("question", FieldString)},
	RequestTypeDocs:        {rule("audience", FieldString), rule("scope", FieldString)},
	RequestTypeTask:        {},
}

func changeFields() []FieldRule {
	scopeOut := rule("scope_out", FieldList)
	scopeOut.Blocking = false
	return []FieldRule{rule("goal", FieldString), rule("value", FieldString), rule("scope_in", FieldList), scopeOut}
}

// RequiredFields returns a copy of the keys type t needs; the only source for CR-REQ-028.
func RequiredFields(t RequestType) []FieldRule {
	return append([]FieldRule(nil), requiredFieldsByType[t]...)
}

// ValidateRequestContent returns every problem in a deterministic order.
func ValidateRequestContent(t RequestType, c RequestContent, level ValidationLevel) []Violation {
	var out []Violation
	add := func(path, code, msg string) { out = append(out, Violation{Path: path, Code: code, Message: msg}) }

	if title := strings.TrimSpace(c.Title); title == "" {
		add("/title", CodeContentFieldRequired, "title is required")
	} else if utf8.RuneCountInString(title) > maxTitleRunes {
		add("/title", CodeContentFieldInvalid, fmt.Sprintf("title is longer than %d characters", maxTitleRunes))
	}
	if len(c.Body) > maxBodyBytes {
		add("/body", CodeArtifactLimitExceeded, fmt.Sprintf("body is longer than %d bytes", maxBodyBytes))
	}
	out = append(out, validateACs(c.AcceptanceCriteria)...)
	rules := RequiredFields(t)
	known := map[string]FieldRule{}
	for _, r := range rules {
		known[r.Key] = r
	}
	for _, r := range rules {
		v, present := c.TypeFields[r.Key]
		if !present {
			continue
		}
		if msg := fieldProblem(r, v); msg != "" {
			add(r.Path(), CodeContentFieldInvalid, msg)
		}
	}
	if snap, err := c.Snapshot(nil); err != nil {
		add("", CodeContentFieldInvalid, err.Error())
	} else if len(snap) > MaxArtifactBytes(ArtifactKindRequest) {
		add("", CodeArtifactLimitExceeded, fmt.Sprintf("request content is %d bytes, limit is %d", len(snap), MaxArtifactBytes(ArtifactKindRequest)))
	}

	if level != ValidationLevelReady {
		return out
	}
	if t == "" {
		add("/type", CodeContentTypeRequired, "type is required")
		return out
	}
	if nonSpace(c.Body) < minReadyBodyChars {
		add("/body", CodeContentBodyTooShort, fmt.Sprintf("body needs at least %d non-space characters", minReadyBodyChars))
	}
	if len(c.AcceptanceCriteria.ActiveIDs()) == 0 {
		add("/acceptance_criteria", CodeContentACRequired, "at least one active acceptance criterion is required")
	}
	for _, r := range rules {
		v, present := c.TypeFields[r.Key]
		if !present || fieldEmpty(r, v) {
			add(r.Path(), CodeContentFieldRequired, fmt.Sprintf("%s is required for type %s", r.Key, t))
		}
	}
	return out
}

func nonSpace(s string) int {
	n := 0
	for _, r := range s {
		if !unicode.IsSpace(r) {
			n++
		}
	}
	return n
}

func validateACs(a AcceptanceCriteria) []Violation {
	var out []Violation
	if len(a.Items) > MaxAcceptanceCriteria {
		out = append(out, Violation{Path: "/acceptance_criteria", Code: CodeArtifactLimitExceeded, Message: fmt.Sprintf("more than %d acceptance criteria", MaxAcceptanceCriteria)})
	}
	seen := map[string]bool{}
	for i, it := range a.Items {
		p := fmt.Sprintf("/acceptance_criteria/%d", i)
		if !acIDPattern.MatchString(it.ID) {
			out = append(out, Violation{Path: p + "/id", Code: CodeContentFieldInvalid, Message: "id must look like AC-1"})
		} else if seen[it.ID] {
			out = append(out, Violation{Path: p + "/id", Code: CodeContentFieldInvalid, Message: "duplicate id " + it.ID})
		}
		seen[it.ID] = true
		if n := utf8.RuneCountInString(strings.TrimSpace(it.Text)); n < 1 || n > MaxACTextRunes {
			out = append(out, Violation{Path: p + "/text", Code: CodeContentFieldInvalid, Message: fmt.Sprintf("text must be 1..%d characters", MaxACTextRunes)})
		}
		if it.Status != ACStatusActive && it.Status != ACStatusRetired {
			out = append(out, Violation{Path: p + "/status", Code: CodeContentFieldInvalid, Message: "status must be active|retired"})
		}
		if !validVerifyHint(it.VerifyHint) {
			out = append(out, Violation{Path: p + "/verify_hint", Code: CodeContentFieldInvalid, Message: "verify_hint must be test|metric|manual|review"})
		}
	}
	return out
}

// fieldProblem checks only the shape of a present value; emptiness is the ready level's business.
func fieldProblem(r FieldRule, v any) string {
	switch r.Kind {
	case FieldString:
		if _, ok := v.(string); !ok {
			return r.Key + " must be a string"
		}
	case FieldList:
		list, ok := v.([]any)
		if !ok {
			return r.Key + " must be a list"
		}
		for _, e := range list {
			if _, isStr := e.(string); !isStr {
				return r.Key + " must be a list of strings"
			}
		}
	case FieldEnum:
		s, ok := v.(string)
		if !ok || (s != "" && !containsString(r.Enum, s)) {
			return fmt.Sprintf("%s must be one of %s", r.Key, strings.Join(r.Enum, "|"))
		}
	case FieldBool:
		if _, ok := v.(bool); !ok {
			return r.Key + " must be true or false"
		}
	case FieldNumber:
		if _, ok := v.(float64); !ok {
			return r.Key + " must be a number"
		}
	case FieldScalar:
		switch v.(type) {
		case string, float64:
		default:
			return r.Key + " must be a string or a number"
		}
	}
	return ""
}

func fieldEmpty(r FieldRule, v any) bool {
	switch r.Kind {
	case FieldString, FieldEnum, FieldScalar:
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s) == ""
		}
		return v == nil
	case FieldList:
		list, ok := v.([]any)
		if !ok || len(list) == 0 {
			return true
		}
		for _, e := range list {
			if s, isStr := e.(string); isStr && strings.TrimSpace(s) != "" {
				return false
			}
		}
		return true
	case FieldNumber:
		n, ok := v.(float64)
		return !ok || n <= 0
	case FieldBool:
		_, ok := v.(bool)
		return !ok
	}
	return v == nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
