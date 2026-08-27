package domain

import "testing"

func TestFileState_Valid(t *testing.T) {
	tests := []struct {
		state FileState
		want  bool
	}{
		{FileStateModified, true},
		{FileStateAdded, true},
		{FileStateDeleted, true},
		{FileStateUntracked, true},
		{FileStateConflicted, true},
		{FileStateRenamed, true},
		{FileState(""), false},
		{FileState("bogus"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.Valid(); got != tt.want {
				t.Errorf("FileState(%q).Valid() = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}
