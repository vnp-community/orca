package domain

import (
	"testing"
	"time"
)

func TestProposalRetentionFromEnv(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		warn bool
	}{
		{"", DefaultProposalRetention, false},
		{"72h", 72 * time.Hour, false},
		{"3d", 72 * time.Hour, false},
		{" 14d ", 14 * 24 * time.Hour, false},
		{"soon", DefaultProposalRetention, true},
		{"xd", DefaultProposalRetention, true},
		{"0d", DefaultProposalRetention, true},
		{"-5h", DefaultProposalRetention, true},
	}
	for _, tc := range cases {
		got, warn := ProposalRetentionFromEnv(func(string) string { return tc.in })
		if got != tc.want || (warn != "") != tc.warn {
			t.Errorf("%q: got %s %q, want %s warn=%v", tc.in, got, warn, tc.want, tc.warn)
		}
	}
}
