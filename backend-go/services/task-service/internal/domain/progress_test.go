package domain

import "testing"

func TestCalculateProgress_LeafDone(t *testing.T) {
	task := Task{Status: StatusDone}
	if got := CalculateProgress(task, nil); got != 100 {
		t.Errorf("expected 100, got %d", got)
	}
}

func TestCalculateProgress_LeafNotDone(t *testing.T) {
	task := Task{Status: StatusOpen}
	if got := CalculateProgress(task, nil); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestCalculateProgress_UniformChildrenAverage(t *testing.T) {
	task := Task{Status: StatusOpen}
	if got := CalculateProgress(task, []int{100, 100, 0}); got != 66 {
		t.Errorf("expected 66, got %d", got)
	}
}

func TestCalculateProgress_MixedDepthCascade(t *testing.T) {
	// Leaf children: one done (100), one not (0) -> parent = 50.
	leafDone := Task{Status: StatusDone}
	leafNotDone := Task{Status: StatusOpen}
	leafDonePercent := CalculateProgress(leafDone, nil)
	leafNotDonePercent := CalculateProgress(leafNotDone, nil)

	parent := Task{Status: StatusOpen}
	parentPercent := CalculateProgress(parent, []int{leafDonePercent, leafNotDonePercent})
	if parentPercent != 50 {
		t.Errorf("expected parent percent 50, got %d", parentPercent)
	}

	// Grandparent whose only child is parent (50) -> grandparent = 50,
	// even though grandparent's own status is not Done.
	grandparent := Task{Status: StatusOpen}
	grandparentPercent := CalculateProgress(grandparent, []int{parentPercent})
	if grandparentPercent != 50 {
		t.Errorf("expected grandparent percent 50, got %d", grandparentPercent)
	}
}
