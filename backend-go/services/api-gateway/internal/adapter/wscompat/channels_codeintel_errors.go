package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"


	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	codeIntelMsgMaxRunes   = 200
	codeIntelDataMaxBytes  = 2048
	codeIntelTimeoutSuffix = `{"retryAfterMs":3000,"inProgress":true}`
)

var codeIntelGrpcErrorWrapper = regexp.MustCompile(`rpc error: code = [A-Za-z]+ desc = `)
var codeIntelStdFormat = regexp.MustCompile(`(?s)^(CODEINTEL_[A-Z0-9_]+):\s*(.*?)(?:\s*\|\s*(.*))?$`)
var absPathScrubber = regexp.MustCompile(`(^|[\s|"'])(?:[A-Za-z]:\\|/)[^/\\\s|]+(?:[/\\][^/\\\s|]+){2,}`)

func codeIntelChannelError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()
	if st, ok := status.FromError(err); ok && st.Code() != codes.OK {
		msg = st.Message()
	} else if !ok {
		// remove grpc wrapper if any
		locs := codeIntelGrpcErrorWrapper.FindAllStringIndex(msg, -1)
		if len(locs) > 0 {
			last := locs[len(locs)-1]
			msg = msg[last[1]:]
		}
	}

	if match := codeIntelStdFormat.FindStringSubmatch(msg); match != nil {
		code := match[1]
		human := match[2]
		dataJSON := match[3]

		// 1. human string scrubbing
		human = strings.ReplaceAll(human, "\n", " ")
		human = strings.ReplaceAll(human, "\t", " ")
		human = strings.Join(strings.Fields(human), " ") // replacing all \s+ with single space
		if utf8.RuneCountInString(human) > codeIntelMsgMaxRunes {
			runes := []rune(human)
			human = string(runes[:codeIntelMsgMaxRunes])
		}
		human = absPathScrubber.ReplaceAllString(human, "$1<path>")

		// 2. json suffix handling
		if dataJSON != "" && strings.HasPrefix(dataJSON, "{") {
			var parsed map[string]interface{}
			if json.Unmarshal([]byte(dataJSON), &parsed) == nil {
				truncated := false
				if c, ok := parsed["candidates"].([]interface{}); ok && len(c) > 10 {
					parsed["candidates"] = c[:10]
					truncated = true
				}
				b, _ := json.Marshal(parsed)
				if truncated || len(b) > codeIntelDataMaxBytes {
					if c, ok := parsed["candidates"].([]interface{}); ok {
						for len(b) > codeIntelDataMaxBytes && len(c) > 0 {
							c = c[:len(c)-1]
							parsed["candidates"] = c
							b, _ = json.Marshal(parsed)
						}
					}
				}
				if len(b) <= codeIntelDataMaxBytes {
					return errors.New(code + ": " + human + " | " + string(b))
				}
			}
		}
		return errors.New(code + ": " + human)
	}

	st, ok := status.FromError(err)
	var code codes.Code
	if ok {
		code = st.Code()
	} else if errors.Is(err, context.DeadlineExceeded) {
		code = codes.DeadlineExceeded
	} else if errors.Is(err, context.Canceled) {
		code = codes.Canceled
	} else {
		code = codes.Unknown
	}

	switch code {
	case codes.Unavailable, codes.Unimplemented:
		return errors.New("CODEINTEL_UNAVAILABLE: code-intel unavailable")
	case codes.DeadlineExceeded:
		return errors.New("CODEINTEL_TIMEOUT: code-intel did not respond in time | " + codeIntelTimeoutSuffix)
	case codes.Canceled:
		return errors.New("CODEINTEL_TIMEOUT: request cancelled")
	case codes.NotFound, codes.PermissionDenied:
		return errors.New("CODEINTEL_NOT_AUTHORIZED: not found or not permitted")
	case codes.InvalidArgument:
		return errors.New("CODEINTEL_INVALID_PARAMS: invalid params")
	case codes.FailedPrecondition:
		return errors.New("CODEINTEL_TOOL_FAILED: tool failed")
	case codes.ResourceExhausted:
		lowerMsg := strings.ToLower(msg)
		if strings.Contains(lowerMsg, "size") || strings.Contains(lowerMsg, "bytes") || strings.Contains(lowerMsg, "too large") || strings.Contains(lowerMsg, "larger than max") {
			return errors.New("CODEINTEL_RESPONSE_TOO_LARGE: response too large")
		}
		return errors.New("CODEINTEL_RATE_LIMITED: rate limited")
	case codes.Aborted, codes.AlreadyExists:
		return errors.New("CODEINTEL_VERSION_CONFLICT: version conflict")
	default:
		return errors.New("CODEINTEL_INTERNAL: internal error")
	}
}

var errCodeIntelUnavailable = errors.New("CODEINTEL_UNAVAILABLE: code-intel unavailable")
var errCodeIntelNotFound = errors.New("CODEINTEL_NOT_AUTHORIZED: not found or not permitted")
var errCodeIntelNotAuthorized = errors.New("CODEINTEL_NOT_AUTHORIZED: not found or not permitted")

func errCodeIntelResponseTooLarge(bytes int, limit int) error {
	return fmt.Errorf("CODEINTEL_RESPONSE_TOO_LARGE: response too large | {\"bytes\":%d,\"limit\":%d}", bytes, limit)
}

