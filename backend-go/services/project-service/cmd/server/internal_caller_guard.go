package main

import (
	"github.com/stablyai/orca-go/common/internalcaller"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"google.golang.org/grpc"
)

// internalCallerGuard restricts the internal-only RPCs to callers holding PROJECT_INTERNAL_CALLER_TOKEN. An empty
// token refuses every call (internalcaller.Guard fails closed), so a missing variable cannot open them.
func internalCallerGuard(token string) grpc.UnaryServerInterceptor {
	return internalcaller.Guard(token, projectv1.ProjectService_ResolveProjectAccess_FullMethodName)
}
