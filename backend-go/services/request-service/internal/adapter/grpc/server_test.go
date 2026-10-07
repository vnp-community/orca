package grpc

import (
	"context"
	"testing"

	"github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestServer_UnimplementedRPCs(t *testing.T) {
	srv := NewServer()
	_, err := srv.GetRequest(context.Background(), &requestv1.GetRequestRequest{})
	if status.Code(err) != codes.Unimplemented {
		t.Errorf("expected Unimplemented, got %v", status.Code(err))
	}
}
