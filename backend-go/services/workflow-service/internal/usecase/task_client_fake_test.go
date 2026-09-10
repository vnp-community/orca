package usecase

import (
	"context"
	"sync"
)

// fakeTaskClient is an in-memory TaskClient — records every
// ReportTaskExecutionResult call for execute_test.go/recover_executions_test.go's
// Engine 3 completion-callback assertions (TASK-FT-002-05). Safe for
// concurrent use: runToCompletion calls it from a background goroutine.
type fakeTaskClient struct {
	mu    sync.Mutex
	calls []ReportTaskExecutionResultInput
	err   error
}

func (f *fakeTaskClient) ReportTaskExecutionResult(ctx context.Context, in ReportTaskExecutionResultInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, in)
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeTaskClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeTaskClient) lastCall() ReportTaskExecutionResultInput {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[len(f.calls)-1]
}
