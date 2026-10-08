package grpc

import (
	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// Framework servers: every RPC answers codes.Unimplemented until its owning CR fills the handler
// (see specs/backend-go/crs/v6/request-service-foundation/RPC-CATALOG.md).

type AiBudgetAdminServer struct {
	requestv1.UnimplementedAiBudgetAdminServiceServer
}

func NewAiBudgetAdminServer() *AiBudgetAdminServer {
	return &AiBudgetAdminServer{}
}
