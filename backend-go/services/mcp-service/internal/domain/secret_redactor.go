package domain

import (
	"net/url"
	"regexp"
	"strings"
)

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`),
	regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]{8,}`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?(?:-----END [A-Z ]*PRIVATE KEY-----|$)`),
	regexp.MustCompile(`(?i)(\b(?:password|passwd|secret|token|api[_-]?key|authorization)\b\s*["']?\s*[:=]\s*["']?)[^\s"',;&]{4,}`),
}

// SecretRedactor is the pattern-based Redactor used for previews, audit
// summaries and egress checks. It is deliberately conservative: a false
// positive only shows [REDACTED], a miss would leak into audit.
type SecretRedactor struct{}

const redactedMarker = "[REDACTED]"

func (SecretRedactor) Redact(s string) (string, bool) {
	out := s
	for i, re := range secretPatterns {
		if i == len(secretPatterns)-1 {
			out = re.ReplaceAllString(out, "${1}"+redactedMarker)
			continue
		}
		out = re.ReplaceAllString(out, redactedMarker)
	}
	return out, out != s
}

var sensitiveQueryKeys = map[string]bool{
	"token": true, "key": true, "secret": true, "password": true, "api_key": true, "apikey": true,
	"sig": true, "signature": true, "access_token": true,
}

// ErrEgressSecret is returned when arguments of an open-world tool carry credentials.
var ErrEgressSecret = errorString("egress_secret_blocked")

type errorString string

func (e errorString) Error() string { return string(e) }

// ValidateEgressArgs rejects strings that look like credentials or URLs
// carrying them: userinfo, sensitive query keys, or anything Redact changes.
func ValidateEgressArgs(raw []byte, r Redactor) error {
	v, err := canonicalArgsValue(raw)
	if err != nil {
		return err
	}
	return walkStrings(v, func(s string) error {
		if r != nil {
			if _, changed := r.Redact(s); changed {
				return ErrEgressSecret
			}
		}
		if strings.Contains(s, "://") {
			for _, f := range strings.Fields(s) {
				u, err := url.Parse(f)
				if err != nil || u.Host == "" {
					continue
				}
				if u.User != nil {
					return ErrEgressSecret
				}
				for k := range u.Query() {
					if sensitiveQueryKeys[strings.ToLower(k)] {
						return ErrEgressSecret
					}
				}
			}
		}
		return nil
	})
}

func walkStrings(v any, fn func(string) error) error {
	switch t := v.(type) {
	case string:
		return fn(t)
	case []any:
		for _, e := range t {
			if err := walkStrings(e, fn); err != nil {
				return err
			}
		}
	case *jsonObject:
		for k, e := range t.vals {
			if err := fn(k); err != nil {
				return err
			}
			if err := walkStrings(e, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// SanitizeClientName strips control/invisible characters and caps the length:
// the name is chosen by the (possibly hostile) client at registration.
func SanitizeClientName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if r < 0x20 || isInvisible(r) {
			continue
		}
		b.WriteRune(r)
		if n++; n >= 80 {
			break
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "Unknown client"
	}
	return out
}
