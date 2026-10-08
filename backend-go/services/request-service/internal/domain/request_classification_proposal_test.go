package domain

import (
	"errors"
	"strings"
	"testing"
)

const validProposal = `{"type":"bug","size":"M","urgency":"normal","confidence":0.82,"reason":"stack trace in body"}`

func TestParseClassificationProposal_Valid(t *testing.T) {
	p, err := ParseClassificationProposal([]byte(validProposal))
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != RequestTypeBug || p.Size != RequestSizeM || p.Urgency != UrgencyNormal || p.Confidence != 0.82 || p.Reason != "stack trace in body" {
		t.Fatalf("got %+v", p)
	}
}

func TestParseClassificationProposal_MarkdownWrapped(t *testing.T) {
	raw := "Sure! Here is the result:\n```json\n" + `{"type":"task","size":"S","urgency":"urgent","confidence":1,"reason":"a } brace and \" quote in text {"}` + "\n```\nDone."
	p, err := ParseClassificationProposal([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if p.Type != RequestTypeTask || p.Urgency != UrgencyUrgent || !strings.Contains(p.Reason, "} brace") {
		t.Fatalf("got %+v", p)
	}
}

func TestParseClassificationProposal_Rejects(t *testing.T) {
	cases := map[string]string{
		"missing field":      `{"type":"bug","size":"M","urgency":"normal","confidence":0.5}`,
		"unknown field":      `{"type":"bug","size":"M","urgency":"normal","confidence":0.5,"reason":"x","extra":1}`,
		"type outside enum":  `{"type":"hotfix2","size":"M","urgency":"normal","confidence":0.5,"reason":"x"}`,
		"shell as type":      `{"type":"rm -rf /","size":"M","urgency":"normal","confidence":0.5,"reason":"x"}`,
		"size outside enum":  `{"type":"bug","size":"XL","urgency":"normal","confidence":0.5,"reason":"x"}`,
		"urgency outside":    `{"type":"bug","size":"M","urgency":"asap","confidence":0.5,"reason":"x"}`,
		"confidence low":     `{"type":"bug","size":"M","urgency":"normal","confidence":-0.1,"reason":"x"}`,
		"confidence high":    `{"type":"bug","size":"M","urgency":"normal","confidence":1.1,"reason":"x"}`,
		"reason too long":    `{"type":"bug","size":"M","urgency":"normal","confidence":0.5,"reason":"` + strings.Repeat("a", 2001) + `"}`,
		"not json":           `I think it is a bug`,
		"unterminated":       `{"type":"bug"`,
		"wrong confidence t": `{"type":"bug","size":"M","urgency":"normal","confidence":"high","reason":"x"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := ParseClassificationProposal([]byte(raw))
			if !errors.Is(err, ErrProposalInvalid) {
				t.Fatalf("want ErrProposalInvalid, got %v", err)
			}
			var pe *ProposalInvalidError
			if !errors.As(err, &pe) || pe.Reason == "" {
				t.Fatalf("want ProposalInvalidError with a reason, got %v", err)
			}
		})
	}
}

func TestParseClassificationProposal_ReasonAtLimitOK(t *testing.T) {
	raw := `{"type":"bug","size":"M","urgency":"normal","confidence":0.5,"reason":"` + strings.Repeat("ệ", 2000) + `"}`
	if _, err := ParseClassificationProposal([]byte(raw)); err != nil {
		t.Fatal(err)
	}
}
