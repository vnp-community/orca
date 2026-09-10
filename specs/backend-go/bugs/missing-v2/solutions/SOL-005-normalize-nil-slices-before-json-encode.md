# SOL-005: Fix BUG-005 — normalize `nil` slices to `[]` at the `wscompat` response-encoding boundary

**Resolves:** BUG-005
**Service:** `api-gateway`
**Affected files:** `internal/adapter/wscompat/registry.go` (or `session_dialect.go`'s `writeDialectResult`/`handler.go`'s native-dialect equivalent — one shared write-back point, see below), `channels_credentials.go` (one additional, non-boundary fix — see "Two different bugs" below)
**Priority:** Medium
**Status:** 🟡 Proposed — not yet implemented

---

## Grounding in `specs/backend-go/tdd/`

`crs/v0/standards/api-design-guidelines.md` doesn't state a `null`-vs-`[]`
rule explicitly for the legacy `wscompat` bridge (it documents the target
gRPC/REST contract, not this transitional shim), but its "Error model"
section's underlying principle — *clients should get a predictable,
never-ambiguous wire shape, not something that varies by incidental server
state* — applies directly: `frontend/`'s real call sites for these 4
channels (`rpc-catalog.md`: `projectGroup.list`, `ssh.listTargets`,
`team.list`, `credentials.list`) all destructure or iterate the result
immediately (`.groups`, `.targets`, array `.map()`/spread) with no
null-guard, because the TS backend's equivalent handlers always returned
`[]`/`{...: []}` for an empty list — never `null`. `wscompat`'s job (per
`docs/execution-plan.md` §0) is to be wire-compatible with that existing,
unmodified frontend; a response shape that only differs from the TS
backend's when the result happens to be empty is exactly the kind of
"looks fine in every manual test with seed data, breaks on a fresh
tenant's very first render" gap this whole `missing-v2` pass is about.

## Design

### Two different bugs, one shared fix pattern

1. **`projectGroup.list` / `ssh.listTargets` / `team.list`** — each
   handler returns a proto-generated getter (`resp.GetGroups()`,
   `resp.GetSshTargets()`, `resp.GetTeams()`) directly as the RPC result.
   This is fixable **once, at the boundary**, since the bug is purely
   "how does a nil slice serialize", not per-handler logic.
2. **`credentials.list`** — `var services []string` is a **locally
   declared** nil slice inside `handleCredentialsList`, not a proto getter.
   A boundary-level JSON-encoding fix (below) still normalizes this
   correctly on the way out, so one fix covers both cases — no separate
   per-handler change needed for `credentials.list` specifically, despite
   its different origin.

### Boundary-level normalization

Every dispatched result passes through one of two write-back functions
(`writeDialectResult` for the `WebSessionClient` dialect,
`handler.go`'s equivalent `ResultMessage` write for the native dialect) —
both eventually call `wsjson.Write` on a struct whose `Result`/`result`
field is `any`. Add a normalization pass immediately before that call,
shared by both dialects (put it in `Registry.Dispatch`'s return path,
alongside SOL-001's identity-attach fix, so every handler's result is
normalized exactly once regardless of which dialect wrote it):

```go
// registry.go — sketch, alongside Dispatch
func (r *Registry) Dispatch(ctx context.Context, id Identity, channel string, args []json.RawMessage) (any, error) {
	// ... (SOL-001's identity attach + timeout) ...
	result, err := handler(ctx, id, args)
	if err != nil {
		return nil, err
	}
	return normalizeNilSlices(result), nil
}

// normalizeNilSlices recursively replaces a nil slice (top-level, or a nil
// slice field one level into a struct/map) with an empty, non-nil slice of
// the same element type, so encoding/json emits [] instead of null. Uses
// reflection because handler return values are heterogeneous `any` (proto
// message getters, ad hoc structs, maps) — a type-switch per handler
// return type isn't practical at this single shared call site.
func normalizeNilSlices(v any) any {
	if v == nil {
		return v
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			return reflect.MakeSlice(rv.Type(), 0, 0).Interface()
		}
	case reflect.Ptr, reflect.Struct:
		// walk exported fields one level deep, normalizing any nil-slice
		// field in place on a shallow copy — deep enough for every current
		// handler's result shape (a flat struct or proto message wrapping
		// at most one list field), not a general-purpose deep-walk.
		return normalizeStructSliceFields(v)
	}
	return v
}
```

(`normalizeStructSliceFields`'s exact depth/scope is an implementation
detail for whoever picks this up — start with "one level of struct/proto
fields," the shape every current affected handler actually has, and widen
only if a future handler needs it; matches this codebase's stated
preference for the simplest fix that satisfies the actual requirement over
speculative generality.)

### Alternative considered and rejected: fix each handler individually

`if x := resp.GetGroups(); x != nil { return x, nil }; return
[]*projectv1.ProjectGroup{}, nil` at each of the (at least) 4 call sites —
rejected as the primary fix because BUG-005's own report already flags
this pattern as **not exhaustively found** (likely present in more
`wscompat` channels than the 4 confirmed live). A boundary-level fix closes
the entire class, including instances this investigation didn't happen to
reproduce, and prevents the same mistake in every future channel handler —
the same "one shared place beats N call sites" reasoning SOL-001 applies
to identity attachment.

## Testing Plan

- Unit test: `normalizeNilSlices` given a `nil` slice, a struct with a
  `nil`-slice field, a non-nil-but-empty slice (`[]T{}` — should be
  returned unchanged, not double-wrapped), and a fully-populated
  slice/struct (unchanged) — table-driven, covering the actual shapes
  `wscompat`'s handlers return today.
- Regression test per confirmed instance: fake `project-service`/
  `infra-fleet-service`/`tenant-service` clients returning an empty
  `Groups`/`SshTargets`/`Teams` response → assert the channel's dispatched
  result `json.Marshal`s to `[]` (or `{"groups":[]}` etc., matching each
  channel's actual wrapping), not `null`.
- Regression test for `credentials.list`: both upstream clients return zero
  credentials → `services` marshals to `[]`, not `null`.
- Re-run `tests/client/rpc-catalog.spec.ts`'s `projectGroup.list`/
  `ssh.listTargets`/`team.list`/`credentials.list` cases against the fixed
  deployment for a tenant with no existing rows — should move from
  `result:null` to `result:[]`/`{"services":[]}`.
