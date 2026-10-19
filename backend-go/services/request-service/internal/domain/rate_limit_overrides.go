package domain

import (
	"fmt"
	"strconv"
	"strings"
)

// rateClassEnv maps a rate class to the middle of its env names (REQUEST_RATE_<CLASS>_PER_SECOND / _BURST).
var rateClassEnv = map[string]string{RateRead: "READ", RateWrite: "WRITE", RateAI: "AI", RateWebhook: "WEBHOOK"}

// LimitsFromEnv returns DefaultLimits with per-class overrides from REQUEST_RATE_<CLASS>_PER_SECOND and
// REQUEST_RATE_<CLASS>_BURST. An invalid value keeps the default and is reported in warnings: a typo must not
// turn the limiter off or lock every caller out.
func LimitsFromEnv(getenv func(string) string) (limits map[string]Limit, warnings []string) {
	limits = make(map[string]Limit, len(DefaultLimits))
	for class, l := range DefaultLimits {
		limits[class] = l
	}
	for class, name := range rateClassEnv {
		l := limits[class]
		if key, v := "REQUEST_RATE_"+name+"_PER_SECOND", strings.TrimSpace(getenv("REQUEST_RATE_"+name+"_PER_SECOND")); v != "" {
			if f, err := strconv.ParseFloat(v, 64); err != nil || f <= 0 {
				warnings = append(warnings, fmt.Sprintf("%s=%q ignored: want a number > 0", key, v))
			} else {
				l.PerSecond = f
			}
		}
		if key, v := "REQUEST_RATE_"+name+"_BURST", strings.TrimSpace(getenv("REQUEST_RATE_"+name+"_BURST")); v != "" {
			if n, err := strconv.Atoi(v); err != nil || n < 1 {
				warnings = append(warnings, fmt.Sprintf("%s=%q ignored: want an integer >= 1", key, v))
			} else {
				l.Burst = n
			}
		}
		limits[class] = l
	}
	return limits, warnings
}
