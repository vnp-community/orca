package domain

import (
	"strings"
	"testing"
)

func TestSolutionOptions_Validate(t *testing.T) {
	validOpt1 := Option{
		ID:       "opt-1",
		Title:    "Opt 1",
		Summary:  "Sum 1",
		Approach: "App 1",
		Effort:   Effort{Size: "S", HoursEstimate: 1},
	}
	validOpt2 := Option{
		ID:       "opt-2",
		Title:    "Opt 2",
		Summary:  "Sum 2",
		Approach: "App 2",
		Effort:   Effort{Size: "M", HoursEstimate: 5},
	}

	cases := []struct {
		name       string
		opts       SolutionOptions
		minOptions int
		wantErr    bool
	}{
		{
			name: "valid",
			opts: SolutionOptions{
				Options:        []Option{validOpt1, validOpt2},
				Recommendation: Recommendation{OptionID: "opt-1"},
			},
			minOptions: 2,
			wantErr:    false,
		},
		{
			name: "too few options",
			opts: SolutionOptions{
				Options:        []Option{validOpt1},
				Recommendation: Recommendation{OptionID: "opt-1"},
			},
			minOptions: 2,
			wantErr:    true,
		},
		{
			name: "too many options",
			opts: SolutionOptions{
				Options:        []Option{validOpt1, validOpt2, validOpt1, validOpt2, validOpt1},
				Recommendation: Recommendation{OptionID: "opt-1"},
			},
			minOptions: 2,
			wantErr:    true,
		},
		{
			name: "duplicate id",
			opts: SolutionOptions{
				Options: []Option{validOpt1, {
					ID:       "opt-1",
					Title:    "Opt 1 dup",
					Summary:  "Sum 1",
					Approach: "App 1",
					Effort:   Effort{Size: "S", HoursEstimate: 1},
				}},
				Recommendation: Recommendation{OptionID: "opt-1"},
			},
			minOptions: 2,
			wantErr:    true,
		},
		{
			name: "invalid id format",
			opts: SolutionOptions{
				Options: []Option{validOpt1, {
					ID:       "foo-2",
					Title:    "Opt 2",
					Summary:  "Sum 2",
					Approach: "App 2",
					Effort:   Effort{Size: "M", HoursEstimate: 5},
				}},
				Recommendation: Recommendation{OptionID: "opt-1"},
			},
			minOptions: 2,
			wantErr:    true,
		},
		{
			name: "missing recommended",
			opts: SolutionOptions{
				Options:        []Option{validOpt1, validOpt2},
				Recommendation: Recommendation{OptionID: "opt-3"},
			},
			minOptions: 2,
			wantErr:    true,
		},
		{
			name: "missing effort size",
			opts: SolutionOptions{
				Options: []Option{validOpt1, {
					ID:       "opt-2",
					Title:    "Opt 2",
					Summary:  "Sum 2",
					Approach: "App 2",
					Effort:   Effort{HoursEstimate: 5},
				}},
				Recommendation: Recommendation{OptionID: "opt-1"},
			},
			minOptions: 2,
			wantErr:    true,
		},
		{
			name: "negative hours",
			opts: SolutionOptions{
				Options: []Option{validOpt1, {
					ID:       "opt-2",
					Title:    "Opt 2",
					Summary:  "Sum 2",
					Approach: "App 2",
					Effort:   Effort{Size: "M", HoursEstimate: -1},
				}},
				Recommendation: Recommendation{OptionID: "opt-1"},
			},
			minOptions: 2,
			wantErr:    true,
		},
		{
			name: "title too long",
			opts: SolutionOptions{
				Options: []Option{validOpt1, {
					ID:       "opt-2",
					Title:    strings.Repeat("a", 121),
					Summary:  "Sum 2",
					Approach: "App 2",
					Effort:   Effort{Size: "M", HoursEstimate: 5},
				}},
				Recommendation: Recommendation{OptionID: "opt-1"},
			},
			minOptions: 2,
			wantErr:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.opts.Validate(tc.minOptions)
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
