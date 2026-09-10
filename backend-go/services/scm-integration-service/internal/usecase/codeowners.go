package usecase

import (
	"path/filepath"
	"strings"
)

// CodeownersRule is one parsed CODEOWNERS line: a gitignore-style pattern
// plus the logins/team-slugs it maps to.
type CodeownersRule struct {
	Pattern string
	Owners  []string // raw tokens, e.g. "@alice", "@org/team-frontend"
}

// ParseCodeowners parses a CODEOWNERS file's content into ordered
// (pattern, owners) rules — gitignore-style patterns, in file order.
// Blank lines and lines starting with "#" are skipped. Pure function, no
// I/O, unit-testable without any port.
func ParseCodeowners(content string) []CodeownersRule {
	var rules []CodeownersRule
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue // a pattern with no owners is not a usable rule
		}
		rules = append(rules, CodeownersRule{Pattern: fields[0], Owners: fields[1:]})
	}
	return rules
}

// MatchOwners returns the union of owners for every changedFile, applying
// last-match-wins per file — the last rule in the file whose pattern
// matches a given path wins for that path, per GitHub/GitLab's own
// documented CODEOWNERS semantics (later rules override earlier ones).
func MatchOwners(rules []CodeownersRule, changedFiles []string) (logins, teams []string) {
	seen := make(map[string]bool)
	var loginsOut, teamsOut []string

	for _, file := range changedFiles {
		var winner *CodeownersRule
		for i := range rules {
			if matchesCodeownersPattern(rules[i].Pattern, file) {
				winner = &rules[i]
			}
		}
		if winner == nil {
			continue
		}
		for _, owner := range winner.Owners {
			if seen[owner] {
				continue
			}
			seen[owner] = true
			if strings.Contains(owner, "/") {
				teamsOut = append(teamsOut, strings.TrimPrefix(owner, "@"))
			} else {
				loginsOut = append(loginsOut, strings.TrimPrefix(owner, "@"))
			}
		}
	}
	return loginsOut, teamsOut
}

// matchesCodeownersPattern reports whether a gitignore-style CODEOWNERS
// pattern matches path. Supports "*" (any path), a trailing "/" (directory
// prefix), and filepath.Match-style single-segment globs — the common
// subset every provider's CODEOWNERS docs describe, not full gitignore
// syntax (double-star recursive globs are treated as a directory-prefix
// match, the closest reasonable approximation without a full gitignore
// matcher dependency).
func matchesCodeownersPattern(pattern, path string) bool {
	if pattern == "*" {
		return true
	}
	pattern = strings.TrimPrefix(pattern, "/")
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern) || strings.HasPrefix(path, strings.TrimSuffix(pattern, "/")+"/")
	}
	if strings.Contains(pattern, "**") {
		prefix := strings.SplitN(pattern, "**", 2)[0]
		return strings.HasPrefix(path, prefix)
	}
	if matched, _ := filepath.Match(pattern, path); matched {
		return true
	}
	// Fall back to a base-name match for a bare filename pattern (e.g.
	// "CODEOWNERS" matching "docs/CODEOWNERS"), matching common provider
	// behavior for patterns with no "/".
	if !strings.Contains(pattern, "/") {
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		return matched
	}
	return false
}
