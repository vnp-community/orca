package domain

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// PlanAutoPolicy holds the timings and limits of automatic Plan generation. Only the fields that REQUEST_PLAN_AUTO_*
// names are tunable; the rest are fixed so a typo cannot turn the worker into a busy loop.
type PlanAutoPolicy struct {
	// MaxExtraAttempts is how many more runs a transient failure may buy (0..2).
	MaxExtraAttempts int
	RetryDelay       time.Duration
	BusyDelay        time.Duration
	// StartWindow is how long a job may wait for a run to start (busy or infrastructure errors) before it gives up.
	StartWindow       time.Duration
	MaxCommitAttempts int
	PollInterval      time.Duration
	LeaseTTL          time.Duration
}

const planAutoMaxExtraAttemptsCeiling = 2

// DefaultPlanAutoPolicy is what a deployment without REQUEST_PLAN_AUTO_* variables runs with.
var DefaultPlanAutoPolicy = PlanAutoPolicy{
	MaxExtraAttempts:  1,
	RetryDelay:        30 * time.Second,
	BusyDelay:         60 * time.Second,
	StartWindow:       30 * time.Minute,
	MaxCommitAttempts: 5,
	PollInterval:      5 * time.Second,
	LeaseTTL:          60 * time.Second,
}

// MaxAttempts is the number of runs a job may use in total.
func (p PlanAutoPolicy) MaxAttempts() int { return 1 + p.MaxExtraAttempts }

// PlanAutoPolicyFromEnv reads REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS, REQUEST_PLAN_AUTO_RETRY_DELAY,
// REQUEST_PLAN_AUTO_BUSY_DELAY, REQUEST_PLAN_AUTO_START_WINDOW and REQUEST_PLAN_AUTO_POLL_INTERVAL (durations in Go syntax).
// A value that cannot be read keeps the default and is reported in warnings; the service still starts.
func PlanAutoPolicyFromEnv(getenv func(string) string) (PlanAutoPolicy, []string) {
	p, warnings := DefaultPlanAutoPolicy, []string(nil)
	if raw := strings.TrimSpace(getenv("REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS")); raw != "" {
		n, err := strconv.Atoi(raw)
		switch {
		case err != nil:
			warnings = append(warnings, fmt.Sprintf("REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS=%q ignored: want an integer from 0 to %d", raw, planAutoMaxExtraAttemptsCeiling))
		case n < 0 || n > planAutoMaxExtraAttemptsCeiling:
			p.MaxExtraAttempts = min(max(n, 0), planAutoMaxExtraAttemptsCeiling)
			warnings = append(warnings, fmt.Sprintf("REQUEST_PLAN_AUTO_MAX_EXTRA_ATTEMPTS=%q is outside 0 to %d; %d applies", raw, planAutoMaxExtraAttemptsCeiling, p.MaxExtraAttempts))
		default:
			p.MaxExtraAttempts = n
		}
	}
	for _, f := range []struct {
		key string
		dst *time.Duration
	}{
		{"REQUEST_PLAN_AUTO_RETRY_DELAY", &p.RetryDelay},
		{"REQUEST_PLAN_AUTO_BUSY_DELAY", &p.BusyDelay},
		{"REQUEST_PLAN_AUTO_START_WINDOW", &p.StartWindow},
		{"REQUEST_PLAN_AUTO_POLL_INTERVAL", &p.PollInterval},
	} {
		raw := strings.TrimSpace(getenv(f.key))
		if raw == "" {
			continue
		}
		d, err := time.ParseDuration(raw)
		if err != nil || d <= 0 {
			warnings = append(warnings, fmt.Sprintf("%s=%q ignored: want a positive duration such as 30s", f.key, raw))
			continue
		}
		*f.dst = d
	}
	return p, warnings
}
