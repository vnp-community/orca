package domain

import (
	"encoding/json"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func newDecision() Decision {
	return Decision{
		ID: "d1", Status: DecisionStatusOpen, RecommendedOptionID: "opt-1",
		Options: []DecisionOption{{ID: "opt-1", Label: "Nâng cấp từng bước"}, {ID: "opt-2", Label: "Viết lại toàn bộ"}},
	}
}

func TestDecision_Choose_RationaleRequiredWhenNotRecommended(t *testing.T) {
	d := newDecision()
	if _, err := d.Choose("opt-2", "u", "  ", RiskNormal, "dig", t0); errCode(err) != "REQUEST_DECISION_RATIONALE_REQUIRED" {
		t.Fatalf("got %v", err)
	}
	if d.ChosenOptionID != "" || d.Status != DecisionStatusOpen {
		t.Fatalf("a refused choice must leave no trace: %+v", d)
	}
	action, err := d.Choose("opt-2", "u", "Nợ kỹ thuật quá lớn", RiskNormal, "dig", t0)
	if err != nil || action != DecisionActionChosen || d.Status != DecisionStatusEffective || d.Rationale != "Nợ kỹ thuật quá lớn" {
		t.Fatalf("%v %v %+v", action, err, d)
	}
	e := newDecision()
	if _, err := e.Choose("opt-1", "u", "", RiskNormal, "dig", t0); err != nil {
		t.Fatalf("the recommended option needs no rationale: %v", err)
	}
	if _, err := e.Choose("opt-9", "u", "x", RiskNormal, "dig", t0); err == nil {
		t.Fatal("unknown option must fail")
	}
}

func TestDecision_HighRiskStaysChosen(t *testing.T) {
	d := newDecision()
	if _, err := d.Choose("opt-1", "u", "", RiskHigh, "dig", t0); err != nil || d.Status != DecisionStatusChosen {
		t.Fatalf("%v %+v", err, d)
	}
}

func TestDecision_RechooseClearsConfirmation(t *testing.T) {
	d := newDecision()
	_, _ = d.Choose("opt-1", "u", "", RiskHigh, "d1", t0)
	if err := d.Confirm("u", "nâng cấp từng bước", t0); err != nil || d.Status != DecisionStatusEffective || d.ConfirmedBy != "u" {
		t.Fatalf("%v %+v", err, d)
	}
	action, err := d.Choose("opt-2", "u", "đổi ý", RiskHigh, "d2", t0)
	if err != nil || action != DecisionActionRechosen {
		t.Fatalf("%v %v", action, err)
	}
	if d.ConfirmedBy != "" || d.ConfirmedAt != nil || d.Status != DecisionStatusChosen || d.SubjectDigest != "d2" {
		t.Fatalf("confirmation must be cleared: %+v", d)
	}
}

func TestDecision_Confirm_VietnameseNFCAndCase(t *testing.T) {
	d := newDecision()
	d.Options[0].Label = "Nâng cấp từng bước"
	_, _ = d.Choose("opt-1", "u", "", RiskHigh, "x", t0)
	for _, text := range []string{
		"Nâng cấp từng bước",
		"  NÂNG CẤP   TỪNG BƯỚC ",
		norm.NFD.String("Nâng cấp từng bước"),
		norm.NFD.String("nâng cấp từng bước"),
	} {
		c := d
		if err := c.Confirm("u", text, t0); err != nil {
			t.Errorf("%q should confirm: %v", text, err)
		}
	}
	for _, text := range []string{"", "Nang cap tung buoc", "Viết lại toàn bộ"} {
		c := d
		if err := c.Confirm("u", text, t0); errCode(err) != "REQUEST_DECISION_CONFIRMATION_MISMATCH" {
			t.Errorf("%q: got %v", text, err)
		}
	}
	open := newDecision()
	if err := open.Confirm("u", "x", t0); errCode(err) != "REQUEST_DECISION_STATE_INVALID" {
		t.Errorf("confirming an open decision: %v", err)
	}
}

func TestDecision_Supersede(t *testing.T) {
	d := newDecision()
	d.Supersede()
	if d.IsLive() {
		t.Fatal("superseded is not live")
	}
	if _, err := d.Choose("opt-1", "u", "", RiskNormal, "x", t0); errCode(err) != "REQUEST_DECISION_STATE_INVALID" {
		t.Fatalf("got %v", err)
	}
}

func TestDecisionRisk_Assess_Table(t *testing.T) {
	cases := []struct {
		s    RiskSignals
		th   int
		want RiskLevel
	}{
		{RiskSignals{}, 3, RiskNormal},
		{RiskSignals{BreakingChange: true}, 3, RiskHigh},
		{RiskSignals{HighSeverity: true}, 3, RiskHigh},
		{RiskSignals{AffectedService: 2}, 3, RiskNormal},
		{RiskSignals{AffectedService: 3}, 3, RiskHigh},
		{RiskSignals{AffectedService: 2}, 2, RiskHigh},
		{RiskSignals{AffectedService: 2}, 0, RiskNormal},
		{RiskSignals{AffectedService: 3}, 0, RiskHigh},
	}
	for i, c := range cases {
		if got := (DecisionRisk{}).Assess(c.s, c.th); got != c.want {
			t.Errorf("case %d: %v -> %s want %s", i, c.s, got, c.want)
		}
	}
}

func TestRiskSignalsFromOption(t *testing.T) {
	s, err := RiskSignalsFromOption(json.RawMessage(`{"breaking_change":true,"risk":{"level":"high"},"risks":[{"severity":"low"},{"severity":"high"}],
		"affected_areas":[{"kind":"service"},{"kind":"service"},{"kind":"library"},{"name":"x"}]}`))
	if err != nil || !s.BreakingChange || !s.HighSeverity || s.AffectedService != 2 {
		t.Fatalf("%+v %v", s, err)
	}
	s, _ = RiskSignalsFromOption(json.RawMessage(`{"risk":{"level":"low"},"risks":[{"severity":"medium"}]}`))
	if s.HighSeverity || s.BreakingChange {
		t.Fatalf("%+v", s)
	}
	if _, err := RiskSignalsFromOption(json.RawMessage(`[`)); err == nil {
		t.Fatal("bad JSON must fail")
	}
}

func TestOptionIndexByID(t *testing.T) {
	opts := []Option{{ID: "opt-1"}, {ID: "opt-2"}, {ID: "opt-3"}}
	if i, ok := OptionIndexByID(opts, "opt-3"); !ok || i != 2 {
		t.Fatalf("%d %v", i, ok)
	}
	if _, ok := OptionIndexByID(opts, "opt-4"); ok {
		t.Fatal("unknown id")
	}
}
