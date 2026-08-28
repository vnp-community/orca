package sshrelay

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/stablyai/orca-go/services/infra-fleet-service/internal/adapter/sshconn"
)

// PrereqResult is the outcome of one remote prerequisite probe — Node/Git
// versions and free disk space on the target host, checked over the raw
// SSH connection before the agent is deployed (BL-FLEET-02's step
// ordering: SSH connect -> check prerequisites -> deploy).
type PrereqResult struct {
	NodeVersion, GitVersion string
	NodeOK, GitOK, DiskOK   bool
	FreeDiskGB              float64
}

// Met reports whether every prerequisite check passed.
func (r PrereqResult) Met() bool {
	return r.NodeOK && r.GitOK && r.DiskOK
}

// ErrPrerequisitesNotMet signals a soft-fail: the remote host does not meet
// the minimum prerequisites, but deploy is still attempted (see
// Provisioner.Provision) — the usecase layer (TASK-FLEET-02-05) maps this
// to domain.DevServerStatusDegraded rather than Unhealthy, and it does not
// consume a retry attempt.
var ErrPrerequisitesNotMet = errors.New("sshrelay: remote host does not meet minimum prerequisites")

// minNodeVersion/minGitVersion match BL-FLEET-02: Node >= 22, Git >= 2.25
// (docs/reference/git-compatibility.md's core-workflow baseline — a
// different concern from this: that doc covers git-gateway-service's own
// executor, this checks the TARGET dev server's git).
var (
	minNodeVersion = version{Major: 22}
	minGitVersion  = version{Major: 2, Minor: 25}
)

const minFreeDiskGB = 5.0

// version is a minimal (major, minor, patch) comparator — this scaffold's
// "at least X.Y" checks don't need a full semver dependency.
type version struct {
	Major, Minor, Patch int
}

func (v version) atLeast(min version) bool {
	if v.Major != min.Major {
		return v.Major > min.Major
	}
	if v.Minor != min.Minor {
		return v.Minor > min.Minor
	}
	return v.Patch >= min.Patch
}

var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

func parseVersion(s string) (version, bool) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return version{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	return version{Major: major, Minor: minor, Patch: patch}, true
}

// parseAndCompareVersion extracts a dotted version number from command
// output (e.g. "v22.3.0" or "git version 2.39.2") and reports it alongside
// whether it meets min. Unparseable output is reported as not-OK, not a
// crash — the raw (trimmed) output is returned so it's still visible for
// diagnostics.
func parseAndCompareVersion(output string, min version) (string, bool) {
	v, ok := parseVersion(output)
	if !ok {
		return strings.TrimSpace(output), false
	}
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch), v.atLeast(min)
}

// parseDiskKB parses `df -P ... | awk '{print $4}'`'s single-number
// available-blocks output. GNU coreutils' df -P (this scaffold's Linux
// target hosts) reports 1024-byte blocks. A value that doesn't parse as a
// number is reported as not-OK, not a crash.
func parseDiskKB(output string) (float64, bool) {
	kb, err := strconv.ParseFloat(strings.TrimSpace(output), 64)
	if err != nil {
		return 0, false
	}
	freeGB := kb / (1024 * 1024)
	return freeGB, freeGB >= minFreeDiskGB
}

// checkPrerequisites runs node --version / git --version / df -P against
// conn and parses the results — the same conn.RunCommand primitive
// deploy.go's checksum step already uses (sshconn.Connection.RunCommand),
// no new transport needed. Never returns an error itself — unparseable
// output or a failed command means the corresponding *OK field stays
// false, not a crash; the caller decides what a failed check means.
func checkPrerequisites(ctx context.Context, conn *sshconn.Connection) (PrereqResult, error) {
	var result PrereqResult

	nodeOut, _, err := conn.RunCommand(ctx, "node --version")
	if err == nil {
		result.NodeVersion, result.NodeOK = parseAndCompareVersion(nodeOut, minNodeVersion)
	}

	gitOut, _, err := conn.RunCommand(ctx, "git --version")
	if err == nil {
		result.GitVersion, result.GitOK = parseAndCompareVersion(gitOut, minGitVersion)
	}

	diskOut, _, err := conn.RunCommand(ctx, "df -P ~ | tail -1 | awk '{print $4}'")
	if err == nil {
		result.FreeDiskGB, result.DiskOK = parseDiskKB(diskOut)
	}

	return result, nil
}
