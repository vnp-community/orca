package main

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stablyai/orca-go/services/request-service/internal/config"
	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"github.com/stablyai/orca-go/services/request-service/internal/usecase"
)

type sweepCounter struct {
	usecase.AnalysisRunStore
	mu    sync.Mutex
	sweep int
}

func (s *sweepCounter) ClaimExpired(context.Context, string, time.Duration, int) ([]domain.AnalysisRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sweep++
	return nil, nil
}

func (s *sweepCounter) sweeps() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sweep
}

func TestWireSolution_RegistersRealHandlersAndStartsTheSweeper(t *testing.T) {
	runs := &sweepCounter{}
	stores := &requestStores{analysisRuns: runs}
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.Config{AnalysisRecoveryInterval: 20 * time.Millisecond, AgentReadonlyUseAgentFlag: true}
	life := &requestLifecycle{Return: &usecase.ReturnRequestToBacklog{}}
	w, err := wireSolution(ctx, &wg, cfg, slog.Default(), stores, life, &approvalDirectories{})
	if err != nil {
		t.Fatal(err)
	}
	defer w.close()

	for _, st := range []domain.SubjectType{domain.SubjectSolution, domain.SubjectFindings, domain.SubjectAnswer} {
		if h := w.handlers[st]; h == nil {
			t.Fatalf("no handler for %s", st)
		} else if _, noop := h.(*usecase.NoopSubjectHandler); noop {
			t.Fatalf("%s has a noop handler", st)
		}
	}
	uc := w.useCases()
	if uc.Generate == nil || uc.List == nil || uc.Choose == nil {
		t.Fatalf("use cases not wired: %+v", uc)
	}

	// Approval on, only these three subjects: the registry accepts the real handlers without allowing noop ones.
	reg, err := buildApprovalRegistry(config.Config{ApprovalEnabled: true, ApprovalSubjects: []string{"solution", "findings", "answer"}}, slog.Default(), w.handlers)
	if err != nil || reg == nil {
		t.Fatalf("registry: %v %v", reg, err)
	}
	w.bindApprovals(nil)

	deadline := time.Now().Add(2 * time.Second)
	for runs.sweeps() < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if runs.sweeps() < 2 {
		t.Fatalf("recovery sweeper ran %d times", runs.sweeps())
	}
	cancel()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the sweeper did not stop with its context")
	}
}

func TestAnalysisSettings_MapsConfigAndKeepsDefaults(t *testing.T) {
	d := usecase.DefaultAnalysisSettings()
	got := analysisSettings(config.Config{AgentReadonlyUseAgentFlag: true})
	if got != d {
		t.Fatalf("empty config must give the defaults:\n got %+v\nwant %+v", got, d)
	}
	got = analysisSettings(config.Config{
		AICompleteTimeout: time.Second, AnalysisLeaseTTL: 2 * time.Second, AnalysisHeartbeat: 3 * time.Second, AnalysisRecoveryInterval: 4 * time.Second,
		AgentReadonlyTimeoutMS: 5, AgentReadonlyMaxPerProject: 6, RequireEnforcedReadonly: true,
	})
	want := usecase.AnalysisSettings{AICompleteTimeout: time.Second, LeaseTTL: 2 * time.Second, Heartbeat: 3 * time.Second, RecoveryInterval: 4 * time.Second,
		AgentTimeoutMS: 5, HotfixTimeoutMS: d.HotfixTimeoutMS, MaxAgentRunsPerProject: 6, UseAgentReadonlyFlag: false, RequireEnforcedReadonly: true}
	if got != want {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestLateApprovalOpener(t *testing.T) {
	opener := &lateApprovalOpener{log: slog.Default()}
	if err := opener.Open(context.Background(), usecase.OpenSolutionApprovalInput{}); err != nil {
		t.Fatalf("unbound opener only warns: %v", err)
	}
	got := &recordingOpener{}
	w := &solutionWiring{opener: opener}
	w.bindApprovals(nil) // approval off: stays unbound
	w.bindApprovals(got)
	_ = opener.Open(context.Background(), usecase.OpenSolutionApprovalInput{SubjectID: "s1"})
	if len(got.calls) != 1 || got.calls[0].SubjectID != "s1" {
		t.Fatalf("opener calls = %+v", got.calls)
	}
}

type recordingOpener struct {
	calls []usecase.OpenSolutionApprovalInput
}

func (r *recordingOpener) Open(_ context.Context, in usecase.OpenSolutionApprovalInput) error {
	r.calls = append(r.calls, in)
	return nil
}
