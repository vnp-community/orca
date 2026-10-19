package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	"github.com/stablyai/orca-go/common/internalcaller"
	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	authgrpc "github.com/stablyai/orca-go/services/auth-service/internal/adapter/grpc"
	svcconfig "github.com/stablyai/orca-go/services/auth-service/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"
)

func guardedAuthClient(t *testing.T, cfg svcconfig.OAuthConfig) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 16)
	guards := internalCallerGuards(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(guards...))
	authv1.RegisterAuthServiceServer(srv, authv1.UnimplementedAuthServiceServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	cc, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cc.Close() })
	return cc
}

func authCallStatus(ctx context.Context, cc *grpc.ClientConn, method string) codes.Code {
	return status.Code(cc.Invoke(ctx, method, &emptypb.Empty{}, &emptypb.Empty{}))
}

func tokenCtx(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), internalcaller.MetadataKey, token)
}

// A secret that is not configured must refuse its RPCs, not open them (ResolveMcpPrincipal used to stay open).
func TestInternalCallerGuards_UnsetSecretsRefuseEveryInternalRPC(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		cc := guardedAuthClient(t, svcconfig.OAuthConfig{Enabled: enabled})
		methods := append(append(append([]string{}, authgrpc.OAuthInternalMethods...), authgrpc.McpSuspensionMethods...), authgrpc.McpPrincipalMethods...)
		for _, m := range methods {
			for name, ctx := range map[string]context.Context{"no token": context.Background(), "empty token": tokenCtx("")} {
				if got := authCallStatus(ctx, cc, m); got != codes.PermissionDenied {
					t.Errorf("enabled=%v %s (%s): %v, want PermissionDenied", enabled, m, name, got)
				}
			}
		}
	}
}

func TestInternalCallerGuards_EachSecretOpensOnlyItsOwnRPCs(t *testing.T) {
	cc := guardedAuthClient(t, svcconfig.OAuthConfig{InternalCallerToken: "oauth-secret", McpPrincipalCallerToken: "principal-secret"})
	oauth, principal := authgrpc.OAuthInternalMethods[0], authgrpc.McpPrincipalMethods[0]

	// Unimplemented comes from the stub behind the guard: the call was admitted.
	if got := authCallStatus(tokenCtx("oauth-secret"), cc, oauth); got != codes.Unimplemented {
		t.Errorf("oauth secret on an OAuth RPC: %v", got)
	}
	if got := authCallStatus(tokenCtx("principal-secret"), cc, principal); got != codes.Unimplemented {
		t.Errorf("principal secret on ResolveMcpPrincipal: %v", got)
	}
	if got := authCallStatus(tokenCtx("oauth-secret"), cc, principal); got != codes.PermissionDenied {
		t.Errorf("oauth secret on ResolveMcpPrincipal: %v, want PermissionDenied", got)
	}
	if got := authCallStatus(tokenCtx("principal-secret"), cc, oauth); got != codes.PermissionDenied {
		t.Errorf("principal secret on an OAuth RPC: %v, want PermissionDenied", got)
	}
}
