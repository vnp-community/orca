package main

import (
	"log/slog"

	"google.golang.org/grpc"

	"github.com/stablyai/orca-go/common/internalcaller"
	projectv1 "github.com/stablyai/orca-go/proto/gen/go/orca/project/v1"
	"github.com/stablyai/orca-go/services/request-service/internal/adapter/grpcclient"
	"github.com/stablyai/orca-go/services/request-service/internal/config"
)

// projectAccess is the connection to project-service that answers "does this project exist, and what role does this user
// have". Lookup is nil when PROJECT_INTERNAL_CALLER_TOKEN is unset: the internal RPC would refuse the call, so callers keep
// the older membership-list behaviour and the degraded mode is announced once at startup.
type projectAccess struct {
	Client projectv1.ProjectServiceClient
	Lookup *grpcclient.ProjectAccessLookup
	Close  func()
}

func dialProjectAccess(cfg config.Config, log *slog.Logger) (*projectAccess, error) {
	if cfg.ProjectServiceAddr == "" {
		return nil, nil
	}
	var opts []grpc.DialOption
	if cfg.ProjectInternalCallerToken != "" {
		opts = append(opts, grpc.WithChainUnaryInterceptor(internalcaller.ClientInterceptor(cfg.ProjectInternalCallerToken)))
	} else {
		log.Warn("PROJECT_INTERNAL_CALLER_TOKEN unset: project existence is not verified and roles come from the member list")
	}
	conn, err := grpcclient.Dial(cfg.ProjectServiceAddr, opts...)
	if err != nil {
		return nil, err
	}
	pa := &projectAccess{Client: projectv1.NewProjectServiceClient(conn), Close: func() { _ = conn.Close() }}
	if cfg.ProjectInternalCallerToken != "" {
		pa.Lookup = grpcclient.NewProjectAccessLookup(pa.Client)
	}
	return pa, nil
}
