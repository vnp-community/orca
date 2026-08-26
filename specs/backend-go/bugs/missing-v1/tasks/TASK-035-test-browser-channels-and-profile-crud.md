# TASK-035: Tests for browser profile CRUD (`infra-fleet-service`) and `browser.*` wscompat channels

**From Solution:** SOL-006 (Test plan section)
**Priority:** P2
**Service:** `infra-fleet-service`, `api-gateway`
**File:** `services/infra-fleet-service/internal/usecase/{list_browser_profiles_test.go,create_browser_profile_test.go,delete_browser_profile_test.go}` (new), `services/api-gateway/internal/adapter/wscompat/{channels_browser_test.go,channels_browser_profiles_test.go}` (new)
**Depends on:** TASK-032, TASK-033, TASK-034
**Status:** `[ ]` TODO

---

## Context

Covers SOL-006's test plan. Explicitly **no agent-side test plan** here —
that belongs to whichever `agent/`-scoped effort implements the
browser-driving capability TASK-034/TASK-033's relay plumbing depends on
(see TASK-036).

---

## Changes to make

### `infra-fleet-service/internal/usecase/{list,create,delete}_browser_profile_test.go` (new)

Fake `BrowserProfileRepository`, tenant-scoping enforced — follow
`create_ssh_target_test.go`'s existing pattern in the same package for the
fake-repo/table-driven shape. Cases:
- `ListBrowserProfiles`: empty `devServerID` → `INFRA_NO_DEV_SERVER`
  without calling the repo; populated case returns the fake's rows
  unmodified.
- `CreateBrowserProfile`: empty `name` → validation error; success case
  asserts the repo receives a generated `ID` and the input's other fields
  verbatim.
- `DeleteBrowserProfile`: empty `id` → validation error; success case
  asserts `(tenantID, id)` passed through.

### `services/api-gateway/internal/adapter/wscompat/channels_browser_test.go` (new)

```go
package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// fakeBrowserRelayClient is scoped to ResolveConnection + Relay — the only
// 2 RPCs browser.* pane-control channels call.
type fakeBrowserRelayClient struct {
	infrafleetv1.InfraFleetServiceClient

	resolveConnectionFunc func(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error)
	relayFunc              func(ctx context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error)
}

func (f *fakeBrowserRelayClient) ResolveConnection(ctx context.Context, in *infrafleetv1.ResolveConnectionRequest, _ ...grpc.CallOption) (*infrafleetv1.ResolveConnectionResponse, error) {
	return f.resolveConnectionFunc(ctx, in)
}

func (f *fakeBrowserRelayClient) Relay(ctx context.Context, in *infrafleetv1.RelayRequest, _ ...grpc.CallOption) (*infrafleetv1.RelayResponse, error) {
	return f.relayFunc(ctx, in)
}

func TestBrowserChannels_RequiresWorktree_FailsFastWithoutResolving(t *testing.T) {
	called := false
	fake := &fakeBrowserRelayClient{
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			called = true
			return nil, nil
		},
	}
	r := NewRegistry()
	registerBrowserChannels(r, fake)

	args := argsJSON(t, map[string]any{"x": 10, "y": 20}) // no worktree
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.mouseMove", args)
	if err == nil {
		t.Fatal("expected BROWSER_NO_WORKTREE error")
	}
	if called {
		t.Error("ResolveConnection must not be called when worktree is missing")
	}
}

func TestBrowserChannels_ResolvesWorktreeThenRelaysFullParams(t *testing.T) {
	var gotResolve *infrafleetv1.ResolveConnectionRequest
	var gotRelay *infrafleetv1.RelayRequest
	fake := &fakeBrowserRelayClient{
		resolveConnectionFunc: func(_ context.Context, in *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			gotResolve = in
			return &infrafleetv1.ResolveConnectionResponse{Connected: true, DevServer: &infrafleetv1.DevServer{Id: "ds-1"}}, nil
		},
		relayFunc: func(_ context.Context, in *infrafleetv1.RelayRequest) (*infrafleetv1.RelayResponse, error) {
			gotRelay = in
			return &infrafleetv1.RelayResponse{ResultJson: `{"ok":true}`}, nil
		},
	}
	r := NewRegistry()
	registerBrowserChannels(r, fake)

	args := argsJSON(t, map[string]any{"worktree": "wt-1", "width": 1280, "height": 720})
	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.viewport", args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotResolve.GetWorktreeId() != "wt-1" {
		t.Errorf("WorktreeId = %q, want wt-1", gotResolve.GetWorktreeId())
	}
	if gotRelay.GetMethod() != "browser.viewport" {
		t.Errorf("Method = %q, want browser.viewport", gotRelay.GetMethod())
	}
	var params map[string]any
	_ = json.Unmarshal([]byte(gotRelay.GetParamsJson()), &params)
	if params["width"] != float64(1280) {
		t.Errorf("params passed through incompletely: %#v", params)
	}
	if result.(map[string]any)["ok"] != true {
		t.Errorf("unexpected result: %#v", result)
	}
}

func TestBrowserChannels_NotConnected_Errors(t *testing.T) {
	fake := &fakeBrowserRelayClient{
		resolveConnectionFunc: func(context.Context, *infrafleetv1.ResolveConnectionRequest) (*infrafleetv1.ResolveConnectionResponse, error) {
			return &infrafleetv1.ResolveConnectionResponse{Connected: false}, nil
		},
	}
	r := NewRegistry()
	registerBrowserChannels(r, fake)

	args := argsJSON(t, map[string]any{"worktree": "wt-1"})
	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1"}, "browser.tabCreate", args)
	if err == nil {
		t.Fatal("expected BROWSER_NO_CONNECTION error")
	}
}
```

Run this table across all 9 Group A/B channel names (`eval`, `keypress`,
`mouseDown`, `mouseMove`, `mouseUp`, `mouseWheel`, `viewport`, `tabCreate`,
`tabClose`) for at least the "resolves then relays" success case, following
`TestAccountsChannels_RelaySuccess`'s (TASK-022) table-driven shape.

### `channels_browser_profiles_test.go` (new)

Group C's 3 Postgres-backed channels (`profileList`/`profileCreate`/
`profileDelete`) against a fake `InfraFleetServiceClient` overriding
`ListBrowserProfiles`/`CreateBrowserProfile`/`DeleteBrowserProfile`; the 3
relay-backed ones (`profileClearDefaultCookies`/`profileDetectBrowsers`/
`profileImportFromBrowser`) against `ResolveConnection`+`Relay`, same
pattern as `channels_browser_test.go` above but keyed by `devServerId`
instead of `worktree` — assert missing `devServerId` fails fast with
`BROWSER_NO_DEV_SERVER` without calling `ResolveConnection`.

---

## Verify

```bash
cd /opt/repos/orca/backend-go
go test ./services/infra-fleet-service/internal/usecase/... -run BrowserProfile -v
go test ./services/api-gateway/internal/adapter/wscompat/... -run Browser -v
```

Expected: all new tests pass.
