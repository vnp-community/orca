package grpcclient

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial opens a lazy client connection (a downstream being down must not fail startup).
// Insecure credentials match the other services' dev setup; production mTLS is a platform-wide gap.
func Dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial %q: %w", addr, err)
	}
	return conn, nil
}
