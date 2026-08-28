package agentwsserver

import (
	"strconv"
	"strings"
)

// isBelowMinimumVersion reports whether version is strictly older than min,
// comparing major.minor.patch numerically — a direct Go port of
// agent-wire-protocol.ts's isAgentVersionBelowMinimum. A non-numeric or
// missing segment on either side is treated as 0, so a malformed version
// string fails open toward "too old" (rejected), never toward "trusted",
// matching the TS reference's own documented posture.
func isBelowMinimumVersion(version, min string) bool {
	if version == "" || min == "" {
		return false // caller's responsibility to skip the check when either is empty
	}
	vParts := versionParts(version)
	mParts := versionParts(min)
	for i := 0; i < 3; i++ {
		if vParts[i] != mParts[i] {
			return vParts[i] < mParts[i]
		}
	}
	return false
}

func versionParts(v string) [3]int {
	var out [3]int
	segments := strings.SplitN(v, ".", 3)
	for i := 0; i < len(segments) && i < 3; i++ {
		n, err := strconv.Atoi(strings.TrimSpace(segments[i]))
		if err != nil {
			n = 0 // non-numeric segment — fail open toward "too old", see doc comment above
		}
		out[i] = n
	}
	return out
}
