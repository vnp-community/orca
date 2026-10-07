package usecase

import "context"

type TasksMdSyncer struct {
	// dependencies
}

func NewTasksMdSyncer() *TasksMdSyncer {
	return &TasksMdSyncer{}
}

func (s *TasksMdSyncer) OnTaskSucceeded(ctx context.Context, requestID, taskID string) error {
	// update tasks_sync_state to pending
	return nil
}

func (s *TasksMdSyncer) Flush(ctx context.Context, requestID string) error {
	return nil
}

func (s *TasksMdSyncer) RunSyncLoop(ctx context.Context) {
	// loop over ListPendingSync and Flush
}
