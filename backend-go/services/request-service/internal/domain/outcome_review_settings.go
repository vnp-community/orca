package domain

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
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
	// ContextTTL is how long a checked context result is reused without asking the agent again; zero means the default.
	ContextTTL time.Duration
	// ContextTimeout bounds one context run; the agent itself stops a run at 15 minutes. Zero means the default.
	ContextTimeout time.Duration
	// ContextMaxFiles is the number of files the agent may open in one run. Zero means the default.
	ContextMaxFiles int
	// ArtifactRetention is how long a drawing that is no longer in force is kept. Zero means the default.
	ArtifactRetention time.Duration
	// ArtifactQuotaMB is the most drawings a tenant keeps. Zero means the default.
	ArtifactQuotaMB int
	// RegenPerDay is how many times a person may regenerate for one Request in a day. Zero means the default.
	RegenPerDay int
	// ConfirmCostUSD is the estimated cost above which a regeneration needs the person to confirm. Zero means never ask.
	ConfirmCostUSD float64
	// DrawDisabled turns drawing off for the whole deployment (REQUEST_REVIEW_DRAW_ENABLED=false): the review is text and tables only.
	DrawDisabled bool
}

const (
	DefaultRegenPerDay      = 5
	MaxConcurrentReviewRuns = 2
	maxRegenPerDay          = 1000
)

func (c OutcomeReviewConfig) RegenPerDayOrDefault() int {
	if c.RegenPerDay > 0 {
		return c.RegenPerDay
	}
	return DefaultRegenPerDay
}

const (
	DefaultReviewContextTTL      = 7 * 24 * time.Hour
	DefaultReviewContextTimeout  = 10 * time.Minute
	MaxReviewContextTimeout      = 15 * time.Minute
	DefaultReviewContextMaxFiles = 60
	MaxReviewContextMaxFiles     = 400
	DefaultArtifactRetention     = 90 * 24 * time.Hour
)

func (c OutcomeReviewConfig) ArtifactRetentionOrDefault() time.Duration {
	if c.ArtifactRetention > 0 {
		return c.ArtifactRetention
	}
	return DefaultArtifactRetention
}

func (c OutcomeReviewConfig) ArtifactQuotaBytes() int64 {
	if c.ArtifactQuotaMB > 0 {
		return int64(c.ArtifactQuotaMB) << 20
	}
	return int64(DefaultArtifactQuotaMB) << 20
}

func (c OutcomeReviewConfig) ContextTTLOrDefault() time.Duration {
	if c.ContextTTL > 0 {
		return c.ContextTTL
	}
	return DefaultReviewContextTTL
}

func (c OutcomeReviewConfig) ContextTimeoutMS() int {
	d := c.ContextTimeout
	if d <= 0 {
		d = DefaultReviewContextTimeout
	}
	if d > MaxReviewContextTimeout {
		d = MaxReviewContextTimeout
	}
	return int(d / time.Millisecond)
}

func (c OutcomeReviewConfig) ContextMaxFilesOrDefault() int {
	if c.ContextMaxFiles > 0 {
		return c.ContextMaxFiles
	}
	return DefaultReviewContextMaxFiles
}

// OutcomeReviewConfigFromEnv reads REQUEST_DEFAULT_OUTCOME_REVIEW (off|manual), REQUEST_REVIEW_KILL_SWITCH and REQUEST_REVIEW_DRAW_ENABLED (true|false).
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
	switch raw := strings.ToLower(strings.TrimSpace(getenv("REQUEST_REVIEW_DRAW_ENABLED"))); raw {
	case "":
	case "true", "1", "yes", "on":
	case "false", "0", "no", "off":
		c.DrawDisabled = true
	default:
		warnings = append(warnings, "REQUEST_REVIEW_DRAW_ENABLED "+strconv.Quote(raw)+" is not true or false; drawing stays on")
	}
	if d, ok, w := envDuration(getenv, "REQUEST_REVIEW_CONTEXT_TTL", time.Hour, 90*24*time.Hour); w != "" {
		warnings = append(warnings, w)
	} else if ok {
		c.ContextTTL = d
	}
	if d, ok, w := envDuration(getenv, "REQUEST_REVIEW_CONTEXT_TIMEOUT", time.Minute, MaxReviewContextTimeout); w != "" {
		warnings = append(warnings, w)
	} else if ok {
		c.ContextTimeout = d
	}
	if raw := strings.TrimSpace(getenv("REQUEST_REVIEW_CONTEXT_MAX_FILES")); raw != "" {
		if n, err := strconv.Atoi(raw); err != nil || n < 5 || n > MaxReviewContextMaxFiles {
			warnings = append(warnings, "REQUEST_REVIEW_CONTEXT_MAX_FILES "+strconv.Quote(raw)+" is not a number from 5 to "+strconv.Itoa(MaxReviewContextMaxFiles)+"; the default applies")
		} else {
			c.ContextMaxFiles = n
		}
	}
	if d, ok, w := envDuration(getenv, "REQUEST_REVIEW_ARTIFACT_RETENTION", 24*time.Hour, 5*365*24*time.Hour); w != "" {
		warnings = append(warnings, w)
	} else if ok {
		c.ArtifactRetention = d
	}
	if raw := strings.TrimSpace(getenv("REQUEST_REVIEW_ARTIFACT_QUOTA_MB")); raw != "" {
		if n, err := strconv.Atoi(raw); err != nil || n < 1 || n > 100_000 {
			warnings = append(warnings, "REQUEST_REVIEW_ARTIFACT_QUOTA_MB "+strconv.Quote(raw)+" is not a number from 1 to 100000; the default applies")
		} else {
			c.ArtifactQuotaMB = n
		}
	}
	if raw := strings.TrimSpace(getenv("REQUEST_REVIEW_REGEN_PER_DAY")); raw != "" {
		if n, err := strconv.Atoi(raw); err != nil || n < 1 || n > maxRegenPerDay {
			warnings = append(warnings, "REQUEST_REVIEW_REGEN_PER_DAY "+strconv.Quote(raw)+" is not a number from 1 to "+strconv.Itoa(maxRegenPerDay)+"; the default applies")
		} else {
			c.RegenPerDay = n
		}
	}
	if raw := strings.TrimSpace(getenv("REQUEST_REVIEW_CONFIRM_COST")); raw != "" {
		if f, err := strconv.ParseFloat(raw, 64); err != nil || f < 0 || f > 10000 {
			warnings = append(warnings, "REQUEST_REVIEW_CONFIRM_COST "+strconv.Quote(raw)+" is not an amount in USD from 0 to 10000; no confirmation is asked")
		} else {
			c.ConfirmCostUSD = f
		}
	}
	return c, warnings
}

// envDuration reads a Go duration within [lo, hi]. An absent variable is not a warning; a bad one keeps the default and says so.
func envDuration(getenv func(string) string, name string, lo, hi time.Duration) (time.Duration, bool, string) {
	raw := strings.TrimSpace(getenv(name))
	if raw == "" {
		return 0, false, ""
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < lo || d > hi {
		return 0, false, name + " " + strconv.Quote(raw) + " is not a duration from " + lo.String() + " to " + hi.String() + "; the default applies"
	}
	return d, true, ""
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
