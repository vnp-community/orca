package domain

import (
	"strings"
	"testing"
)

func TestAC_Add_NeverReusesRetiredNumber(t *testing.T) {
	var a AcceptanceCriteria
	first, _ := a.Add("một", "")
	second, _ := a.Add("hai", "test")
	if first.ID != "AC-1" || second.ID != "AC-2" {
		t.Fatalf("ids %s %s", first.ID, second.ID)
	}
	if err := a.Retire("AC-2"); err != nil {
		t.Fatal(err)
	}
	third, _ := a.Add("ba", "")
	if third.ID != "AC-3" {
		t.Fatalf("a retired number must not come back: got %s", third.ID)
	}
}

func TestAC_Retire_KeepsNextCounter(t *testing.T) {
	var a AcceptanceCriteria
	_, _ = a.Add("một", "")
	next := a.Next
	_ = a.Retire("AC-1")
	if a.Next != next {
		t.Fatalf("retire changed Next from %d to %d", next, a.Next)
	}
	if got, ok := a.Find("AC-1"); !ok || got.Status != ACStatusRetired {
		t.Fatalf("retired AC must stay readable: %+v %v", got, ok)
	}
	if len(a.ActiveIDs()) != 0 {
		t.Fatal("no active AC expected")
	}
	if err := a.Retire("AC-9"); err == nil {
		t.Fatal("unknown id must fail")
	}
}

func TestAC_TextTooLong(t *testing.T) {
	var a AcceptanceCriteria
	if _, err := a.Add(strings.Repeat("ế", MaxACTextRunes+1), ""); err == nil {
		t.Fatal("501 runes must be refused")
	}
	if _, err := a.Add(strings.Repeat("ế", MaxACTextRunes), ""); err != nil {
		t.Fatalf("500 runes (multi-byte) must pass: %v", err)
	}
	if _, err := a.Add("  ", ""); err == nil {
		t.Fatal("blank text must be refused")
	}
	if _, err := a.Add("x", "magic"); err == nil {
		t.Fatal("unknown verify_hint must be refused")
	}
}

func TestAC_LimitFifty(t *testing.T) {
	var a AcceptanceCriteria
	for i := 0; i < MaxAcceptanceCriteria; i++ {
		if _, err := a.Add("x", ""); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.Add("x", ""); errCode(err) != CodeArtifactLimitExceeded {
		t.Fatalf("got %v", err)
	}
}

func TestAC_Reconcile(t *testing.T) {
	var a AcceptanceCriteria
	_, _ = a.Add("giữ", "")
	_, _ = a.Add("bỏ", "")
	got, err := a.Reconcile([]ACInput{{ID: "AC-1", Text: "giữ (sửa)"}, {Text: "mới"}})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]AcceptanceCriterion{}
	for _, it := range got.Items {
		byID[it.ID] = it
	}
	if byID["AC-1"].Text != "giữ (sửa)" || byID["AC-1"].Status != ACStatusActive {
		t.Errorf("AC-1 = %+v", byID["AC-1"])
	}
	if byID["AC-2"].Status != ACStatusRetired {
		t.Errorf("an omitted AC must be retired, got %+v", byID["AC-2"])
	}
	if byID["AC-3"].Text != "mới" || got.Next != 4 {
		t.Errorf("new AC = %+v next=%d", byID["AC-3"], got.Next)
	}
	if len(a.Items) != 2 {
		t.Error("Reconcile must not mutate the receiver")
	}
	if _, err := a.Reconcile([]ACInput{{ID: "AC-7", Text: "x"}}); err == nil {
		t.Error("unknown id must fail")
	}
	if _, err := a.Reconcile([]ACInput{{ID: "AC-1", Text: "x"}, {ID: "AC-1", Text: "y"}}); err == nil {
		t.Error("duplicate id must fail")
	}
	// A retired AC cannot be revived by sending it back.
	retired, _ := got.Reconcile([]ACInput{{ID: "AC-1", Text: "a"}, {ID: "AC-2", Text: "revive"}, {ID: "AC-3", Text: "c"}})
	if it, _ := retired.Find("AC-2"); it.Status != ACStatusRetired || it.Text != "bỏ" {
		t.Errorf("retired AC changed: %+v", it)
	}
}
