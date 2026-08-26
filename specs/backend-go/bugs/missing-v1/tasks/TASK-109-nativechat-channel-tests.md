# TASK-109: Tests for `nativeChat.readSession`'s relay wiring

**From Solution:** SOL-017
**Priority:** P2
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_nativechat_test.go` (new)
**Depends on:** TASK-108
**Status:** `[partial]` — usecase/adapter/wscompat tests written and passing (30 new wscompat tests total across the group). Postgres integration test halves written, compile under `-tags=integration`, but not executed — no Docker/Postgres in this environment. Worktree `agent-a412325f0d1276bb5`, committed as `c29ca9e6a`.

---

## Context

Implements SOL-017's "Test plan" section exactly — 4 tests mirroring
`channels_test.go`'s `TestDevServerListChannel_Success`/`_PropagatesError`
shape, using the existing `fakeInfraFleetClient` test double (extend it
with a `relayFunc` field; do not create a second fake).

## Changes to make

### New file `services/api-gateway/internal/adapter/wscompat/channels_nativechat_test.go`

```go
package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

func TestNativeChatReadSessionChannel_RelaysToInfraFleet(t *testing.T) {
	var gotReq *infrafleetv1.RelayRequest
	fake := &fakeInfraFleetClient{
		relayFunc: func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			gotReq = in
			return &infrafleetv1.RelayResponse{ResultJson: `{"messages":[]}`}, nil
		},
	}
	r := NewRegistry()
	registerNativeChatChannels(r, fake)

	args := argsJSON(t, map[string]any{
		"agent": "claude", "sessionId": "sess-1", "limit": 50,
		"transcriptPath": "/path/to.jsonl", "connectionId": "conn-1",
	})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "nativeChat.readSession", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotReq.GetConnectionId() != "conn-1" {
		t.Errorf("want ConnectionId=conn-1, got %q", gotReq.GetConnectionId())
	}
	if gotReq.GetMethod() != "nativeChat.readSession" {
		t.Errorf("want Method=nativeChat.readSession, got %q", gotReq.GetMethod())
	}
	var params map[string]any
	if err := json.Unmarshal([]byte(gotReq.GetParamsJson()), &params); err != nil {
		t.Fatalf("params_json not valid JSON: %v", err)
	}
	if params["agent"] != "claude" || params["sessionId"] != "sess-1" || params["transcriptPath"] != "/path/to.jsonl" {
		t.Errorf("params_json missing expected fields: %+v", params)
	}
	if _, ok := result.(json.RawMessage); !ok {
		t.Fatalf("want json.RawMessage result, got %T", result)
	}
}

func TestNativeChatReadSessionChannel_MissingConnectionID_FailsClosed(t *testing.T) {
	fake := &fakeInfraFleetClient{
		relayFunc: func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			t.Fatal("Relay must not be called when connectionId is missing")
			return nil, nil
		},
	}
	r := NewRegistry()
	registerNativeChatChannels(r, fake)

	args := argsJSON(t, map[string]any{"agent": "claude", "sessionId": "sess-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "nativeChat.readSession", args)
	if err == nil {
		t.Fatal("expected error when connectionId is absent")
	}
	const wantSubstr = "connectionId is required"
	if !contains(err.Error(), wantSubstr) {
		t.Errorf("want error containing %q, got %q", wantSubstr, err.Error())
	}
}

func TestNativeChatReadSessionChannel_PropagatesRelayError(t *testing.T) {
	wantErr := errors.New("dev server agent unreachable")
	fake := &fakeInfraFleetClient{
		relayFunc: func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerNativeChatChannels(r, fake)

	args := argsJSON(t, map[string]any{"agent": "claude", "sessionId": "sess-1", "connectionId": "conn-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "nativeChat.readSession", args)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want %v, got %v", wantErr, err)
	}
}

func TestNativeChatReadSessionChannel_PassesThroughResultJSONVerbatim(t *testing.T) {
	for _, resultJSON := range []string{`{"messages":[{"role":"user","content":"hi"}]}`, `{"error":"transcript not found"}`} {
		fake := &fakeInfraFleetClient{
			relayFunc: func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
				return &infrafleetv1.RelayResponse{ResultJson: resultJSON}, nil
			},
		}
		r := NewRegistry()
		registerNativeChatChannels(r, fake)

		args := argsJSON(t, map[string]any{"agent": "claude", "sessionId": "sess-1", "connectionId": "conn-1"})
		result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "nativeChat.readSession", args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		raw, ok := result.(json.RawMessage)
		if !ok || string(raw) != resultJSON {
			t.Errorf("result not passed through verbatim: got %v, want %s", result, resultJSON)
		}
	}
}

// contains is a tiny helper so this file doesn't need to import strings
// just for one substring check — replace with strings.Contains directly if
// this package already imports "strings" elsewhere.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
```

Prefer `strings.Contains` from the standard library over the hand-rolled
`contains`/`indexOf` helpers above if `channels_test.go` or any sibling
test file in this package already imports `"strings"` — those two helpers
exist here only to avoid a one-off import; delete them and use
`strings.Contains(err.Error(), wantSubstr)` directly if that's cleaner in
context.

### Extend `fakeInfraFleetClient` (`channels_test.go`, same package)

```go
type fakeInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	listDevServersFunc    func(ctx context.Context, in *infrafleetv1.ListDevServersRequest) (*infrafleetv1.ListDevServersResponse, error)
	registerDevServerFunc func(ctx context.Context, in *infrafleetv1.RegisterDevServerRequest) (*infrafleetv1.RegisterDevServerResponse, error)
	getFleetHealthFunc    func(ctx context.Context, in *infrafleetv1.GetFleetHealthRequest) (*infrafleetv1.GetFleetHealthResponse, error)
	relayFunc             func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) // NEW
}

func (f *fakeInfraFleetClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFunc(ctx, in)
}
```

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/api-gateway/internal/adapter/wscompat/... -run TestNativeChat -count=1 -v
```
