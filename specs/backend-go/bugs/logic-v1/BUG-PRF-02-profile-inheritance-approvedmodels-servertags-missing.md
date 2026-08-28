# BUG-PRF-02: Profile-resolution merge algorithm is real and correct, but approvedModels fallback and allowedServerTags intersection are missing

**Business Logic:** [BL-PRF-02](../../../../docs/logic/profile/BL-PRF-02-profile-inheritance.md) — Profile Inheritance Resolution (3-layer merge)
**Priority (per spec):** P0
**Status:** PARTIAL
**Severity:** Medium
**Symptom:** A user whose personal or department profile sets `agent.preferredModel` to a model the company has NOT approved gets that unapproved model back from `profile.getResolved` unchanged — no fallback to `company.approvedModels[0]`, no `_modelFallbackReason` explanation surfaced to the UI. Similarly, a developer whose profile is supposed to be restricted to a subset of `fleet.allowedServerTags` gets no enforcement of that subset at resolution time — nothing in the merge narrows the tag set.

---

## Spec summary

`ProfileResolver.resolveProfile()` deep-merges Company → Department → User (and per this codebase's extension, Team) into a `ResolvedProfile`, with `security.*` company-locked, `shell.pathAdditions` concatenated (not replaced), `shell.envVars` merged with later layers overriding earlier ones, and two additional Company-anchored guards: (1) if `agent.preferredModel` isn't in `company.approvedModels`, it's forced back to `approvedModels[0]` with a `_modelFallbackReason`; (2) `fleet.allowedServerTags` is an intersection — a lower layer can only narrow, never expand, the tag set. A `_sources` map records which layer won each field, and results are cached with TTL=60s, invalidated per-scope on writes.

## What backend-go has

This is real, tested domain logic — not just an RPC wrapper — and covers most of the spec faithfully:

- `domain.ResolveProfile` (`backend-go/services/tenant-service/internal/domain/profile_resolution.go:89-103`) is a pure function performing the actual 4-layer (company/department/team/user) deep merge, called from the usecase `backend-go/services/tenant-service/internal/usecase/get_resolved_profile.go:74`.
- Security lock: `lockSecurity` (`profile_resolution.go:171-186`) forces `resolved.security` from `company` only, regardless of what dept/team/user define — matches spec exactly.
- `shell.pathAdditions` concatenation across every layer, company-first through user-last: `mergePathAdditions` (`profile_resolution.go:188-218`).
- `shell.envVars` override-merge (later layer's keys win over earlier) is achieved for free by the generic recursive `mergeInto` (`profile_resolution.go:150-169`), since `envVars` is a nested object, not a scalar/array — this satisfies the spec's "user keys override dept keys" rule without needing a special case.
- `_sources` per-field attribution: `sources` map populated by every merge path (`mergeInto` line 167, `lockSecurity` line 184, `mergePathAdditions` line 217, `mergeMCPServers` line 263) — matches the spec's `ResolvedProfileWithMeta._sources` concept, plus an `mcp.servers` name-keyed dedup the spec doesn't even ask for.
- 60s TTL cache: `CachedGetResolvedProfile` (`backend-go/services/tenant-service/internal/usecase/cached_get_resolved_profile.go:16,31-36`).
- Cache invalidation scoping matches the spec's table (company update → all users, department update → dept members, user/team change → that user only) — see `update_company.go:39-48`, `update_department.go:47-56`, `update_user_profile.go:65-71`, `set_user_department.go:69-76`, `add_team_member.go:61`, `remove_team_member.go:46`.

## What's missing

- **`approvedModels` validation + fallback is entirely absent.** `grep -rn "approvedModel" backend-go/services/tenant-service/` returns zero matches. `ResolveProfile` never reads `company.approvedModels` nor checks `resolved.agent.preferredModel` against it, so step 7 of the spec's algorithm (fallback to `approvedModels[0]` + `_modelFallbackReason`) never runs.
- **`fleet.allowedServerTags` intersection is entirely absent.** `grep -rn "allowedServerTags" backend-go/services/tenant-service/` returns zero matches. The generic merge (`mergeInto`) treats `allowedServerTags` as an ordinary array field — meaning the highest-priority layer's array simply replaces lower layers' wholesale (last-write-wins), not the spec's required intersect-only-narrows semantics. A department or user profile could therefore *expand* the tag set relative to company, which the spec explicitly forbids.

## See also

- `specs/backend-go/bugs/missing-v1/BUG-019-profile-channels-not-implemented.md` — covers the (now-resolved) wscompat wiring gap for `profile.getResolved`; this report is about the merge algorithm's own completeness, not wiring.

## References

- `backend-go/services/tenant-service/internal/domain/profile_resolution.go:89-272` — `ResolveProfile` and its helpers (full merge algorithm)
- `backend-go/services/tenant-service/internal/usecase/get_resolved_profile.go:34-79` — layer-fetch + merge orchestration
- `backend-go/services/tenant-service/internal/usecase/cached_get_resolved_profile.go` — TTL cache decorator
- `docs/logic/profile/BL-PRF-02-profile-inheritance.md:60-88` — Merge Rules chi tiết table (approvedModels, allowedServerTags rows)
- `docs/logic/profile/BL-PRF-02-profile-inheritance.md:129-138` — Tiêu chí chấp nhận (acceptance criteria) — "approvedModels validation với fallback" is unchecked/unimplemented
