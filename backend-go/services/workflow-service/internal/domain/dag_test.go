package domain

import (
	"errors"
	"testing"
)

func TestParseDAG_EmptyStringIsEmptyDefinition(t *testing.T) {
	d, err := ParseDAG("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(d.Steps) != 0 {
		t.Errorf("expected no steps, got %d", len(d.Steps))
	}
}

func TestParseDAG_InvalidJSON(t *testing.T) {
	_, err := ParseDAG("{not json")
	if err == nil {
		t.Fatal("expected an error for invalid JSON")
	}
}

func TestDAGDefinition_Validate(t *testing.T) {
	tests := []struct {
		name    string
		dagJSON string
		wantErr error
	}{
		{
			name:    "valid chain",
			dagJSON: `{"steps":[{"id":"a","type":"shell"},{"id":"b","type":"shell","dependsOn":["a"]}]}`,
			wantErr: nil,
		},
		{
			name:    "empty step id",
			dagJSON: `{"steps":[{"id":"","type":"shell"}]}`,
			wantErr: ErrEmptyStepID,
		},
		{
			name:    "duplicate step id",
			dagJSON: `{"steps":[{"id":"a","type":"shell"},{"id":"a","type":"shell"}]}`,
			wantErr: ErrDuplicateStepID,
		},
		{
			name:    "self-referencing step",
			dagJSON: `{"steps":[{"id":"a","type":"shell","dependsOn":["a"]}]}`,
			wantErr: ErrSelfReferencingStep,
		},
		{
			name:    "unknown dependency",
			dagJSON: `{"steps":[{"id":"a","type":"shell","dependsOn":["ghost"]}]}`,
			wantErr: ErrStepDependencyNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := ParseDAG(tt.dagJSON)
			if err != nil {
				t.Fatalf("unexpected parse error: %v", err)
			}
			err = d.Validate()
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error wrapping %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func stepIDs(steps []Step) []string {
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.ID
	}
	return ids
}

func TestDAGDefinition_BuildWaves_EmptyDefinition(t *testing.T) {
	d := DAGDefinition{}
	waves, err := d.BuildWaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 0 {
		t.Fatalf("expected no waves for an empty definition, got %v", waves)
	}
}

func TestDAGDefinition_BuildWaves_AllIndependentStepsLandInWaveZero(t *testing.T) {
	d, err := ParseDAG(`{"steps":[{"id":"a","type":"shell"},{"id":"b","type":"shell"},{"id":"c","type":"shell"}]}`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	waves, err := d.BuildWaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 1 {
		t.Fatalf("expected exactly 1 wave, got %d: %v", len(waves), waves)
	}
	if got := stepIDs(waves[0]); len(got) != 3 {
		t.Fatalf("expected all 3 independent steps in wave 0, got %v", got)
	}
}

func TestDAGDefinition_BuildWaves_LinearChain(t *testing.T) {
	d, err := ParseDAG(`{"steps":[
		{"id":"a","type":"shell"},
		{"id":"b","type":"shell","dependsOn":["a"]},
		{"id":"c","type":"shell","dependsOn":["b"]}
	]}`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	waves, err := d.BuildWaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves for a linear chain, got %d: %v", len(waves), waves)
	}
	for i, want := range []string{"a", "b", "c"} {
		if got := stepIDs(waves[i]); len(got) != 1 || got[0] != want {
			t.Fatalf("wave %d: expected [%s], got %v", i, want, got)
		}
	}
}

func TestDAGDefinition_BuildWaves_DiamondDependency(t *testing.T) {
	// a -> {b, c} -> d: b and c both depend only on a, and nothing depends
	// on either of them until d — they must land in the same wave.
	d, err := ParseDAG(`{"steps":[
		{"id":"a","type":"shell"},
		{"id":"b","type":"shell","dependsOn":["a"]},
		{"id":"c","type":"shell","dependsOn":["a"]},
		{"id":"d","type":"shell","dependsOn":["b","c"]}
	]}`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	waves, err := d.BuildWaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waves) != 3 {
		t.Fatalf("expected 3 waves for a diamond dependency, got %d: %v", len(waves), waves)
	}
	if got := stepIDs(waves[0]); len(got) != 1 || got[0] != "a" {
		t.Fatalf("wave 0: expected [a], got %v", got)
	}
	wave1 := stepIDs(waves[1])
	if len(wave1) != 2 || !((wave1[0] == "b" && wave1[1] == "c") || (wave1[0] == "c" && wave1[1] == "b")) {
		t.Fatalf("wave 1: expected [b c] (order-preserving), got %v", wave1)
	}
	if got := stepIDs(waves[2]); len(got) != 1 || got[0] != "d" {
		t.Fatalf("wave 2: expected [d], got %v", got)
	}
}

func TestDAGDefinition_BuildWaves_ThreeNodeCycleDetected(t *testing.T) {
	// a -> b -> c -> a: every edge resolves to a real, distinct step, so
	// Validate (pairwise self-reference/unknown-dependency checks only)
	// does not catch this — only BuildWaves' full pass does.
	d, err := ParseDAG(`{"steps":[
		{"id":"a","type":"shell","dependsOn":["c"]},
		{"id":"b","type":"shell","dependsOn":["a"]},
		{"id":"c","type":"shell","dependsOn":["b"]}
	]}`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("expected Validate to pass (pairwise checks can't see this cycle), got %v", err)
	}

	_, err = d.BuildWaves()
	if err == nil {
		t.Fatal("expected BuildWaves to detect the 3-node cycle")
	}
	if !errors.Is(err, ErrCyclicDependency) {
		t.Fatalf("expected error wrapping ErrCyclicDependency, got %v", err)
	}
}

func TestDAGDefinition_BuildWaves_PreservesOriginalOrderWithinAWave(t *testing.T) {
	d, err := ParseDAG(`{"steps":[{"id":"z","type":"shell"},{"id":"a","type":"shell"},{"id":"m","type":"shell"}]}`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	waves, err := d.BuildWaves()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := stepIDs(waves[0]); len(got) != 3 || got[0] != "z" || got[1] != "a" || got[2] != "m" {
		t.Fatalf("expected wave 0 to preserve definition order [z a m], got %v", got)
	}
}
