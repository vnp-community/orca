package auditclient

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

// fakeAuthServiceClient stubs AppendAuditEntry only — every other
// AuthServiceClient method is left nil-embedded and unused, matching this
// codebase's established partial-fake convention (see
// services/api-gateway/internal/adapter/authclient/jwks_client_test.go).
type fakeAuthServiceClient struct {
	authv1.AuthServiceClient
	err     error
	calls   int
	lastReq *authv1.AppendAuditEntryRequest
}

func (f *fakeAuthServiceClient) AppendAuditEntry(_ context.Context, in *authv1.AppendAuditEntryRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.calls++
	f.lastReq = in
	if f.err != nil {
		return nil, f.err
	}
	return &emptypb.Empty{}, nil
}

func TestAppend_NeverReturnsErrorOnRPCFailure(t *testing.T) {
	fake := &fakeAuthServiceClient{err: errors.New("boom: auth-service unreachable")}
	c := New(fake)

	// Append has no return value at all — this test's real assertion is
	// that this call completes without panicking. A compile-time signature
	// change to add an error return would be the actual regression this
	// guards against.
	c.Append(context.Background(), "t1", "u1", "project.update", "project:p1", "allowed", "203.0.113.7")

	if fake.calls != 1 {
		t.Fatalf("expected AppendAuditEntry to be called once, got %d", fake.calls)
	}
}

func TestAppend_ForwardsAllFields(t *testing.T) {
	fake := &fakeAuthServiceClient{}
	c := New(fake)

	c.Append(context.Background(), "t1", "u1", "project.update", "project:p1", "allowed", "203.0.113.7")

	if fake.lastReq == nil {
		t.Fatal("expected AppendAuditEntry to be called")
	}
	want := &authv1.AppendAuditEntryRequest{
		TenantId: "t1", ActorId: "u1", Action: "project.update", Target: "project:p1", Outcome: "allowed", IpAddress: "203.0.113.7",
	}
	got := fake.lastReq
	if got.TenantId != want.TenantId || got.ActorId != want.ActorId || got.Action != want.Action ||
		got.Target != want.Target || got.Outcome != want.Outcome || got.IpAddress != want.IpAddress || got.ActorType != "" {
		t.Fatalf("expected request %+v, got %+v", want, got)
	}
}

func TestAppendDetailed_DropsInvalidActorType(t *testing.T) {
	fake := &fakeAuthServiceClient{}
	c := New(fake)

	c.AppendDetailed(context.Background(), Entry{
		TenantID:  "t1",
		ActorID:   "u1",
		ActorType: "invalid_type",
		Action:    "test.action",
	})

	if fake.lastReq == nil {
		t.Fatal("expected AppendAuditEntry to be called")
	}
	if fake.lastReq.ActorType != "" {
		t.Errorf("expected ActorType to be dropped (empty string), got %q", fake.lastReq.ActorType)
	}

	// Valid types should be kept
	for _, valid := range []string{"user", "agent", "system"} {
		c.AppendDetailed(context.Background(), Entry{ActorType: valid})
		if fake.lastReq.ActorType != valid {
			t.Errorf("expected ActorType %q to be kept, got %q", valid, fake.lastReq.ActorType)
		}
	}
}

func TestAppendDetailed_TruncatesLargeMetadata(t *testing.T) {
	fake := &fakeAuthServiceClient{}
	c := New(fake)

	largeMeta := string(make([]byte, 4097))
	c.AppendDetailed(context.Background(), Entry{
		MetadataJSON: largeMeta,
	})

	if fake.lastReq == nil {
		t.Fatal("expected AppendAuditEntry to be called")
	}
	wantMeta := `{"truncated":true}`
	if fake.lastReq.MetadataJson != wantMeta {
		t.Errorf("expected MetadataJson to be truncated to %q, got %q", wantMeta, fake.lastReq.MetadataJson)
	}
}

func TestAppendDetailed_ForwardsAllFields(t *testing.T) {
	fake := &fakeAuthServiceClient{}
	c := New(fake)

	c.AppendDetailed(context.Background(), Entry{
		TenantID:     "t1",
		ActorID:      "a1",
		ActorType:    "agent",
		Action:       "request.sync",
		Target:       "request:r1",
		TargetType:   "request",
		TargetID:     "r1",
		Outcome:      "allowed",
		IPAddress:    "127.0.0.1",
		MetadataJSON: `{"foo":"bar"}`,
	})

	if fake.lastReq == nil {
		t.Fatal("expected AppendAuditEntry to be called")
	}

	want := &authv1.AppendAuditEntryRequest{
		TenantId:     "t1",
		ActorId:      "a1",
		ActorType:    "agent",
		Action:       "request.sync",
		Target:       "request:r1",
		TargetType:   "request",
		TargetId:     "r1",
		Outcome:      "allowed",
		IpAddress:    "127.0.0.1",
		MetadataJson: `{"foo":"bar"}`,
	}
	got := fake.lastReq
	if got.TenantId != want.TenantId || got.ActorId != want.ActorId || got.ActorType != want.ActorType ||
		got.Action != want.Action || got.Target != want.Target || got.TargetType != want.TargetType ||
		got.TargetId != want.TargetId || got.Outcome != want.Outcome || got.IpAddress != want.IpAddress ||
		got.MetadataJson != want.MetadataJson {
		t.Fatalf("expected request %+v, got %+v", want, got)
	}
}

func TestAppend_SucceedsOnHappyPath(t *testing.T) {
	fake := &fakeAuthServiceClient{}
	c := New(fake)
	c.Append(context.Background(), "t1", "u1", "project.update", "project:p1", "allowed", "")
	if fake.calls != 1 {
		t.Fatalf("expected AppendAuditEntry to be called once, got %d", fake.calls)
	}
}

func TestAppendDetailedStrict_ReturnsRPCError(t *testing.T) {
	boom := errors.New("auth down")
	fake := &fakeAuthServiceClient{err: boom}
	c := New(fake)

	if err := c.AppendDetailedStrict(context.Background(), Entry{TenantID: "t1", Action: "request.erase"}); !errors.Is(err, boom) {
		t.Fatalf("strict variant must surface the RPC error, got %v", err)
	}
	// The best-effort variant keeps swallowing the same failure.
	c.AppendDetailed(context.Background(), Entry{TenantID: "t1", Action: "request.erase"})
	if fake.calls != 2 {
		t.Fatalf("calls = %d, want 2", fake.calls)
	}

	ok := &fakeAuthServiceClient{}
	if err := New(ok).AppendDetailedStrict(context.Background(), Entry{TenantID: "t1", MetadataJSON: "{\"audit_id\":\"a\"}"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok.lastReq.MetadataJson != "{\"audit_id\":\"a\"}" {
		t.Fatalf("metadata not forwarded: %q", ok.lastReq.MetadataJson)
	}
}
