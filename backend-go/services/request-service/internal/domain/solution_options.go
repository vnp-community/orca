package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"unicode/utf8"
)

const MaxOptionsBytes = 64 * 1024

const maxSolutionOptions = 4

var (
	ErrSolutionOptionsInvalid = errors.New("invalid solution options")
	optionIDPattern           = regexp.MustCompile(`^opt-[0-9]+$`)
)

type SolutionOptions struct {
	SchemaVersion  int            `json:"schema_version"`
	Options        []Option       `json:"options"`
	Recommendation Recommendation `json:"recommendation"`
	Assumptions    []Assumption   `json:"assumptions,omitempty"`
	OpenQuestions  []OpenQuestion `json:"open_questions,omitempty"`
	// RequirementCoverage maps each active AC of the Request to the options that satisfy it (CR-REQ-027).
	RequirementCoverage []CoverageEntry `json:"requirement_coverage,omitempty"`
}

// Assumption accepts the old plain-string form and the structured form; ids let a clarification answer one by key.
type Assumption struct {
	ID                string `json:"id,omitempty"`
	Text              string `json:"text"`
	NeedsConfirmation bool   `json:"needs_confirmation,omitempty"`
}

// OpenQuestion accepts the old plain-string form and the structured form; a blocking one must be answered before approval.
type OpenQuestion struct {
	ID       string `json:"id,omitempty"`
	Text     string `json:"text"`
	Blocking bool   `json:"blocking,omitempty"`
}

func (a *Assumption) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		a.ID, a.NeedsConfirmation = "", false
		return json.Unmarshal(b, &a.Text)
	}
	type plain Assumption
	return json.Unmarshal(b, (*plain)(a))
}

// MarshalJSON keeps the string form for legacy items so an old document round-trips unchanged.
func (a Assumption) MarshalJSON() ([]byte, error) {
	if a.ID == "" && !a.NeedsConfirmation {
		return json.Marshal(a.Text)
	}
	type plain Assumption
	return json.Marshal(plain(a))
}

func (q *OpenQuestion) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		q.ID, q.Blocking = "", false
		return json.Unmarshal(b, &q.Text)
	}
	type plain OpenQuestion
	return json.Unmarshal(b, (*plain)(q))
}

func (q OpenQuestion) MarshalJSON() ([]byte, error) {
	if q.ID == "" && !q.Blocking {
		return json.Marshal(q.Text)
	}
	type plain OpenQuestion
	return json.Marshal(plain(q))
}

type Option struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Summary        string         `json:"summary"`
	Approach       string         `json:"approach"`
	Pros           []string       `json:"pros,omitempty"`
	Cons           []string       `json:"cons,omitempty"`
	Risks          []Risk         `json:"risks,omitempty"`
	Effort         Effort         `json:"effort"`
	AffectedAreas  []AffectedArea `json:"affected_areas,omitempty"`
	BreakingChange bool           `json:"breaking_change"`
	Rollback       string         `json:"rollback,omitempty"`
	Recommended    bool           `json:"recommended"`
}

type Risk struct {
	Description string `json:"description"`
	Severity    string `json:"severity"`
}

type Effort struct {
	Size          string   `json:"size"`
	HoursEstimate *float64 `json:"hours_estimate,omitempty"`
}

type AffectedArea struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type Recommendation struct {
	OptionID string `json:"option_id"`
	Reason   string `json:"reason"`
}

// MinOptionsFor is the fewest options a type's solution needs. refactor accepts one because the
// README only demands two for change_request (open question 1 of BE-REQ-SOL-007).
func MinOptionsFor(t RequestType) int {
	if t == RequestTypeRefactor {
		return 1
	}
	return 2
}

func optionsErr(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrSolutionOptionsInvalid, fmt.Sprintf(format, args...))
}

// ParseSolutionOptions decodes strictly: oversized input and duplicate keys are rejected before typing.
func ParseSolutionOptions(raw []byte) (SolutionOptions, error) {
	var opts SolutionOptions
	if len(raw) > MaxOptionsBytes {
		return opts, optionsErr("document exceeds %d bytes", MaxOptionsBytes)
	}
	if _, err := parseStrictJSON(raw); err != nil {
		return opts, fmt.Errorf("%w: %v", ErrSolutionOptionsInvalid, err)
	}
	if err := json.Unmarshal(raw, &opts); err != nil {
		return opts, fmt.Errorf("%w: %v", ErrSolutionOptionsInvalid, err)
	}
	return opts, nil
}

// Marshal re-serialises the typed document, which drops fields the AI invented.
func (o SolutionOptions) Marshal() ([]byte, error) {
	return json.Marshal(o)
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }

