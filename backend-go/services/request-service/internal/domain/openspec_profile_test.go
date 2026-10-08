package domain

import "testing"

func TestOpenSpecProfileFor_AllElevenTypes(t *testing.T) {
	allTypes := AllRequestTypes()
	if len(allTypes) < 11 {
		t.Errorf("expected at least 11 request types, got %d", len(allTypes))
	}

	profiles := make(map[OpenSpecProfile]int)
	for _, rt := range allTypes {
		prof := OpenSpecProfileFor(rt)
		if prof != OpenSpecProfileFull && prof != OpenSpecProfileLight && prof != OpenSpecProfileNone {
			t.Errorf("unexpected profile %s for type %s", prof, rt)
		}
		profiles[prof]++
	}

	if profiles[OpenSpecProfileFull] == 0 {
		t.Errorf("no type maps to full")
	}
	if profiles[OpenSpecProfileLight] == 0 {
		t.Errorf("no type maps to light")
	}
	if profiles[OpenSpecProfileNone] == 0 {
		t.Errorf("no type maps to none")
	}
}

func TestFlowFor_CarriesOpenSpecProfile(t *testing.T) {
	f1, err := FlowFor(RequestTypeChangeRequest)
	if err != nil {
		t.Fatal(err)
	}
	if f1.OpenSpecProfile != OpenSpecProfileFull {
		t.Errorf("expected full, got %v", f1.OpenSpecProfile)
	}

	if _, err := FlowFor("unknown"); err == nil {
		t.Errorf("unknown type must not have a flow")
	}
	if got := OpenSpecProfileFor("unknown"); got != OpenSpecProfileNone {
		t.Errorf("expected none, got %v", got)
	}
}
