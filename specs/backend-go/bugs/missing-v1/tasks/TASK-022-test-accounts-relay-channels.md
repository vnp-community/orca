# TASK-022: Test `accounts.*` wscompat relay channels

**From Solution:** SOL-004
**Priority:** P2
**Service:** `api-gateway`
**File:** `services/api-gateway/internal/adapter/wscompat/channels_accounts_test.go` (new)
**Depends on:** TASK-021
**Status:** `[ ]` TODO

---

## Context

Covers SOL-004's test plan: one test per channel against a fake
`InfraFleetServiceClient.Relay`, the missing-`connectionId` fail-fast path,
and `Relay` error passthrough. Defines its own small fake client scoped to
`Relay` only (does not touch the existing `fakeInfraFleetClient` in
`channels_test.go`, which has no `Relay` method today) so this task has no
edit-ordering dependency on any other test task in this batch.

---

## Changes to make

### New file: `services/api-gateway/internal/adapter/wscompat/channels_accounts_test.go`

```go
package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeAccountsRelayClient is a minimal test double scoped to Relay only —
// every accounts.* channel handler calls only this one RPC.
type fakeAccountsRelayClient struct {
	infrafleetv1.InfraFleetServiceClient

	relayFunc func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)
}

func (f *fakeAccountsRelayClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFunc(ctx, in)
}

func TestAccountsChannels_RelaySuccess(t *testing.T) {
	cases := []struct {
		channel      string
		wantMethod   string
	}{
		{"accounts.selectClaude", "accounts.selectClaude"},
		{"accounts.selectCodex", "accounts.selectCodex"},
		{"accounts.removeClaude", "accounts.removeClaude"},
		{"accounts.removeCodex", "accounts.removeCodex"},
	}
	for _, tc := range cases {
		t.Run(tc.channel, func(t *testing.T) {
			var gotReq *infrafleetv1.RelayRequest
			fake := &fakeAccountsRelayClient{
				relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
					gotReq = in
					return &infrafleetv1.RelayResponse{ResultJson: `{"ok":true}`}, nil
				},
			}
			r := NewRegistry()
			registerAccountsChannels(r, fake)

			args := argsJSON(t, map[string]any{"accountId": "acct-1", "connectionId": "conn-1"})
			result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, tc.channel, args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got, ok := result.(map[string]any)
			if !ok || got["ok"] != true {
				t.Fatalf("unexpected result: %#v", result)
			}
			if gotReq.GetConnectionId() != "conn-1" {
				t.Errorf("ConnectionId = %q, want conn-1", gotReq.GetConnectionId())
			}
			if gotReq.GetMethod() != tc.wantMethod {
				t.Errorf("Method = %q, want %q", gotReq.GetMethod(), tc.wantMethod)
			}
			var params map[string]any
			if err := json.Unmarshal([]byte(gotReq.GetParamsJson()), &params); err != nil {
				t.Fatalf("params_json not valid JSON: %v", err)
			}
			if params["accountId"] != "acct-1" {
				t.Errorf("params accountId = %v, want acct-1", params["accountId"])
			}
		})
	}
}

func TestAccountsChannels_MissingConnectionID_FailsFastWithoutCallingRelay(t *testing.T) {
	called := false
	fake := &fakeAccountsRelayClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			called = true
			return nil, nil
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	args := argsJSON(t, map[string]any{"accountId": "acct-1"}) // no connectionId
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "accounts.selectClaude", args)
	if err == nil {
		t.Fatal("expected an error for missing connectionId")
	}
	if called {
		t.Error("Relay must not be called when connectionId is missing")
	}
}

func TestAccountsChannels_RelayErrorPassesThroughVerbatim(t *testing.T) {
	wantErr := errors.New("INFRA_CONNECTION_NOT_FOUND: no dev server owns this connectionId")
	fake := &fakeAccountsRelayClient{
		relayFunc: func(context.Context, *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			return nil, wantErr
		},
	}
	r := NewRegistry()
	registerAccountsChannels(r, fake)

	args := argsJSON(t, map[string]any{"accountId": "acct-1", "connectionId": "conn-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "accounts.removeCodex", args)
	if !errors.Is(err, wantErr) && err.Error() != wantErr.Error() {
		t.Fatalf("expected Relay's error to pass through verbatim, got: %v", err)
	}
}
```

`argsJSON` is the existing test helper already defined in `channels_test.go`
— reused here, not redefined.

---

## Verify

```bash
cd /opt/repos/orca/backend-go/services/api-gateway
go test ./internal/adapter/wscompat/... -run TestAccountsChannels -v
go build ./... && go vet ./...
```

Expected: all 3 new test functions pass; existing `wscompat` tests
unaffected.
