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
