# TASK-012: `normalizeInboundMessage` always populates `Args[0]`, even when `params` is absent/`null`

**From Solution:** SOL-006
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/session_dialect.go`
**Depends on:** none
**Status:** `[x]` DONE — implemented exactly as this doc specifies in `session_dialect.go`. `go build`/`go vet`/`go test ./services/api-gateway/...` all clean.

---

## Context

`normalizeInboundMessage` only sets `msg.Args` when `msg.Params != nil` —
a `WebSessionClient` call with no `params` key (the natural way to call
any method whose real contract is `params: null`, e.g. `status.get`,
`repo.list`, `profile.getResolved`) leaves `Args` empty, and every
`decodeArg`-based handler then fails with `"missing arg[0]"` (BUG-006).

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/session_dialect.go`

Current code:

```go
func normalizeInboundMessage(msg InboundMessage) (dialect, InboundMessage) {
	if msg.Type != "" || msg.Method == "" {
		return dialectNative, msg
	}
	msg.Type = "invoke"
	msg.Channel = msg.Method
	if msg.Params != nil {
		msg.Args = []json.RawMessage{msg.Params}
	}
	return dialectSessionClient, msg
}
```

Replace with:

```go
// emptyParams is substituted for a session-client request's absent/null
// params so decodeArg[T](args, 0) always has an args[0] to decode against
// — an omitted `params` key and an explicit `"params":null` both mean "no
// arguments," not "zero arguments were passed at all." See
// specs/backend-go/bugs/missing-v2/BUG-006.
var emptyParams = json.RawMessage("{}")

func normalizeInboundMessage(msg InboundMessage) (dialect, InboundMessage) {
	if msg.Type != "" || msg.Method == "" {
		return dialectNative, msg
	}
	msg.Type = "invoke"
	msg.Channel = msg.Method
	params := msg.Params
	if params == nil {
		params = emptyParams
	}
	msg.Args = []json.RawMessage{params}
	return dialectSessionClient, msg
}
```

This only touches the `dialectSessionClient` branch — `dialectNative`
(the native `{type, channel, args}` dialect mobile/desktop clients speak,
per `specs/frontend/api/mobile-rpc-catalog.md`'s architecture note) is
unaffected, since it returns before reaching this logic.

Do **not** change `decodeArg` itself (`registry.go:133-142`) — its strict
`index >= len(args)` check is relied on elsewhere for genuinely-required
args on the native dialect; this fix is scoped to the one place the
session-client dialect's own normalization happens, per SOL-006's "Why not
fix `decodeArg` instead" reasoning.

## Verify

```bash
cd backend-go
go build ./services/api-gateway/...
go vet ./services/api-gateway/...
go test ./services/api-gateway/internal/adapter/wscompat/... -count=1
```

Expected: clean build, all existing tests pass (including
`TestSessionClientDialect_RoundTripsThroughRegisteredChannel`, which
already sends explicit `params` and should be unaffected). TASK-013 adds
the regression test for the specific no-`params` case this fixes.
