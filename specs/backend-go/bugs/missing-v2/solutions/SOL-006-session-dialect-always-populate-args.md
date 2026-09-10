# SOL-006: Fix BUG-006 — `normalizeInboundMessage` always populates `Args[0]` for the session-client dialect, even with no `params`

**Resolves:** BUG-006
**Service:** `api-gateway`
**Affected files:** `internal/adapter/wscompat/session_dialect.go`
**Priority:** Medium
**Status:** 🟡 Proposed — not yet implemented

---

## Grounding in `specs/backend-go/tdd/`

`crs/v0/standards/api-design-guidelines.md`'s "Request/response
conventions": *"no bare primitives as top-level request/response types,
even for simple calls, so a field can be added later without a breaking
change"* — the broader principle behind why request messages are always a
struct, never `null`/absent, extends naturally to args decoding: a
no-argument call should decode as an empty struct, not be treated as
structurally different from an explicit `{}`. The two are semantically
identical for every affected method's real params contract (each is either
`params: null` in the old TS backend, meaning "no fields," or every field
is optional) — `decodeArg`'s current strict `index >= len(args)` check
makes them behave differently by accident, not by any documented design
choice.

This is the same "one shared normalization step, not per-handler
special-casing" pattern SOL-001 and SOL-005 both apply — `08-inter-service-communication.md`'s
"No service hand-rolls this per-RPC" framing, again, just for arg-decoding
instead of identity or response-shape.

## Design

Fix at the one place that already exists specifically to normalize this
dialect — `normalizeInboundMessage` — rather than touching `decodeArg` (a
generic helper also used by the native dialect, which already works
correctly and shouldn't change behavior) or every affected handler:

```go
// session_dialect.go — sketch
var emptyParams = json.RawMessage("{}")

func normalizeInboundMessage(msg InboundMessage) (dialect, InboundMessage) {
	if msg.Type != "" || msg.Method == "" {
		return dialectNative, msg
	}
	msg.Type = "invoke"
	msg.Channel = msg.Method
	// Always populate Args[0] — an omitted or explicit-null `params` on
	// this dialect means "no arguments," not "zero arguments were passed
	// at all." decodeArg[T](args, 0) needs args[0] to exist to correctly
	// zero-value any T; json.Unmarshal([]byte("{}"), &v) does that for any
	// struct T uniformly for both cases: struct with all-optional fields,
	// or a real params: null contract.
	params := msg.Params
	if params == nil {
		params = emptyParams
	}
	msg.Args = []json.RawMessage{params}
	return dialectSessionClient, msg
}
```

This is a strictly additive change to the `dialectSessionClient` branch
only — the `dialectNative` early-return path (native `{type, channel,
args}` callers, e.g. the mobile/desktop `rpc-client.ts` clients per
`specs/frontend/api/mobile-rpc-catalog.md`'s architecture note that they
speak this dialect, not `WebSessionClient`'s) is untouched, so this can't
regress the dialect that already works correctly.

### Why not fix `decodeArg` instead

Considered making `decodeArg` itself tolerant (treat `index >= len(args)`
as "decode against an empty object" universally) — rejected: that changes
behavior for the **native** dialect too, where a genuinely-missing
required arg should keep failing loudly (several native-dialect handlers
rely on `decodeArg`'s strictness for real required params, not just
optional ones — conflating "no params sent" with "required param missing"
at that shared, generic helper would silently break argument validation
for methods that actually need one). Fixing at
`normalizeInboundMessage` instead keeps the fix scoped to exactly the
dialect and exactly the failure mode BUG-006 describes.

## Testing Plan

- Unit test: `normalizeInboundMessage` given a message with `Method` set
  and no `Params` key at all → returns `dialectSessionClient` with
  `Args == []json.RawMessage{[]byte("{}")}` (not `nil`/empty).
- Unit test: same, with an explicit `"params":null` → identical result
  (both absent-key and explicit-null collapse to the same normalized
  shape).
- Unit test: same, with a real `"params":{"foo":"bar"}` → `Args[0]` is
  that exact payload, unchanged (guards against the fix accidentally
  overwriting real params).
- Regression test against a real `decodeArg`-based handler (e.g.
  `repo.list`'s fake-client test already in
  `channels_repo_ssh_status_workspace_test.go`): dispatch it via the
  session-client dialect with no `params` key → the fake client's
  `ProjectID` decodes to `""` (the correct zero value) instead of the
  dispatch failing with `"missing arg[0]"` before the fake client is even
  reached.
- Re-run `tests/client/rpc-client.ts`'s calling convention — once this
  lands, `rpc-client.ts`'s existing `wireParams = params ?? {}` workaround
  (added to route around this exact bug while testing) becomes
  unnecessary; safe to simplify back to sending `params` as-is (including
  omitted) after confirming the fix live, though leaving the client-side
  workaround in place causes no harm either.
