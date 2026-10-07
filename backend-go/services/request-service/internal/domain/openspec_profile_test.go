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
	f1 := FlowFor(RequestTypeChangeRequest)
	if f1.OpenSpecProfile != OpenSpecProfileFull {
		t.Errorf("expected full, got %v", f1.OpenSpecProfile)
	}

	f2 := FlowFor("unknown")
	if f2.OpenSpecProfile != OpenSpecProfileNone {
		t.Errorf("expected none, got %v", f2.OpenSpecProfile)
	}
}
