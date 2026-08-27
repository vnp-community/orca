package usecase

import (
	"context"
	"sort"
	"sync"

	"github.com/stablyai/orca-go/services/workflow-service/internal/domain"
)

// fakeStepExecutionRepository is an in-memory StepExecutionRepository —
// safe for concurrent use since waveDispatcher updates step_executions
// rows from multiple goroutines within a wave, which is exactly the
// behavior these tests need to exercise.
type fakeStepExecutionRepository struct {
	mu        sync.Mutex
	rows      map[string]domain.StepExecution
	createErr error
	updateErr error
}

func newFakeStepExecutionRepository() *fakeStepExecutionRepository {
	return &fakeStepExecutionRepository{rows: make(map[string]domain.StepExecution)}
}

func (f *fakeStepExecutionRepository) CreateStepExecution(ctx context.Context, se domain.StepExecution) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createErr != nil {
		return f.createErr
	}
	f.rows[se.ID] = se
	return nil
}

func (f *fakeStepExecutionRepository) UpdateStepExecution(ctx context.Context, se domain.StepExecution) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updateErr != nil {
		return f.updateErr
	}
	f.rows[se.ID] = se
	return nil
}

func (f *fakeStepExecutionRepository) ListStepExecutions(ctx context.Context, tenantID, executionID string) ([]domain.StepExecution, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.StepExecution
	for _, se := range f.rows {
		if se.ExecutionID == executionID {
			out = append(out, se)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Wave != out[j].Wave {
			return out[i].Wave < out[j].Wave
		}
		return out[i].StepID < out[j].StepID
	})
	return out, nil
}

// byExecution is a test helper mirroring ListStepExecutions without the
// tenant/error plumbing, for assertions that don't care about those.
func (f *fakeStepExecutionRepository) byExecution(executionID string) []domain.StepExecution {
	rows, _ := f.ListStepExecutions(context.Background(), "", executionID)
	return rows
}
