# TASK-013: Regression tests — session-client dialect call with no `params` reaches a strict `decodeArg`-based handler

**From Solution:** SOL-006
**Priority:** P1
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/handler_test.go`
**Depends on:** TASK-012
**Status:** `[ ]` TODO

---

## Context

Add tests using this file's existing real-WebSocket test harness
(`newTestHandlerServer`/`dialTestClient`/`writeRaw`/
`readSessionClientWireMessage`, already used by
`TestSessionClientDialect_RoundTripsThroughRegisteredChannel` and
siblings) — the direct regression test for BUG-006, plus a unit test for
`normalizeInboundMessage` in isolation.

## Changes to make

**File:** `services/api-gateway/internal/adapter/wscompat/handler_test.go`

Add near the existing `TestSessionClientDialect_*` tests:

```go
// TestSessionClientDialect_NoParams_ReachesHandlerWithEmptyArgs is the
// direct regression test for BUG-006 (specs/backend-go/bugs/missing-v2/):
// a WebSessionClient call with no "params" key at all (the natural shape
// for any method whose real contract is params: null) must reach a
// strict decodeArg-based handler with a decodable args[0], not fail with
// "missing arg[0]" before the handler is ever invoked.
func TestSessionClientDialect_NoParams_ReachesHandlerWithEmptyArgs(t *testing.T) {
	registry := NewRegistry()
	var handlerCalled bool
	registry.Register("repo.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		handlerCalled = true
		type listArgs struct {
			ProjectID string `json:"projectId"`
		}
		in, err := decodeArg[listArgs](args, 0)
		if err != nil {
			return nil, fmt.Errorf("decodeArg: %w", err) // this is exactly BUG-006's failure if the fix is missing
		}
		return map[string]string{"projectId": in.ProjectID}, nil // empty string is the correct zero value
	})

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// No "params" key at all — the exact shape that broke before TASK-012.
	if err := writeRaw(ctx, client, `{"id":"web-session-rpc-1","authToken":"cookie-auth","method":"repo.list"}`); err != nil {
		t.Fatalf("writing session-client invoke: %v", err)
	}

	got := readSessionClientWireMessage(t, ctx, client)
	if !handlerCalled {
		t.Fatal("expected the repo.list handler to be invoked — request never reached it")
	}
	if !got.OK {
		t.Fatalf("ok = false, want true (error=%+v) — this is BUG-006's exact failure if unfixed", got.Error)
	}
}

// TestSessionClientDialect_ExplicitNullParams_SameAsOmitted covers the
// second BUG-006 trigger shape: an explicit "params":null behaves
// identically to an omitted params key.
func TestSessionClientDialect_ExplicitNullParams_SameAsOmitted(t *testing.T) {
	registry := NewRegistry()
	registry.Register("repo.list", func(ctx context.Context, id Identity, args []json.RawMessage) (any, error) {
		type listArgs struct {
			ProjectID string `json:"projectId"`
		}
		if _, err := decodeArg[listArgs](args, 0); err != nil {
			return nil, fmt.Errorf("decodeArg: %w", err)
		}
		return map[string]bool{"ok": true}, nil
	})

	ts := newTestHandlerServer(t, registry)
	client := dialTestClient(t, ts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := writeRaw(ctx, client, `{"id":"web-session-rpc-1","authToken":"cookie-auth","method":"repo.list","params":null}`); err != nil {
		t.Fatalf("writing session-client invoke: %v", err)
	}

	got := readSessionClientWireMessage(t, ctx, client)
	if !got.OK {
		t.Fatalf("ok = false, want true (error=%+v)", got.Error)
	}
}
```

Also add a package-level unit test (no WebSocket harness needed) directly
against `normalizeInboundMessage`, if this package's test conventions
favor a fast unit-level check alongside the integration-style ones above:

```go
func TestNormalizeInboundMessage_NoParams_PopulatesEmptyObjectArg(t *testing.T) {
	d, normalized := normalizeInboundMessage(InboundMessage{ID: "x", Method: "repo.list"})
	if d != dialectSessionClient {
		t.Fatalf("expected dialectSessionClient, got %v", d)
	}
	if len(normalized.Args) != 1 || string(normalized.Args[0]) != "{}" {
		t.Fatalf("expected Args[0] = {}, got %v", normalized.Args)
	}
}

func TestNormalizeInboundMessage_RealParams_PassedThroughUnchanged(t *testing.T) {
	d, normalized := normalizeInboundMessage(InboundMessage{ID: "x", Method: "repo.list", Params: json.RawMessage(`{"projectId":"p1"}`)})
	if d != dialectSessionClient {
		t.Fatalf("expected dialectSessionClient, got %v", d)
	}
	if len(normalized.Args) != 1 || string(normalized.Args[0]) != `{"projectId":"p1"}` {
		t.Fatalf("expected real params passed through unchanged, got %v", normalized.Args)
	}
}
```

Add `"fmt"` to `handler_test.go`'s imports if not already present.

## Verify

```bash
cd backend-go
go test ./services/api-gateway/internal/adapter/wscompat/... -count=1 -v -run 'TestSessionClientDialect_NoParams|TestSessionClientDialect_ExplicitNullParams|TestNormalizeInboundMessage_'
```

Expected: all 4 new tests pass. Then re-run the full `wscompat` suite to
confirm nothing regressed:

```bash
go test ./services/api-gateway/internal/adapter/wscompat/... -count=1
```
