package tools

import (
	"regexp"
	"strings"
)

// Rules are applied to every string in a result. Order matters: PEM first so
// its body is not partially matched by the token rules.
var redactionRules = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)`), "***"},
	// userinfo in any URL: https://user:token@host -> https://***@host
	{regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.\-]*://)[^/\s:@"']+(?::[^@\s/"']*)?@`), "${1}***@"},
	{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`), "***"},
	{regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`), "***"},
	{regexp.MustCompile(`\bglpat-[A-Za-z0-9_\-]{15,}`), "***"},
	{regexp.MustCompile(`\bxox[bap]-[A-Za-z0-9\-]{10,}`), "***"},
	{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "***"},
	{regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=\-]{16,}`), "Bearer ***"},
}

// secretKey matches JSON keys whose value is never returned.
var secretKey = regexp.MustCompile(`(?i)^(credential\w*|api_?key|token|access_?token|refresh_?token|id_?token|secret|client_?secret|password|passwd|authorization|private_?key)$`)

func redactString(s string) string {
	for _, r := range redactionRules {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}

// redactValue walks a decoded JSON value in place-copy fashion.
func redactValue(v any) any {
	switch t := v.(type) {
	case string:
		return redactString(t)
	case []any:
		for i := range t {
			t[i] = redactValue(t[i])
		}
		return t
	case map[string]any:
		for k, x := range t {
			if secretKey.MatchString(k) {
				if x != nil {
					t[k] = "***"
				}
				continue
			}
			t[k] = redactValue(x)
		}
		return t
	}
	return v
}

// camelKey turns snake_case keys (raw proto structs marshalled by
// encoding/json) into the camelCase protojson would have produced.
func camelKey(k string) string {
	if !strings.Contains(k, "_") {
		return k
	}
	parts := strings.Split(k, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func camelizeKeys(v any) any {
	switch t := v.(type) {
	case []any:
		for i := range t {
			t[i] = camelizeKeys(t[i])
		}
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, x := range t {
			out[camelKey(k)] = camelizeKeys(x)
		}
		return out
	}
	return v
}
