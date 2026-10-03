package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"
	tt "github.com/stablyai/orca-go/services/mcp-service/internal/usecase/usecasetest"
)

// The tenant kill switch suspends PATs on the way in and restores them on the
// way out; the restore needs its own cleanup pass because switching off leaves
// nothing else pending.
func TestKillSwitchSuspendsAndRestoresPATs(t *testing.T) {
	h := tt.NewHarness(true)
	adm := tt.Ctx(tenantA, userAdm, "admin")
	run := func() int {
		n, err := h.Cleanup.Execute(context.Background(), 10)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	if err := h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "incident 7", Active: true}); err != nil {
		t.Fatal(err)
	}
	if run() != 1 || !h.Pats.Suspended[tenantA] {
		t.Fatalf("PATs of the tenant must be suspended: %+v", h.Pats.Suspended)
	}
	if run() != 0 {
		t.Fatal("cleanup must be done once")
	}
	if err := h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "all clear", Active: false}); err != nil {
		t.Fatal(err)
	}
	if run() != 1 || h.Pats.Suspended[tenantA] {
		t.Fatalf("PATs must work again once the kill switch is off: %+v", h.Pats.Suspended)
	}
	if run() != 0 {
		t.Fatal("restore must be done once")
	}
}

func TestKillSwitchPATSuspensionRetriesAndIgnoresOtherScopes(t *testing.T) {
	h := tt.NewHarness(true)
	adm := tt.Ctx(tenantA, userAdm, "admin")
	h.Pats.Err = errors.New("auth-service unavailable")
	if err := h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "tenant", Reason: "incident 8", Active: true}); err != nil {
		t.Fatal(err)
	}
	if n, _ := h.Cleanup.Execute(context.Background(), 10); n != 0 {
		t.Fatal("a failed suspension must leave the cleanup pending")
	}
	h.Pats.Err = nil
	if n, _ := h.Cleanup.Execute(context.Background(), 10); n != 1 || !h.Pats.Suspended[tenantA] {
		t.Fatalf("retry must suspend: n=%d %+v", n, h.Pats.Suspended)
	}
	before := h.Pats.Calls
	if err := h.KillAdmin.Set(adm, usecase.SetKillSwitchInput{Scope: "session", TargetID: "s-1", Reason: "one session", Active: true}); err != nil {
		t.Fatal(err)
	}
	_, _ = h.Cleanup.Execute(context.Background(), 10)
	if h.Pats.Calls != before {
		t.Fatal("only the tenant scope touches PATs")
	}
}
