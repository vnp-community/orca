package wscompat

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"golang.org/x/crypto/nacl/secretbox"
	"google.golang.org/grpc"

	infrafleetv1 "github.com/stablyai/orca-go/proto/gen/go/orca/infrafleet/v1"
)

// sealMobileDispatchFixture mimics the mobile client's TweetNaCl secretbox
// seal directly over the raw prompt bytes (NOT via sealMobileEnvelope,
// which JSON-marshals its input for the response/encrypt-side channels —
// mobile.dispatch's wire contract is a raw ciphertext of the prompt string
// itself, see mobileDispatchArgs's doc comment).
func sealMobileDispatchFixture(t *testing.T, plaintext string, secret []byte) (ciphertextB64, nonceB64 string) {
	t.Helper()
	var key [32]byte
	copy(key[:], secret)
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatalf("generating nonce: %v", err)
	}
	sealed := secretbox.Seal(nil, []byte(plaintext), &nonce, &key)
	return base64.StdEncoding.EncodeToString(sealed), base64.StdEncoding.EncodeToString(nonce[:])
}

// fakeDispatchInfraFleetClient is a minimal test double for
// infrafleetv1.InfraFleetServiceClient (mirrors fakeInfraFleetClient's
// embed-the-nil-interface-and-override convention, channels_test.go) —
// overrides only DispatchPrompt, this file's channel handler's one call.
type fakeDispatchInfraFleetClient struct {
	infrafleetv1.InfraFleetServiceClient

	dispatchResp *infrafleetv1.DispatchPromptResponse
	dispatchErr  error
	calls        int
	lastReq      *infrafleetv1.DispatchPromptRequest
}

func (f *fakeDispatchInfraFleetClient) DispatchPrompt(ctx context.Context, in *infrafleetv1.DispatchPromptRequest, _ ...grpc.CallOption) (*infrafleetv1.DispatchPromptResponse, error) {
	f.calls++
	f.lastReq = in
	if f.dispatchErr != nil {
		return nil, f.dispatchErr
	}
	return f.dispatchResp, nil
}

// fakeDeviceSecretResolver is a minimal DeviceSecretResolver test double.
type fakeDeviceSecretResolver struct {
	secret []byte
	err    error
	calls  int
}

func (f *fakeDeviceSecretResolver) ResolveSharedSecret(ctx context.Context, deviceID string) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.secret, nil
}

func mobileDispatchTestRegistry(client infrafleetv1.InfraFleetServiceClient, devices DeviceSecretResolver) *Registry {
	r := NewRegistry()
	registerMobileDispatchChannel(r, client, devices)
	return r
}

func mustMarshalArg(t *testing.T, v any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal arg: %v", err)
	}
	return b
}

// TestMobileDispatch_NoDeviceID_RejectedBeforeDecrypt asserts a plain
// browser session (no DeviceID) is rejected before ResolveSharedSecret is
// ever called.
func TestMobileDispatch_NoDeviceID_RejectedBeforeDecrypt(t *testing.T) {
	devices := &fakeDeviceSecretResolver{secret: make([]byte, 32)}
	client := &fakeDispatchInfraFleetClient{}
	r := mobileDispatchTestRegistry(client, devices)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1"}, "mobile.dispatch",
		[]json.RawMessage{mustMarshalArg(t, mobileDispatchArgs{PtyID: "pty-1"})})

	if !errors.Is(err, errNotAMobileSession) {
		t.Fatalf("err = %v, want errNotAMobileSession", err)
	}
	if devices.calls != 0 {
		t.Fatalf("ResolveSharedSecret called %d times, want 0", devices.calls)
	}
	if client.calls != 0 {
		t.Fatalf("DispatchPrompt called %d times, want 0", client.calls)
	}
}

// TestMobileDispatch_MalformedCiphertext_NeverCallsDispatchPrompt asserts a
// decode/decrypt failure never reaches DispatchPrompt.
func TestMobileDispatch_MalformedCiphertext_NeverCallsDispatchPrompt(t *testing.T) {
	devices := &fakeDeviceSecretResolver{secret: make([]byte, 32)}
	client := &fakeDispatchInfraFleetClient{}
	r := mobileDispatchTestRegistry(client, devices)

	_, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", DeviceID: "device-1"}, "mobile.dispatch",
		[]json.RawMessage{mustMarshalArg(t, mobileDispatchArgs{
			PtyID: "pty-1", EncryptedBody: "not-valid-base64!!!", Nonce: base64.StdEncoding.EncodeToString(make([]byte, 24)),
		})})

	if err == nil {
		t.Fatal("expected a decrypt error, got nil")
	}
	if client.calls != 0 {
		t.Fatalf("DispatchPrompt called %d times, want 0", client.calls)
	}
}

// TestMobileDispatch_ValidPayload_RoundTripsDecryptedPrompt asserts a
// correctly-sealed payload decrypts to the exact plaintext prompt and is
// forwarded to DispatchPrompt with DispatchedByDeviceId set.
func TestMobileDispatch_ValidPayload_RoundTripsDecryptedPrompt(t *testing.T) {
	secret := make([]byte, 32)
	for i := range secret {
		secret[i] = byte(i + 1)
	}
	const plaintext = "write a haiku about worktrees"
	ciphertextB64, nonceB64 := sealMobileDispatchFixture(t, plaintext, secret)

	devices := &fakeDeviceSecretResolver{secret: secret}
	client := &fakeDispatchInfraFleetClient{
		dispatchResp: &infrafleetv1.DispatchPromptResponse{Outcome: infrafleetv1.DispatchPromptResponse_INJECTED_IMMEDIATELY},
	}
	r := mobileDispatchTestRegistry(client, devices)

	result, err := r.Dispatch(context.Background(), Identity{TenantID: "t1", UserID: "u1", DeviceID: "device-1"}, "mobile.dispatch",
		[]json.RawMessage{mustMarshalArg(t, mobileDispatchArgs{
			PtyID: "pty-1", EncryptedBody: ciphertextB64, Nonce: nonceB64, Overwrite: true,
		})})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if devices.calls != 1 {
		t.Fatalf("ResolveSharedSecret called %d times, want 1", devices.calls)
	}
	if client.calls != 1 {
		t.Fatalf("DispatchPrompt called %d times, want 1", client.calls)
	}
	if client.lastReq.GetPtyId() != "pty-1" {
		t.Fatalf("PtyId = %q, want %q", client.lastReq.GetPtyId(), "pty-1")
	}
	if client.lastReq.GetPrompt() != plaintext {
		t.Fatalf("Prompt = %q, want %q", client.lastReq.GetPrompt(), plaintext)
	}
	if !client.lastReq.GetOverwrite() {
		t.Fatal("Overwrite = false, want true")
	}
	if client.lastReq.GetDispatchedByDeviceId() != "device-1" {
		t.Fatalf("DispatchedByDeviceId = %q, want %q", client.lastReq.GetDispatchedByDeviceId(), "device-1")
	}
	view, ok := result.(dispatchOutcomeView)
	if !ok {
		t.Fatalf("result type = %T, want dispatchOutcomeView", result)
	}
	if view.Outcome != infrafleetv1.DispatchPromptResponse_INJECTED_IMMEDIATELY.String() {
		t.Fatalf("Outcome = %q, want %q", view.Outcome, infrafleetv1.DispatchPromptResponse_INJECTED_IMMEDIATELY.String())
	}
}
