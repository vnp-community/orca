package domain

import "testing"

func intPtr(v int) *int { return &v }

func TestOsc133Scanner_SingleChunk_BELTerminated(t *testing.T) {
	s := &Osc133Scanner{}
	markers := s.Feed("\x1b]133;C\x07hello\x1b]133;D;0\x07")
	if len(markers) != 2 {
		t.Fatalf("expected 2 markers, got %d: %+v", len(markers), markers)
	}
	if markers[0].Kind != "C" {
		t.Errorf("expected first marker Kind=C, got %q", markers[0].Kind)
	}
	if markers[1].Kind != "D" || markers[1].ExitCode == nil || *markers[1].ExitCode != 0 {
		t.Errorf("expected second marker Kind=D ExitCode=0, got %+v", markers[1])
	}
}

func TestOsc133Scanner_SingleChunk_STTerminated(t *testing.T) {
	s := &Osc133Scanner{}
	markers := s.Feed("\x1b]133;C\x1b\\output\x1b]133;D;1\x1b\\")
	if len(markers) != 2 {
		t.Fatalf("expected 2 markers, got %d: %+v", len(markers), markers)
	}
	if markers[1].ExitCode == nil || *markers[1].ExitCode != 1 {
		t.Errorf("expected ExitCode=1, got %+v", markers[1].ExitCode)
	}
}

func TestOsc133Scanner_PrefixSplitAcrossChunks(t *testing.T) {
	s := &Osc133Scanner{}
	full := "\x1b]133;C\x07"
	split := len(full) - 3 // split mid-prefix-ish
	markers := s.Feed(full[:split])
	if len(markers) != 0 {
		t.Fatalf("expected no markers from a partial prefix, got %+v", markers)
	}
	markers = s.Feed(full[split:])
	if len(markers) != 1 || markers[0].Kind != "C" {
		t.Fatalf("expected a single C marker after the carry completes, got %+v", markers)
	}
}

func TestOsc133Scanner_TerminatorSplitAcrossChunks(t *testing.T) {
	s := &Osc133Scanner{}
	markers := s.Feed("\x1b]133;D;0")
	if len(markers) != 0 {
		t.Fatalf("expected no markers before the terminator arrives, got %+v", markers)
	}
	markers = s.Feed("\x07")
	if len(markers) != 1 || markers[0].Kind != "D" || markers[0].ExitCode == nil || *markers[0].ExitCode != 0 {
		t.Fatalf("expected a single D marker with ExitCode=0 once the terminator arrives, got %+v", markers)
	}
}

func TestOsc133Scanner_ExitCodePresentAbsentMalformed(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    *int
	}{
		{"present", "\x1b]133;D;42\x07", intPtr(42)},
		{"absent", "\x1b]133;D\x07", nil},
		{"malformed", "\x1b]133;D;notanumber\x07", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &Osc133Scanner{}
			markers := s.Feed(tc.payload)
			if len(markers) != 1 || markers[0].Kind != "D" {
				t.Fatalf("expected a single D marker, got %+v", markers)
			}
			got := markers[0].ExitCode
			if (got == nil) != (tc.want == nil) {
				t.Fatalf("ExitCode presence mismatch: got %v, want %v", got, tc.want)
			}
			if got != nil && tc.want != nil && *got != *tc.want {
				t.Fatalf("ExitCode = %d, want %d", *got, *tc.want)
			}
		})
	}
}

func TestOsc133Scanner_Reset_DropsCarry(t *testing.T) {
	s := &Osc133Scanner{}
	s.Feed("\x1b]133;")
	if s.carry == "" {
		t.Fatal("expected a non-empty carry after a partial prefix")
	}
	s.Reset()
	if s.carry != "" {
		t.Errorf("expected Reset to drop the carry, got %q", s.carry)
	}
}
