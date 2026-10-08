package usecase

import (
	"context"
	"fmt"
	"testing"

	"github.com/stablyai/orca-go/services/task-service/internal/domain"
)

func seedListTasks(repo *fakeTaskRepository) {
	for id, typ := range map[string]string{"a": "task", "b": "bug", "c": "plan", "d": "phase", "e": ""} {
		repo.tasks[id] = domain.Task{ID: id, TenantID: "tenant-1", Title: id, Type: typ, RequestID: map[bool]string{true: "req-1", false: ""}[id == "c" || id == "d"]}
	}
	d := repo.tasks["d"]
	d.ParentID = "c"
	repo.tasks["d"] = d
}

func TestListTasks_DefaultHidesPlanPhase(t *testing.T) {
	repo := newFakeTaskRepository()
	seedListTasks(repo)
	res, err := NewListTasks(repo).Execute(withIdentity(context.Background(), "tenant-1", "u"), ListTasksInput{})
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range res.Tasks {
		if task.Type == "plan" || task.Type == "phase" {
			t.Errorf("default listing leaked %s", task.Type)
		}
	}
	if len(res.Tasks) != 3 {
		t.Errorf("want task, bug and untyped task: got %d", len(res.Tasks))
	}
}

func TestListTasks_PlanByRequestID(t *testing.T) {
	repo := newFakeTaskRepository()
	seedListTasks(repo)
	res, err := NewListTasks(repo).Execute(withIdentity(context.Background(), "tenant-1", "u"),
		ListTasksInput{TaskTypes: []string{"plan"}, RequestIDs: []string{"req-1"}})
	if err != nil || len(res.Tasks) != 1 || res.Tasks[0].ID != "c" {
		t.Fatalf("got %+v, %v", res.Tasks, err)
	}
	res, err = NewListTasks(repo).Execute(withIdentity(context.Background(), "tenant-1", "u"), ListTasksInput{TaskTypes: []string{"phase"}, ParentID: "c"})
	if err != nil || len(res.Tasks) != 1 || res.Tasks[0].ID != "d" {
		t.Fatalf("parent filter: got %+v, %v", res.Tasks, err)
	}
}

func TestListTasks_TooManyRequestIDs(t *testing.T) {
	ids := make([]string, 101)
	for i := range ids {
		ids[i] = fmt.Sprintf("r%d", i)
	}
	_, err := NewListTasks(newFakeTaskRepository()).Execute(withIdentity(context.Background(), "tenant-1", "u"), ListTasksInput{RequestIDs: ids})
	if !hasCode(err, "TASK_LIST_TOO_MANY_REQUEST_IDS") {
		t.Fatalf("got %v", err)
	}
	if _, err := NewListTasks(newFakeTaskRepository()).Execute(withIdentity(context.Background(), "tenant-1", "u"), ListTasksInput{RequestIDs: ids[:100]}); err != nil {
		t.Fatalf("100 ids must be accepted: %v", err)
	}
}

func TestListTasks_InvalidType(t *testing.T) {
	_, err := NewListTasks(newFakeTaskRepository()).Execute(withIdentity(context.Background(), "tenant-1", "u"), ListTasksInput{TaskTypes: []string{"xyz"}})
	if !hasCode(err, "TASK_INVALID_TYPE") {
		t.Fatalf("got %v", err)
	}
}
