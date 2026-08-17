package usecase

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// stepOrderLog records executor invocation events with a stable order,
// safe for concurrent recording from multiple wave-dispatch goroutines.
type stepOrderLog struct {
	mu    sync.Mutex
	order []string
}

func (l *stepOrderLog) record(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.order = append(l.order, event)
}

func (l *stepOrderLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.order))
	copy(out, l.order)
	return out
}

// orderedFakeExecutor is a domain.StepExecutor that records "<name>-start"
// and "<name>-end" into a shared stepOrderLog, and can be made to block
// mid-Execute on a channel the test controls — the "artificial delay, not
// a sleep" mechanism the wave-gate test needs to prove ordering
// deterministically rather than via a timing race.
type orderedFakeExecutor struct {
	name    string
	log     *stepOrderLog
	block   <-chan struct{} // if non-nil, Execute blocks here until closed
	started chan<- struct{} // if non-nil, closed once Execute begins
	result  domain.StepResult
	err     error
}

func (e *orderedFakeExecutor) Execute(_ context.Context, _ string) (domain.StepResult, error) {
	e.log.record(e.name + "-start")
	if e.started != nil {
		close(e.started)
	}
	if e.block != nil {
		<-e.block
	}
	e.log.record(e.name + "-end")
	if e.err != nil {
		return domain.StepResult{}, e.err
	}
	return e.result, nil
}

func TestWaveDispatcher_DispatchWaves_EmptyWavesSucceed(t *testing.T) {
	d := newWaveDispatcher(newFakeStepExecutionRepository(), newFakeRegistry(), 10)
	if !d.dispatchWaves(context.Background(), "exec-1", nil) {
		t.Fatal("expected an empty wave list to trivially succeed")
	}
}

func TestWaveDispatcher_DispatchWaves_AllSucceed(t *testing.T) {
	stepRepo := newFakeStepExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted, OutputJSON: `{"ok":true}`}}
	registry.executors[domain.StepTypeWebhook] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}

	d := newWaveDispatcher(stepRepo, registry, 10)
	waves := [][]domain.Step{
		{{ID: "a", Type: domain.StepTypeShell}, {ID: "b", Type: domain.StepTypeShell}},
		{{ID: "c", Type: domain.StepTypeWebhook}},
	}

	if !d.dispatchWaves(context.Background(), "exec-1", waves) {
		t.Fatal("expected the run to succeed")
	}

	rows := stepRepo.byExecution("exec-1")
	if len(rows) != 3 {
		t.Fatalf("expected 3 persisted step_executions rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.Status != domain.StepExecutionStatusCompleted {
			t.Errorf("expected step %s to be completed, got %v", row.StepID, row.Status)
		}
		if !row.Terminal() {
			t.Errorf("expected step %s to be terminal, got %v", row.StepID, row.Status)
		}
	}
}

func TestWaveDispatcher_DispatchWaves_OneStepFailsAbortsExecution(t *testing.T) {
	stepRepo := newFakeStepExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusFailed, OutputJSON: `{"error":"boom"}`}}
	wave1Executor := &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}
	registry.executors[domain.StepTypeWebhook] = wave1Executor

	d := newWaveDispatcher(stepRepo, registry, 10)
	waves := [][]domain.Step{
		{{ID: "a", Type: domain.StepTypeShell}},
		{{ID: "b", Type: domain.StepTypeWebhook}},
	}

	if d.dispatchWaves(context.Background(), "exec-1", waves) {
		t.Fatal("expected the run to fail")
	}
	if wave1Executor.invocations != 0 {
		t.Errorf("expected wave 1 to never dispatch after wave 0 failed, got %d invocations", wave1Executor.invocations)
	}

	rows := stepRepo.byExecution("exec-1")
	if len(rows) != 1 {
		t.Fatalf("expected only wave 0's step_executions row to have been persisted, got %d", len(rows))
	}
	if rows[0].Status != domain.StepExecutionStatusFailed {
		t.Errorf("expected the failed step's row to be marked failed, got %v", rows[0].Status)
	}
}

func TestWaveDispatcher_DispatchWaves_HardExecutorErrorAlsoAbortsExecution(t *testing.T) {
	stepRepo := newFakeStepExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{err: errors.New("infra-fleet-service unreachable")}

	d := newWaveDispatcher(stepRepo, registry, 10)
	waves := [][]domain.Step{{{ID: "a", Type: domain.StepTypeShell}}}

	if d.dispatchWaves(context.Background(), "exec-1", waves) {
		t.Fatal("expected a hard executor error to fail the run")
	}

	rows := stepRepo.byExecution("exec-1")
	if len(rows) != 1 || rows[0].Status != domain.StepExecutionStatusFailed {
		t.Fatalf("expected the step's row to be marked failed, got %+v", rows)
	}
	if rows[0].Error == "" {
		t.Error("expected the hard error message to be recorded on the step execution")
	}
}

