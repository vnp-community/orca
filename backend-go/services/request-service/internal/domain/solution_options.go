package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"
)

const MaxOptionsBytes = 64 * 1024

type SolutionOptions struct {
	SchemaVersion  int            `json:"schema_version"`
	Options        []Option       `json:"options"`
	Recommendation Recommendation `json:"recommendation"`
	Assumptions    []string       `json:"assumptions,omitempty"`
	OpenQuestions  []string       `json:"open_questions,omitempty"`
}

type Option struct {
	ID            string       `json:"id"`
	Title         string       `json:"title"`
	Summary       string       `json:"summary"`
	Approach      string       `json:"approach"`
	Effort        Effort       `json:"effort"`
	Risk          Risk         `json:"risk"`
	AffectedAreas []AffectedArea `json:"affected_areas"`
}

type Effort struct {
	Size          string `json:"size"`
	HoursEstimate int    `json:"hours_estimate"`
}

type Risk struct {
	Level       string `json:"level"`
	Description string `json:"description"`
}

type AffectedArea struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Recommendation struct {
	OptionID string `json:"option_id"`
	Reason   string `json:"reason"`
}

func ParseSolutionOptions(raw []byte) (SolutionOptions, error) {
	var opts SolutionOptions
	if err := json.Unmarshal(raw, &opts); err != nil {
		return opts, err
	}
	return opts, nil
}

func (o SolutionOptions) Validate(minOptions int) error {
	if len(o.Options) < minOptions || len(o.Options) > 4 {
		return fmt.Errorf("number of options must be between %d and 4, got %d", minOptions, len(o.Options))
	}

	seenIDs := make(map[string]bool)
	foundRecommended := false

	for i, opt := range o.Options {
		if opt.ID == "" {
			return fmt.Errorf("option %d is missing id", i)
		}
		if seenIDs[opt.ID] {
			return fmt.Errorf("duplicate option id: %s", opt.ID)
		}
		seenIDs[opt.ID] = true

		// Check if it's opt-N format
		if len(opt.ID) < 5 || opt.ID[:4] != "opt-" {
			return fmt.Errorf("option id must be in opt-N format, got %s", opt.ID)
		}

		if utf8.RuneCountInString(opt.Title) < 1 || utf8.RuneCountInString(opt.Title) > 120 {
			return fmt.Errorf("option %s title length must be 1..120", opt.ID)
		}
		if utf8.RuneCountInString(opt.Summary) < 1 || utf8.RuneCountInString(opt.Summary) > 600 {
			return fmt.Errorf("option %s summary length must be 1..600", opt.ID)
		}
		if utf8.RuneCountInString(opt.Approach) < 1 || utf8.RuneCountInString(opt.Approach) > 4000 {
			return fmt.Errorf("option %s approach length must be 1..4000", opt.ID)
		}
		if opt.Effort.Size == "" {
			return fmt.Errorf("option %s effort.size is required", opt.ID)
		}
		if opt.Effort.HoursEstimate < 0 {
			return fmt.Errorf("option %s effort.hours_estimate must be >= 0", opt.ID)
		}

		if o.Recommendation.OptionID == opt.ID {
			foundRecommended = true
		}
	}

	if !foundRecommended {
		return errors.New("recommendation.option_id must match exactly one option id")
	}

	raw, _ := json.Marshal(o)
	if len(raw) > MaxOptionsBytes {
		return fmt.Errorf("options exceed maximum size of 64KB")
	}

	return nil
}
