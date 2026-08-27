# SOL-PRF-02: `approvedModels` fallback and `allowedServerTags` intersection in `ResolveProfile`

**Resolves:** [BUG-PRF-02](../BUG-PRF-02-profile-inheritance-approvedmodels-servertags-missing.md)
**Service:** `tenant-service`
**Affected files (proposed):**
- `backend-go/services/tenant-service/internal/domain/profile_resolution.go` (edit: two new merge steps)
- `backend-go/services/tenant-service/internal/domain/profile_resolution_test.go` (edit: new coverage)
**Status:** 📋 Proposed — not yet implemented

---

## Design rationale (grounded in TDD)

This is a narrow, contained gap: `tenant-service.md` §4 already documents
`ResolveProfile` as "the one real piece of business logic here... a pure
domain function" performing "the actual 4-layer... deep merge," and lists
its merge rules (security lock, `pathAdditions` concat, `envVars`
override-merge, `_sources` attribution) as things the current
implementation already gets right per BUG-PRF-02's own "What backend-go
has" section. The two missing rules — `agent.approvedModels` fallback and
`fleet.allowedServerTags` intersection — are exactly two more rows in the
same table (`docs/logic/profile/BL-PRF-02-profile-inheritance.md:78-89`'s
Merge Rules chi tiết), not a different kind of logic. The existing
implementation already has the right *shape* for adding them:
`mergePathAdditions`/`mergeMCPServers` (`profile_resolution.go:188-272`)
are both "override whatever the generic per-key merge produced for this one
field with a hand-written rule," called from `ResolveProfile`
(`profile_resolution.go:89-103`) after the generic pass — this solution
adds two more calls in that exact spot, no structural change.

