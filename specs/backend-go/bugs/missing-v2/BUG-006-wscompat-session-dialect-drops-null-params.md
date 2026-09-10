# BUG-006: `WebSessionClient` dialect bridge drops `params` when a call has none — every `params: null`-contract method fails with `"missing arg[0]"`

**Service:** `api-gateway`
**File:** `internal/adapter/wscompat/session_dialect.go`
**Severity:** Medium — breaks every channel whose real contract takes no arguments (`status.get`, `repo.list`, `profile.getResolved`, `team.list`, …) when the caller omits `params` entirely, which is the natural way to call a no-arg RPC and exactly what the old TS backend's own `params: null` methods expect
**Symptom:** A `WebSessionClient`-dialect request with no `params` field (e.g. `{"id":"x","authToken":"cookie-auth","method":"repo.list"}`) fails every handler that decodes its args via `decodeArg` (the non-tolerant variant), even though the same channel succeeds when the caller sends `"params":{}` instead.
**Status:** 🔴 Open — found live 2026-08-27 via `tests/client/rpc-client.ts` (this session's new RPC test client), root-caused by source inspection.

---

## Description

`normalizeInboundMessage` (part of BUG-005/SOL-005's dialect bridge, see
`api-v1/BUG-005-websessionclient-dialect-dropped.md`) translates a
session-client-dialect message onto the shared `Channel`/`Args` shape every
handler dispatches against:

```go
// session_dialect.go:50-60
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

When the inbound JSON has no `"params"` key (or an explicit `"params":null`),
`msg.Params` unmarshals to Go `nil`, the `if` is skipped, and `msg.Args`
stays `nil` — an empty slice. Every handler that calls the strict
`decodeArg[T](args, 0)` (not the tolerant `decodeOptionalArg`) then hits:

```go
// registry.go:133-137
func decodeArg[T any](args []json.RawMessage, index int) (T, error) {
	var v T
	if index >= len(args) {
		return v, fmt.Errorf("missing arg[%d]", index)
	}
	// ...
}
```

`len(args) == 0`, `index == 0` ⇒ `0 >= 0` ⇒ always errors, regardless of
whether the handler's real params schema is optional or `null`. This is
NOT specific to one channel — it hits every `decodeArg`-based handler
called with no `params`.

The old TS backend's RPC methods commonly declare `params: null` for
no-argument methods (`status.get`, `repo.list`, `profile.getResolved`,
`team.list`, `ssh.listTargets`, `devServer.list`, `projectGroup.list`,
`folderWorkspace.list`, …) — `frontend/src/renderer/src/web/web-session-client.ts`'s
`call(method, params?, ...)` happily omits `params` for these, exactly the
shape that breaks here.

## Confirmed

- `session_dialect.go:56-58` — the exact `if msg.Params != nil` branch that
  drops params-less calls, read directly.
- `registry.go:133-142` — `decodeArg`'s strict `index >= len(args)` check,
  confirmed as the failure site (`decodeOptionalArg`, `registry.go:150-153`,
  is a separate, tolerant variant a few handlers already use instead —
  not applied here).
- Live-verified 2026-08-27 against `172.20.2.39:6769`: calling `repo.list`
  with the `method`/no-`params` shape reproduced
  `{"error":{"code":"internal","message":"missing arg[0]"}}`; the
  **identical call with `"params":{}` added** succeeded past this step
  (reached `PROJECT_MEMBERSHIP_LOOKUP_FAILED`/`PROJECT_POLICY_EVAL_FAILED`
  instead — see BUG-003 — proving the only difference was the presence of
  an empty `params` object, not anything else about the request).

## Related, Already-Known Limitation (not a new bug, noted for context)

While tracing this, confirmed `writeDialectError` (`session_dialect.go:85-91`)
always encodes session-client-dialect errors with a hardcoded
`Code: "internal"`, regardless of the real underlying error (a
`method_not_found`, `invalid_argument`, or `unauthorized` from the native
dialect all flatten to `"internal"` for a `WebSessionClient` caller). This
is explicitly documented in that function's own comment as an accepted
"Phase 1 simplification" from BUG-005/SOL-005, not a new finding — noted
here only because it means error-message **text** (not code) is the only
signal available to distinguish failure causes over this dialect, which is
what this report and BUG-001–005 rely on throughout.

## Suggested Fix

In `normalizeInboundMessage`, always set `msg.Args = []json.RawMessage{}`
(or `[]json.RawMessage{msg.Params}` when non-nil, falling back to a JSON
`{}` literal `json.RawMessage("{}")` when nil) instead of leaving `Args`
unset — so `decodeArg[T](args, 0)` always has an `args[0]` to decode
(`json.Unmarshal([]byte("{}"), &v)` correctly zero-values any struct `T`).
This fixes every affected channel at the one shared normalization site,
matching how BUG-005 itself was fixed at the boundary rather than
per-handler.

## Regression Test Gap

`handler_test.go`'s session-client dialect tests (added for BUG-005) cover
the round-trip and error-path shape but — based on this file's own
description of what those tests assert — don't appear to include a
params-omitted call against a real `decodeArg`-based handler; that's the
specific gap this bug lives in.
