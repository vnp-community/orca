package main

import (
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
)

// A nil *ProjectAccessLookup stored in an interface is a non-nil interface: calling it panics inside the RPC handler.
func TestProjectAccess_ExistenceIsNilWhenTheLookupIsOff(t *testing.T) {
	if got := (&projectAccess{}).existence(); got != nil {
		t.Fatalf("existence = %#v, want a true nil interface", got)
	}
	var none *projectAccess
	if got := none.existence(); got != nil {
		t.Fatalf("existence of no connection = %#v", got)
	}
	if (&projectAccess{Lookup: grpcclient.NewProjectAccessLookup(nil)}).existence() == nil {
		t.Fatal("a configured lookup must be used")
	}
}
