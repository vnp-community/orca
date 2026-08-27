# TASK-010: Normalize `nil` slices to `[]` in `Registry.Dispatch`'s return path

**From Solution:** SOL-005
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/registry.go`
**Depends on:** TASK-001 (both edit `Dispatch`; land as one coordinated change per `solutions/README.md`'s grouping note)
**Status:** `[ ]` TODO

---

## Context

`projectGroup.list`/`ssh.listTargets`/`team.list` return a proto getter
directly (`resp.GetGroups()` etc.) — `nil` for an empty repeated field,
which `encoding/json` marshals as `null`, not `[]`. `credentials.list`
has the same symptom from a locally-declared `var services []string`
never appended to. Every real frontend caller destructures/iterates these
results with no null-guard (BUG-005). Fix once, in `Dispatch`'s return
path (same file TASK-001 already modified), so every current AND future
channel is covered without a per-handler change.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/registry.go`

Building on TASK-001's version of `Dispatch` (identity attach + timeout
already added), wrap the handler's result before returning:

```go
func (r *Registry) Dispatch(ctx context.Context, id Identity, channel string, args []json.RawMessage) (any, error) {
	h, ok := r.handlers[channel]
	if !ok {
		return notImplementedHandler(ctx, id, channel)
	}
	ctx = gatewaygrpc.AttachIdentity(ctx, usecase.Identity{TenantID: id.TenantID, UserID: id.UserID})
	ctx, cancel := context.WithTimeout(ctx, dispatchRPCTimeout)
	defer cancel()
	result, err := h(ctx, id, args)
	if err != nil {
		return nil, err
	}
	return normalizeNilSlices(result), nil
}

// normalizeNilSlices replaces a nil slice — at the top level, or one level
// into a struct/pointer's exported fields — with a non-nil, empty slice of
// the same type, so encoding/json emits [] instead of null. See
// specs/backend-go/bugs/missing-v2/BUG-005: several channel handlers
// return a proto-generated getter or a locally-declared `var xs []T` that
// stays nil for an empty result, and every real frontend caller for those
// channels destructures/iterates the result with no null-guard.
//
// Scope: one level deep is deliberate, not a general deep-walk — every
// handler result shape this fixes today is a flat struct/proto message
// wrapping at most one list field (Groups, SshTargets, Teams, Services).
// Widen only if a future handler genuinely needs more (e.g. a nested list
// two levels down), not speculatively.
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
		return v
	case reflect.Ptr:
		if rv.IsNil() {
			return v
		}
		return normalizeNilSlices(rv.Elem().Interface())
	case reflect.Struct:
		return normalizeStructSliceFields(v, rv)
	case reflect.Map:
		// Handlers occasionally return map[string]any (e.g. status.get) —
		// normalize any nil-slice VALUE one level deep the same way, since
		// a map is exactly as capable of holding a nil-slice field as a
		// struct is for this codebase's actual handler shapes.
		if rv.IsNil() {
			return v
		}
		out := reflect.MakeMapWithSize(rv.Type(), rv.Len())
		iter := rv.MapRange()
		for iter.Next() {
			out.SetMapIndex(iter.Key(), reflect.ValueOf(normalizeNilSliceValue(iter.Value())))
		}
		return out.Interface()
	default:
		return v
	}
}

func normalizeStructSliceFields(v any, rv reflect.Value) any {
	t := rv.Type()
	out := reflect.New(t).Elem()
	out.Set(rv)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		fv := out.Field(i)
		if fv.Kind() == reflect.Slice && fv.IsNil() {
			fv.Set(reflect.MakeSlice(fv.Type(), 0, 0))
		}
	}
	return out.Interface()
}

func normalizeNilSliceValue(v reflect.Value) any {
	if !v.IsValid() {
		return nil
	}
	return normalizeNilSlices(v.Interface())
}
```

Add `"reflect"` to `registry.go`'s imports.

### Handling proto message pointers specifically

Proto-generated types are usually returned as a bare `[]*pb.T` slice
directly (already covered by the top-level `reflect.Slice` case — this is
`projectGroup.list`'s/`ssh.listTargets`'s/`team.list`'s actual shape, not
a wrapping struct), or occasionally a wrapping Go struct
(`credentials.list`'s `{Services []string, Mode string}` literal, covered
by the `reflect.Struct` case). Confirm each of the 4 confirmed BUG-005
instances' exact handler return shape against the real current
`channels_*.go` source at implementation time — the sketch above is
designed to cover both shapes, but verify neither introduces a case this
reflection code doesn't handle (e.g. an unexported proto struct field,
which `field.IsExported()` correctly skips rather than panicking on).

## Verify

```bash
cd backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -count=1
```

Expected: clean build, all existing tests pass. TASK-011 adds the
regression tests for this specific fix.
