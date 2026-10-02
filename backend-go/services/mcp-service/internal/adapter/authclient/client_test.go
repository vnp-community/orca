package authclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/common/grpcmw"
	"github.com/stablyai/orca-go/common/tenant"
	"github.com/stablyai/orca-go/services/mcp-service/internal/usecase"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
)

type fakeAuth struct {
	authv1.AuthServiceClient // other methods panic if unexpectedly called
	gotMD                    metadata.MD
	hasDeadline              bool
	err                      error
}

func (f *fakeAuth) capture(ctx context.Context) {
	f.gotMD, _ = metadata.FromOutgoingContext(ctx)
	_, f.hasDeadline = ctx.Deadline()
}

func (f *fakeAuth) OAuthIssueAuthCode(ctx context.Context, _ *authv1.OAuthIssueAuthCodeRequest, _ ...grpc.CallOption) (*authv1.OAuthIssueAuthCodeResponse, error) {
	f.capture(ctx)
	if f.err != nil {
		return nil, f.err
	}
	return &authv1.OAuthIssueAuthCodeResponse{Code: "c"}, nil
}

func TestCallForwardsIdentityAndBoundsWithDeadline(t *testing.T) {
	f := &fakeAuth{}
	c := New(f, time.Second)
	ctx := tenant.WithTenantID(tenant.WithUserID(context.Background(), "u1"), "t1")
	ctx = tenant.WithClientIP(ctx, "203.0.113.9")
	if code, err := c.IssueAuthCode(ctx, usecase.IssueCodeInput{}); err != nil || code != "c" {
		t.Fatalf("%q %v", code, err)
	}
	if f.gotMD.Get(grpcmw.MetadataTenantID)[0] != "t1" || f.gotMD.Get(grpcmw.MetadataUserID)[0] != "u1" || f.gotMD.Get(grpcmw.MetadataClientIP)[0] != "203.0.113.9" {
		t.Fatalf("metadata = %v", f.gotMD)
	}
	if !f.hasDeadline {
		t.Fatal("every outbound call needs a deadline")
	}
}

func TestTranslateKeepsAuthServiceCodes(t *testing.T) {
	cases := []struct {
		in       error
		wantCode string
		wantKind apperrors.Kind
	}{
		{status.Error(codes.InvalidArgument, "OAUTH_INVALID_REDIRECT_URI: redirect_uri does not match"), "OAUTH_INVALID_REDIRECT_URI", apperrors.KindInvalidArgument},
		{status.Error(codes.PermissionDenied, "OAUTH_UNAUTHORIZED_CLIENT: nope"), "OAUTH_UNAUTHORIZED_CLIENT", apperrors.KindPermissionDenied},
		{status.Error(codes.NotFound, "OAUTH_CLIENT_NOT_FOUND: x"), "OAUTH_CLIENT_NOT_FOUND", apperrors.KindNotFound},
		{status.Error(codes.Internal, "internal error"), "MCP_INTERNAL", apperrors.KindInternal},
	}
	for _, tc := range cases {
		var ae *apperrors.AppError
		if !errors.As(translate(tc.in), &ae) || ae.Code != tc.wantCode || ae.Kind != tc.wantKind {
			t.Errorf("%v -> %+v", tc.in, ae)
		}
	}
	if translate(nil) != nil {
		t.Fatal("nil must stay nil")
	}
}

func TestTranslateTransportFailuresKeepGRPCCodes(t *testing.T) {
	if status.Code(translate(status.Error(codes.Unavailable, "dial tcp"))) != codes.Unavailable {
		t.Error("Unavailable must stay Unavailable (gateway maps it to MCP_UNAVAILABLE)")
	}
	if status.Code(translate(context.DeadlineExceeded)) == codes.OK {
		t.Error("deadline lost")
	}
	if status.Code(translate(status.Error(codes.DeadlineExceeded, "x"))) != codes.DeadlineExceeded {
		t.Error("DeadlineExceeded must stay DeadlineExceeded")
	}
	if got := status.Convert(translate(status.Error(codes.Unavailable, "dial tcp 10.0.0.1:9090: refused"))).Message(); got != "MCP_UNAVAILABLE: authorization server is unavailable" {
		t.Errorf("internal address leaked: %q", got)
	}
}
