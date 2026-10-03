package tools

import (
	"regexp"
	"strings"
)

var (
	emailRE = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9\-]+(?:\.[A-Za-z0-9\-]+)+`)
	// E.164-like (+84912345678) or grouped digits (0912 345 678, (555) 123-4567).
	phoneRE = regexp.MustCompile(`\+\d[\d\s.\-]{7,16}\d|\b\(?\d{2,4}\)?[\s.\-]\d{3,4}[\s.\-]\d{3,4}\b`)
	// US SSN style and 12-digit national ids (e.g. Vietnamese CCCD).
	nationalIDRE = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b|\b\d{12}\b`)
	uuidRE       = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	piiKeyRE     = regexp.MustCompile(`(?i)^(phone\w*|mobile\w*|tel|telephone|ssn|national_?id|tax_?id|passport\w*|id_?number|cccd)$`)
)

func maskEmail(m string) string {
	at := strings.LastIndexByte(m, '@')
	if at <= 0 {
		return "***"
	}
	return m[:1] + "***" + m[at:]
}

func maskDigits(m string) string {
	digits := 0
	for _, r := range m {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	keep := 2 // trailing digits left visible so a human can still tell records apart
	var b strings.Builder
	seen := 0
	for _, r := range m {
		if r >= '0' && r <= '9' {
			seen++
			if seen > digits-keep {
				b.WriteRune(r)
			} else {
				b.WriteByte('*')
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func maskPIIString(s string) string {
	if uuidRE.MatchString(s) {
		return s
	}
	s = emailRE.ReplaceAllStringFunc(s, maskEmail)
	s = phoneRE.ReplaceAllStringFunc(s, maskDigits)
	return nationalIDRE.ReplaceAllStringFunc(s, maskDigits)
}

// maskPII masks emails, phone numbers and national ids in every string of a
// decoded result; values under phone/ssn-like keys are masked wholesale.
func maskPII(v any) any {
	switch t := v.(type) {
	case string:
		return maskPIIString(t)
	case []any:
		for i := range t {
			t[i] = maskPII(t[i])
		}
	case map[string]any:
		for k, x := range t {
			if s, ok := x.(string); ok && piiKeyRE.MatchString(k) && s != "" {
				t[k] = maskDigitsOrAll(s)
				continue
			}
			t[k] = maskPII(x)
		}
	}
	return v
}

func maskDigitsOrAll(s string) string {
	if strings.ContainsAny(s, "0123456789") {
		return maskDigits(s)
	}
	return "***"
}

// piiApplies tells whether the result of spec is masked under the mode.
func piiApplies(mode string, spec *ToolSpec) bool {
	switch mode {
	case PIIMaskAll:
		return true
	case PIIMaskOff:
		return false
	}
	return spec.PII
}
