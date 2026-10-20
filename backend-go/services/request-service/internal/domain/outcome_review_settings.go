package domain

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// OutcomeReviewMode is whether a project offers the review. There is no automatic mode: a review costs an agent run, so a person asks.
type OutcomeReviewMode string

const (
	OutcomeReviewOff    OutcomeReviewMode = "off"
	OutcomeReviewManual OutcomeReviewMode = "manual"
)

func (m OutcomeReviewMode) Valid() bool { return m == OutcomeReviewOff || m == OutcomeReviewManual }

// Enabled reads an empty mode as off, so rows older than the setting stay off.
func (m OutcomeReviewMode) Enabled() bool { return m == OutcomeReviewManual }

// OutcomeReviewConfig is what the deployment decides: the default for a project without a saved choice and the kill switch.
type OutcomeReviewConfig struct {
	Default    OutcomeReviewMode
	KillSwitch bool
}

// OutcomeReviewConfigFromEnv reads REQUEST_DEFAULT_OUTCOME_REVIEW (off|manual) and REQUEST_REVIEW_KILL_SWITCH (true|false).
// An unreadable value keeps the safe answer (off, no kill) and says so.
func OutcomeReviewConfigFromEnv(getenv func(string) string) (OutcomeReviewConfig, []string) {
	c, warnings := OutcomeReviewConfig{Default: OutcomeReviewOff}, []string(nil)
	switch raw := strings.ToLower(strings.TrimSpace(getenv("REQUEST_DEFAULT_OUTCOME_REVIEW"))); raw {
	case "":
	case string(OutcomeReviewOff), string(OutcomeReviewManual):
		c.Default = OutcomeReviewMode(raw)
	default:
		warnings = append(warnings, "REQUEST_DEFAULT_OUTCOME_REVIEW "+strconv.Quote(raw)+" is not off or manual; off applies")
	}
	switch raw := strings.ToLower(strings.TrimSpace(getenv("REQUEST_REVIEW_KILL_SWITCH"))); raw {
	case "":
	case "true", "1", "yes", "on":
		c.KillSwitch = true
	case "false", "0", "no", "off":
	default:
		warnings = append(warnings, "REQUEST_REVIEW_KILL_SWITCH "+strconv.Quote(raw)+" is not true or false; the switch stays off")
	}
	return c, warnings
}

// Effective is the mode a project runs with: the kill switch wins over everything, then the project's choice, then the default.
func (c OutcomeReviewConfig) Effective(project OutcomeReviewMode) OutcomeReviewMode {
	if c.KillSwitch {
		return OutcomeReviewOff
	}
	if project.Valid() && project != "" {
		return project
	}
	if c.Default.Valid() {
		return c.Default
	}
	return OutcomeReviewOff
}

var planTargetRe = regexp.MustCompile("(?:modify|create)\\s+`([^`]+)`")

// PlanTargets lists the files the Code: line of each task description says the task will modify or create, sorted and without
// duplicates. A description without such a line adds nothing.
func PlanTargets(descriptions []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, d := range descriptions {
		line := TaskDescriptionLine(d, "Code:")
		for _, m := range planTargetRe.FindAllStringSubmatch(line, -1) {
			p := normalizeRel(m[1])
			if p != "" && !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Strings(out)
	return out
}

// FactsDigest hashes only what the database holds (criteria, coverage, task states, the plan's targets). It lets a read notice
// that a review is stale without asking git, which a read must not do.
func FactsDigest(phase ReviewPhase, f Facts) (string, error) {
	type ac struct{ ID, Text, Status string }
	d := struct {
		Schema   int
		Phase    ReviewPhase
		Criteria []ac
		Coverage []CoverageRow
		Tasks    map[string]CoveringTask
		DataOK   bool
		Targets  []string
	}{Schema: OutcomeReviewSchemaVersion, Phase: phase, Coverage: sortedCoverage(f.Requests.Coverage), Tasks: f.Requests.Tasks,
		DataOK: f.Requests.DataOK, Targets: sortedStrings(f.PlanTargets)}
	for _, c := range f.Requests.Criteria {
		d.Criteria = append(d.Criteria, ac{c.ID, NormalizeNFC(c.Text), c.Status})
	}
	sort.Slice(d.Criteria, func(i, j int) bool { return d.Criteria[i].ID < d.Criteria[j].ID })
	return DigestValue(d)
}
