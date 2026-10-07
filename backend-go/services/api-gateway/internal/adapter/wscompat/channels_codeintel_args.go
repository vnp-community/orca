package wscompat

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type codeIntelArgs interface {
	validate() error
}

type codeIntelParamError struct {
	Code   string
	Field  string
	Reason string
	Limit  int
}

func (e *codeIntelParamError) Error() string {
	code := e.Code
	if code == "" {
		code = "CODEINTEL_INVALID_PARAMS"
	}
	desc := "invalid params"
	if code == "CODEINTEL_PATH_NOT_ALLOWED" {
		desc = "path not allowed"
	} else if e.Reason == "too_large" {
		desc = "arguments too large"
	} else if e.Field != "" {
		desc = "invalid parameter: " + e.Field
	}

	payload := make(map[string]any)
	if e.Field != "" {
		payload["field"] = e.Field
	}
	if e.Reason != "" {
		payload["reason"] = e.Reason
	}
	if e.Limit > 0 {
		payload["limit"] = e.Limit
	}

	b, _ := json.Marshal(payload)
	return fmt.Sprintf("%s: %s | %s", code, desc, string(b))
}

func invalidParam(field, reason string) error {
	return &codeIntelParamError{
		Code:   "CODEINTEL_INVALID_PARAMS",
		Field:  field,
		Reason: reason,
	}
}

func tooLarge(limit int) error {
	return &codeIntelParamError{
		Code:   "CODEINTEL_INVALID_PARAMS",
		Field:  "args",
		Reason: "too_large",
		Limit:  limit,
	}
}

func pathNotAllowed(field string) error {
	return &codeIntelParamError{
		Code:   "CODEINTEL_PATH_NOT_ALLOWED",
		Field:  field,
		Reason: "path_not_allowed",
	}
}

var codeIntelForbiddenKeys = map[string]struct{}{
	"tenantid":      {},
	"userid":        {},
	"deviceid":      {},
	"role":          {},
	"devserverid":   {},
	"workspaceroot": {},
	"repo":          {},
	"args":          {},
	"command":       {},
	"cypher":        {},
}

func isForbiddenCodeIntelKey(key string) bool {
	_, found := codeIntelForbiddenKeys[strings.ToLower(key)]
	return found
}

func decodeCodeIntelArgs[A codeIntelArgs](spec codeIntelChannelSpec, args []json.RawMessage) (A, error) {
	var zero A
	if len(args) > 1 {
		return zero, invalidParam("args", "expected_single_params_object")
	}

	raw := []byte("{}")
	if len(args) == 1 {
		raw = args[0]
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
			raw = []byte("{}")
		}
	}

	trimmedLead := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmedLead) == 0 || trimmedLead[0] != '{' {
		return zero, invalidParam("args", "not_an_object")
	}

	if spec.MaxArgsBytes > 0 && len(raw) > spec.MaxArgsBytes {
		return zero, tooLarge(spec.MaxArgsBytes)
	}

	decKeys := json.NewDecoder(bytes.NewReader(raw))
	var topKeys map[string]json.RawMessage
	if err := decKeys.Decode(&topKeys); err != nil {
		return zero, invalidParam("args", "invalid_json")
	}

	for k := range topKeys {
		if isForbiddenCodeIntelKey(k) {
			return zero, invalidParam(k, "not_allowed")
		}
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var target A
	if err := dec.Decode(&target); err != nil {
		var ute *json.UnmarshalTypeError
		if errors.As(err, &ute) {
			field := ute.Field
			if field == "" {
				field = "args"
			}
			return zero, invalidParam(field, "wrong_type")
		}
		msg := err.Error()
		if strings.HasPrefix(msg, `json: unknown field "`) {
			field := strings.TrimSuffix(strings.TrimPrefix(msg, `json: unknown field "`), `"`)
			return zero, invalidParam(field, "unknown_field")
		}
		return zero, invalidParam("args", "wrong_type")
	}

	var trailing json.RawMessage
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		return zero, invalidParam("args", "trailing_data")
	}

	if err := target.validate(); err != nil {
		return zero, err
	}

	return target, nil
}
