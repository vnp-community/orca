package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// PlanLimitsFromEnv returns DefaultPlanLimits with REQUEST_PLAN_MAX_PHASES, REQUEST_PLAN_MAX_TASKS_PER_CONTAINER,
// REQUEST_PLAN_MAX_TASKS_TOTAL and REQUEST_PLAN_MAX_TITLE_CHARS applied. An invalid value keeps the default and is
// reported in warnings.
func PlanLimitsFromEnv(getenv func(string) string) (PlanLimits, []string) {
	lim := DefaultPlanLimits
	var warnings []string
	for _, f := range []struct {
		key string
		dst *int
	}{
		{"REQUEST_PLAN_MAX_PHASES", &lim.MaxPhases},
		{"REQUEST_PLAN_MAX_TASKS_PER_CONTAINER", &lim.MaxTasksPerContainer},
		{"REQUEST_PLAN_MAX_TASKS_TOTAL", &lim.MaxTasksTotal},
		{"REQUEST_PLAN_MAX_TITLE_CHARS", &lim.MaxTitleRunes},
	} {
		v := strings.TrimSpace(getenv(f.key))
		if v == "" {
			continue
		}
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			warnings = append(warnings, fmt.Sprintf("%s=%q ignored: want an integer >= 1", f.key, v))
			continue
		}
		*f.dst = n
	}
	return lim, warnings
}