`ResolveProfile`'s signature already receives `company Settings` as its own
parameter (`profile_resolution.go:89`, used today only by `lockSecurity`)
— the `approvedModels` fallback needs exactly this same value (company is
the sole source of the approved list, per BL-PRF-02 §step 7 and the Merge
Rules table's "Company only" row), so no signature change is needed.

---

## Design — domain (`profile_resolution.go`)

```go
// ResolveProfile (edit): two new calls after the existing three.
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
	applyApprovedModelsFallback(resolved, sources, company)
	mergeAllowedServerTags(resolved, sources, layers)

	return ResolvedProfile{Settings: resolved, Sources: sources}, nil
}
```

### `approvedModels` fallback

```go
// Special-cased keys this file already declares (extended):
const (
	agentKey            = "agent"
	preferredModelKey   = "preferredModel"
	approvedModelsKey   = "approvedModels"
	modelFallbackKey    = "_modelFallbackReason"
	fleetKey            = "fleet"
	allowedServerTagsKey = "allowedServerTags"
)

// applyApprovedModelsFallback enforces BL-PRF-02 step 7 / Merge Rules
// table's "agent.approvedModels: Company only — user/dept cannot expand
// list" rule: if company defines a non-empty approvedModels list and the
// resolved (post-merge, so possibly dept/team/user-overridden)
// agent.preferredModel isn't in it, force it back to approvedModels[0] and
// record why in agent._modelFallbackReason — mirrors the TS reference
// algorithm's step 7 (docs/logic/profile/BL-PRF-02-profile-inheritance.md:60-67)
// verbatim, including the human-readable reason string shape.
func applyApprovedModelsFallback(resolved Settings, sources map[string]string, company Settings) {
	companyAgent, ok := asMap(emptySettings(company)[agentKey])
	if !ok {
		return
	}
	rawModels, ok := companyAgent[approvedModelsKey].([]any)
	if !ok || len(rawModels) == 0 {
		return // no restriction configured — nothing to enforce
	}
	approved := make([]string, 0, len(rawModels))
	allowed := map[string]bool{}
	for _, m := range rawModels {
		if name, ok := m.(string); ok && name != "" {
			approved = append(approved, name)
			allowed[name] = true
		}
	}
	if len(approved) == 0 {
		return
	}

	resolvedAgent, ok := asMap(resolved[agentKey])
	if !ok {
		return // no agent section at all in the resolved profile — nothing to fall back
	}
	preferred, _ := resolvedAgent[preferredModelKey].(string)
	if preferred == "" || allowed[preferred] {
		return // unset, or already approved — no fallback needed
	}

	resolvedAgent[preferredModelKey] = approved[0]
	resolvedAgent[modelFallbackKey] = fmt.Sprintf("%q not in approved list", preferred)
	resolved[agentKey] = Settings(resolvedAgent)
	sources[agentKey+"."+preferredModelKey] = SourceCompany
}
```

Notes on fidelity to the spec pseudocode
(`docs/logic/profile/BL-PRF-02-profile-inheritance.md:60-67`):
- Runs strictly **after** the generic merge pass (which already applied
  dept/team/user's own `preferredModel` overrides) — the fallback corrects
  the *post-merge* value, not the company's own default, matching "if
  `company.approvedModels?.length > 0`... `resolved.agent.preferredModel`."
- `_sources["agent.preferredModel"]` is overwritten to `SourceCompany` when
  a fallback fires — the winning value *is* company's, even though the
  pre-fallback attribution belonged to whichever layer set the rejected
  preference. This mirrors `mergePathAdditions`/`mergeMCPServers`'s existing
  convention of overwriting `sources` for any field they hand-correct.
- No fallback fires when `company.approvedModels` is empty/absent —
  matches "if (company.approvedModels?.length > 0)" guarding the whole
  block in the reference algorithm; an unrestricted company imposes no
  fallback.

### `fleet.allowedServerTags` intersection

```go
// mergeAllowedServerTags enforces the Merge Rules table's "Intersect: user
// subset ⊆ dept ⊆ company" rule — a lower layer can only NARROW the tag
// set, never expand it, unlike the generic per-key merge's last-write-wins
// replacement (which would let a dept/user layer's array simply overwrite
// company's wholesale — the exact bug BUG-PRF-02 reports). Runs
// company-first through user-last; a layer that doesn't define the field
// leaves the running set unchanged (doesn't reset to unrestricted).
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

Notes:
- **Order-independence of the result** (but not of `sources`): because
  intersection is commutative, the final tag set is the same regardless of
  which layers define the field, but `sources` records the layers *in
  priority order* they contributed a narrowing constraint — same
  `"+"`-joined convention `mergePathAdditions` already established for
  multi-layer-contributed fields.
- **Company-absent = unrestricted, department/user still narrows from
  nothing.** If company never sets `allowedServerTags` but department does,
  `running` starts from department's set (first-definer-establishes-baseline),
  matching the spec's literal "user subset ⊆ dept ⊆ company" only where
  company actually constrains something — an absent company constraint
  isn't "the empty set," it's "no constraint," so a department that DOES
  set the field is still meaningfully enforced. This reading is this
  solution's own interpretation, not spelled out explicitly in BL-PRF-02 —
  flagged as a genuine ambiguity resolution, not a citation.
- Consumers of this field (BUG-PRF-03's `getProjectsForUser` visibility
  filter) read `resolved.fleet.allowedServerTags` — absent key means
  "unrestricted," an empty array means "no servers allowed" (a company/dept
  that explicitly sets `allowedServerTags: []` locks a user out of every
  server) — this distinction (`nil` map vs. present-but-empty) is preserved
  by `definedAny`/`running` staying separate variables above, not collapsed
  into one nil-vs-empty-slice check.

---

## Test plan

- `profile_resolution_test.go` — `approvedModels` fallback:
  - preferred model in company's approved list → unchanged, no
    `_modelFallbackReason`, `_sources["agent.preferredModel"]` stays
    whichever layer actually set it (fallback doesn't fire, so it doesn't
    overwrite that attribution).
  - preferred model NOT in list → forced to `approvedModels[0]`,
    `_modelFallbackReason` set to the exact expected string, `_sources`
    overwritten to `"company"`.
  - `company.approvedModels` absent/empty → no fallback regardless of
    `preferredModel`'s value (even an obviously bogus model name passes
    through unmodified — write-time validation, SOL-PRF-01, is what would
    have caught a bad company list in the first place; this merge step
    trusts an already-valid company list).
  - `resolved.agent` section entirely absent (no layer ever set any
    `agent.*` field) → no panic, no fallback (guards the `asMap` failure
    path).
- `profile_resolution_test.go` — `allowedServerTags` intersection:
  - company `["gpu","eu"]`, department `["gpu"]`, user unset → resolved
    `["gpu"]` (department narrows; user inherits the narrowed set).
  - company `["gpu","eu"]`, user `["gpu","asia"]` (attempting to expand
    with `"asia"`, not in company's set) → resolved `["gpu"]` only —
    regression guard against the exact bug this solution closes ("a
    department or user profile could... expand the tag set").
  - no layer defines `allowedServerTags` at all → resolved has no
    `fleet.allowedServerTags` key (not an empty array) — distinguishes
    "unrestricted" from "explicitly locked to zero servers."
  - company doesn't define it, department sets `["gpu"]` → resolved
    `["gpu"]` (baseline established by the first definer, per this file's
    "Company-absent = unrestricted" note).
  - department explicitly sets `[]` → resolved `[]` (explicit lockout,
    distinct from "not defined").
  - deterministic ordering: same input layers in any construction order
    produce the same sorted output slice.
- Full-stack regression: `get_resolved_profile_test.go` (existing usecase
  test file) gains one case combining both new rules with the existing
  merge (security lock + pathAdditions + these two) in one resolve call, to
  guard against a future edit to `ResolveProfile`'s call order breaking
  either new step's interaction with the others.

## References

- `specs/backend-go/tdd/services/tenant-service.md:114-150` (§4 domain
  model — `ResolveProfile` as pure function, merge rules table, `_sources`
  convention)
- `backend-go/services/tenant-service/internal/domain/profile_resolution.go:59-103`
  (`ResolveProfile`'s existing call sequence this solution extends),
  `:188-272` (`mergePathAdditions`/`mergeMCPServers` — the exact structural
  pattern `applyApprovedModelsFallback`/`mergeAllowedServerTags` follow)
- `backend-go/services/tenant-service/internal/domain/settings.go:7-45`
  (`Settings`/`asMap`/`emptySettings` helpers reused, not reinvented)
- `docs/logic/profile/BL-PRF-02-profile-inheritance.md:19-74` (full
  `resolveProfile()` reference algorithm, steps 6-7), `:78-89` (Merge Rules
  chi tiết table — the two missing rows this solution closes), `:129-138`
  (acceptance criteria — "approvedModels validation với fallback" row)
- `specs/backend-go/bugs/logic-v1/BUG-PRF-03-project-devserver-assignment-partial.md`
  (See also section — this solution is the prerequisite BUG-PRF-03's
  `allowedServerTags`-based visibility filter needs `resolved.fleet.allowedServerTags`
  to exist at all)
