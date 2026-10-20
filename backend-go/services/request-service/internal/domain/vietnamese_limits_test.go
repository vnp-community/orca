package domain

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// vietnameseRunes returns exactly n runes of precomposed Vietnamese letters (NFC), each of which has a longer NFD form.
func vietnameseRunes(n int) string {
	cycle := []rune("ệếưỡạờ ")
	out := make([]rune, n)
	for i := range out {
		out[i] = cycle[i%len(cycle)]
	}
	return string(out)
}

// limitCase checks one rune limit with accented text: at the limit passes, one more is refused, and the NFD spelling of the
// at-limit text counts more runes and is refused too (CR-REQ-046 R5: the limits do not normalise).
type limitCase struct {
	name  string
	limit int
	check func(text string) error
	want  string
}

func (c limitCase) run(t *testing.T) {
	t.Helper()
	atLimit := vietnameseRunes(c.limit)
	if err := c.check(atLimit); err != nil {
		t.Errorf("%s: %d accented runes must pass: %v", c.name, c.limit, err)
	}
	if err := c.check(vietnameseRunes(c.limit + 1)); err == nil || !strings.Contains(err.Error(), c.want) {
		t.Errorf("%s: %d accented runes must be refused with %q, got %v", c.name, c.limit+1, c.want, err)
	}
	nfd := norm.NFD.String(atLimit)
	if utf8.RuneCountInString(nfd) <= c.limit {
		t.Fatalf("%s: the NFD fixture does not count more runes than the NFC one", c.name)
	}
	if err := c.check(nfd); err == nil || !strings.Contains(err.Error(), c.want) {
		t.Errorf("%s: the NFD spelling of the same %d characters counts %d runes and must be refused, got %v", c.name, c.limit, utf8.RuneCountInString(nfd), err)
	}
}

func optionsWith(t *testing.T, set func(o *Option, text string)) func(string) error {
	return func(text string) error {
		o := loadValidOptions(t)
		set(&o.Options[0], text)
		return o.Validate(2)
	}
}

func TestVietnameseTextAgainstTheRuneLimits(t *testing.T) {
	const confidence = 0.5
	for _, c := range []limitCase{
		{"option approach", 4000, optionsWith(t, func(o *Option, s string) { o.Approach = s }), "approach length must be 1..4000"},
		{"option title", 120, optionsWith(t, func(o *Option, s string) { o.Title = s }), "title length must be 1..120"},
		{"option summary", 600, optionsWith(t, func(o *Option, s string) { o.Summary = s }), "summary length must be 1..600"},
		{"evidence excerpt", 600, func(s string) error {
			return validateEvidence("diagnosis", []SupportingRef{{Ref: "a.go:1", Excerpt: s}})
		}, "excerpt exceeds 600 characters"},
		{"answer_markdown", 8000, func(s string) error {
			return AnswerDocument{SchemaVersion: 1, Kind: "answer", AnswerMarkdown: s, Confidence: confidence}.Validate()
		}, "answer_markdown must be 1..8000"},
		{"plan task title", 200, func(s string) error {
			lim := PlanLimits{MaxPhases: 2, MaxTasksPerContainer: 3, MaxTasksTotal: 4, MaxTitleRunes: 200}
			return ValidatePlanProposal(RequestTypeTask, ShapeTaskListShell, lim, planOf(okTask(s)))
		}, "longer than 200 characters"},
	} {
		t.Run(c.name, c.run)
	}
}

func TestVietnameseOptionSurvivesParseAndMarshal(t *testing.T) {
	o := loadValidOptions(t)
	o.Options[0].Approach = vietnameseRunes(4000)
	raw, err := o.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseSolutionOptions(raw)
	if err != nil || back.Options[0].Approach != o.Options[0].Approach {
		t.Fatalf("a 4000 rune accented approach changed on the way through JSON: %v", err)
	}
	if err := back.Validate(2); err != nil {
		t.Fatal(err)
	}
}
