package domain

import (
	"sort"
	"strings"
)

// MissingField is one gap in the Definition of Ready. Blocking=false gaps are advice only.
type MissingField struct {
	Path     string
	Rule     string
	Blocking bool
}

type ReadinessReport struct {
	Type    RequestType
	Ready   bool
	Missing []MissingField
}

// Blocker returns the blocking gaps, in report order.
func (r ReadinessReport) Blocker() []MissingField {
	var out []MissingField
	for _, m := range r.Missing {
		if m.Blocking {
			out = append(out, m)
		}
	}
	return out
}

// ReadinessPolicy evaluates the Definition of Ready by type; it owns no rules of its own, only
// ValidateRequestContent(level=ready) and RequiredFields (CR-REQ-027) do.
type ReadinessPolicy struct{}

func (ReadinessPolicy) Evaluate(t RequestType, c RequestContent) ReadinessReport {
	rules := RequiredFields(t)
	rank := func(path string) int {
		switch {
		case path == "/title":
			return 0
		case path == "/type":
			return 1
		case path == "/body":
			return 2
		case path == "/acceptance_criteria" || strings.HasPrefix(path, "/acceptance_criteria/"):
			return 1000
		}
		for i, r := range rules {
			if r.Path() == path {
				return 10 + i
			}
		}
		return 2000
	}
	blocking := func(path string) bool {
		for _, r := range rules {
			if r.Path() == path {
				return r.Blocking
			}
		}
		return true
	}
	var missing []MissingField
	for _, v := range ValidateRequestContent(t, c, ValidationLevelReady) {
		missing = append(missing, MissingField{Path: v.Path, Rule: v.Code, Blocking: blocking(v.Path)})
	}
	sort.SliceStable(missing, func(i, j int) bool { return rank(missing[i].Path) < rank(missing[j].Path) })
	report := ReadinessReport{Type: t, Ready: true, Missing: missing}
	for _, m := range missing {
		if m.Blocking {
			report.Ready = false
		}
	}
	return report
}
