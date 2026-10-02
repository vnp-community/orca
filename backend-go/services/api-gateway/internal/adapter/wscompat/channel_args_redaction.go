package wscompat

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

// sensitiveArgChannels lists channels whose args carry a secret, and the arg
// field holding it (CONTRACT C11, decision D1: plaintext over TLS once). The
// WS handler, Registry.Dispatch and the gRPC client stack currently log and
// trace no args at all; this is the single place any future capture path
// (request log, span attribute, trace store) must go through, and Dispatch
// already uses it to keep errors from echoing the value.
var sensitiveArgChannels = map[string][]string{
	"mcp.externalServer.setSecret": {"value"},
}

const redactedArgs = "[redacted]"

// IsSensitiveChannel reports whether channel's args must never be recorded.
func IsSensitiveChannel(channel string) bool {
	_, ok := sensitiveArgChannels[channel]
	return ok
}

// RedactChannelArgs returns what may be logged or traced for a call: the args
// unchanged for ordinary channels, a fixed placeholder for sensitive ones.
func RedactChannelArgs(channel string, args []json.RawMessage) []json.RawMessage {
	if !IsSensitiveChannel(channel) {
		return args
	}
	if len(args) == 0 {
		return args
	}
	out := make([]json.RawMessage, len(args))
	for i := range args {
		out[i] = json.RawMessage(`"` + redactedArgs + `"`)
	}
	return out
}

var codedChannelError = regexp.MustCompile(`^[A-Z][A-Z0-9_]+: [^\n]*$`)

// secretStrings collects the sensitive field values of the args so an error
// message can be checked against them.
func secretStrings(channel string, args []json.RawMessage) []string {
	fields := sensitiveArgChannels[channel]
	var out []string
	for _, a := range args {
		var m map[string]json.RawMessage
		if json.Unmarshal(a, &m) != nil {
			continue
		}
		for _, f := range fields {
			var s string
			if json.Unmarshal(m[f], &s) == nil && s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// scrubSensitiveChannelError makes an error from a sensitive channel safe to
// send to the client and to log: only a coded "CODE: message" that does not
// contain any secret value survives; anything else becomes a generic error.
func scrubSensitiveChannelError(channel string, args []json.RawMessage, err error) error {
	if err == nil || !IsSensitiveChannel(channel) {
		return err
	}
	msg := err.Error()
	if !codedChannelError.MatchString(msg) {
		return errors.New("MCP_INTERNAL: internal error")
	}
	for _, s := range secretStrings(channel, args) {
		if strings.Contains(msg, s) {
			return errors.New("MCP_INTERNAL: internal error")
		}
	}
	return err
}
