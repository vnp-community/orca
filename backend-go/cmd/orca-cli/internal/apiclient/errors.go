package apiclient

import "encoding/json"

// CLIError maps api-gateway's writeJSONError {code, message} shape
// (git_routes.go and every other handler already use it) into a typed,
// catchable error the CLI's exit-code mapping (see internal/output) keys
// off directly.
type CLIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *CLIError) Error() string {
	return e.Code + ": " + e.Message
}

// errorBody mirrors api-gateway's writeJSONError JSON shape:
// {"error": {"code": "...", "message": "..."}}.
type errorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func decodeErrorBody(statusCode int, body []byte) error {
	var eb errorBody
	if err := json.Unmarshal(body, &eb); err != nil || eb.Error.Code == "" {
		return &CLIError{StatusCode: statusCode, Code: "UNKNOWN", Message: string(body)}
	}
	return &CLIError{StatusCode: statusCode, Code: eb.Error.Code, Message: eb.Error.Message}
}
