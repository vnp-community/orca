package domain

import (
	"path"
	"sort"
	"strings"
)

// Release flags a reviewer must see before anything ships.
const (
	FlagMigration = "migration"
	FlagConfig    = "config"
	FlagBreaking  = "breaking"
)

// ChangedPath is one file the code changed, as git-gateway reports it.
type ChangedPath struct {
	Path    string
	OldPath string
	Status  string
	Added   int
	Removed int
}

// PathClasses is what the changed paths say about a release. Areas counts files by their first directory.
type PathClasses struct {
	Migration bool
	Config    bool
	Areas     map[string]int
	Files     int
	Added     int
	Removed   int
}

// ClassifyChangedPaths looks only at paths. A path inside a migrations directory means a schema change to deploy first; a
// deployment file (compose, env example, deploy/) means a configuration change.
func ClassifyChangedPaths(changes []ChangedPath) PathClasses {
	pc := PathClasses{Areas: map[string]int{}}
	for _, c := range changes {
		p := normalizeRel(c.Path)
		if p == "" {
			continue
		}
		pc.Files++
		pc.Added += c.Added
		pc.Removed += c.Removed
		pc.Areas[firstSegment(p)]++
		switch {
		case isMigrationPath(p):
			pc.Migration = true
		case isDeploymentConfigPath(p):
			pc.Config = true
		}
	}
	return pc
}

// ReleaseFlags returns the flags in a fixed order. breaking is nil when nobody measured it: the flag is then left out and the
// caller says "unknown" instead of "no".
func ReleaseFlags(pc PathClasses, breaking *bool) []string {
	var out []string
	if pc.Migration {
		out = append(out, FlagMigration)
	}
	if pc.Config {
		out = append(out, FlagConfig)
	}
	if breaking != nil && *breaking {
		out = append(out, FlagBreaking)
	}
	return out
}

func normalizeRel(p string) string {
	p = strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	p = strings.TrimPrefix(p, "./")
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return ""
	}
	return path.Clean(p)
}

func firstSegment(p string) string {
	if i := strings.IndexByte(p, '/'); i > 0 {
		return p[:i]
	}
	return "."
}

func isMigrationPath(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if seg == "migrations" || seg == "migration" {
			return true
		}
	}
	return false
}

func isDeploymentConfigPath(p string) bool {
	base := path.Base(p)
	switch {
	case strings.HasPrefix(p, "deploy/"), strings.Contains(p, "/deploy/"):
		return true
	case strings.HasPrefix(base, "docker-compose"), base == "compose.yaml", base == "compose.yml":
		return true
	case base == ".env.example", strings.HasSuffix(base, ".env.example"), strings.HasSuffix(base, ".env.sample"):
		return true
	}
	return false
}

// SortedAreas lists the areas by file count, then name, for a stable display.
func (pc PathClasses) SortedAreas() []string {
	names := make([]string, 0, len(pc.Areas))
	for n := range pc.Areas {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if pc.Areas[names[i]] != pc.Areas[names[j]] {
			return pc.Areas[names[i]] > pc.Areas[names[j]]
		}
		return names[i] < names[j]
	})
	return names
}
