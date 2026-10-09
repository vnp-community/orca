package domain

import "strings"

// Value types a client uses to collect and encode an answer (ClarificationQuestion.value_type).
const (
	ValueTypeString  = "string"
	ValueTypeNumber  = "number"
	ValueTypeBoolean = "boolean"
	ValueTypeList    = "list"
)

// fieldValueTypes maps each type_fields key to its value type, from the same rules that validate the answer, so
// the hint cannot drift from what the server accepts. A key that two request types declare with different
// kinds is left out and falls back to string.
var fieldValueTypes = buildFieldValueTypes()

func buildFieldValueTypes() map[string]string {
	out := map[string]string{}
	conflict := map[string]bool{}
	for _, rules := range requiredFieldsByType {
		for _, r := range rules {
			vt := valueTypeOfField(r.Kind)
			if prev, seen := out[r.Key]; seen && prev != vt {
				conflict[r.Key] = true
			}
			out[r.Key] = vt
		}
	}
	for k := range conflict {
		delete(out, k)
	}
	return out
}

func valueTypeOfField(k FieldKind) string {
	switch k {
	case FieldBool:
		return ValueTypeBoolean
	case FieldNumber:
		return ValueTypeNumber
	case FieldList:
		return ValueTypeList
	}
	return ValueTypeString // string, enum and scalar (a metric typed as text is accepted as a string)
}

// ClarificationValueType tells a client how to collect the answer: by question kind first, then by the field the
// answer is written to.
func ClarificationValueType(kind QuestionKind, targetPath string) string {
	switch kind {
	case QuestionKindBoolean:
		return ValueTypeBoolean
	case QuestionKindMultiChoice:
		return ValueTypeList
	}
	switch {
	case targetPath == "acceptance_criteria":
		return ValueTypeList
	case strings.HasPrefix(targetPath, "type_fields."):
		if vt, ok := fieldValueTypes[strings.TrimPrefix(targetPath, "type_fields.")]; ok {
			return vt
		}
	}
	return ValueTypeString
}
