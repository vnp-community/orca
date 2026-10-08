package grpc

import (
	"context"

	"github.com/stablyai/orca-go/common/grpcmw"
	"google.golang.org/grpc/metadata"
)

func incomingWithActor(v string) context.Context {
	md := metadata.MD{}
	if v != "" {
		md.Set(grpcmw.MetadataActorType, v)
	}
	return metadata.NewIncomingContext(context.Background(), md)
}
