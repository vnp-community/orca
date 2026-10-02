package prompts

import (
	"strconv"
	"strings"
)

// ResolveLocale picks "en" or "vi" from an Accept-Language header: the
// supported tag with the highest q wins, ties go to the earlier one. The
// profile locale step of the design is not available (the Identity carries no
// locale), so anything else falls back to "en".
func ResolveLocale(acceptLanguage string) string {
	best, bestQ := "en", -1.0
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		q := 1.0
		if k, v, ok := strings.Cut(strings.TrimSpace(params), "="); ok && strings.TrimSpace(k) == "q" {
			f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil || f < 0 || f > 1 {
				continue
			}
			q = f
		}
		primary, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(tag)), "-")
		if q > bestQ && q > 0 && (primary == "en" || primary == "vi") {
			best, bestQ = primary, q
		}
	}
	return best
}
