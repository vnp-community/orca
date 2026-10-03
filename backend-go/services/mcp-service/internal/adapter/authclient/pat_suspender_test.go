package authclient

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/stablyai/orca-go/common/grpcmw"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

type fakeSuspendAuth struct {
	authv1.AuthServiceClient
	md  metadata.MD
	req *authv1.SetMcpPatSuspensionRequest
	err error
}

func (f *fakeSuspendAuth) SetMcpPatSuspension(ctx context.Context, in *authv1.SetMcpPatSuspensionRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	f.md, _ = metadata.FromOutgoingContext(ctx)
	f.req = in
	return &emptypb.Empty{}, f.err
}

func TestPatSuspender_SendsTenantInMetadataNotBody(t *testing.T) {
	f := &fakeSuspendAuth{}
	p := NewPatSuspender(New(f, time.Second))
	if err := p.SetPatSuspension(context.Background(), "tenant-1", true, "incident"); err != nil {
		t.Fatal(err)
	}
	if got := f.md.Get(grpcmw.MetadataTenantID); len(got) != 1 || got[0] != "tenant-1" {
		t.Fatalf("tenant metadata = %v", got)
	}
	if !f.req.GetSuspended() || f.req.GetReason() != "incident" {
		t.Fatalf("request = %+v", f.req)
	}
	f.err = status.Error(codes.Unavailable, "down")
	if err := p.SetPatSuspension(context.Background(), "tenant-1", false, ""); err == nil {
		t.Fatal("transport failure must surface so the cleanup retries")
	}
}