func TestWaveDispatcher_DispatchWaves_UnregisteredStepTypeFailsExecution(t *testing.T) {
	stepRepo := newFakeStepExecutionRepository()
	registry := newFakeRegistry() // nothing registered

	d := newWaveDispatcher(stepRepo, registry, 10)
	waves := [][]domain.Step{{{ID: "a", Type: domain.StepTypeAgent}}}

	if d.dispatchWaves(context.Background(), "exec-1", waves) {
		t.Fatal("expected dispatch to fail when no executor is registered for the step type")
	}
}

// TestWaveDispatcher_WaveGate_Wave1NeverStartsBeforeWave0Terminates proves
// the wave gate is real: wave 1's step must never be dispatched before
// every step in wave 0 has reached a terminal status. wave 0's step
// blocks mid-Execute on a test-controlled channel (an artificial delay,
// not a sleep) until the test has confirmed it actually started, then the
// test releases it and waits for the whole run to finish before inspecting
// the recorded event order — so the assertion is a structural proof about
// what the code did, not a timing guess about what it might have done.
func TestWaveDispatcher_WaveGate_Wave1NeverStartsBeforeWave0Terminates(t *testing.T) {
	stepRepo := newFakeStepExecutionRepository()
	registry := newFakeRegistry()
	log := &stepOrderLog{}

	block := make(chan struct{})
	started := make(chan struct{})
	slow := &orderedFakeExecutor{name: "slow", log: log, block: block, started: started, result: domain.StepResult{Status: domain.ResultStatusCompleted}}
	fast := &orderedFakeExecutor{name: "fast", log: log, result: domain.StepResult{Status: domain.ResultStatusCompleted}}
	registry.executors[domain.StepTypeShell] = slow
	registry.executors[domain.StepTypeWebhook] = fast

	d := newWaveDispatcher(stepRepo, registry, 10)
	waves := [][]domain.Step{
		{{ID: "s0", Type: domain.StepTypeShell}},
		{{ID: "s1", Type: domain.StepTypeWebhook}},
	}

	done := make(chan bool, 1)
	go func() {
		done <- d.dispatchWaves(context.Background(), "exec-1", waves)
	}()

	// Block until wave 0's step has genuinely entered Execute — only then
	// does releasing it prove anything about ordering (if it hadn't
	// started yet, "fast didn't start first" would be trivially true for
	// the wrong reason).
	<-started
	close(block)

	succeeded := <-done
	if !succeeded {
		t.Fatal("expected the run to succeed")
	}

	order := log.snapshot()
	slowEndIdx, fastStartIdx := -1, -1
	for i, e := range order {
		switch e {
		case "slow-end":
			slowEndIdx = i
		case "fast-start":
			if fastStartIdx == -1 {
				fastStartIdx = i
			}
		}
	}
	if slowEndIdx == -1 || fastStartIdx == -1 {
		t.Fatalf("expected both slow-end and fast-start to be recorded, got %v", order)
	}
	if fastStartIdx < slowEndIdx {
		t.Fatalf("wave gate violated: wave 1's step started (event index %d) before wave 0's step finished (event index %d): %v", fastStartIdx, slowEndIdx, order)
	}
}

func TestWaveDispatcher_DispatchWave_BoundsConcurrency(t *testing.T) {
	// A wave with more steps than the configured concurrency cap must
	// still dispatch every step and gate correctly — this isn't a strict
	// "never more than N in flight" proof (that needs instrumentation
	// beyond this fake), but it does prove the pool doesn't deadlock or
	// drop steps when the wave is larger than the pool.
	stepRepo := newFakeStepExecutionRepository()
	registry := newFakeRegistry()
	registry.executors[domain.StepTypeShell] = &fakeStepExecutor{result: domain.StepResult{Status: domain.ResultStatusCompleted}}

	d := newWaveDispatcher(stepRepo, registry, 2)
	var steps []domain.Step
	for i := 0; i < 7; i++ {
		steps = append(steps, domain.Step{ID: string(rune('a' + i)), Type: domain.StepTypeShell})
	}
	waves := [][]domain.Step{steps}

	if !d.dispatchWaves(context.Background(), "exec-1", waves) {
		t.Fatal("expected the run to succeed")
	}
	rows := stepRepo.byExecution("exec-1")
	if len(rows) != 7 {
		t.Fatalf("expected all 7 steps to be dispatched despite the concurrency cap, got %d", len(rows))
	}
}
