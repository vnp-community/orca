package wscompat

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
)

// fakeMobileStatusProjectClient is a minimal test double for
// projectv1.ProjectServiceClient (mirrors fakeProjectServiceClient's
// embed-the-nil-interface-and-override convention, channels_worktree_test.go)
// — overrides only GetMobileWorktreeStatus, this file's channel handlers'
// one call.
type fakeMobileStatusProjectClient struct {
	projectv1.ProjectServiceClient

	getStatusFunc func(ctx context.Context, in *projectv1.GetMobileWorktreeStatusRequest) (*projectv1.GetMobileWorktreeStatusResponse, error)
	calls         atomic.Int32
}

func (f *fakeMobileStatusProjectClient) GetMobileWorktreeStatus(ctx context.Context, in *projectv1.GetMobileWorktreeStatusRequest, _ ...grpc.CallOption) (*projectv1.GetMobileWorktreeStatusResponse, error) {
	f.calls.Add(1)
	return f.getStatusFunc(ctx, in)
}

func mobileStatusTestRegistry(client projectv1.ProjectServiceClient, devices DeviceSecretResolver) *Registry {
	r := NewRegistry()
	registerMobileStatusChannels(r, client, devices)
	return r
}

// TestMobileStatus_NoDeviceID_RejectedBeforeAnyRPC asserts a non-mobile
// Identity is rejected before GetMobileWorktreeStatus or
// ResolveSharedSecret are ever called.
func TestMobileStatus_NoDeviceID_RejectedBeforeAnyRPC(t *testing.T) {
	devices := &fakeDeviceSecretResolver{secret: make([]byte, 32)}
	client := &fakeMobileStatusProjectClient{
		getStatusFunc: func(context.Context, *projectv1.GetMobileWorktreeStatusRequest) (*projectv1.GetMobileWorktreeStatusResponse, error) {
			t.Fatal("GetMobileWorktreeStatus must not be called for a non-mobile Identity")
			return nil, nil
		},
	}
	r := mobileStatusTestRegistry(client, devices)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "mobile.status", nil)
	if err != errNotAMobileSession {
		t.Fatalf("err = %v, want errNotAMobileSession", err)
	}
	if devices.calls != 0 {
		t.Fatalf("ResolveSharedSecret called %d times, want 0", devices.calls)
	}
	if client.calls.Load() != 0 {
		t.Fatalf("GetMobileWorktreeStatus called %d times, want 0", client.calls.Load())
	}
}

// TestMobileStatus_ResponseIsAlwaysSealedEnvelope asserts mobile.status's
// raw return value is the sealed {ciphertext, nonce} envelope, never the
// raw MobileWorktreeStatus JSON shape (BR-MB-13).
func TestMobileStatus_ResponseIsAlwaysSealedEnvelope(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	devices := &fakeDeviceSecretResolver{secret: secret}
	client := &fakeMobileStatusProjectClient{
		getStatusFunc: func(context.Context, *projectv1.GetMobileWorktreeStatusRequest) (*projectv1.GetMobileWorktreeStatusResponse, error) {
			return &projectv1.GetMobileWorktreeStatusResponse{
				Worktrees: []*projectv1.MobileWorktreeStatus{{Id: "wt-1", Name: "main", Status: "idle"}},
			}, nil
		},
	}
	r := mobileStatusTestRegistry(client, devices)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", DeviceID: "device-1"}, "mobile.status", nil)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	envelope, ok := result.(mobileEnvelope)
	if !ok {
		t.Fatalf("result type = %T, want mobileEnvelope (sealed, never raw JSON)", result)
	}
	if envelope.Ciphertext == "" || envelope.Nonce == "" {
		t.Fatalf("envelope has empty ciphertext/nonce: %+v", envelope)
	}
	// The raw response must never appear un-sealed anywhere reachable from
	// this call's return value — confirm decrypting the envelope recovers
	// the exact worktree the fake returned.
	plaintext, err := unsealMobilePayload(envelope.Ciphertext, envelope.Nonce, secret)
	if err != nil {
		t.Fatalf("unsealMobilePayload: %v", err)
	}
	// GetMobileWorktreeStatusResponse isn't itself JSON-tagged the way a
	// protojson.Unmarshal expects, but sealMobileEnvelope used
	// encoding/json.Marshal (not protojson) — decode with the matching
	// plain-struct shape instead of the proto message.
	var raw struct {
		Worktrees []struct {
			Id     string `json:"id"`
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"worktrees"`
	}
	if err := json.Unmarshal([]byte(plaintext), &raw); err != nil {
		t.Fatalf("decoding sealed plaintext: %v", err)
	}
	if len(raw.Worktrees) != 1 || raw.Worktrees[0].Id != "wt-1" {
		t.Fatalf("decrypted worktrees = %+v, want one entry with id wt-1", raw.Worktrees)
	}
}

// readMobileStatusEvent reads the next PushEvent off events, failing the
// test if none arrives within a short timeout.
func readMobileStatusEvent(t *testing.T, events <-chan PushEvent) PushEvent {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("events channel closed unexpectedly")
		}
		return ev
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for a PushEvent")
	}
	return PushEvent{}
}

// TestMobileStatusSubscribe_IdenticalPollsProduceExactlyOnePushEvent is the
// regression guard channels_mobile_status.go's doc comment calls out: two
// consecutive identical polls must produce exactly one PushEvent (the
// first), not two.
func TestMobileStatusSubscribe_IdenticalPollsProduceExactlyOnePushEvent(t *testing.T) {
	orig := mobileStatusPollInterval
	mobileStatusPollInterval = 5 * time.Millisecond
	t.Cleanup(func() { mobileStatusPollInterval = orig })

	secret := make([]byte, 32)
	var call atomic.Int32
	client := &fakeMobileStatusProjectClient{
		getStatusFunc: func(context.Context, *projectv1.GetMobileWorktreeStatusRequest) (*projectv1.GetMobileWorktreeStatusResponse, error) {
			n := call.Add(1)
			// First 3 ticks return the SAME worktree list — must NOT push a
			// second time. The 4th tick changes Status — must produce exactly
			// one more PushEvent.
			status := "idle"
			if n >= 4 {
				status = "running"
			}
			return &projectv1.GetMobileWorktreeStatusResponse{
				Worktrees: []*projectv1.MobileWorktreeStatus{{Id: "wt-1", Status: status}},
			}, nil
		},
	}
	devices := &fakeDeviceSecretResolver{secret: secret}
	r := mobileStatusTestRegistry(client, devices)

	sh, ok := r.StreamHandlerFor("mobile.statusSubscribe")
	if !ok {
		t.Fatal("mobile.statusSubscribe not registered as a StreamHandler")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := sh(ctx, Identity{TenantID: "t1", UserID: "u1", DeviceID: "device-1"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	first := readMobileStatusEvent(t, events)
	if first.Channel != "mobile.statusEvent" {
		t.Fatalf("first event channel = %q, want mobile.statusEvent", first.Channel)
	}

	second := readMobileStatusEvent(t, events)
	envelope, ok := second.Args[0].(mobileEnvelope)
	if !ok {
		t.Fatalf("second event arg type = %T, want mobileEnvelope", second.Args[0])
	}
	plaintext, err := unsealMobilePayload(envelope.Ciphertext, envelope.Nonce, secret)
	if err != nil {
		t.Fatalf("unsealMobilePayload: %v", err)
	}
	if want := `"running"`; !strings.Contains(plaintext, want) {
		t.Fatalf("second event's decrypted payload = %q, want it to contain %q (the changed status, meaning exactly the 2 distinct polls each produced exactly 1 event)", plaintext, want)
	}
}
