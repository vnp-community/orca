package usecase

import (
	"context"

	"github.com/stablyai/orca-go/common/errx"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

type ListExecutionStatesInput struct {
	TaskIDs []string
}

type ListExecutionStates struct {
	reader ExecutionStateReader
}

func NewListExecutionStates(reader ExecutionStateReader) *ListExecutionStates {
	return &ListExecutionStates{reader: reader}
}

func (uc *ListExecutionStates) Execute(ctx context.Context, tenantID string, in ListExecutionStatesInput) ([]domain.ExecutionState, error) {
	if tenantID == "" {
		return nil, errx.NewUnauthenticated("missing tenant_id")
	}

	if len(in.TaskIDs) == 0 {
		return []domain.ExecutionState{}, nil
	}

	uniqueIDs := make([]string, 0, len(in.TaskIDs))
	seen := make(map[string]bool)
	for _, id := range in.TaskIDs {
		if id != "" && !seen[id] {
			seen[id] = true
			uniqueIDs = append(uniqueIDs, id)
		}
	}

	if len(uniqueIDs) == 0 {
		return []domain.ExecutionState{}, nil
	}

	if len(uniqueIDs) > 500 {
		return nil, errx.NewInvalidArgument("TASK_STATES_TOO_MANY_IDS", "requested too many task execution states")
	}

	states, err := uc.reader.ListExecutionStates(ctx, tenantID, uniqueIDs)
	if err != nil {
		return nil, errx.NewInternal("TASK_STATES_FAILED", err.Error())
	}

	stateMap := make(map[string]domain.ExecutionState)
	for _, s := range states {
		stateMap[s.TaskID] = s
	}

	result := make([]domain.ExecutionState, 0, len(in.TaskIDs))
	for _, id := range in.TaskIDs {
		if id == "" {
			continue
		}
		if state, ok := stateMap[id]; ok {
			result = append(result, state)
		} else {
			result = append(result, domain.ExecutionState{
				TaskID:           id,
				BlockedByTaskIDs: nil,
				LastEngine:       "",
				LastLinkStatus:   "",
				FailedAttempts:   0,
			})
		}
	}

	return result, nil
}
