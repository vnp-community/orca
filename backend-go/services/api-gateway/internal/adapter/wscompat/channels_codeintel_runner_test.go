package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	codeintelv1 "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeCodeIntelClient struct {
	codeintelv1.CodeIntelServiceClient
	called bool
}

type testUnaryArgs struct {
	codeIntelSelector
}

func (a testUnaryArgs) validate() error {
	return a.codeIntelSelector.validate()
}

type testSettingsArgs struct{}

func (a testSettingsArgs) validate() error {
	return nil
}

func TestCodeIntelRunner(t *testing.T) {
	r := NewRegistry()
	fake := &fakeCodeIntelClient{}

	deps := codeIntelDeps{
		core: fake,
		limits: CodeIntelLimits{
			MaxResponseBytes: 2 << 20,
		},
	}

	testCall := func(ctx context.Context, c codeIntelCaller, id Identity, in testUnaryArgs) (any, error) {
		fake.called = true
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Errorf("expected deadline in context")
		} else {
			remaining := time.Until(deadline)
			// spec.Timeout for codeIntel.structure is 20s
			if remaining < 18*time.Second || remaining > 21*time.Second {
				t.Errorf("unexpected deadline remaining: %v", remaining)
			}
		}
		return json.RawMessage(`{"status":"ok"}`), nil
	}

	registerCodeIntelUnary(r, deps, "codeIntel.structure", testCall)

	validSel := `{"projectId":"p1","worktreeId":"wt1"}`

	t.Run("nil client returns CODEINTEL_UNAVAILABLE immediately", func(t *testing.T) {
		rNil := NewRegistry()
		nilDeps := codeIntelDeps{}
		registerCodeIntelUnary(rNil, nilDeps, "codeIntel.structure", testCall)

		// Even with broken args
		_, err := rNil.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "codeIntel.structure", []json.RawMessage{json.RawMessage(`bad-json`)})
		if err == nil || !strings.Contains(err.Error(), "CODEINTEL_UNAVAILABLE") {
			t.Fatalf("expected CODEINTEL_UNAVAILABLE on nil client, got %v", err)
		}
	})

	t.Run("empty identity returns not found / not permitted", func(t *testing.T) {
		_, err := r.Dispatch(context.Background(), Identity{}, "codeIntel.structure", []json.RawMessage{json.RawMessage(validSel)})
		if err == nil || !strings.Contains(err.Error(), "CODEINTEL_NOT_AUTHORIZED") {
			t.Fatalf("expected not authorized on empty identity, got %v", err)
		}
	})

	t.Run("device ID forbidden unless AllowDevice", func(t *testing.T) {
		// Device session on structure (AllowDevice is false)
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", DeviceID: "dev-1"}, "codeIntel.structure", []json.RawMessage{json.RawMessage(validSel)})
		if err == nil || !strings.Contains(err.Error(), "CODEINTEL_NOT_AUTHORIZED") {
			t.Fatalf("expected not authorized on device session, got %v", err)
		}

		// Channel with AllowDevice = true: settings.get
		registerCodeIntelUnary(r, deps, "codeIntel.settings.get", func(ctx context.Context, c codeIntelCaller, id Identity, in testSettingsArgs) (any, error) {
			return json.RawMessage(`{}`), nil
		})
		_, err = r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", DeviceID: "dev-1"}, "codeIntel.settings.get", []json.RawMessage{})
		if err != nil {
			t.Fatalf("expected device session to be allowed on settings.get, got %v", err)
		}
	})

	t.Run("forbidden keys reject before calling service", func(t *testing.T) {
		fake.called = false
		_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "codeIntel.structure", []json.RawMessage{
			json.RawMessage(`{"projectId":"p1","worktreeId":"wt1","tenantId":"attack"}`),
		})
		if err == nil || !strings.Contains(err.Error(), "not_allowed") {
			t.Fatalf("expected not_allowed error, got %v", err)
		}
		if fake.called {
			t.Fatalf("downstream client was invoked despite forbidden keys in args")
		}
	})

	t.Run("error mapping from downstream", func(t *testing.T) {
		// Unimplemented => CODEINTEL_UNAVAILABLE
		rErr := NewRegistry()
		registerCodeIntelUnary(rErr, deps, "codeIntel.reindexStatus", func(ctx context.Context, c codeIntelCaller, id Identity, in testUnaryArgs) (any, error) {
			return nil, status.Error(codes.Unimplemented, "not ready")
		})
		_, err := rErr.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "codeIntel.reindexStatus", []json.RawMessage{json.RawMessage(validSel)})
		if err == nil || !strings.Contains(err.Error(), "CODEINTEL_UNAVAILABLE") {
			t.Fatalf("expected CODEINTEL_UNAVAILABLE, got %v", err)
		}

		// Explicit CODEINTEL_DISABLED
		rDisabled := NewRegistry()
		registerCodeIntelUnary(rDisabled, deps, "codeIntel.reindexStatus", func(ctx context.Context, c codeIntelCaller, id Identity, in testUnaryArgs) (any, error) {
			return nil, status.Error(codes.FailedPrecondition, "CODEINTEL_DISABLED: code intel is off")
		})
		_, err = rDisabled.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "codeIntel.reindexStatus", []json.RawMessage{json.RawMessage(validSel)})
		if err == nil || !strings.Contains(err.Error(), "CODEINTEL_DISABLED") {
			t.Fatalf("expected CODEINTEL_DISABLED, got %v", err)
		}
	})

	t.Run("finishCodeIntelResponse size bounds", func(t *testing.T) {
		spec := mustCatalogSpec("codeIntel.symbol") // limit = 320 KiB
		limits := CodeIntelLimits{MaxResponseBytes: 2 << 20}

		// Message within limit
		smallMsg := &codeintelv1.SymbolDetail{
			Symbol: &codeintelv1.SymbolRef{Name: "Foo"},
		}
		out, err := finishCodeIntelResponse(spec, limits, smallMsg, nil)
		if err != nil {
			t.Fatalf("expected small message to pass, got %v", err)
		}
		if out == nil {
			t.Fatalf("expected non-nil output")
		}

		// Message exceeding limit: 320 KiB = 327680 bytes
		largeText := strings.Repeat("A", 330000)
		largeMsg := &codeintelv1.SymbolDetail{
			Source: &codeintelv1.SymbolSource{
				Text: largeText,
			},
		}
		_, err = finishCodeIntelResponse(spec, limits, largeMsg, nil)
		if err == nil || !strings.Contains(err.Error(), "CODEINTEL_RESPONSE_TOO_LARGE") {
			t.Fatalf("expected CODEINTEL_RESPONSE_TOO_LARGE, got %v", err)
		}
	})
}
