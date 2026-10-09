package main

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/common/internalcaller"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func guardCode(token, sent, method string) codes.Code {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(internalcaller.MetadataKey, sent))
	_, err := internalCallerGuard(token)(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) { return nil, nil })
	return status.Code(err)
}

func TestInternalCallerGuard(t *testing.T) {
	resolve := projectv1.ProjectService_ResolveProjectAccess_FullMethodName
	if got := guardCode("", "", resolve); got != codes.PermissionDenied {
		t.Errorf("unset token, empty header: %v", got)
	}
	if got := guardCode("", "anything", resolve); got != codes.PermissionDenied {
		t.Errorf("unset token, some header: %v", got)
	}
	if got := guardCode("s3cret", "s3cret", resolve); got != codes.OK {
		t.Errorf("right token: %v", got)
	}
	if got := guardCode("s3cret", "other", resolve); got != codes.PermissionDenied {
		t.Errorf("wrong token: %v", got)
	}
	// Only the internal RPC is guarded; public project RPCs keep working without the secret.
	if got := guardCode("", "", projectv1.ProjectService_GetProject_FullMethodName); got != codes.OK {
		t.Errorf("a public RPC must not need the secret: %v", got)
	}
}
