package usecase

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/errx"
	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)


func TestListExecutionStates_EmptyInput(t *testing.T) {
	uc := NewListExecutionStates(&FakeExecutionStateReader{})
	res, err := uc.Execute(context.Background(), "t1", ListExecutionStatesInput{TaskIDs: []string{}})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty result, got %v", res)
	}
}

func TestListExecutionStates_NoTenant_Unauthenticated(t *testing.T) {
	uc := NewListExecutionStates(&FakeExecutionStateReader{})
	_, err := uc.Execute(context.Background(), "", ListExecutionStatesInput{TaskIDs: []string{"task-1"}})
	if !errx.IsUnauthenticated(err) {
		t.Fatalf("expected Unauthenticated error, got %v", err)
	}
}

func TestListExecutionStates_TooManyIDs(t *testing.T) {
	uc := NewListExecutionStates(&FakeExecutionStateReader{})
	var ids []string
	for i := 0; i < 501; i++ {
		ids = append(ids, fmt.Sprintf("task-%d", i))
	}
	_, err := uc.Execute(context.Background(), "t1", ListExecutionStatesInput{TaskIDs: ids})
	if !errx.IsInvalidArgument(err) || err.(*errx.Error).Code != "TASK_STATES_TOO_MANY_IDS" {
		t.Fatalf("expected TASK_STATES_TOO_MANY_IDS, got %v", err)
	}
}

func TestListExecutionStates_DedupesAndKeepsOrder(t *testing.T) {
	reader := &FakeExecutionStateReader{
		MockListExecutionStates: func(ctx context.Context, tenantID string, taskIDs []string) ([]domain.ExecutionState, error) {
			if len(taskIDs) != 2 {
				t.Errorf("expected 2 unique ids, got %v", taskIDs)
			}
			return []domain.ExecutionState{
				{TaskID: "task-1", LastLinkStatus: "completed"},
				{TaskID: "task-2", LastLinkStatus: "failed"},
			}, nil
		},
	}
	uc := NewListExecutionStates(reader)
	
	in := ListExecutionStatesInput{TaskIDs: []string{"task-1", "task-2", "task-1", "task-2"}}
	res, err := uc.Execute(context.Background(), "t1", in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 4 {
		t.Fatalf("expected 4 results mirroring input order, got %d", len(res))
	}
	if res[0].TaskID != "task-1" || res[1].TaskID != "task-2" || res[2].TaskID != "task-1" || res[3].TaskID != "task-2" {
		t.Errorf("unexpected order: %v", res)
	}
}

func TestListExecutionStates_FillsMissingTasks(t *testing.T) {
	now := time.Now()
	reader := &FakeExecutionStateReader{
		MockListExecutionStates: func(ctx context.Context, tenantID string, taskIDs []string) ([]domain.ExecutionState, error) {
			return []domain.ExecutionState{
				{TaskID: "task-1", LastLinkStatus: "completed", LastStartedAt: now},
			}, nil
		},
	}
	uc := NewListExecutionStates(reader)
	
	in := ListExecutionStatesInput{TaskIDs: []string{"task-1", "task-missing"}}
	res, err := uc.Execute(context.Background(), "t1", in)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 results, got %d", len(res))
	}
	
	want := []domain.ExecutionState{
		{TaskID: "task-1", LastLinkStatus: "completed", LastStartedAt: now},
		{TaskID: "task-missing", LastLinkStatus: "", FailedAttempts: 0},
	}
	if !reflect.DeepEqual(res, want) {
		t.Errorf("expected %v, got %v", want, res)
	}
}
