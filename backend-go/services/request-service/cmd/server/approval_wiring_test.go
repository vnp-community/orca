package main

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

func TestBuildApprovalRegistry_DisabledByDefault(t *testing.T) {
	reg, err := buildApprovalRegistry(config.Config{}, slog.Default(), nil)
	if err != nil || reg != nil {
		t.Fatalf("default config must start without approval: reg=%v err=%v", reg, err)
	}
}

func TestBuildApprovalRegistry_EnabledWithoutRealHandlerFails(t *testing.T) {
	_, err := buildApprovalRegistry(config.Config{ApprovalEnabled: true}, slog.Default(), nil)
	if err == nil || !strings.Contains(err.Error(), "no real handler") {
		t.Fatalf("want missing-handler error, got %v", err)
	}
}

func TestBuildApprovalRegistry_OnlyRequestedSubjectsNeedHandlers(t *testing.T) {
	real := map[domain.SubjectType]usecase.SubjectHandler{domain.SubjectSolution: &usecase.NoopSubjectHandler{}}
	cfg := config.Config{ApprovalEnabled: true, ApprovalSubjects: []string{"solution"}}
	reg, err := buildApprovalRegistry(cfg, slog.Default(), real)
	if err != nil || reg == nil || reg.Get(domain.SubjectSolution) == nil {
		t.Fatalf("subset with handler must pass: reg=%v err=%v", reg, err)
	}
	if reg.Get(domain.SubjectPlan) != nil {
		t.Fatalf("subjects outside the subset must not be registered")
	}
}

func TestBuildApprovalRegistry_DevNoopOptIn(t *testing.T) {
	cfg := config.Config{ApprovalEnabled: true, AllowNoopApprovalHandlers: true}
	reg, err := buildApprovalRegistry(cfg, slog.Default(), nil)
	if err != nil {
		t.Fatalf("noop opt-in: %v", err)
	}
	if err := reg.MustCoverAll(); err != nil {
		t.Fatalf("all subjects should be covered: %v", err)
	}
}

func TestBuildApprovalRegistry_UnknownSubject(t *testing.T) {
	cfg := config.Config{ApprovalEnabled: true, ApprovalSubjects: []string{"bogus"}}
	if _, err := buildApprovalRegistry(cfg, slog.Default(), nil); err == nil {
		t.Fatalf("unknown subject must fail")
	}
}

type wiredProbe struct {
	Repo  fmt.Stringer
	Tx    func()
	Log   *slog.Logger
	Extra map[string]int
	note  *int // unexported: ignored
}

func TestRequireWired_ReportsFirstNilDependencyAndHonoursOptionalFields(t *testing.T) {
	ok := &wiredProbe{Repo: stringerStub{}, Tx: func() {}, Extra: map[string]int{}}
	if err := requireWired("probe", ok, "Log"); err != nil {
		t.Fatalf("optional Log may be nil: %v", err)
	}
	if err := requireWired("probe", ok); err == nil || !strings.Contains(err.Error(), "probe.Log is nil") {
		t.Fatalf("Log is required by default: %v", err)
	}
	bad := &wiredProbe{Tx: func() {}, Log: slog.Default(), Extra: map[string]int{}}
	if err := requireWired("probe", bad); err == nil || !strings.Contains(err.Error(), "probe.Repo is nil") {
		t.Fatalf("nil interface must be reported: %v", err)
	}
	if err := requireWired("probe", wiredProbe{}); err == nil {
		t.Fatal("a non-pointer must be refused")
	}
	if err := requireWired("probe", (*wiredProbe)(nil)); err == nil {
		t.Fatal("a nil pointer must be refused")
	}
}

type stringerStub struct{}

func (stringerStub) String() string { return "" }

func TestFillApprovalRegistry_RealHandlersCoverAllSubjectsAndNoopStaysDevOnly(t *testing.T) {
	real := map[domain.SubjectType]usecase.SubjectHandler{}
	for _, st := range domain.AllSubjectTypes {
		real[st] = &usecase.TransitionSubjectHandler{}
	}
	reg := usecase.NewSubjectHandlerRegistry()
	if err := fillApprovalRegistry(config.Config{ApprovalEnabled: true}, slog.Default(), reg, real); err != nil {
		t.Fatalf("real handlers for every subject: %v", err)
	}
	if err := reg.MustCoverAll(); err != nil {
		t.Fatal(err)
	}
	delete(real, domain.SubjectPhase)
	err := fillApprovalRegistry(config.Config{ApprovalEnabled: true}, slog.Default(), usecase.NewSubjectHandlerRegistry(), real)
	if err == nil || !strings.Contains(err.Error(), "phase") {
		t.Fatalf("a missing handler must refuse to start unless the dev flag is set: %v", err)
	}
}
