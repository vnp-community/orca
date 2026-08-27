package wscompat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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
	if !strings.Contains(err.Error(), wantSubstr) {
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
