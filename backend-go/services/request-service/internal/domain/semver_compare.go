package domain

import (
	"errors"
	"regexp"
	"strconv"
)

var semverRegex = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)

type Semver struct {
	Major int
	Minor int
	Patch int
}

func ParseSemver(s string) (Semver, error) {
	matches := semverRegex.FindStringSubmatch(s)
	if len(matches) < 4 {
		return Semver{}, errors.New("invalid semver format")
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	return Semver{Major: major, Minor: minor, Patch: patch}, nil
}

func CompareSemver(a, b Semver) int {
	if a.Major != b.Major {
		if a.Major > b.Major {
			return 1
		}
		return -1
	}
	if a.Minor != b.Minor {
		if a.Minor > b.Minor {
			return 1
		}
		return -1
	}
	if a.Patch != b.Patch {
		if a.Patch > b.Patch {
			return 1
		}
		return -1
	}
	return 0
}
