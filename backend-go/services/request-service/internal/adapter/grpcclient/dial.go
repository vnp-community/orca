package grpcclient

import (
	"fmt"

	"github.com/stablyai/orca-go/common/grpcmw"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Dial opens a lazy client connection (a downstream being down must not fail startup).
// Insecure credentials match the other services' dev setup; production mTLS is a platform-wide gap.
func Dial(addr string, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	conn, err := grpcmw.NewClient(addr, append([]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, extra...)...)
	if err != nil {
		return nil, fmt.Errorf("grpcclient: dial %q: %w", addr, err)
	}
	return conn, nil
}
