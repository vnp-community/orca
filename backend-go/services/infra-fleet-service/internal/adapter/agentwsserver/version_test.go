package agentwsserver

import "testing"

func TestIsBelowMinimumVersion(t *testing.T) {
	cases := []struct {
		name    string
		version string
		min     string
		want    bool
	}{
		{"older patch", "0.9.0", "1.0.0", true},
		{"equal", "1.0.0", "1.0.0", false},
		{"newer minor", "1.2.0", "1.0.0", false},
		{"empty version skips check", "", "1.0.0", false},
		{"non-numeric fails open toward rejected", "bad", "1.0.0", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isBelowMinimumVersion(tc.version, tc.min)
			if got != tc.want {
				t.Errorf("isBelowMinimumVersion(%q, %q) = %v, want %v", tc.version, tc.min, got, tc.want)
			}
		})
	}
}
