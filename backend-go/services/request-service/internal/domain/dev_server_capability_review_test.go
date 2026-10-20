package domain

import (
	"testing"
	"time"
)

func profile(t *testing.T, features []string, skills string) DevServerCapability {
	t.Helper()
	body := `{"schemaVersion":1}`
	if skills != "" {
		body = `{"schemaVersion":1,"skills":` + skills + `}`
	}
	c, err := ParseCapabilityProfile("agent", false, true, "1.0.0", 1, features, []byte(body), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestReviewContextGaps(t *testing.T) {
	ready := []string{FeatureKnowledgeHub, FeatureReadonly}
	cases := []struct {
		name     string
		features []string
		skills   string
		codes    []string
	}{
		{"ready", ready, `[{"name":"tdd-knowledge","installed":true,"sha256":"aa"}]`, nil},
		{"skills not reported: not guessed missing", ready, "", nil},
		{"skill missing", ready, `[{"name":"tdd-knowledge","installed":false}]`, []string{CodeReviewSkillMissing}},
		{"only the write-back skill", ready, `[{"name":"tdd-knowledge-writeback","installed":true,"sha256":"bb"}]`, []string{CodeReviewSkillMissing}},
		{"no hub", []string{FeatureReadonly}, `[{"name":"tdd-knowledge","installed":true,"sha256":"aa"}]`, []string{CodeReviewAgentTooOld}},
		{"no readonly and no hub", nil, `[{"name":"tdd-knowledge","installed":false}]`, []string{CodeReviewAgentTooOld, CodeReviewSkillMissing}},
	}
	for _, c := range cases {
		gaps := ReviewContextGaps(profile(t, c.features, c.skills))
		got := map[string]bool{}
		for _, g := range gaps {
			got[g.Code] = true
			if g.Message == "" {
				t.Errorf("%s: a gap needs a message", c.name)
			}
		}
		if len(got) != len(c.codes) {
			t.Errorf("%s: gaps %v, want codes %v", c.name, gaps, c.codes)
		}
		for _, code := range c.codes {
			if !got[code] {
				t.Errorf("%s: missing %s in %v", c.name, code, gaps)
			}
		}
		if SupportsReviewContext(profile(t, c.features, c.skills)) != (len(c.codes) == 0) {
			t.Errorf("%s: SupportsReviewContext disagrees with the gaps", c.name)
		}
	}
	if ReviewSkillMissing(profile(t, nil, "")) || !ReviewSkillMissing(profile(t, nil, `[{"name":"tdd-knowledge","installed":false}]`)) {
		t.Error("ReviewSkillMissing is true only when the agent reported the skill absent")
	}
}
