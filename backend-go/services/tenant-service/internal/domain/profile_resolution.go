package domain

import (
	"sort"
	"strings"
)

// Layer-source labels recorded in ResolvedProfile.Sources — ported from the
// TS ProfileResolver's convention so a debugging session can answer "why
// did this user get this setting" without re-deriving the cascade by hand
// (tenant-service.md §4).
const (
	SourceCompany    = "company"
	SourceDepartment = "department"
	SourceUser       = "user"
)

// Special-cased top-level keys, handled outside the generic per-key merge —
// see ResolveProfile's doc comment for the rules each one implements.
const (
	securityKey      = "security"
	shellKey         = "shell"
	pathAdditionsKey = "pathAdditions"
	mcpKey           = "mcp"
	serversKey       = "servers"
	nameKey          = "name"
)

// TeamSource builds the _sources label for a team layer, e.g. "team:t1".
func TeamSource(teamID string) string {
	return "team:" + teamID
}

// TeamSettingsLayer is one team's contribution to profile resolution: its
// Settings plus the TeamMember.Priority tiebreaker (tenant-service.md §4).
// Pre-fetched by the usecase layer — ResolveProfile itself does no I/O and
// doesn't care where these came from.
type TeamSettingsLayer struct {
	TeamID   string
	Priority int32
	Settings Settings
}

// ResolvedProfile is the deep-merge output: the merged Settings plus a
// per-field Sources map recording which layer won each leaf field (dot-path
// -> source label, e.g. "agent.model" -> "team:t1").
type ResolvedProfile struct {
	Settings Settings
	Sources  map[string]string
}

// profileLayer is the internal, already-ordered representation ResolveProfile
// folds left-to-right (lowest to highest priority).
type profileLayer struct {
	source   string
	settings Settings
}

// ResolveProfile deep-merges four already-fetched Settings layers into one
// ResolvedProfile — the one real piece of business logic in tenant-service,
// kept as a pure function with zero I/O/repository/cache awareness, per
// specs/backend-go/architecture/03-clean-architecture-guidelines.md and
// tenant-service.md §4/§6.
//
// Layer order, lowest to highest priority: company < department < teams
// (ascending TeamSettingsLayer.Priority, ties broken by TeamID for
// determinism) < user. department may be nil (no department assigned —
// company-only inheritance); teams may be nil/empty; user may be nil/empty
// (no per-user overrides yet). company should always be provided (the
// tenant root) but a nil/empty value is handled the same as any other layer
// rather than treated as an error.
//
// Merge rules (tenant-service.md §4, ported from the TS reference
// implementation ProfileResolver.ts):
//   - "security" section: company-locked. No other layer can ever override
//     any field under it, even if a later layer defines "security" —
//     enforced here in the domain function, not left to caller convention.
//   - Every other field: deep-merged key by key; for a given leaf field,
//     the highest-priority layer that defines it wins (a lower layer's
//     sibling fields under the same parent object are preserved, not
//     clobbered by a higher layer that only redefines one of them).
//   - "shell.pathAdditions": concatenated across every layer that defines
//     it, company-first through user-last — additive, never overridden.
//   - "mcp.servers": treated as a list of objects keyed by each entry's
//     "name" field, deduplicated across layers; on a name collision the
//     highest-priority layer's entry wins, kept in the position it was
//     first seen. Unnamed entries are dropped — they can't be deduplicated
//     or attributed to a source.
func ResolveProfile(company, department Settings, teams []TeamSettingsLayer, user Settings) (ResolvedProfile, error) {
	layers := buildLayers(company, department, teams, user)

	resolved := Settings{}
	sources := map[string]string{}
	for _, l := range layers {
		mergeInto(resolved, sources, "", withoutTopKey(l.settings, securityKey), l.source)
	}

	lockSecurity(resolved, sources, company)
	mergePathAdditions(resolved, sources, layers)
	mergeMCPServers(resolved, sources, layers)

	return ResolvedProfile{Settings: resolved, Sources: sources}, nil
}

// buildLayers orders the four layer kinds lowest-to-highest priority,
// sorting multiple teams ascending by Priority (ties broken by TeamID).
func buildLayers(company, department Settings, teams []TeamSettingsLayer, user Settings) []profileLayer {
	sortedTeams := make([]TeamSettingsLayer, len(teams))
	copy(sortedTeams, teams)
	sort.SliceStable(sortedTeams, func(i, j int) bool {
		if sortedTeams[i].Priority != sortedTeams[j].Priority {
			return sortedTeams[i].Priority < sortedTeams[j].Priority
		}
		return sortedTeams[i].TeamID < sortedTeams[j].TeamID
	})

	layers := make([]profileLayer, 0, 3+len(sortedTeams))
	layers = append(layers, profileLayer{SourceCompany, emptySettings(company)})
	layers = append(layers, profileLayer{SourceDepartment, emptySettings(department)})
	for _, t := range sortedTeams {
		layers = append(layers, profileLayer{TeamSource(t.TeamID), emptySettings(t.Settings)})
	}
	layers = append(layers, profileLayer{SourceUser, emptySettings(user)})
	return layers
}

