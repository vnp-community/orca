package domain

import (
	"errors"
	"testing"
)

func TestExpandSpec_SingleNodeNoDeps(t *testing.T) {
	specJSON := []byte(`[{"tempId":"a","title":"Root task","spec":{"connectionId":"c1"},"deps":[]}]`)
	tasks, err := ExpandSpec("tenant-1", "run-1", "origin-1", specJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].Status != TaskStatusReady {
		t.Errorf("expected TaskStatusReady for a no-dep node, got %s", tasks[0].Status)
	}
	if tasks[0].OriginTaskID != "origin-1" {
		t.Errorf("expected root node OriginTaskID to be set, got %q", tasks[0].OriginTaskID)
	}
	if tasks[0].CoordinatorRunID != "run-1" || tasks[0].TenantID != "tenant-1" {
		t.Errorf("expected scoping fields to be set, got %+v", tasks[0])
	}
}

func TestExpandSpec_DepChain(t *testing.T) {
	specJSON := []byte(`[
		{"tempId":"a","title":"Root","spec":{},"deps":[]},
		{"tempId":"b","title":"Child","spec":{},"deps":["a"]}
	]`)
	tasks, err := ExpandSpec("tenant-1", "run-1", "origin-1", specJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(tasks))
	}
	if tasks[0].Status != TaskStatusReady {
		t.Errorf("expected root (no deps) to be ready, got %s", tasks[0].Status)
	}
	if tasks[1].Status != TaskStatusPending {
		t.Errorf("expected child (has deps) to be pending, got %s", tasks[1].Status)
	}
	if tasks[1].OriginTaskID != "" {
		t.Errorf("expected non-root node OriginTaskID to be empty, got %q", tasks[1].OriginTaskID)
	}
}

func TestExpandSpec_DuplicateTempID(t *testing.T) {
	specJSON := []byte(`[
		{"tempId":"a","title":"One","spec":{},"deps":[]},
		{"tempId":"a","title":"Two","spec":{},"deps":[]}
	]`)
	if _, err := ExpandSpec("tenant-1", "run-1", "origin-1", specJSON); !errors.Is(err, ErrDuplicateTempID) {
		t.Fatalf("expected ErrDuplicateTempID, got %v", err)
	}
}

func TestExpandSpec_DanglingDep(t *testing.T) {
	specJSON := []byte(`[{"tempId":"a","title":"One","spec":{},"deps":["missing"]}]`)
	if _, err := ExpandSpec("tenant-1", "run-1", "origin-1", specJSON); !errors.Is(err, ErrDanglingDep) {
		t.Fatalf("expected ErrDanglingDep, got %v", err)
	}
}

func TestExpandSpec_EmptyNodeArray(t *testing.T) {
	if _, err := ExpandSpec("tenant-1", "run-1", "origin-1", []byte(`[]`)); !errors.Is(err, ErrEmptySpec) {
		t.Fatalf("expected ErrEmptySpec, got %v", err)
	}
}

func TestExpandSpec_MalformedJSON(t *testing.T) {
	_, err := ExpandSpec("tenant-1", "run-1", "origin-1", []byte(`not json`))
	if err == nil {
		t.Fatal("expected an error for malformed spec_json, got nil")
	}
	if errors.Is(err, ErrEmptySpec) || errors.Is(err, ErrDuplicateTempID) || errors.Is(err, ErrDanglingDep) {
		t.Fatalf("expected a wrapped JSON error, not a sentinel, got %v", err)
	}
}
