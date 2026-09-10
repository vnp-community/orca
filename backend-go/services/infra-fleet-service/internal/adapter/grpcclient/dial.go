package grpcclient

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial opens a gRPC client connection to addr — same lazy-dial pattern as
// git-gateway-service/task-service's own grpcclient.Dial (grpc.NewClient
// doesn't block on connect, so ai-provider-service being down doesn't fail
// this service's startup).
//
// Insecure transport credentials — acceptable for local dev only; see
// api-gateway's Dial doc comment for the production mTLS gap this mirrors.
func Dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial %q: %w", addr, err)
	}
	return conn, nil
}
