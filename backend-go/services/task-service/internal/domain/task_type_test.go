package domain

import (
	"errors"
	"testing"
)

func TestParseTaskType(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", TypeTask, false},
		{"task", TypeTask, false},
		{"bug", TypeBug, false},
		{"feature", TypeFeature, false},
		{"epic", TypeEpic, false},
		{"plan", TypePlan, false},
		{"phase", TypePhase, false},
		{"xyz", "", true},
		{"Plan", "", true},
	}
	for _, c := range cases {
		got, err := ParseTaskType(c.in)
		if c.wantErr {
			if !errors.Is(err, ErrInvalidTaskType) {
				t.Errorf("ParseTaskType(%q): want ErrInvalidTaskType, got %v", c.in, err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("ParseTaskType(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestIsContainerType(t *testing.T) {
	for _, s := range []string{TypePlan, TypePhase} {
		if !IsContainerType(s) {
			t.Errorf("%q should be a container", s)
		}
	}
	// epic must stay a numbered work task: treating it as a container dropped its task_number.
	for _, s := range []string{"", TypeTask, TypeBug, TypeFeature, TypeEpic} {
		if IsContainerType(s) {
			t.Errorf("%q should not be a container", s)
		}
	}
}

func TestValidateHierarchy(t *testing.T) {
	plan := &Task{ID: "p", Type: TypePlan, ProjectID: "proj"}
	phase := &Task{ID: "ph", Type: TypePhase, ProjectID: "proj"}
	work := &Task{ID: "w", Type: TypeTask, ProjectID: "proj"}
	cases := []struct {
		name      string
		typ, proj string
		parent    *Task
		want      error
	}{
		{"plan root ok", TypePlan, "proj", nil, nil},
		{"plan needs project", TypePlan, "", nil, ErrPlanProjectRequired},
		{"plan with parent", TypePlan, "proj", plan, ErrPlanCannotHaveParent},
		{"phase under plan ok", TypePhase, "proj", plan, nil},
		{"phase other project", TypePhase, "other", plan, ErrPhaseRequiresPlanParent},
		{"phase no parent", TypePhase, "proj", nil, ErrPhaseRequiresPlanParent},
		{"phase under phase", TypePhase, "proj", phase, ErrPhaseRequiresPlanParent},
		{"task under anything ok", TypeTask, "proj", work, nil},
	}
	for _, c := range cases {
		if got := ValidateHierarchy(c.typ, c.proj, c.parent); !errors.Is(got, c.want) && got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
	if err := ValidateContainerParent(TypePhase, work); !errors.Is(err, ErrContainerUnderWorkTask) {
		t.Errorf("phase under work task: got %v", err)
	}
	if err := ValidateContainerParent(TypeTask, work); err != nil {
		t.Errorf("task under work task: got %v", err)
	}
}

func TestValidatePriorityAndVisibility(t *testing.T) {
	for _, p := range []string{"", "low", "medium", "high", "urgent"} {
		if err := ValidatePriority(p); err != nil {
			t.Errorf("priority %q: %v", p, err)
		}
	}
	if !errors.Is(ValidatePriority("critical"), ErrInvalidPriority) {
		t.Error("expected ErrInvalidPriority")
	}
	for _, v := range []string{"", "private", "team", "public"} {
		if err := ValidateVisibility(v); err != nil {
			t.Errorf("visibility %q: %v", v, err)
		}
	}
	if !errors.Is(ValidateVisibility("secret"), ErrInvalidVisibility) {
		t.Error("expected ErrInvalidVisibility")
	}
}