// withoutTopKey returns a shallow copy of s without the given top-level
// key, or s itself if the key isn't present (no copy needed) — used to keep
// "security" out of the generic merge pass entirely, so it's handled
// exclusively by lockSecurity.
func withoutTopKey(s Settings, key string) Settings {
	if _, ok := s[key]; !ok {
		return s
	}
	out := make(Settings, len(s))
	for k, v := range s {
		if k == key {
			continue
		}
		out[k] = v
	}
	return out
}

// mergeInto recursively deep-merges src into dst: nested objects merge
// key-by-key (a lower layer's sibling fields survive a higher layer that
// only redefines some of them); any non-object value (scalar or array) is
// replaced wholesale by the higher-priority layer and its winning source is
// recorded at its dot-path.
func mergeInto(dst Settings, sources map[string]string, path string, src Settings, source string) {
	for k, v := range src {
		childPath := k
		if path != "" {
			childPath = path + "." + k
		}
		if childMap, ok := asMap(v); ok {
			existingMap, ok := asMap(dst[k])
			if !ok {
				existingMap = map[string]any{}
			}
			existing := Settings(existingMap)
			mergeInto(existing, sources, childPath, Settings(childMap), source)
			dst[k] = existing
			continue
		}
		dst[k] = v
		sources[childPath] = source
	}
}

// lockSecurity sets resolved["security"] exclusively from company's
// "security" section, regardless of what any other layer defined under
// that key (which the generic merge pass never even saw — see
// withoutTopKey in ResolveProfile). This is the domain-layer enforcement of
// tenant-service.md §9's "security profile-section values ... the merge
// algorithm's refusal to let department/team/user override that section is
// a security control, enforced in the domain layer".
func lockSecurity(resolved Settings, sources map[string]string, company Settings) {
	secVal, ok := asMap(emptySettings(company)[securityKey])
	if !ok {
		return
	}
	locked := Settings{}
	mergeInto(locked, sources, "", Settings{securityKey: secVal}, SourceCompany)
	resolved[securityKey] = locked[securityKey]
}

// mergePathAdditions concatenates shell.pathAdditions across every layer
// that defines it, company-first through user-last, overriding whatever the
// generic per-key merge produced for that one field (which would otherwise
// have just taken the highest-priority layer's list wholesale).
func mergePathAdditions(resolved Settings, sources map[string]string, layers []profileLayer) {
	var additions []any
	var contributing []string
	for _, l := range layers {
		shell, ok := asMap(l.settings[shellKey])
		if !ok {
			continue
		}
		items, ok := shell[pathAdditionsKey].([]any)
		if !ok || len(items) == 0 {
			continue
		}
		additions = append(additions, items...)
		contributing = append(contributing, l.source)
	}
	if len(additions) == 0 {
		return
	}

	shell, ok := asMap(resolved[shellKey])
	if !ok {
		shell = map[string]any{}
	}
	shell[pathAdditionsKey] = additions
	resolved[shellKey] = Settings(shell)
	sources[shellKey+"."+pathAdditionsKey] = strings.Join(contributing, "+")
}

// mergeMCPServers deduplicates mcp.servers by each entry's "name" field
// across every layer that defines it: on a name collision the
// highest-priority layer's entry wins, kept in the position it was first
// seen. Overrides whatever the generic per-key merge produced for that one
// field (which would otherwise have just taken the highest-priority layer's
// list wholesale, losing entries only a lower layer defined).
func mergeMCPServers(resolved Settings, sources map[string]string, layers []profileLayer) {
	var order []string
	byName := map[string]any{}
	sourceByName := map[string]string{}

	for _, l := range layers {
		mcp, ok := asMap(l.settings[mcpKey])
		if !ok {
			continue
		}
		items, ok := mcp[serversKey].([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			entry, ok := asMap(item)
			if !ok {
				continue
			}
			name, _ := entry[nameKey].(string)
			if name == "" {
				continue
			}
			if _, seen := byName[name]; !seen {
				order = append(order, name)
			}
			byName[name] = entry
			sourceByName[name] = l.source
		}
	}
	if len(order) == 0 {
		return
	}

	servers := make([]any, 0, len(order))
	for _, name := range order {
		servers = append(servers, byName[name])
		sources[mcpKey+"."+serversKey+"."+name] = sourceByName[name]
	}

	mcp, ok := asMap(resolved[mcpKey])
	if !ok {
		mcp = map[string]any{}
	}
	mcp[serversKey] = servers
	resolved[mcpKey] = Settings(mcp)
}
