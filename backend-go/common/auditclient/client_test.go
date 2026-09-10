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
		got.Target != want.Target || got.Outcome != want.Outcome || got.IpAddress != want.IpAddress {
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
