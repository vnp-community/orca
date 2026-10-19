package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"testing"

	authv1 "github.com/stablyai/orca-go/proto/gen/go/orca/auth/v1"
	authgrpc "github.com/stablyai/orca-go/services/auth-service/internal/adapter/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func directoryClient(t *testing.T, token string) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1 << 16)
	guard := directoryCallerGuard(token, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(guard))
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

func directoryCall(ctx context.Context, cc *grpc.ClientConn) codes.Code {
	return authCallStatus(ctx, cc, authgrpc.DirectoryInternalMethods[0])
}

// Fail closed: an unset secret must refuse the directory read, with or without a token header.
func TestDirectoryCallerGuard_EmptySecretRefusesEveryCall(t *testing.T) {
	cc := directoryClient(t, "")
	for name, ctx := range map[string]context.Context{"no token": context.Background(), "empty token": tokenCtx("")} {
		if got := directoryCall(ctx, cc); got != codes.PermissionDenied {
			t.Errorf("%s: %v, want PermissionDenied", name, got)
		}
	}
}

func TestDirectoryCallerGuard_OnlyItsOwnSecretOpensTheRPC(t *testing.T) {
	cc := directoryClient(t, "dir-secret")
	// Unimplemented comes from the stub behind the guard: the call was admitted.
	if got := directoryCall(tokenCtx("dir-secret"), cc); got != codes.Unimplemented {
		t.Errorf("own secret: %v, want admitted", got)
	}
	for name, ctx := range map[string]context.Context{"other secret": tokenCtx("oauth-secret"), "no token": context.Background()} {
		if got := directoryCall(ctx, cc); got != codes.PermissionDenied {
			t.Errorf("%s: %v, want PermissionDenied", name, got)
		}
	}
}
