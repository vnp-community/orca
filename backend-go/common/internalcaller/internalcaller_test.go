package internalcaller

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func call(t *testing.T, g grpc.UnaryServerInterceptor, method string, md metadata.MD) error {
	t.Helper()
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := g(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { return "ok", nil })
	return err
}

func TestGuard(t *testing.T) {
	g := Guard("s3cret", "/svc/Internal")
	if err := call(t, g, "/svc/Internal", metadata.Pairs(MetadataKey, "s3cret")); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for name, md := range map[string]metadata.MD{
		"no metadata": nil, "wrong token": metadata.Pairs(MetadataKey, "nope"),
		"prefix of token": metadata.Pairs(MetadataKey, "s3cre"), "empty token": metadata.Pairs(MetadataKey, ""),
	} {
		if status.Code(call(t, g, "/svc/Internal", md)) != codes.PermissionDenied {
			t.Errorf("%s: not denied", name)
		}
	}
	if err := call(t, g, "/svc/Public", nil); err != nil {
		t.Fatalf("unguarded method blocked: %v", err)
	}
}

func TestGuard_EmptyExpectedTokenFailsClosed(t *testing.T) {
	g := Guard("", "/svc/Internal")
	if status.Code(call(t, g, "/svc/Internal", metadata.Pairs(MetadataKey, ""))) != codes.PermissionDenied {
		t.Fatal("empty configured token must deny")
	}
}

type fakeStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (f fakeStream) Context() context.Context { return f.ctx }

func streamCall(g grpc.StreamServerInterceptor, method string, md metadata.MD) error {
	ss := fakeStream{ctx: metadata.NewIncomingContext(context.Background(), md)}
	return g(nil, ss, &grpc.StreamServerInfo{FullMethod: method}, func(any, grpc.ServerStream) error { return nil })
}

func TestStreamGuard(t *testing.T) {
	g := StreamGuard("s3cret", "/svc/Stream")
	if err := streamCall(g, "/svc/Stream", metadata.Pairs(MetadataKey, "s3cret")); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for name, md := range map[string]metadata.MD{"no metadata": nil, "wrong token": metadata.Pairs(MetadataKey, "nope")} {
		if status.Code(streamCall(g, "/svc/Stream", md)) != codes.PermissionDenied {
			t.Errorf("%s: not denied", name)
		}
	}
	if err := streamCall(g, "/svc/Other", nil); err != nil {
		t.Fatalf("unguarded stream blocked: %v", err)
	}
	if status.Code(streamCall(StreamGuard("", "/svc/Stream"), "/svc/Stream", metadata.Pairs(MetadataKey, ""))) != codes.PermissionDenied {
		t.Fatal("empty configured token must deny")
	}
}
