package usecase

import (
	"errors"
	"reflect"
	"testing"
)

func TestMergePreflightStatuses_RelayOverridesLocalByID(t *testing.T) {
	local := []PreflightCheckResult{
		{ID: "git", Status: PreflightOK, Source: PreflightSourceLocal},
	}
	relay := []PreflightCheckResult{
		{ID: "git", Status: PreflightError, Message: "boom", Source: PreflightSourceRelay},
	}
	got := MergePreflightStatuses(local, relay, nil)
	want := []PreflightCheckResult{
		{ID: "git", Status: PreflightError, Message: "boom", Source: PreflightSourceRelay},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMergePreflightStatuses_RelayOnlyIDsAppended(t *testing.T) {
	local := []PreflightCheckResult{
		{ID: "git", Status: PreflightOK, Source: PreflightSourceLocal},
	}
	relay := []PreflightCheckResult{
		{ID: "disk-space", Status: PreflightOK, Source: PreflightSourceRelay},
	}
	got := MergePreflightStatuses(local, relay, nil)
	want := []PreflightCheckResult{
		{ID: "git", Status: PreflightOK, Source: PreflightSourceLocal},
		{ID: "disk-space", Status: PreflightOK, Source: PreflightSourceRelay},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMergePreflightStatuses_RelayErrProducesConnectivityWarningOnly(t *testing.T) {
	local := []PreflightCheckResult{
		{ID: "git", Status: PreflightOK, Source: PreflightSourceLocal},
	}
	// A non-empty relay slice must still be ignored when relayErr is set —
	// "local checks only" is the only honest answer.
	relay := []PreflightCheckResult{
		{ID: "disk-space", Status: PreflightOK, Source: PreflightSourceRelay},
	}
	got := MergePreflightStatuses(local, relay, errors.New("connection refused"))
	want := []PreflightCheckResult{
		{ID: "git", Status: PreflightOK, Source: PreflightSourceLocal},
		{
			ID: "relay-connectivity", Status: PreflightWarning,
			Message: "Cannot reach Dev Server — showing local checks only",
			Source:  PreflightSourceLocal,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMergePreflightStatuses_OutputOrderStable(t *testing.T) {
	local := []PreflightCheckResult{
		{ID: "b", Status: PreflightOK, Source: PreflightSourceLocal},
		{ID: "a", Status: PreflightOK, Source: PreflightSourceLocal},
	}
	relay := []PreflightCheckResult{
		{ID: "a", Status: PreflightWarning, Source: PreflightSourceRelay}, // overrides, keeps position
		{ID: "z", Status: PreflightOK, Source: PreflightSourceRelay},      // relay-only, appended
	}
	got := MergePreflightStatuses(local, relay, nil)
	wantOrder := []string{"b", "a", "z"}
	if len(got) != len(wantOrder) {
		t.Fatalf("got %d results, want %d: %+v", len(got), len(wantOrder), got)
	}
	for i, id := range wantOrder {
		if got[i].ID != id {
			t.Fatalf("position %d: got id %q, want %q (full: %+v)", i, got[i].ID, id, got)
		}
	}
	if got[1].Status != PreflightWarning {
		t.Fatalf("expected relay override of local at position 1, got %+v", got[1])
	}
}

func TestMergePreflightStatuses_EmptyInputs(t *testing.T) {
	got := MergePreflightStatuses(nil, nil, nil)
	if len(got) != 0 {
		t.Fatalf("expected empty result, got %+v", got)
	}
}
