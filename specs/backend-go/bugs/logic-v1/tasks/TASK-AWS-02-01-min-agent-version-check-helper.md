# TASK-AWS-02-01: Add `MinAgentVersion` config + `isBelowMinimumVersion` helper

**From Solution:** SOL-AWS-02
**Priority:** P1
**Service:** `infra-fleet-service`
**File:** `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/version.go` (new), `config.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

`AGENT_MIN_VERSION` exists agent-side
(`agent/src/shared/agent-wire-protocol.ts:31`, `'1.0.0'`) and
`inboundHandshakeParams.AgentVersion` (`server.go:53`) is already captured,
but nothing compares them. This adds the comparison helper and its config
knob, ported from `agent-wire-protocol.ts`'s `isAgentVersionBelowMinimum`
(major.minor.patch only; non-numeric segments fail open toward "too old").
Decision already settled by the TS-era fix
(`specs/backend/bugs/hld-v1/solutions/SOLUTION-agent-ws-protocol-exact.md`
§B) — reuse WS close code 1008, not a custom 4000-range code.

## Changes to make

In `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/config.go`,
add the field to `Config` and its default in `LoadConfigFromEnv`:

```go
type Config struct {
	Port        int
	APISecret   string
	OrcaVersion string
	// MinAgentVersion gates a direct-websocket agent's handshake — an agent
	// reporting an older AgentVersion is rejected with 1008 (see
	// isBelowMinimumVersion, rejectVersion in version.go). Empty disables
	// the check entirely (fail open — matches an agent build too old to
	// send agentVersion at all, see server.go's firstNonEmpty fallback).
	MinAgentVersion string
	// MaxConcurrentSessions caps live direct-websocket sessions accepted by
	// this process — see capacity.go (TASK-AWS-02-03). <= 0 disables the
	// check.
	MaxConcurrentSessions int
}

func LoadConfigFromEnv(port int, orcaVersion string) Config {
	return Config{
		Port:                  port,
		APISecret:             os.Getenv("ORCA_AGENT_API_SECRET"),
		OrcaVersion:           orcaVersion,
		MinAgentVersion:       os.Getenv("ORCA_AGENT_MIN_VERSION"), // e.g. "1.0.0"; empty = no check
		MaxConcurrentSessions: 500,                                 // circuit-breaker default, see capacity.go's doc comment — not a tuned production limit
	}
}
```

Create `backend-go/services/infra-fleet-service/internal/adapter/agentwsserver/version.go`:

```go
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
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/infra-fleet-service/internal/adapter/agentwsserver/...
```

Expected: clean build. Add `version_test.go`: `isBelowMinimumVersion("0.9.0",
"1.0.0")` → true; `("1.0.0", "1.0.0")` → false; `("1.2.0", "1.0.0")` →
false; `("", "1.0.0")` → false (empty skips); `("bad", "1.0.0")` → true
(fails open toward rejected).
