# TASK-PRF-02-02: Add `fleet.allowedServerTags` intersection to `ResolveProfile`

**From Solution:** SOL-PRF-02
**Priority:** P0
**Service:** `tenant-service`
**File:** `backend-go/services/tenant-service/internal/domain/profile_resolution.go`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

The Merge Rules table's second missing row: `fleet.allowedServerTags` must
**intersect** across layers (a lower layer can only narrow the set, never
expand it), but today's generic per-key merge does last-write-wins
replacement — the exact bug BUG-PRF-02 reports, and the prerequisite
BUG-PRF-03's dev-server visibility filter needs this field to exist at all.
Independent of TASK-PRF-02-01 (different field, same file) — either can land
first.

## Changes to make

In `backend-go/services/tenant-service/internal/domain/profile_resolution.go`,
extend the special-cased-keys `const` block:

```go
	fleetKey             = "fleet"             // NEW
	allowedServerTagsKey = "allowedServerTags"  // NEW
```

Add the call in `ResolveProfile`, after `applyApprovedModelsFallback` (or
after the existing three calls if TASK-PRF-02-01 hasn't landed yet — order
between the two new calls doesn't matter, they touch disjoint fields):

```go
	mergeAllowedServerTags(resolved, sources, layers) // NEW
```

Add the function itself:

```go
// mergeAllowedServerTags enforces the Merge Rules table's "Intersect: user
// subset ⊆ dept ⊆ company" rule — a lower layer can only NARROW the tag
// set, never expand it, unlike the generic per-key merge's last-write-wins
// replacement. Runs company-first through user-last; a layer that doesn't
// define the field leaves the running set unchanged (doesn't reset to
// unrestricted). Company-absent = unrestricted: if company never sets the
// field but department does, department's set becomes the baseline
// (first-definer-establishes-baseline) — this reading is this task's own
// interpretation of an ambiguity BL-PRF-02 doesn't spell out explicitly.
func mergeAllowedServerTags(resolved Settings, sources map[string]string, layers []profileLayer) {
	var running map[string]bool // nil = unrestricted (no layer has defined the field yet)
	var contributing []string
	definedAny := false

	for _, l := range layers {
		fleet, ok := asMap(l.settings[fleetKey])
		if !ok {
			continue
		}
		items, ok := fleet[allowedServerTagsKey].([]any)
		if !ok {
			continue
		}
		definedAny = true
		layerTags := map[string]bool{}
		for _, it := range items {
			if tag, ok := it.(string); ok && tag != "" {
				layerTags[tag] = true
			}
		}
		if running == nil {
			running = layerTags // first layer to define it establishes the baseline
		} else {
			for tag := range running {
				if !layerTags[tag] {
					delete(running, tag) // intersect: drop anything the new layer doesn't also allow
				}
			}
		}
		contributing = append(contributing, l.source)
	}
	if !definedAny {
		return // no layer restricts server tags — resolved profile has no fleet.allowedServerTags key, same as today
	}

	tags := make([]any, 0, len(running))
	for tag := range running {
		tags = append(tags, tag)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].(string) < tags[j].(string) }) // deterministic output

	fleet, ok := asMap(resolved[fleetKey])
	if !ok {
		fleet = map[string]any{}
	}
	fleet[allowedServerTagsKey] = tags
	resolved[fleetKey] = Settings(fleet)
	sources[fleetKey+"."+allowedServerTagsKey] = strings.Join(contributing, "+") // same multi-source convention as mergePathAdditions
}
```

`sort`/`strings` are already imported by this file — no new imports needed.

**Nil vs. present-but-empty distinction — do not collapse.** Absent key means
"unrestricted" (no filtering downstream); a present, explicitly-empty
`allowedServerTags: []` means "no servers allowed" — `definedAny`/`running`
must stay separate variables (as above), not merged into one nil-vs-empty
check, so `BUG-PRF-03`'s visibility filter can tell the two apart.

## Verify

```bash
cd /opt/repos/orca/backend-go
go build ./services/tenant-service/...
go test ./services/tenant-service/internal/domain/... -run ResolveProfile -v
```

Add test cases to `profile_resolution_test.go` per SOL-PRF-02's Test plan:
company `["gpu","eu"]` + department `["gpu"]` -> resolved `["gpu"]`; company
`["gpu","eu"]` + user attempting to expand with `["gpu","asia"]` -> resolved
`["gpu"]` only (regression guard for the bug this closes); no layer defines
the field -> resolved has no `fleet.allowedServerTags` key at all (not an
empty array); company absent, department `["gpu"]` -> resolved `["gpu"]`;
department explicitly sets `[]` -> resolved `[]` (explicit lockout, distinct
from "not defined"); deterministic ordering regardless of layer construction
order.