func (o SolutionOptions) Validate(minOptions int) error {
	if o.SchemaVersion != 1 {
		return optionsErr("schema_version must be 1, got %d", o.SchemaVersion)
	}
	if len(o.Options) < minOptions || len(o.Options) > maxSolutionOptions {
		return optionsErr("number of options must be between %d and %d, got %d", minOptions, maxSolutionOptions, len(o.Options))
	}
	seen := map[string]bool{}
	recommended := ""
	recommendedCount := 0
	for i, opt := range o.Options {
		if !optionIDPattern.MatchString(opt.ID) {
			return optionsErr("option %d id must look like opt-N, got %q", i, opt.ID)
		}
		if seen[opt.ID] {
			return optionsErr("duplicate option id %s", opt.ID)
		}
		seen[opt.ID] = true
		if n := runeLen(opt.Title); n < 1 || n > 120 {
			return optionsErr("option %s title length must be 1..120", opt.ID)
		}
		if n := runeLen(opt.Summary); n < 1 || n > 600 {
			return optionsErr("option %s summary length must be 1..600", opt.ID)
		}
		if n := runeLen(opt.Approach); n < 1 || n > 4000 {
			return optionsErr("option %s approach length must be 1..4000", opt.ID)
		}
		switch opt.Effort.Size {
		case "S", "M", "L":
		default:
			return optionsErr("option %s effort.size must be S, M or L", opt.ID)
		}
		if h := opt.Effort.HoursEstimate; h != nil && (*h < 0 || math.IsNaN(*h) || math.IsInf(*h, 0)) {
			return optionsErr("option %s effort.hours_estimate must be >= 0", opt.ID)
		}
		for _, r := range opt.Risks {
			switch r.Severity {
			case "low", "medium", "high":
			default:
				return optionsErr("option %s risk severity must be low, medium or high", opt.ID)
			}
		}
		for _, a := range opt.AffectedAreas {
			switch a.Kind {
			case "service", "module", "api", "schema", "ui", "infra":
			default:
				return optionsErr("option %s affected_areas kind %q is not allowed", opt.ID, a.Kind)
			}
		}
		if opt.Recommended {
			recommendedCount++
			recommended = opt.ID
		}
	}
	if recommendedCount != 1 {
		return optionsErr("exactly one option must have recommended=true, got %d", recommendedCount)
	}
	if o.Recommendation.OptionID != recommended {
		return optionsErr("recommendation.option_id %q must match the recommended option %q", o.Recommendation.OptionID, recommended)
	}
	if err := o.validateStructuredItems(); err != nil {
		return err
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return optionsErr("marshal: %v", err)
	}
	if len(raw) > MaxOptionsBytes {
		return optionsErr("document exceeds %d bytes", MaxOptionsBytes)
	}
	return nil
}

// IndexOf returns the 0-based position of option id; the API speaks ids, storage speaks positions.
func (o SolutionOptions) IndexOf(id string) (int, bool) {
	for i, opt := range o.Options {
		if opt.ID == id {
			return i, true
		}
	}
	return 0, false
}

var (
	assumptionIDPattern = regexp.MustCompile(`^A-[1-9][0-9]*$`)
	questionIDPattern   = regexp.MustCompile(`^Q-[1-9][0-9]*$`)
)

// validateStructuredItems checks the CR-REQ-027 members; legacy string items (empty id) pass as before.
func (o SolutionOptions) validateStructuredItems() error {
	seen := map[string]bool{}
	for _, a := range o.Assumptions {
		if a.ID == "" {
			continue
		}
		if !assumptionIDPattern.MatchString(a.ID) || a.Text == "" || seen[a.ID] {
			return optionsErr("assumption %q must have a unique id like A-N and a text", a.ID)
		}
		seen[a.ID] = true
	}
	seen = map[string]bool{}
	for _, q := range o.OpenQuestions {
		if q.ID == "" {
			continue
		}
		if !questionIDPattern.MatchString(q.ID) || q.Text == "" || seen[q.ID] {
			return optionsErr("open question %q must have a unique id like Q-N and a text", q.ID)
		}
		seen[q.ID] = true
	}
	options := map[string]bool{}
	for _, opt := range o.Options {
		options[opt.ID] = true
	}
	for _, c := range o.RequirementCoverage {
		if !acIDPattern.MatchString(c.ACID) {
			return optionsErr("requirement_coverage ac_id %q must look like AC-N", c.ACID)
		}
		switch c.Status {
		case "covered", "partial", "out_of_scope":
		default:
			return optionsErr("requirement_coverage %s status must be covered, partial or out_of_scope", c.ACID)
		}
		for _, id := range c.OptionIDs {
			if !options[id] {
				return optionsErr("requirement_coverage %s names unknown option %s", c.ACID, id)
			}
		}
	}
	return nil
}
