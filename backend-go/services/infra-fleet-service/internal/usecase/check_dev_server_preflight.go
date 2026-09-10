package usecase

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/common/apperrors"
)

// preflightScript composes BL-FLEET-04 Step 4's checks (git --version,
// node --version, df -P ~/.orca, a port probe, gh --version) into one
// shell.exec round trip — the same composable-shell.exec mechanism
// SOL-FLEET-03 uses for fleet-health metrics collection, reused here for a
// different purpose.
const preflightScript = `
echo "GIT:$(git --version 2>/dev/null)"
echo "NODE:$(node --version 2>/dev/null)"
echo "DISK:$(df -P ~/.orca 2>/dev/null | tail -1 | awk '{print $4}')"
echo "GH:$(gh --version 2>/dev/null | head -1)"
node -e "require('net').createServer().listen(%d,'127.0.0.1',()=>{console.log('PORT:FREE');process.exit(0)}).on('error',()=>{console.log('PORT:BUSY');process.exit(0)})"
`

// preflightMinDiskGB matches BL-FLEET-04 Step 4's "Disk space >= 5GB" —
// same threshold as SOL-FLEET-02's prereq.go (different package: usecase
// must never import adapter/sshrelay, so the threshold/parsing logic is
// duplicated here in miniature rather than shared).
const preflightMinDiskGB = 5.0

// preflightMinGit/preflightMinNode match BL-FLEET-04 Step 4 verbatim:
// Git >= 2.25, Node.js >= 22.
var (
	preflightMinGit  = preflightVersion{Major: 2, Minor: 25}
	preflightMinNode = preflightVersion{Major: 22}
)

type CheckResult struct {
	Installed bool
	Version   string
	MeetsMin  bool
}
type DiskCheckResult struct {
	FreeGB   float64
	MeetsMin bool
}
type PortCheckResult struct {
	Port      int32
	Available bool
}
type PreflightCheckResult struct {
	Git, Node CheckResult
	Disk      DiskCheckResult
	Port      PortCheckResult
	GH        CheckResult // installed-only, no version-min
}

// CheckDevServerPreflight implements BL-FLEET-04 Step 4. Per BL-FLEET-04's
// "proceed with warnings" policy, this usecase never itself hard-fails on
// a check result — it returns the full result and lets the caller decide.
type CheckDevServerPreflight struct {
	devServers DevServerRepository
	agent      DevServerAgentClient
}

func NewCheckDevServerPreflight(devServers DevServerRepository, agent DevServerAgentClient) *CheckDevServerPreflight {
	return &CheckDevServerPreflight{devServers: devServers, agent: agent}
}

func (uc *CheckDevServerPreflight) Execute(ctx context.Context, tenantID, devServerID string, probePort int32) (PreflightCheckResult, error) {
	ds, err := uc.devServers.Get(ctx, tenantID, devServerID)
	if err != nil {
		return PreflightCheckResult{}, err
	}
	result, err := uc.agent.Exec(ctx, ds, "shell.exec", map[string]any{
		"script": fmt.Sprintf(preflightScript, probePort), "timeoutMs": 8000,
	})
	if err != nil {
		// A shell.exec failure is itself informative (agent unreachable) —
		// surfaced as a typed error, not a synthesized all-false result.
		return PreflightCheckResult{}, apperrors.New(apperrors.KindInternal, "INFRA_PREFLIGHT_FAILED", "failed to run remote preflight check", err)
	}
	stdout, _ := result["stdout"].(string)
	return parsePreflightOutput(stdout, probePort), nil
}

// preflightVersion is a minimal (major, minor, patch) comparator — mirrors
// adapter/sshrelay's identical private type, duplicated (not imported,
// see this file's package-level doc comment) since usecase must never
// import an adapter package.
type preflightVersion struct {
	Major, Minor, Patch int
}

func (v preflightVersion) atLeast(min preflightVersion) bool {
	if v.Major != min.Major {
		return v.Major > min.Major
	}
	if v.Minor != min.Minor {
		return v.Minor > min.Minor
	}
	return v.Patch >= min.Patch
}

var preflightVersionPattern = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

func parsePreflightVersion(s string) (preflightVersion, bool) {
	m := preflightVersionPattern.FindStringSubmatch(s)
	if m == nil {
		return preflightVersion{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	return preflightVersion{Major: major, Minor: minor, Patch: patch}, true
}

func checkVersionedCommand(raw string, min preflightVersion) CheckResult {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return CheckResult{}
	}
	v, ok := parsePreflightVersion(raw)
	if !ok {
		return CheckResult{Installed: true, Version: raw}
	}
	return CheckResult{Installed: true, Version: raw, MeetsMin: v.atLeast(min)}
}

// parsePreflightOutput is pure parsing of preflightScript's GIT:/NODE:/
// DISK:/GH:/PORT:-prefixed stdout — malformed output degrades every field
// to Installed=false/MeetsMin=false, never panics.
func parsePreflightOutput(stdout string, probePort int32) PreflightCheckResult {
	var result PreflightCheckResult
	result.Port = PortCheckResult{Port: probePort}

	for _, line := range strings.Split(stdout, "\n") {
		switch {
		case strings.HasPrefix(line, "GIT:"):
			result.Git = checkVersionedCommand(strings.TrimPrefix(line, "GIT:"), preflightMinGit)
		case strings.HasPrefix(line, "NODE:"):
			result.Node = checkVersionedCommand(strings.TrimPrefix(line, "NODE:"), preflightMinNode)
		case strings.HasPrefix(line, "DISK:"):
			result.Disk = parseDiskCheckResult(strings.TrimPrefix(line, "DISK:"))
		case strings.HasPrefix(line, "GH:"):
			ghRaw := strings.TrimSpace(strings.TrimPrefix(line, "GH:"))
			result.GH = CheckResult{Installed: ghRaw != "", Version: ghRaw}
		case strings.HasPrefix(line, "PORT:"):
			portRaw := strings.TrimSpace(strings.TrimPrefix(line, "PORT:"))
			result.Port.Available = portRaw == "FREE"
		}
	}
	return result
}

// parseDiskCheckResult parses `df -P ~/.orca | tail -1 | awk '{print $4}'`'s
// single-number available-blocks output — GNU coreutils' df -P (this
// scaffold's Linux target hosts) reports 1024-byte blocks, same convention
// as adapter/sshrelay's prereq.go.
func parseDiskCheckResult(raw string) DiskCheckResult {
	kb, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return DiskCheckResult{}
	}
	freeGB := kb / (1024 * 1024)
	return DiskCheckResult{FreeGB: freeGB, MeetsMin: freeGB >= preflightMinDiskGB}
}
